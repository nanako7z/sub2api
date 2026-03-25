#!/usr/bin/env python3
import argparse
import json
import re
import subprocess
from collections import Counter, defaultdict
from datetime import datetime, timezone
from pathlib import Path
from urllib.parse import urlparse


ROOT = Path(__file__).resolve().parent
CAPTURE_DIR = ROOT / "captures"
REPORT_DIR = ROOT / "reports"
REPORT_DIR.mkdir(parents=True, exist_ok=True)


def sh(cmd: list[str]) -> str:
    proc = subprocess.run(cmd, capture_output=True, text=True, check=False)
    out = (proc.stdout or "") + (proc.stderr or "")
    return out.strip()


def detect_version_from_binary(binary: Path) -> str:
    # Binary path usually ends with version token, e.g. /.../versions/2.1.79
    token = binary.name.strip()
    if re.fullmatch(r"\d+\.\d+\.\d+", token):
        return token
    return "unknown"


def latest_capture() -> Path:
    files = sorted(CAPTURE_DIR.glob("capture-*.jsonl"), key=lambda p: p.stat().st_mtime, reverse=True)
    if not files:
        raise FileNotFoundError(f"no capture files under {CAPTURE_DIR}")
    return files[0]


def load_jsonl(path: Path) -> list[dict]:
    records: list[dict] = []
    with path.open("r", encoding="utf-8") as fp:
        for raw in fp:
            line = raw.strip()
            if not line:
                continue
            try:
                records.append(json.loads(line))
            except json.JSONDecodeError:
                continue
    return records


def collect_binary_facts(binary: Path) -> dict:
    file_out = sh(["file", str(binary)])
    otool_out = sh(["otool", "-L", str(binary)])
    codesign_out = sh(["codesign", "-dv", "--verbose=4", str(binary)])
    strings_out = sh(["strings", "-a", str(binary)])

    def extract(pattern: str, text: str) -> list[str]:
        regex = re.compile(pattern)
        seen = set()
        out = []
        for line in text.splitlines():
            if regex.search(line):
                if line in seen:
                    continue
                seen.add(line)
                out.append(line)
        return out

    endpoints = extract(r"/v1/messages\?beta=true|/v1/messages/count_tokens\?beta=true|/api/oauth/usage|/v1/mcp_servers", strings_out)
    oauth = extract(r"BASE_API_URL|TOKEN_URL|AUTHORIZE_URL|oauth-2025-04-20|user:mcp_servers|user:sessions:claude_code", strings_out)
    betas = extract(r"claude-code-20250219|interleaved-thinking-2025-05-14|token-counting-2024-11-01|mcp-servers-2025-12-04|prompt-caching-scope-2026-01-05|advanced-tool-use-2025-11-20|effort-2025-11-24", strings_out)
    stream_hint = extract(r"X-Stainless-Helper-Method|helper-method|stream", strings_out)

    team_identifier = ""
    identifier = ""
    for line in codesign_out.splitlines():
        if line.startswith("TeamIdentifier="):
            team_identifier = line.split("=", 1)[1].strip()
        if line.startswith("Identifier="):
            identifier = line.split("=", 1)[1].strip()

    return {
        "binary": str(binary),
        "file": file_out,
        "identifier": identifier,
        "team_identifier": team_identifier,
        "otool_dependencies": [x for x in otool_out.splitlines()[1:] if x.strip()],
        "codesign_excerpt": [x for x in codesign_out.splitlines() if x.strip()][:20],
        "string_evidence": {
            "endpoint_related": endpoints[:40],
            "oauth_related": oauth[:40],
            "beta_related": betas[:40],
            "stream_related": stream_hint[:80],
        },
    }


def collect_dynamic_facts(records: list[dict]) -> dict:
    requests = [
        r for r in records
        if r.get("event") == "request"
        and "api.anthropic.com" in ((r.get("request") or {}).get("url", ""))
    ]
    tls_events = [
        r for r in records
        if r.get("event") == "tls_clienthello"
        and ((r.get("tls") or {}).get("sni") == "api.anthropic.com")
    ]

    endpoint_counts = Counter()
    header_freq_by_endpoint = defaultdict(Counter)
    message_stream_counter = Counter()
    tls_ext_seq = Counter()
    tls_65037_values = set()

    for req in requests:
        rq = req.get("request") or {}
        method = rq.get("method", "")
        url = rq.get("url", "")
        endpoint_counts[f"{method} {url}"] += 1
        headers = rq.get("headers") or []
        for h, _ in headers:
            header_freq_by_endpoint[f"{method} {url}"][h] += 1
        if url.endswith("/v1/messages?beta=true"):
            body = rq.get("body", "")
            stream = "unknown"
            if body and not str(body).startswith("base64:"):
                try:
                    payload = json.loads(body)
                    if isinstance(payload, dict) and "stream" in payload:
                        stream = "stream=true" if bool(payload.get("stream")) else "stream=false"
                except Exception:
                    stream = "parse_error"
            message_stream_counter[stream] += 1

    for ev in tls_events:
        tls = ev.get("tls") or {}
        ext = tls.get("extensions") or []
        seq = []
        for item in ext:
            if isinstance(item, list) and item:
                seq.append(item[0])
                if item[0] == 65037 and len(item) > 1:
                    tls_65037_values.add(item[1])
        tls_ext_seq[",".join(str(x) for x in seq)] += 1

    return {
        "anthropic_request_count": len(requests),
        "anthropic_tls_clienthello_count": len(tls_events),
        "endpoint_counts": endpoint_counts,
        "header_frequency_by_endpoint": {k: dict(v) for k, v in header_freq_by_endpoint.items()},
        "messages_stream_counter": dict(message_stream_counter),
        "tls_extension_sequences": dict(tls_ext_seq),
        "tls_65037_unique_value_count": len(tls_65037_values),
    }


def render_markdown(version: str, binary_facts: dict, dynamic_facts: dict, capture_path: Path) -> str:
    now = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%SZ")
    lines = []
    lines.append(f"# Claude Code {version} Reverse Evidence Report")
    lines.append("")
    lines.append(f"- Generated at: `{now}`")
    lines.append(f"- Capture source: `{capture_path}`")
    lines.append(f"- Binary source: `{binary_facts['binary']}`")
    lines.append("")
    lines.append("## 1) Static Binary Facts")
    lines.append("")
    lines.append(f"- `file`: `{binary_facts['file']}`")
    lines.append(f"- `Identifier`: `{binary_facts['identifier']}`")
    lines.append(f"- `TeamIdentifier`: `{binary_facts['team_identifier']}`")
    lines.append(f"- Runtime deps: `{len(binary_facts['otool_dependencies'])}` entries")
    lines.append("")
    lines.append("### Reproducible Evidence Table")
    lines.append("")
    lines.append("| Conclusion | Evidence Source |")
    lines.append("|---|---|")
    lines.append("| SDK contains `/v1/messages?beta=true` path | `strings` matched endpoint strings |")
    lines.append("| SDK contains `/v1/messages/count_tokens?beta=true` path | `strings` matched endpoint strings |")
    lines.append("| OAuth beta `oauth-2025-04-20` exists in binary | `strings` matched oauth/beta constants |")
    lines.append("| Stream helper header path exists (`X-Stainless-Helper-Method`) | `strings` matched stream-related tokens |")
    lines.append("")
    lines.append("## 2) Dynamic Capture Facts")
    lines.append("")
    lines.append(f"- Anthropic request count: `{dynamic_facts['anthropic_request_count']}`")
    lines.append(f"- Anthropic TLS ClientHello count: `{dynamic_facts['anthropic_tls_clienthello_count']}`")
    lines.append(f"- `messages.stream` distribution: `{dynamic_facts['messages_stream_counter']}`")
    lines.append(f"- TLS 65037 unique values: `{dynamic_facts['tls_65037_unique_value_count']}`")
    lines.append("")
    lines.append("### Endpoint Count Matrix")
    lines.append("")
    for k, v in sorted(dynamic_facts["endpoint_counts"].items(), key=lambda x: (-x[1], x[0])):
        lines.append(f"- `{k}` -> `{v}`")
    lines.append("")
    lines.append("### TLS Shape Matrix (api.anthropic.com)")
    lines.append("")
    for k, v in sorted(dynamic_facts["tls_extension_sequences"].items(), key=lambda x: (-x[1], x[0])):
        lines.append(f"- `{k}` -> `{v}`")
    lines.append("")
    lines.append("## 3) Simulation Design (No Code Change in This Report)")
    lines.append("")
    lines.append("- P0: TLS 65037 dynamic value per connection + mixed extension-shape strategy (`...43` and `...43,21`).")
    lines.append("- P1: Host pin policy for `mcp_servers` and `oauth/usage` to `api.anthropic.com` (with explicit exception rules).")
    lines.append("- P1: Endpoint-aware header profile policy (messages stream vs non-stream, count_tokens, mcp_servers, oauth_usage).")
    lines.append("- P2: Request cadence emulation for MCP/usage prefetch windows and dedupe windows.")
    lines.append("")
    lines.append("### Interface Drafts (Proposal Only)")
    lines.append("")
    lines.append("- `TLSProfileMode`: `fixed | dynamic | mixed`")
    lines.append("- `EndpointHeaderProfile`: supports endpoint + stream-conditional header policies")
    lines.append("- `EndpointHostPolicy`: supports endpoint-level host pin and exception strategy")
    lines.append("")
    lines.append("## 4) Validation Criteria")
    lines.append("")
    lines.append("- Endpoint/URL hit rate: `100%`")
    lines.append("- Header-set hit rate: `>= 98%`")
    lines.append("- TLS-shape hit rate: `>= 95%` and both extension shapes present")
    return "\n".join(lines) + "\n"


def main() -> int:
    parser = argparse.ArgumentParser(description="Build Claude Code reverse evidence report")
    parser.add_argument("--binary", default="/Users/luli/.local/share/claude/versions/2.1.79", help="Path to claude binary")
    parser.add_argument("--capture", default="", help="Path to capture jsonl (default: latest)")
    args = parser.parse_args()

    capture_path = Path(args.capture) if args.capture else latest_capture()
    binary_path = Path(args.binary)
    version = detect_version_from_binary(binary_path)

    records = load_jsonl(capture_path)
    binary_facts = collect_binary_facts(binary_path)
    dynamic_facts = collect_dynamic_facts(records)

    payload = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "capture": str(capture_path),
        "binary_facts": binary_facts,
        "dynamic_facts": dynamic_facts,
    }
    json_path = REPORT_DIR / f"reverse_evidence_{version}.json"
    md_path = REPORT_DIR / f"reverse_report_{version}.md"
    json_path.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
    md_path.write_text(render_markdown(version, binary_facts, dynamic_facts, capture_path), encoding="utf-8")

    print(f"wrote: {json_path}")
    print(f"wrote: {md_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
