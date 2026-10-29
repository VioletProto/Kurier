"""Private model-matched API names only. Never retain paths/query/header/body data.

This module is diagnostic, not an authorization classifier. It describes held
requests only; a model match never changes the resource-scoped policy decision.
"""

import functools
import hashlib
import json
import re
from pathlib import Path
from urllib.parse import parse_qsl, urlsplit

from private_diagnostics import OPERATION_FILENAME, persist

MODEL_SHA = "f53fa36be1d92506430709b755ec59d8c78416bb35e8238f66f52befcaefc09a"
CATALOG_SHA = "44aecbaac03513c4bb75e47f54fdcab33267501b6dc9dd4c349042abb62f0627"
MAX_TARGET = 8192
MAX_CANDIDATES = 4
METHODS = {"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}


@functools.lru_cache(maxsize=1)
def rules():
    path = Path(__file__).with_name("s3control-operation-routes.json")
    raw = path.read_bytes()
    if hashlib.sha256(raw).hexdigest() != CATALOG_SHA:
        raise OSError("operation-model-pin-held")
    model = json.loads(raw)
    if model["sourceVersion"] != "1.43.108" or model["sourceModelSha256"] != MODEL_SHA:
        raise OSError("operation-model-pin-held")
    result = []
    for operation in model["operations"]:
        name = operation["name"]
        if not re.fullmatch(r"[A-Za-z][A-Za-z0-9]{0,95}", name):
            raise OSError("operation-model-name-held")
        http = operation
        target = urlsplit(http["requestUri"])
        pieces = re.split(r"(\{[^{}]+\})", target.path)
        pattern = "".join(
            (r".+" if p.endswith("+}") else r"[^/]+")
            if p.startswith("{") and p.endswith("}")
            else re.escape(p)
            for p in pieces
        )
        result.append(
            (
                name,
                http["method"],
                re.compile(pattern),
                tuple(parse_qsl(target.query, keep_blank_values=True)),
            )
        )
    return tuple(result)


def identify(method, target):
    safe_method = method if isinstance(method, str) and method in METHODS else "OTHER"
    base = {"method": safe_method, "candidateOperations": []}
    if not isinstance(target, str) or not target or safe_method == "OTHER":
        return {**base, "matchStatus": "invalid"}
    if len(target) > MAX_TARGET:
        return {**base, "matchStatus": "overlong"}
    if (
        not target.isascii()
        or not target.startswith("/")
        or any(ord(c) < 32 or ord(c) == 127 for c in target)
    ):
        return {**base, "matchStatus": "invalid"}
    try:
        url = urlsplit(target)
    except ValueError:
        return {**base, "matchStatus": "invalid"}
    if url.scheme or url.netloc or url.fragment:
        return {**base, "matchStatus": "invalid"}
    try:
        query = parse_qsl(url.query, keep_blank_values=True, max_num_fields=64)
    except ValueError:
        return {**base, "matchStatus": "invalid"}
    candidates = sorted(
        {
            name
            for name, verb, pattern, selectors in rules()
            if verb == method
            and pattern.fullmatch(url.path)
            and all(selector in query for selector in selectors)
        }
    )
    if len(candidates) > MAX_CANDIDATES:
        return {**base, "matchStatus": "ambiguous-overflow"}
    return {
        **base,
        "candidateOperations": candidates,
        "matchStatus": "unique"
        if len(candidates) == 1
        else "ambiguous"
        if candidates
        else "unknown",
    }


def capture_operation(directory, method, target, count):
    if type(count) is not int or not 0 <= count <= 500:
        raise OSError("private-operation-count-held")
    value = {
        "version": 1,
        "phase": "request",
        "count": count,
        "modelSha256": MODEL_SHA,
        **identify(method, target),
    }
    persist(
        directory,
        OPERATION_FILENAME,
        (json.dumps(value, separators=(",", ":")) + "\n").encode("ascii"),
    )
