"""Actual S3 payload admission: immutable bytes, including verified multipart parts."""

import base64
import binascii
import hashlib
import re
import xml.etree.ElementTree as ET
from urllib.parse import parse_qsl

from common import Held, digest


def xml(data):
    if len(data) > 131072 or b"<!" in data:
        raise Held("xml-held")
    root = ET.fromstring(data)
    for item in root.iter():
        item.tag = item.tag.split("}")[-1]
    return root


def payload(body, headers):
    if headers.get("content-encoding", "") != "aws-chunked":
        return body
    if headers.get("x-amz-content-sha256") not in {
        "STREAMING-UNSIGNED-PAYLOAD-TRAILER",
        "STREAMING-AWS4-HMAC-SHA256-PAYLOAD",
        "STREAMING-AWS4-HMAC-SHA256-PAYLOAD-TRAILER",
    }:
        raise Held("streaming-mode-held")
    remaining = body
    chunks = []
    while True:
        line, sep, remaining = remaining.partition(b"\r\n")
        if not sep or not re.fullmatch(
            rb"[0-9a-fA-F]{1,8}(?:;chunk-signature=[0-9a-f]{64})?", line
        ):
            raise Held("chunk-frame-held")
        size = int(line.split(b";")[0], 16)
        if not size:
            break
        if len(remaining) < size + 2 or remaining[size : size + 2] != b"\r\n":
            raise Held("chunk-size-held")
        chunks.append(remaining[:size])
        remaining = remaining[size + 2 :]
    decoded = b"".join(chunks)
    if headers.get("x-amz-decoded-content-length") != str(len(decoded)):
        raise Held("decoded-length-held")
    if remaining == b"\r\n":
        return decoded
    fields = {}
    for line in remaining.split(b"\r\n"):
        if not line:
            continue
        k, sep, v = line.partition(b":")
        if not sep or k in fields:
            raise Held("trailer-held")
        fields[k] = v
    name = headers.get("x-amz-trailer", "").encode()
    expected = {
        b"x-amz-checksum-crc32": base64.b64encode(
            (binascii.crc32(decoded) & 0xFFFFFFFF).to_bytes(4, "big")
        ),
        b"x-amz-checksum-sha256": base64.b64encode(hashlib.sha256(decoded).digest()),
        b"x-amz-checksum-sha1": base64.b64encode(
            hashlib.sha1(decoded, usedforsecurity=False).digest()
        ),
    }
    if set(fields) != {name} or name not in expected or fields[name] != expected[name]:
        raise Held("checksum-trailer-held")
    return decoded


class Uploads:
    def __init__(self, objects):
        # Values contain the actual reviewed bytes loaded once by the host;
        # children cannot modify these objects or change the allowlist.
        self.objects = objects
        self.multipart = {}
        self.pending = {}
        self.finished = set()

    def request(self, method, bucket, key, query, body, headers):
        target = (bucket, key)
        if target not in self.objects:
            raise Held("unreviewed-upload-resource")
        expected = self.objects[target]
        pairs = parse_qsl(query, keep_blank_values=True, strict_parsing=False)
        if len(dict(pairs)) != len(pairs):
            raise Held("duplicate-upload-query")
        q = dict(pairs)
        if method == "PUT" and not q:
            data = payload(body, headers)
            if len(data) != len(expected) or digest(data) != digest(expected):
                raise Held("upload-bytes-held")
            return "PutObject", {"target": target, "finish": True}
        if method == "POST" and q == {"uploads": ""} and not body:
            if target in self.pending:
                raise Held("repeat-multipart-create")
            self.pending[target] = True
            return "CreateMultipartUpload", {"target": target, "create": True}
        if set(q) == {"partNumber", "uploadId"} and method == "PUT":
            if not re.fullmatch(r"[1-9][0-9]{0,3}", q["partNumber"]):
                raise Held("part-number-held")
            upload = self.multipart.get(q["uploadId"])
            n = int(q["partNumber"])
            if not upload or upload["target"] != target or n in upload["parts"]:
                raise Held("part-resource-order-or-retry-held")
            data = payload(body, headers)
            offset = expected.find(data) if data else -1
            if (
                offset < 0
                or expected.find(data, offset + 1) >= 0
                or (n == 1 and offset != 0)
                or (n > 1 and offset == 0)
                or (offset + len(data) < len(expected) and len(data) < 5 * 1024 * 1024)
            ):
                raise Held("part-bytes-held")
            if any(
                not (
                    offset + len(data) <= p["offset"]
                    or p["offset"] + p["bytes"] <= offset
                )
                for p in upload["parts"].values()
            ):
                raise Held("part-overlap-held")
            upload["parts"][n] = {
                "offset": offset,
                "bytes": len(data),
                "etag": None,
                "sha256": digest(data),
            }
            return "UploadPart", {"upload": q["uploadId"], "part": n}
        if method == "POST" and set(q) == {"uploadId"}:
            upload = self.multipart.get(q["uploadId"])
            if (
                not upload
                or upload["target"] != target
                or sum(p["bytes"] for p in upload["parts"].values()) != len(expected)
            ):
                raise Held("multipart-coverage-held")
            root = xml(body)
            if root.tag != "CompleteMultipartUpload" or len(root) != len(
                upload["parts"]
            ):
                raise Held("complete-shape-held")
            for n, (part, known) in enumerate(
                zip(
                    root,
                    [
                        upload["parts"].get(n)
                        for n in range(1, len(upload["parts"]) + 1)
                    ],
                    strict=True,
                ),
                1,
            ):
                if known is None or known["offset"] != sum(
                    upload["parts"][j]["bytes"] for j in range(1, n)
                ):
                    raise Held("multipart-order-coverage-held")
                if (
                    part.tag != "Part"
                    or [v.tag for v in part] != ["ETag", "PartNumber"]
                    and [v.tag for v in part] != ["PartNumber", "ETag"]
                ):
                    raise Held("complete-part-held")
                if part.findtext("ETag") != known["etag"] or part.findtext(
                    "PartNumber"
                ) != str(n):
                    raise Held("complete-manifest-held")
            return "CompleteMultipartUpload", {
                "target": target,
                "finish": True,
                "upload": q["uploadId"],
            }
        raise Held("upload-operation-held")

    def response(self, meta, status, headers, body):
        if not 200 <= status < 300:
            raise Held("upload-failure")
        if meta.get("create"):
            root = xml(body)
            identifier = root.findtext("UploadId")
            if not identifier or len(identifier) > 2048 or identifier in self.multipart:
                raise Held("upload-id-held")
            self.multipart[identifier] = {"target": meta["target"], "parts": {}}
        if "part" in meta:
            etag = headers.get("etag")
            if not etag or len(etag) > 256:
                raise Held("part-etag-held")
            self.multipart[meta["upload"]]["parts"][meta["part"]]["etag"] = etag
        if meta.get("finish"):
            if "upload" in meta and body and xml(body).tag == "Error":
                raise Held("multipart-completion-error")
            self.finished.add(meta["target"])
            if "upload" in meta:
                del self.multipart[meta["upload"]]
