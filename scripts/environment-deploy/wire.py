"""Closed execution wire policy. Reviewed identities/inputs, never operation-name grants."""

import re
from urllib.parse import parse_qsl, quote, unquote, urlencode, urlsplit

from botocore.session import Session
from common import Held, canonical, digest, strict_json
from uploads import Uploads, payload, xml


class Models:
    def __init__(self):
        s = Session()
        self.models = {
            k: s.get_service_model(v)
            for k, v in {
                "lambda": "lambda",
                "apigateway": "apigatewayv2",
                "cloudfront-keyvaluestore": "cloudfront-keyvaluestore",
                "cloudfront": "cloudfront",
            }.items()
        }

    def rest(self, service, method, path, query, body, headers):
        matches = []
        querypairs = parse_qsl(query, keep_blank_values=True, strict_parsing=True)
        if len(querypairs) != len(dict(querypairs)):
            raise Held("duplicate-query")
        original_query = dict(querypairs)
        for name in self.models[service].operation_names:
            op = self.models[service].operation_model(name)
            if op.http["method"] != method:
                continue
            uri = urlsplit(op.http["requestUri"])
            selectors = dict(parse_qsl(uri.query, keep_blank_values=True))
            if any(original_query.get(k) != v for k, v in selectors.items()):
                continue
            template = uri.path
            labels = []
            pieces = []
            cursor = 0
            for m in re.finditer(r"\{([A-Za-z0-9]+)(\+)?\}", template):
                pieces.append(re.escape(template[cursor : m.start()]))
                pieces.append("(.+)" if m[2] else "([^/]+)")
                labels.append(m[1])
                cursor = m.end()
            pieces.append(re.escape(template[cursor:]))
            matched = re.fullmatch("".join(pieces), path)
            if matched:
                matches.append(
                    (
                        name,
                        op,
                        dict(zip(labels, matched.groups(), strict=True)),
                        selectors,
                    )
                )
        if matches:
            most = max(len(v[3]) for v in matches)
            matches = [v for v in matches if len(v[3]) == most]
        if len(matches) != 1:
            raise Held("model-route-held")
        name, op, labels, selectors = matches[0]
        params = {}
        querypairs = parse_qsl(query, keep_blank_values=True, strict_parsing=True)
        if len(querypairs) != len(dict(querypairs)):
            raise Held("duplicate-query")
        q = {k: v for k, v in querypairs if k not in selectors}
        data = strict_json(body) if body and service != "cloudfront" else {}
        if not isinstance(data, dict):
            raise Held("json-shape-held")
        for key, shape in (op.input_shape.members if op.input_shape else {}).items():
            wire = shape.serialization.get("name", key)
            location = shape.serialization.get("location")
            if location == "uri" and wire in labels:
                raw = labels.pop(wire)
                value = unquote(raw, errors="strict")
                if (
                    raw not in {quote(value, safe="-_.~"), value}
                    or "%" in value
                    or ".." in value.split("/")
                ):
                    raise Held("uri-encoding-held")
                params[key] = value
            elif location == "querystring" and wire in q:
                params[key] = q.pop(wire)
            elif location == "header" and wire.lower() in headers:
                params[key] = headers[wire.lower()]
            elif not location and wire in data:
                params[key] = data.pop(wire)
        if labels or q or data:
            raise Held("unexpected-parameter-held")
        if service == "cloudfront" and body:
            params["XmlBody"] = body
        return name, params


def optional_subset(value, required, optional):
    return required <= set(value) and set(value) <= required | optional


class Policy:
    def __init__(
        self,
        scope,
        release,
        objects,
        *,
        phase,
        stage_transaction=None,
        closed_marker=None,
    ):
        self.s, self.release, self.phase = scope, release, phase
        self.models = Models()
        self.uploads = Uploads(objects)
        self.stage_transaction = stage_transaction
        self.closed_marker = closed_marker
        self.closed_verified = False
        self.identity_verified = False
        self.artifacts_ready = False
        self.kv_etag = None
        self.last_marker_sha256 = None
        self.integrations = set()
        self.created_routes = set()
        self.invalidations = set()
        self.write_tickets = set()
        self.invocations = set()
        self.metadata_versions = set()
        self.lock_empty = False
        self.lock_created = False
        self.update_id = None
        self.known_residuals = set()
        self.roles = {f["role"].rsplit("/", 1)[-1] for f in scope["functions"].values()}
        self.policies = {
            p for f in scope["functions"].values() for p in f["managedPolicyArns"]
        }
        self.inline = {
            (f["role"].rsplit("/", 1)[-1], p)
            for f in scope["functions"].values()
            for p in f["inlinePolicyNames"]
        }
        self.functions = {f["name"] for f in scope["functions"].values()}
        self.api_function = scope["functions"]["UsersProjectsApi"]["name"]

    def host(self, host):
        if host in {"iam.amazonaws.com", "cloudfront.amazonaws.com"}:
            return host.split(".")[0]
        if host == "freetier.us-east-1.api.aws":
            return "freetier"
        if host == self.s["kvsHost"]:
            return "cloudfront-keyvaluestore"
        if host == self.s["account"] + ".s3-control.us-east-2.amazonaws.com":
            return "s3control"
        for b in [
            self.s[k]
            for k in ["stateBucket", "assetBucket", "webBucket", "evidenceBucket"]
        ]:
            if host == b + ".s3.us-east-2.amazonaws.com":
                return "s3"
        for name in [
            "ssm",
            "sts",
            "lambda",
            "apigateway",
            "dynamodb",
            "sqs",
            "events",
            "freetier",
            "cognito-idp",
            "kms",
        ]:
            if host == name + ".us-east-2.amazonaws.com":
                return name
        raise Held("endpoint-held")

    def request(self, method, host, target, headers, body):
        if host != host.lower() or host.endswith("."):
            raise Held("host-alias-held")
        service = self.host(host)
        u = urlsplit(target)
        if u.scheme or u.netloc or u.fragment or "#" in target:
            raise Held("target-held")
        fields = headers.fields if hasattr(headers, "fields") else headers.items()
        h = {}
        for k, v in fields:
            k = k.decode("ascii") if isinstance(k, bytes) else k
            v = v.decode("ascii") if isinstance(v, bytes) else v
            k = k.lower()
            if k in h:
                raise Held("duplicate-header")
            h[k] = v
        if any(
            k in h
            for k in [
                "x-amz-copy-source",
                "x-amz-grant-full-control",
                "x-amz-grant-read",
                "x-amz-server-side-encryption-customer-key",
            ]
        ):
            raise Held("unexpected-resource-selector")
        if "host" in h and h["host"].removesuffix(":443") != host:
            raise Held("host-authority-held")
        if (
            h.get("x-amz-acl", "private") != "private"
            or h.get("x-amz-server-side-encryption", "AES256") != "AES256"
            or h.get("x-amz-storage-class", "STANDARD") != "STANDARD"
            or any(
                k.startswith(("x-amz-meta-", "x-amz-object-lock-"))
                or k in {"x-amz-server-side-encryption-aws-kms-key-id", "x-amz-tagging"}
                for k in h
            )
        ):
            raise Held("upload-metadata-held")
        if service == "s3":
            return self.write_headers(
                self.s3(method, host, u.path, u.query, h, body), h
            )
        if service == "s3control":
            from s3control_tags import allowed

            if not allowed(method, host, target, headers, body):
                raise Held("s3control-resource-held")
            return {"operation": "s3control/ListTagsForResource", "write": False}
        if service in self.models.models:
            op, p = self.models.rest(service, method, u.path, u.query, body, h)
        elif service in {"iam", "sts"}:
            if method != "POST" or u.path != "/" or u.query:
                raise Held("query-endpoint-held")
            pairs = parse_qsl(
                body.decode("ascii"), keep_blank_values=True, strict_parsing=True
            )
            if len(pairs) != len(dict(pairs)):
                raise Held("query-duplicate-held")
            p = dict(pairs)
            op = p.pop("Action", None)
            version = p.pop("Version", None)
            if version != {"iam": "2010-05-08", "sts": "2011-06-15"}[service]:
                raise Held("query-version-held")
        else:
            if method != "POST" or u.path != "/" or u.query:
                raise Held("json-endpoint-held")
            p = strict_json(body)
            op = h.get("x-amz-target", "").rsplit(".", 1)[-1]
            prefixes = {
                "ssm": "AmazonSSM",
                "dynamodb": "DynamoDB_20120810",
                "sqs": "AmazonSQS",
                "events": "AWSEvents",
                "cognito-idp": "AWSCognitoIdentityProviderService",
                "kms": "TrentService",
                "freetier": "AWSFreeTierService",
            }
            if h.get("x-amz-target") != prefixes.get(service, "") + "." + op:
                raise Held("json-target-held")
        if self.read(service, op, p):
            return {
                "operation": service + "/" + op,
                "write": False,
                "service": service,
                "api": op,
                "params": p,
            }
        if (
            not self.identity_verified
            or not self.closed_verified
            and not (service == "dynamodb" and self.phase == "close")
        ):
            raise Held("fresh-identity-or-closed-marker-required")
        if (
            service == "dynamodb"
            and op == "TransactWriteItems"
            and self.phase in {"close", "activate"}
        ):
            if self.stage_transaction is None or canonical(p) != canonical(
                self.stage_transaction
            ):
                raise Held("stage-transaction-held")
            return self.write_headers(
                self.once(
                    "stage/" + self.phase,
                    {"operation": "dynamodb/TransactWriteItems", "write": True},
                ),
                h,
            )
        if self.phase != "deploy":
            raise Held("write-phase-held")
        return self.write_headers(self.rest_write(service, op, p), h)

    def write_headers(self, meta, h):
        if not meta["write"]:
            return meta
        identifier = h.get("amz-sdk-invocation-id", "")
        fields = {}
        for part in h.get("amz-sdk-request", "").split("; "):
            k, sep, v = part.partition("=")
            if not sep or k in fields or k not in {"attempt", "max", "ttl"}:
                raise Held("write-retry-metadata-held")
            fields[k] = v
        if (
            fields.get("attempt") != "1"
            or not re.fullmatch(
                r"[a-fA-F0-9]{8}(?:-[a-fA-F0-9]{4}){3}-[a-fA-F0-9]{12}", identifier
            )
            or identifier in self.invocations
        ):
            raise Held("automatic-write-retry-held")
        self.invocations.add(identifier)
        return meta

    def read(self, s, op, p):
        if s == "sts":
            return op == "GetCallerIdentity" and not p
        if s == "ssm":
            return (
                op == "GetParameter"
                and p.get("Name") in self.s["parameters"]
                and set(p) <= {"Name", "WithDecryption"}
            )
        if s == "dynamodb":
            if p.get("TableName") not in [
                self.s["controlTable"],
                self.s["protectedTable"],
            ]:
                return False
            if op == "DescribeTable":
                return set(p) == {"TableName"}
            if op == "GetItem":
                return (
                    set(p) == {"TableName", "Key", "ConsistentRead"}
                    and p["ConsistentRead"] is True
                    and p["TableName"] == self.s["controlTable"]
                    and p["Key"]
                    in [
                        {"PK": {"S": "STAGE#dev-api"}, "SK": {"S": k}}
                        for k in ["META", "QUOTA"]
                    ]
                )
            if op == "Scan":
                allowed = {
                    "TableName",
                    "ConsistentRead",
                    "Limit",
                    "ProjectionExpression",
                    "ExpressionAttributeNames",
                    "ExclusiveStartKey",
                }
                expected = set(self.s["projectionFields"])
                return (
                    set(p) <= allowed
                    and p.get("ConsistentRead") is True
                    and type(p.get("Limit")) is int
                    and p.get("Limit") == 100
                    and set(p.get("ExpressionAttributeNames", {}).values()) == expected
                    and set(p.get("ProjectionExpression", "").split(","))
                    == set(p.get("ExpressionAttributeNames", {}))
                    and (
                        "ExclusiveStartKey" not in p
                        or set(p["ExclusiveStartKey"]) == {"PK", "SK"}
                    )
                )
            return False
        if s == "lambda" and op == "GetAccountSettings":
            return not p
        if s == "lambda":
            return (
                p.get("FunctionName") in self.functions
                and op
                in {
                    "GetFunctionConfiguration",
                    "GetPolicy",
                    "GetFunctionConcurrency",
                    "ListProvisionedConcurrencyConfigs",
                    "ListEventSourceMappings",
                }
                and set(p) <= {"FunctionName", "Marker", "MaxItems"}
            )
        if s == "iam":
            return (
                (
                    op in {"GetRole", "ListRolePolicies", "ListAttachedRolePolicies"}
                    and p.get("RoleName") in self.roles
                    and set(p) <= {"RoleName", "Marker", "MaxItems"}
                )
                or (
                    op == "GetRolePolicy"
                    and (p.get("RoleName"), p.get("PolicyName")) in self.inline
                    and set(p) == {"RoleName", "PolicyName"}
                )
                or (
                    op in {"GetPolicy", "GetPolicyVersion"}
                    and p.get("PolicyArn") in self.policies
                    and set(p) <= {"PolicyArn", "VersionId"}
                    and (
                        "VersionId" not in p
                        or re.fullmatch(r"v[1-9][0-9]{0,4}", p["VersionId"])
                    )
                )
            )
        if s == "sqs":
            return (
                op in {"GetQueueAttributes", "ListQueueTags"}
                and p.get("QueueUrl") == self.s["outputs"]["executionQueueUrl"]
                and set(p) <= {"QueueUrl", "AttributeNames"}
            )
        if s == "events":
            return (
                op in {"DescribeRule", "ListTargetsByRule"}
                and p.get("Name", p.get("Rule")) == self.s["outputs"]["cleanupSchedule"]
                and set(p) <= {"Name", "Rule", "NextToken", "Limit"}
            )
        if s == "freetier":
            return op == "GetAccountPlanState" and not p
        if s == "lambda" and op == "GetAccountSettings":
            return not p
        if s == "cognito-idp":
            return (
                op in {"DescribeUserPool", "DescribeUserPoolClient"}
                and p.get("UserPoolId") == self.s.get("poolId")
                and set(p) <= {"UserPoolId", "ClientId"}
                and ("ClientId" not in p or p["ClientId"] == self.s.get("clientId"))
            )
        if s == "apigateway" and op in {"GetIntegration", "GetRoute"}:
            return p.get("ApiId") == self.s["apiId"] and (
                p.get("IntegrationId") in self.integrations
                if op == "GetIntegration"
                else p.get("RouteId") in self.created_routes
            )
        if s == "apigateway":
            return (
                p.get("ApiId") == self.s["apiId"]
                and op in {"GetApi", "GetStage", "GetRoutes", "GetIntegrations"}
                and set(p) <= {"ApiId", "StageName", "NextToken", "MaxResults"}
                and ("StageName" not in p or p["StageName"] == "$default")
            )
        if s == "cloudfront":
            return (
                (p.get("DistributionId") if op == "GetInvalidation" else p.get("Id"))
                == self.s["distribution"]
                and op
                in {"GetDistribution", "GetDistributionConfig", "GetInvalidation"}
                and set(p) <= {"Id", "DistributionId", "IfMatch"}
                and (
                    "Id" not in p
                    or op != "GetInvalidation"
                    or p["Id"] in self.invalidations
                )
            )
        if s == "cloudfront-keyvaluestore":
            return (
                p.get("KvsARN") == self.s["kvsArn"]
                and op in {"DescribeKeyValueStore", "GetKey", "ListKeys"}
                and set(p) <= {"KvsARN", "Key", "NextToken", "MaxResults"}
                and (
                    "Key" not in p
                    or p["Key"]
                    in {self.s["namespace"] + ":" + k for k in self.s["kvEntries"]}
                )
            )
        return False

    def once(self, ticket, meta):
        if ticket in self.write_tickets:
            raise Held("write-retry-held")
        self.write_tickets.add(ticket)
        return meta

    def rest_write(self, s, op, p):
        fn = p.get("FunctionName")
        if s == "lambda" and op == "UpdateFunctionCode" and fn in self.functions:
            logical = next(k for k, v in self.s["functions"].items() if v["name"] == fn)
            package = next(v for v in self.release["packages"] if v["name"] == logical)
            if (
                p.get("S3Bucket") != self.s["assetBucket"]
                or p.get("S3Key") != f"assets/{logical}-code-{package['zipSha256']}.zip"
                or not optional_subset(
                    p,
                    {"FunctionName", "S3Bucket", "S3Key"},
                    {"S3ObjectVersion", "Publish", "RevisionId", "Architectures"},
                )
                or p.get("Publish", False) is not False
                or p.get("Architectures", ["arm64"]) != ["arm64"]
            ):
                raise Held("lambda-code-held")
            if (self.s["assetBucket"], p["S3Key"]) not in self.uploads.finished:
                raise Held("code-upload-not-confirmed")
            return self.once(
                "code/" + fn, {"operation": "lambda/UpdateFunctionCode", "write": True}
            )
        if s == "lambda" and op == "AddPermission" and fn == self.api_function:
            expected = next(
                (
                    v
                    for v in self.s["permissions"]
                    if v["statementId"] == p.get("StatementId")
                ),
                None,
            )
            if (
                not expected
                or set(p)
                != {"FunctionName", "Action", "Principal", "SourceArn", "StatementId"}
                or any(
                    p[k] != expected[k[0].lower() + k[1:]]
                    for k in ["Action", "Principal", "SourceArn", "StatementId"]
                )
            ):
                raise Held("invoke-permission-held")
            return self.once(
                "permission/" + p["StatementId"],
                {"operation": "lambda/AddPermission", "write": True},
            )
        if s == "apigateway" and op == "CreateIntegration":
            expected = {
                "ApiId": self.s["apiId"],
                "ConnectionType": "INTERNET",
                "IntegrationType": "AWS_PROXY",
                "IntegrationUri": self.s["functions"]["UsersProjectsApi"]["arn"],
                "PassthroughBehavior": "WHEN_NO_MATCH",
                "PayloadFormatVersion": "2.0",
            }
            if (
                p != expected
                or sum(t.startswith("integration/") for t in self.write_tickets) >= 8
            ):
                raise Held("integration-held")
            return self.once(
                "integration/" + str(len(self.write_tickets)),
                {
                    "operation": "apigateway/CreateIntegration",
                    "write": True,
                    "learnIntegration": True,
                },
            )
        if s == "apigateway" and op == "CreateRoute":
            if (
                p.get("ApiId") != self.s["apiId"]
                or p.get("RouteKey") not in self.s["routes"]
                or p.get("AuthorizationType") != "NONE"
                or p.get("Target", "").removeprefix("integrations/")
                not in self.integrations
                or not optional_subset(
                    p,
                    {"ApiId", "RouteKey", "AuthorizationType", "Target"},
                    {"ApiKeyRequired"},
                )
                or p.get("ApiKeyRequired", False) is not False
            ):
                raise Held("route-held")
            return self.once(
                "route/" + p["RouteKey"],
                {
                    "operation": "apigateway/CreateRoute",
                    "write": True,
                    "learnRoute": True,
                },
            )
        if s == "cloudfront-keyvaluestore" and op == "UpdateKeys":
            expected = {
                self.s["namespace"] + ":" + k: v for k, v in self.s["kvEntries"].items()
            }
            puts = p.get("Puts", [])
            if (
                p.get("KvsARN") != self.s["kvsArn"]
                or not self.kv_etag
                or p.get("IfMatch") != self.kv_etag
                or p.get("Deletes")
                or not optional_subset(p, {"KvsARN", "IfMatch", "Puts"}, {"Deletes"})
                or len(puts) > len(expected)
                or not puts
                or len({v.get("Key") for v in puts}) != len(puts)
                or any(
                    set(v) != {"Key", "Value"} or expected.get(v["Key"]) != v["Value"]
                    for v in puts
                )
            ):
                raise Held("kv-write-held")
            return {
                "operation": "cloudfront-keyvaluestore/UpdateKeys",
                "write": True,
                "kvUpdate": True,
            }
        if s == "cloudfront" and op == "CreateInvalidation":
            root = xml(p.get("XmlBody", b""))
            paths = root.find("Paths")
            if (
                p.get("DistributionId") != self.s["distribution"]
                or root.tag != "InvalidationBatch"
                or [c.tag for c in root]
                not in [["Paths", "CallerReference"], ["CallerReference", "Paths"]]
                or paths is None
                or paths.findtext("Quantity") != "1"
                or [v.text for v in paths.findall("Items/Path")] != ["/*"]
            ):
                raise Held("invalidation-held")
            return self.once(
                "invalidation",
                {
                    "operation": "cloudfront/CreateInvalidation",
                    "write": True,
                    "learnInvalidation": True,
                },
            )
        raise Held("unreviewed-mutation")

    def s3(self, method, host, path, query, h, body):
        bucket = host.removesuffix(".s3.us-east-2.amazonaws.com")
        key = unquote(path.lstrip("/"), errors="strict")
        if (
            path != "/" + quote(key, safe="/-_.~")
            or "%" in key
            or ".." in key.split("/")
        ):
            raise Held("s3-key-encoding-held")
        pairs = parse_qsl(query, keep_blank_values=True)
        q = dict(pairs)
        if len(q) != len(pairs):
            raise Held("s3-duplicate-query")
        if any("%" in part.split("=", 1)[0] for part in query.split("&")):
            raise Held("s3-query-key-alias-held")
        operation_id = q.pop("x-id", None)
        if operation_id is not None:
            expected_id = (
                "CreateMultipartUpload"
                if method == "POST" and "uploads" in q
                else "CompleteMultipartUpload"
                if method == "POST" and "uploadId" in q
                else "UploadPart"
                if method == "PUT" and "partNumber" in q
                else "PutObject"
                if method == "PUT"
                else "DeleteObject"
                if method == "DELETE"
                else "ListObjectsV2"
                if "list-type" in q
                else "HeadObject"
                if method == "HEAD" and key
                else "HeadBucket"
                if method == "HEAD"
                else "GetObject"
            )
            if operation_id != expected_id:
                raise Held("s3-operation-id-held")
        query = urlencode(q)

        if method in {"GET", "HEAD"}:
            keys = {k for b, k in self.uploads.objects if b == bucket}
            if bucket == self.s["assetBucket"]:
                keys.update(v["key"] for v in self.s["oldCodeObjects"])
            if bucket == self.s["stateBucket"] and (
                key
                in {
                    "app/kurier/dev-api.json",
                    "secret/kurier/dev-api.json",
                    "secret/kurier/_fallback.json",
                    "lock/kurier/dev-api.json",
                }
                or re.fullmatch(
                    r"(?:snapshot|eventlog|summary|update)/kurier/dev-api/[A-Za-z0-9_-]+\.json",
                    key,
                )
            ):
                keys.add(key)
            if (
                key in keys
                and set(q) <= {"versionId", "x-id"}
                or not key
                and (not q or set(q) <= {"location"})
            ):
                return {
                    "operation": "s3/" + method,
                    "write": False,
                    "lockRead": bucket == self.s["stateBucket"]
                    and key == "lock/kurier/dev-api.json",
                }
            if (
                not key
                and q.get("list-type") == "2"
                and set(q)
                <= {
                    "list-type",
                    "prefix",
                    "max-keys",
                    "continuation-token",
                    "encoding-type",
                }
            ):
                prefix = q.get("prefix", "")
                if (
                    bucket == self.s["webBucket"]
                    or bucket == self.s["stateBucket"]
                    and any(
                        prefix.startswith(v + "/kurier/dev-api")
                        for v in [
                            "app",
                            "lock",
                            "snapshot",
                            "eventlog",
                            "summary",
                            "update",
                        ]
                    )
                ):
                    return {"operation": "s3/ListObjectsV2", "write": False}
                if (
                    bucket == self.s["evidenceBucket"]
                    and prefix
                    in {"dev-api/projects/" + p + "/" for p in self.known_residuals}
                    and q.get("max-keys") == "1"
                ):
                    return {"operation": "s3/ListObjectsV2", "write": False}
            raise Held("s3-read-scope-held")
        if (
            self.phase != "deploy"
            or not self.closed_verified
            or not self.identity_verified
        ):
            raise Held("s3-write-phase-held")
        if bucket == self.s["stateBucket"]:
            if method == "PUT" and not q:
                data = payload(body, h)
                if not data or len(data) > 32 * 1024 * 1024:
                    raise Held("metadata-body-held")
                if key == "lock/kurier/dev-api.json":
                    lock = strict_json(data)
                    if (
                        not self.lock_empty
                        or self.update_id is not None
                        or set(lock)
                        != {"created", "updateID", "runID", "command", "ignore"}
                        or lock["command"] != "deploy"
                        or lock["ignore"] is not True
                        or not re.fullmatch(r"[A-Za-z0-9_-]{1,128}", lock["updateID"])
                    ):
                        raise Held("owned-sst-lock-held")
                    self.update_id = lock["updateID"]
                    return self.once(
                        "sst-lock",
                        {
                            "operation": "s3/SstStageMetadataPut",
                            "write": True,
                            "lockCreate": True,
                        },
                    )
                allowed = {"app/kurier/dev-api.json"} | {
                    f"{k}/kurier/dev-api/{self.update_id}.json"
                    for k in ["snapshot", "eventlog", "summary", "update"]
                }
                if not self.lock_created or key not in allowed:
                    raise Held("metadata-update-id-held")
                return self.once(
                    "metadata/" + key + "/" + digest(data),
                    {"operation": "s3/SstStageMetadataPut", "write": True},
                )
            if (
                method == "DELETE"
                and key == "lock/kurier/dev-api.json"
                and not q
                and self.lock_created
            ):
                return self.once(
                    "unlock", {"operation": "s3/SstStageUnlock", "write": True}
                )
            raise Held("sst-metadata-write-held")
        if (
            method == "DELETE"
            and bucket == self.s["assetBucket"]
            and key in {v["key"] for v in self.s["oldCodeObjects"]}
            and set(q) <= {"versionId", "x-id"}
        ):
            return self.once(
                "old-code-delete/" + key,
                {"operation": "s3/DeleteOldCodeObject", "write": True},
            )
        if (
            bucket == self.s["webBucket"]
            and method in {"PUT", "POST"}
            and (not q or "uploads" in q)
        ):
            metadata = next(
                (v for v in self.s["frontendObjects"] if v["key"] == key), None
            )
            if (
                not metadata
                or h.get("content-type") != metadata["contentType"]
                or h.get("cache-control") != metadata["cacheControl"]
            ):
                raise Held("frontend-upload-headers-held")
        op, meta = self.uploads.request(method, bucket, key, query, body, h)
        if op == "PutObject":
            warm = (
                bucket == self.s["webBucket"]
                and key.startswith("assets/")
                and not self.artifacts_ready
            )
            self.once(("warm-put/" if warm else "deploy-put/") + bucket + "/" + key, {})
        return {
            "operation": "s3/" + op,
            "write": True,
            "upload": meta,
            "prewarm": bucket == self.s["webBucket"] and key.startswith("assets/"),
        }

    def response(self, meta, status, headers, body):
        if meta.get("lockRead"):
            if status == 404:
                self.lock_empty = True
            elif status == 200:
                value = strict_json(body)
                self.lock_empty = (
                    not value.get("created")
                    or value.get("created") == "0001-01-01T00:00:00Z"
                )
        if not 200 <= status < 300:
            return
        if meta.get("lockCreate"):
            self.lock_created = True
        if "upload" in meta:
            self.uploads.response(meta["upload"], status, headers, body)
        if meta.get("learnIntegration"):
            value = strict_json(body).get("integrationId")
            if (
                not isinstance(value, str)
                or not re.fullmatch(r"[a-z0-9]{1,32}", value)
                or value in self.integrations
            ):
                raise Held("integration-response-held")
            self.integrations.add(value)
        if meta.get("learnRoute"):
            value = strict_json(body).get("routeId")
            if (
                not isinstance(value, str)
                or not re.fullmatch(r"[a-z0-9]{1,32}", value)
                or value in self.created_routes
            ):
                raise Held("route-response-held")
            self.created_routes.add(value)
        if meta.get("learnInvalidation"):
            value = xml(body).findtext("Id")
            if not value or len(value) > 128:
                raise Held("invalidation-response-held")
            self.invalidations.add(value)
        if meta.get("service") == "sts":
            root = xml(body)
            if (
                root.findtext("GetCallerIdentityResult/Account") != self.s["account"]
                or root.findtext("GetCallerIdentityResult/Arn") != self.s["identityArn"]
            ):
                raise Held("identity-response-held")
            self.identity_verified = True
        if (
            meta.get("service") == "dynamodb"
            and meta.get("api") == "GetItem"
            and meta["params"]["Key"]["SK"]["S"] == "META"
        ):
            item = strict_json(body).get("Item")
            self.last_marker_sha256 = digest(canonical(item))
            if self.closed_marker is not None and item == self.closed_marker:
                self.closed_verified = True
        if meta.get("service") == "cloudfront-keyvaluestore" or meta.get("kvUpdate"):
            data = strict_json(body)
            etag = headers.get("etag") or data.get("ETag")
            if etag:
                self.kv_etag = etag
        if meta.get("service") == "dynamodb" and meta.get("api") == "Scan":
            for row in strict_json(body).get("Items", []):
                if row.get("kind", {}).get("S") == "executionResidual":
                    self.known_residuals.add(row.get("projectId", {}).get("S"))
