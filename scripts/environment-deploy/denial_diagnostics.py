"""Static first-denial labels only: no raw targets, values, bodies or hashes."""

from urllib.parse import parse_qs, urlsplit

CONDITIONS = {
    "s3-read-scope-held",
    "s3-operation-id-held",
    "s3-query-key-alias-held",
    "s3-key-encoding-held",
    "s3-duplicate-query",
    "s3control-resource-held",
    "endpoint-held",
    "host-authority-held",
    "duplicate-header",
    "unexpected-resource-selector",
    "upload-metadata-held",
    "target-held",
    "fresh-identity-or-closed-marker-required",
    "unreviewed-mutation",
    "phase-or-call-limit",
    "phase-window-exhausted",
    "latched-stop",
    "original-window-exhausted-or-boot-changed",
    "closed-window-exhausted",
}


def denial_projection(request, scope, condition, *, admitted=False):
    operation = "unclassified"
    # Read no headers/body at all. Even rejected resource values stay private in
    # memory; only this fixed operation vocabulary can enter the diagnostic.
    if request.host == scope["webBucket"] + ".s3.us-east-2.amazonaws.com":
        parsed = urlsplit(request.path)
        if request.method == "GET" and parsed.path == "/":
            query = parse_qs(parsed.query, keep_blank_values=True)
            for key, label in [
                ("policy", "GetBucketPolicy"),
                ("location", "GetBucketLocation"),
                ("tagging", "GetBucketTagging"),
            ]:
                if key in query:
                    operation = "s3/" + label
                    break
        elif request.method == "GET":
            operation = "s3/GetObject"
    elif request.host == scope["account"] + ".s3-control.us-east-2.amazonaws.com":
        from s3control_tags import target_allowed

        if target_allowed(request.method, request.host, request.path):
            operation = "s3control/ListTagsForResource"
    return {
        "operation": operation,
        "condition": condition if condition in CONDITIONS else "unclassified-held",
        "admitted": admitted,
        "beforeForward": not admitted,
    }
