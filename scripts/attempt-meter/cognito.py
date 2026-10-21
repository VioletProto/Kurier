"""Narrow browser CORS compatibility. Does not authorize any Cognito POST."""

HOST = "cognito-idp.us-east-2.amazonaws.com"
ORIGIN = "https://d2kpq905qyh9zg.cloudfront.net"
PREFLIGHT_HEADERS = {
    "amz-sdk-invocation-id",
    "amz-sdk-request",
    "cache-control",
    "content-type",
    "x-amz-target",
    "x-amz-user-agent",
}


def auth_preflight(host, path, headers, content):
    names = headers.get("access-control-request-headers", "").lower().split(",")
    names = [name.strip() for name in names]
    return (
        host == HOST
        and path == "/"
        and not content
        and headers.get("origin") == ORIGIN
        and headers.get("access-control-request-method") == "POST"
        and len(names) == len(set(names))
        and {"content-type", "x-amz-target"}.issubset(names)
        and set(names).issubset(PREFLIGHT_HEADERS)
        and not any(
            headers.get(k)
            for k in ("authorization", "cookie", "x-amz-security-token", "x-amz-target")
        )
    )


SAFE_OPERATIONS = {
    "GET",
    "HEAD",
    "POST",
    "PUT",
    "PATCH",
    "DELETE",
    "OPTIONS",
    "CognitoAuthPreflight",
    "CreateOAuth2Token",
    "InitiateAuth",
    "RespondToAuthChallenge",
    "RevokeToken",
    "GetTokensFromRefreshToken",
    "AdminCreateUser",
    "AdminSetUserPassword",
    "AdminDeleteUser",
    "AdminUserGlobalSignOut",
    "AdminGetUser",
    "GetItem",
    "Query",
    "Scan",
    "BatchGetItem",
    "TransactWriteItems",
    "GetCallerIdentity",
    "GetQueueAttributes",
    "GetPolicyVersion",
    "GetPolicy",
    "GetRole",
    "GetRolePolicy",
    "ListRolePolicies",
    "ListAttachedRolePolicies",
}


def held_diagnostics(method, host, category="unknown", operation="unknown"):
    if method == "OPTIONS" and host == HOST:
        category, operation = "aws", "CognitoAuthPreflight"
    return {
        "method": method
        if method in {"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
        else "unknown",
        "category": category if category in {"aws", "api", "static"} else "unknown",
        "operation": operation if operation in SAFE_OPERATIONS else "unknown",
    }
