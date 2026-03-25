import base64
import json
import time
from pathlib import Path

from mitmproxy import ctx, http, tls


ROOT = Path(__file__).resolve().parent
CAPTURE_DIR = ROOT / "captures"
CAPTURE_DIR.mkdir(parents=True, exist_ok=True)
CAPTURE_FILE = CAPTURE_DIR / f"capture-{int(time.time())}.jsonl"

SENSITIVE_HEADERS = {
    "authorization",
    "cookie",
    "proxy-authorization",
    "x-api-key",
}

tls_by_client = {}


def _mask_headers(headers):
    out = []
    for key, value in headers.items(multi=True):
        if key.lower() in SENSITIVE_HEADERS:
            value = "***"
        out.append([key, value])
    return out


def _body_to_text(content):
    if not content:
        return ""
    try:
        return content.decode("utf-8")
    except UnicodeDecodeError:
        return "base64:" + base64.b64encode(content).decode("ascii")


def _now():
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def _json_default(value):
    if isinstance(value, bytes):
        try:
            return value.decode("utf-8")
        except UnicodeDecodeError:
            return "base64:" + base64.b64encode(value).decode("ascii")
    return str(value)


def _write_record(record):
    with CAPTURE_FILE.open("a", encoding="utf-8") as fp:
        fp.write(json.dumps(record, ensure_ascii=False, default=_json_default) + "\n")


def tls_clienthello(data: tls.ClientHelloData):
    tls_by_client[data.context.client.id] = {
        "sni": data.client_hello.sni,
        "alpn": list(data.client_hello.alpn_protocols or []),
        "cipher_suites": list(data.client_hello.cipher_suites or []),
        "extensions": list(data.client_hello.extensions or []),
    }
    _write_record(
        {
            "timestamp": _now(),
            "event": "tls_clienthello",
            "client_id": data.context.client.id,
            "tls": tls_by_client[data.context.client.id],
        }
    )


def requestheaders(flow: http.HTTPFlow):
    _write_record(
        {
            "timestamp": _now(),
            "event": "request_headers",
            "flow_id": flow.id,
            "request": {
                "method": flow.request.method,
                "url": flow.request.pretty_url,
                "headers": _mask_headers(flow.request.headers),
            },
            "tls": tls_by_client.get(flow.client_conn.id, {}),
        }
    )


def request(flow: http.HTTPFlow):
    _write_record(
        {
            "timestamp": _now(),
            "event": "request",
            "flow_id": flow.id,
            "request": {
                "method": flow.request.method,
                "url": flow.request.pretty_url,
                "headers": _mask_headers(flow.request.headers),
                "body": _body_to_text(flow.request.raw_content),
                "body_size": len(flow.request.raw_content or b""),
            },
            "tls": tls_by_client.get(flow.client_conn.id, {}),
        }
    )


def response(flow: http.HTTPFlow):
    _write_record(
        {
            "timestamp": _now(),
            "event": "response",
            "flow_id": flow.id,
            "request": {
                "method": flow.request.method,
                "url": flow.request.pretty_url,
                "headers": _mask_headers(flow.request.headers),
                "body": _body_to_text(flow.request.raw_content),
                "body_size": len(flow.request.raw_content or b""),
            },
            "response": {
                "status_code": flow.response.status_code if flow.response else 0,
                "headers": _mask_headers(flow.response.headers) if flow.response else [],
                "body": _body_to_text(flow.response.raw_content if flow.response else b""),
                "body_size": len(flow.response.raw_content or b"") if flow.response else 0,
            },
            "tls": tls_by_client.get(flow.client_conn.id, {}),
        }
    )


def error(flow: http.HTTPFlow):
    _write_record(
        {
            "timestamp": _now(),
            "event": "error",
            "flow_id": flow.id,
            "request": {
                "method": flow.request.method if flow.request else "",
                "url": flow.request.pretty_url if flow.request else "",
                "headers": _mask_headers(flow.request.headers) if flow.request else [],
                "body": _body_to_text(flow.request.raw_content) if flow.request else "",
                "body_size": len(flow.request.raw_content or b"") if flow.request else 0,
            },
            "error": {
                "message": str(flow.error) if flow.error else "",
            },
            "tls": tls_by_client.get(flow.client_conn.id, {}),
        }
    )


ctx.log.info(f"writing MITM captures to {CAPTURE_FILE}")
