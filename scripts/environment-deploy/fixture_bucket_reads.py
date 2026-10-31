"""Offline-only response oracle. This module grants no execution-gate reads."""

from urllib.parse import parse_qs, urlsplit

from botocore.session import Session

# Fixed read-only configuration operations, checked against the installed model.
# Only requests actually observed by the fixture enter the review proposal.
RESPONSES = {
    "cors": ("GetBucketCors", 404, "NoSuchCORSConfiguration"),
    "website": ("GetBucketWebsite", 404, "NoSuchWebsiteConfiguration"),
    "versioning": ("GetBucketVersioning", 200, "VersioningConfiguration"),
    "accelerate": ("GetBucketAccelerateConfiguration", 200, "AccelerateConfiguration"),
    "requestPayment": ("GetBucketRequestPayment", 200, "RequestPaymentConfiguration"),
    "logging": ("GetBucketLogging", 200, "BucketLoggingStatus"),
    "lifecycle": (
        "GetBucketLifecycleConfiguration",
        404,
        "NoSuchLifecycleConfiguration",
    ),
    "replication": (
        "GetBucketReplication",
        404,
        "ReplicationConfigurationNotFoundError",
    ),
    "encryption": (
        "GetBucketEncryption",
        404,
        "ServerSideEncryptionConfigurationNotFoundError",
    ),
    "object-lock": (
        "GetObjectLockConfiguration",
        404,
        "ObjectLockConfigurationNotFoundError",
    ),
}


def validate_model():
    model = Session().get_service_model("s3")
    for selector, (name, _status, _root) in RESPONSES.items():
        operation = model.operation_model(name)
        assert operation.http["method"] == "GET"
        assert operation.http["requestUri"] == "/{Bucket}?" + selector


def fixture_response(request, scope):
    if (
        request.method != "GET"
        or request.host != scope["webBucket"] + ".s3.us-east-2.amazonaws.com"
        or request.raw_content
    ):
        return None
    parsed = urlsplit(request.path)
    if parsed.path != "/" or parsed.fragment or parsed.scheme or parsed.netloc:
        return None
    for selector, (operation, status, root) in RESPONSES.items():
        if parsed.query not in {selector, selector + "="}:
            continue
        if parse_qs(parsed.query, keep_blank_values=True) != {selector: [""]}:
            return None
        if status == 404:
            body = "<Error><Code>" + root + "</Code></Error>"
        else:
            value = "<Payer>BucketOwner</Payer>" if selector == "requestPayment" else ""
            body = (
                "<"
                + root
                + ' xmlns="http://s3.amazonaws.com/doc/2006-03-01/">'
                + value
                + "</"
                + root
                + ">"
            )
        return operation, selector, status, body.encode()
    return None
