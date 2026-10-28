"""Opt-in, host-only denied CONNECT evidence. Never disclose its contents.

The fixed receipt lives beside the private host ledger, outside child mounts.
It may contain secrets in DNS labels and requires review before any publication.
No URL, path, query, header or body is accepted by this interface.
"""

import ipaddress
import json
import os
import re
import stat

FILENAME = "denied-connect.private.json"
MAX_HOST = 253
MAX_RECORD = 512


def hostname_status(host):
    if not isinstance(host, str) or not host:
        return "invalid"
    if len(host) > MAX_HOST:
        return "overlong"
    if not host.isascii():
        return "invalid"
    # Preserve the parsed host exactly, including case/trailing dot. Do not
    # truncate or reinterpret malformed authority/URL material as a hostname.
    name = host.removesuffix(".")
    if all(
        re.fullmatch(r"[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?", label)
        for label in name.split(".")
    ):
        return "captured"
    if re.fullmatch(r"[0-9a-fA-F:]+", host):
        try:
            ipaddress.IPv6Address(host)
            return "captured"
        except ValueError:
            pass
    return "invalid"


def capture(directory, host, port, count):
    """One exclusive bounded receipt; persistence failures must fail closed."""
    if (
        type(port) is not int
        or not 1 <= port <= 65535
        or type(count) is not int
        or not 0 <= count <= 500
    ):
        raise OSError("private-capture-metadata-held")
    status = hostname_status(host)
    raw = (
        json.dumps(
            {
                "version": 1,
                "phase": "connect",
                "count": count,
                "port": port,
                "hostnameStatus": status,
                "hostname": host if status == "captured" else None,
            },
            separators=(",", ":"),
        )
        + "\n"
    ).encode("ascii")
    if len(raw) > MAX_RECORD:
        raise OSError("private-capture-size-held")
    directory_fd = os.open(directory, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        info = os.fstat(directory_fd)
        if info.st_uid != os.getuid() or stat.S_IMODE(info.st_mode) != 0o700:
            raise OSError("private-capture-directory-held")
        fd = os.open(
            FILENAME,
            os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW,
            0o600,
            dir_fd=directory_fd,
        )
        try:
            os.fchmod(fd, 0o600)
            while raw:
                n = os.write(fd, raw)
                if n <= 0:
                    raise OSError("private-capture-write-held")
                raw = raw[n:]
            os.fsync(fd)
        finally:
            os.close(fd)
        os.fsync(directory_fd)
    finally:
        os.close(directory_fd)
