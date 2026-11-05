"""Bounded typed transport diagnostics. Never stringify exceptions or read URLs."""

import errno
import socket
import ssl

from botocore import exceptions as sdk
from urllib3 import exceptions as pool


class DeadlineExpired(BaseException):
    pass


DNS_CODES = {
    getattr(socket, name)
    for name in (
        "EAI_AGAIN",
        "EAI_BADFLAGS",
        "EAI_FAIL",
        "EAI_FAMILY",
        "EAI_MEMORY",
        "EAI_NONAME",
        "EAI_SERVICE",
        "EAI_SOCKTYPE",
        "EAI_SYSTEM",
        "EAI_NODATA",
    )
    if hasattr(socket, name)
}
OS_CODES = {
    getattr(errno, name)
    for name in (
        "ECONNREFUSED",
        "ECONNRESET",
        "ECONNABORTED",
        "ENETUNREACH",
        "EHOSTUNREACH",
        "ETIMEDOUT",
        "EPIPE",
        "ENETDOWN",
        "ENOTCONN",
        "EADDRNOTAVAIL",
        "EINTR",
    )
}
TLS_CODES = {
    getattr(ssl, name)
    for name in (
        "SSL_ERROR_SSL",
        "SSL_ERROR_WANT_READ",
        "SSL_ERROR_WANT_WRITE",
        "SSL_ERROR_SYSCALL",
        "SSL_ERROR_ZERO_RETURN",
        "SSL_ERROR_EOF",
    )
}
# Selected OpenSSL X509 verification reasons; never retain verify_message.
CERT_CODES = {2, 3, 4, 5, 7, 8, 9, 10, 18, 19, 20, 21, 23, 24, 26, 27, 28, 50, 62, 64}


def safe_code(value, allowed):
    return value if type(value) is int and value in allowed else None


def typed(error):
    """Return static category/phase/rank and an explicitly allowlisted code."""
    code, domain = None, None
    if isinstance(error, socket.gaierror):
        code, domain = safe_code(error.errno, DNS_CODES), "gai"
        category, phase, rank = "dns", "dns", 100
    elif isinstance(error, ssl.SSLCertVerificationError):
        code, domain = (
            safe_code(getattr(error, "verify_code", None), CERT_CODES),
            "x509",
        )
        category, phase, rank = "tls-certificate", "tls", 95
    elif isinstance(error, (ssl.SSLError, sdk.SSLError, pool.SSLError)):
        if isinstance(error, ssl.SSLError):
            code, domain = safe_code(error.errno, TLS_CODES), "ssl"
        category, phase, rank = "tls", "tls", 90
    elif isinstance(error, DeadlineExpired):
        category, phase, rank = "deadline", "unknown", 90
    elif isinstance(error, (sdk.ReadTimeoutError, pool.ReadTimeoutError)):
        category, phase, rank = "read-timeout", "read", 80
    elif isinstance(error, (sdk.EndpointConnectionError, pool.NewConnectionError)):
        category, phase, rank = "connection", "connect", 75
    elif isinstance(error, (sdk.ConnectTimeoutError, pool.ConnectTimeoutError)):
        category, phase, rank = "connect-timeout", "connect", 80
    elif isinstance(error, TimeoutError):
        code, domain = safe_code(error.errno, OS_CODES), "errno"
        category, phase, rank = "timeout", "unknown", 70
    elif isinstance(error, (sdk.ProxyConnectionError, pool.ProxyError)):
        category, phase, rank = "proxy", "unknown", 60
    elif isinstance(error, OSError) and safe_code(error.errno, OS_CODES) is not None:
        code, domain = safe_code(error.errno, OS_CODES), "errno"
        phase = (
            "connect"
            if code
            in {
                errno.ECONNREFUSED,
                errno.ENETUNREACH,
                errno.EHOSTUNREACH,
                errno.ENETDOWN,
                errno.EADDRNOTAVAIL,
            }
            else "unknown"
        )
        category, rank = "connection", 65
    elif isinstance(error, (sdk.ConnectionClosedError, pool.ProtocolError)):
        category, phase, rank = "connection-closed", "unknown", 55
    elif isinstance(error, sdk.HTTPClientError):
        category, phase, rank = "transport", "unknown", 10
    else:
        category, phase, rank = "unknown", "unknown", 0
    return rank, category, phase, code, domain if code is not None else None


def transport_diagnostic(error):
    """Walk at most16 exception objects, without reading string arguments.

    SDK kwargs['error'], urllib3 reason/_reason/original_error, and exception
    links may expose a more specific cause. Ignore messages, pool/request/
    response objects, other kwargs and all non-exception args. Cycles are bounded.
    """
    queue, seen, candidates = [error], set(), []
    while queue and len(seen) < 16:
        current = queue.pop(0)
        if not isinstance(current, BaseException) or id(current) in seen:
            continue
        seen.add(id(current))
        candidates.append(typed(current))
        state = current.__dict__
        children = [current.__cause__, current.__context__]
        if isinstance(current, (sdk.HTTPClientError, sdk.ConnectionError)):
            kwargs = state.get("kwargs")
            if type(kwargs) is dict:
                children.append(kwargs.get("error"))
        if isinstance(
            current, (pool.MaxRetryError, pool.NameResolutionError, pool.ProxyError)
        ):
            children.extend(
                state.get(k) for k in ("reason", "_reason", "original_error")
            )
        children.extend(x for x in current.args[:8] if isinstance(x, BaseException))
        queue.extend(x for x in children if isinstance(x, BaseException))
    best = (
        max(candidates, key=lambda x: x[0])
        if candidates
        else (0, "unknown", "unknown", None, None)
    )
    code, domain = best[3:]
    if code is None:
        code, domain = next(
            ((x[3], x[4]) for x in candidates if x[3] is not None), (None, None)
        )
    return {
        "exceptionCategory": best[1],
        "failurePhase": best[2],
        "errorCode": code,
        "errorCodeDomain": domain,
    }
