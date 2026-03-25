#!/usr/bin/env python3
import json
import sys
from collections import Counter, defaultdict
from pathlib import Path
from urllib.parse import urlparse


ROOT = Path(__file__).resolve().parent
CAPTURE_DIR = ROOT / "captures"
REPORT_DIR = ROOT / "reports"
REPORT_DIR.mkdir(parents=True, exist_ok=True)


def load_records():
    records = []
    for path in sorted(CAPTURE_DIR.glob("*.jsonl")):
        with path.open("r", encoding="utf-8") as fp:
            for line in fp:
                line = line.strip()
                if line:
                    records.append(json.loads(line))
    return records


def build_flows(records):
    flows = {}
    tls_events = []
    for record in records:
        event = record.get("event")
        if event == "tls_clienthello":
            tls_events.append(record)
            continue
        flow_id = record.get("flow_id")
        if not flow_id:
            continue
        flow = flows.setdefault(flow_id, {})
        flow["timestamp"] = record.get("timestamp", flow.get("timestamp"))
        flow["request"] = record.get("request", flow.get("request", {}))
        if record.get("tls"):
            flow["tls"] = record["tls"]
        if event == "response":
            flow["response"] = record.get("response", {})
        if event == "error":
            flow["error"] = record.get("error", {})
    return flows, tls_events


def classify(url):
    parsed = urlparse(url)
    host = parsed.netloc
    path = parsed.path
    if host.startswith("127.0.0.1") or host.startswith("localhost"):
        return "local"
    if "statsig" in host:
        return "telemetry"
    if "sentry" in host:
        return "error_reporting"
    if "install" in path or "manifest" in path:
        return "update"
    if "login" in path or "oauth" in path:
        return "auth"
    if "mcp" in path:
        return "mcp"
    if "messages" in path or "models" in path or "count_tokens" in path:
        return "model_api"
    return "other"


def dump(path, payload):
    with path.open("w", encoding="utf-8") as fp:
        json.dump(payload, fp, indent=2, ensure_ascii=False, default=dict)


def main():
    records = load_records()
    flows, tls_events = build_flows(records)
    hosts = Counter()
    endpoints = Counter()
    categories = Counter()
    headers = defaultdict(Counter)
    sse_events = Counter()
    tls_profiles = Counter()
    errors = Counter()
    status_codes = Counter()

    for flow in flows.values():
        request = flow.get("request", {})
        url = request.get("url", "")
        if not url:
            continue
        parsed = urlparse(url)
        host = parsed.netloc
        endpoint = f"{parsed.netloc}{parsed.path}"
        hosts[host] += 1
        endpoints[endpoint] += 1
        categories[classify(url)] += 1

        for header, _ in request.get("headers", []):
            headers[host][header] += 1

        response = flow.get("response", {})
        if response:
            status_codes[str(response.get("status_code", 0))] += 1

        if flow.get("error", {}).get("message"):
            errors[flow["error"]["message"]] += 1

        body = response.get("body", "")
        if "event:" in body or "data:" in body:
            for line in body.splitlines():
                if line.startswith("event:"):
                    sse_events[line[6:].strip()] += 1

        tls_info = flow.get("tls", {})
        if tls_info:
            key = json.dumps(tls_info, sort_keys=True)
            tls_profiles[key] += 1

    for record in tls_events:
        tls_info = record.get("tls", {})
        if tls_info:
            key = json.dumps(tls_info, sort_keys=True)
            tls_profiles[key] += 1

    dump(REPORT_DIR / "hosts.json", hosts)
    dump(REPORT_DIR / "endpoints.json", endpoints)
    dump(REPORT_DIR / "headers.json", {host: dict(counter) for host, counter in headers.items()})
    dump(REPORT_DIR / "tls_profiles.json", tls_profiles)
    dump(REPORT_DIR / "sse_events.json", sse_events)
    dump(REPORT_DIR / "errors.json", errors)
    dump(REPORT_DIR / "status_codes.json", status_codes)
    dump(
        REPORT_DIR / "nonessential_traffic.json",
        {
            "telemetry": categories.get("telemetry", 0),
            "error_reporting": categories.get("error_reporting", 0),
        },
    )
    dump(
        REPORT_DIR / "mode_diff_oauth_vs_apikey.json",
        {"note": "populate by running multiple capture sessions and diffing reports"},
    )
    dump(
        REPORT_DIR / "geo_env_diff.json",
        {"note": "populate by running the same workflow from multiple proxy regions"},
    )

    print(f"analyzed {len(records)} events and {len(flows)} flows into {REPORT_DIR}")


if __name__ == "__main__":
    sys.exit(main())
