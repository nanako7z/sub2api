#!/usr/bin/env python3
import argparse
import json
import base64
from collections import Counter, defaultdict
from datetime import datetime, timezone
from pathlib import Path


ROOT = Path(__file__).resolve().parent
REPORT_DIR = ROOT / "reports"
REPORT_DIR.mkdir(parents=True, exist_ok=True)


def load_jsonl(path: Path) -> list[dict]:
    rows = []
    with path.open("r", encoding="utf-8") as fp:
        for raw in fp:
            line = raw.strip()
            if not line:
                continue
            try:
                rows.append(json.loads(line))
            except json.JSONDecodeError:
                continue
    return rows


def signature(rows: list[dict]) -> dict:
    endpoint = Counter()
    headers = defaultdict(Counter)
    tls_shape = Counter()
    tls_combo = Counter()
    tls65037 = set()

    for r in rows:
        if r.get("event") == "request":
            rq = r.get("request") or {}
            url = rq.get("url", "")
            if "api.anthropic.com" not in url:
                continue
            key = f"{rq.get('method', '')} {url}"
            endpoint[key] += 1
            for h, _ in rq.get("headers", []):
                headers[key][h.lower()] += 1

        if r.get("event") == "tls_clienthello":
            tls = r.get("tls") or {}
            if tls.get("sni") != "api.anthropic.com":
                continue
            ext = []
            ext_65037_len = 0
            ext_21_len = 0
            for item in tls.get("extensions", []):
                if isinstance(item, list) and item:
                    ext_id = item[0]
                    ext.append(ext_id)
                    if ext_id == 65037 and len(item) > 1:
                        tls65037.add(item[1])
                        ext_65037_len = _decoded_len(item[1])
                    if ext_id == 21 and len(item) > 1:
                        ext_21_len = _decoded_len(item[1])
            tls_shape[",".join(str(x) for x in ext)] += 1
            tls_combo[f"{ext_65037_len},{ext_21_len}"] += 1

    return {
        "endpoint": endpoint,
        "headers": {k: dict(v) for k, v in headers.items()},
        "tls_shape": tls_shape,
        "tls_combo": tls_combo,
        "tls65037_count": len(tls65037),
    }


def _decoded_len(value) -> int:
    if not isinstance(value, str):
        return 0
    if value.startswith("base64:"):
        try:
            return len(base64.b64decode(value[7:]))
        except Exception:
            return 0
    return len(value.encode("utf-8"))


def endpoint_hit_rate(base: Counter, cand: Counter) -> float:
    if not base:
        return 1.0
    base_total = sum(base.values())
    hit = 0
    for k, v in base.items():
        hit += min(v, cand.get(k, 0))
    return hit / base_total


def header_hit_rate(base: dict, cand: dict) -> float:
    if not base:
        return 1.0
    scores = []
    for endpoint, hmap in base.items():
        bset = set(hmap.keys())
        cset = set((cand.get(endpoint) or {}).keys())
        if not bset:
            continue
        scores.append(len(bset & cset) / len(bset))
    if not scores:
        return 1.0
    return sum(scores) / len(scores)


def tls_shape_hit_rate(base: Counter, cand: Counter) -> float:
    if not base:
        return 1.0
    total = sum(base.values())
    hit = 0
    for k, v in base.items():
        hit += min(v, cand.get(k, 0))
    return hit / total


def l1_distance(base: Counter, cand: Counter) -> float:
    base_total = sum(base.values())
    cand_total = sum(cand.values())
    if base_total == 0 and cand_total == 0:
        return 0.0
    keys = set(base.keys()) | set(cand.keys())
    dist = 0.0
    for k in keys:
        pb = (base.get(k, 0) / base_total) if base_total > 0 else 0.0
        pc = (cand.get(k, 0) / cand_total) if cand_total > 0 else 0.0
        dist += abs(pb - pc)
    return dist


def combo_table(base: Counter, cand: Counter) -> list[dict]:
    base_total = sum(base.values())
    cand_total = sum(cand.values())
    keys = sorted(set(base.keys()) | set(cand.keys()))
    rows = []
    for k in keys:
        b = base.get(k, 0)
        c = cand.get(k, 0)
        rows.append(
            {
                "combo": k,
                "baseline_count": b,
                "baseline_percent": round((b / base_total) if base_total > 0 else 0.0, 6),
                "candidate_count": c,
                "candidate_percent": round((c / cand_total) if cand_total > 0 else 0.0, 6),
            }
        )
    return rows


def top_missing(base: Counter, cand: Counter, limit: int = 10) -> list[dict]:
    misses = []
    for k, v in base.items():
        diff = v - cand.get(k, 0)
        if diff > 0:
            misses.append({"signature": k, "missing": diff})
    misses.sort(key=lambda x: x["missing"], reverse=True)
    return misses[:limit]


def main() -> int:
    parser = argparse.ArgumentParser(description="Compare baseline/candidate MITM captures")
    parser.add_argument("--baseline", required=True, help="Baseline capture jsonl")
    parser.add_argument("--candidate", required=True, help="Candidate capture jsonl")
    parser.add_argument(
        "--strict",
        action="store_true",
        help="Exit with non-zero code when any threshold check fails",
    )
    args = parser.parse_args()

    baseline = Path(args.baseline)
    candidate = Path(args.candidate)
    base_sig = signature(load_jsonl(baseline))
    cand_sig = signature(load_jsonl(candidate))

    endpoint_rate = endpoint_hit_rate(base_sig["endpoint"], cand_sig["endpoint"])
    header_rate = header_hit_rate(base_sig["headers"], cand_sig["headers"])
    tls_rate = tls_shape_hit_rate(base_sig["tls_shape"], cand_sig["tls_shape"])
    tls_combo_l1 = l1_distance(base_sig["tls_combo"], cand_sig["tls_combo"])
    tls_combo_rows = combo_table(base_sig["tls_combo"], cand_sig["tls_combo"])
    has_two_tls_shapes = len(cand_sig["tls_shape"]) >= 2

    report = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "baseline": str(baseline),
        "candidate": str(candidate),
        "scores": {
            "endpoint_url_hit_rate": round(endpoint_rate, 6),
            "header_set_hit_rate": round(header_rate, 6),
            "tls_shape_hit_rate": round(tls_rate, 6),
            "tls_combo_l1_distance": round(tls_combo_l1, 6),
            "candidate_tls_shape_count": len(cand_sig["tls_shape"]),
            "candidate_tls_65037_unique_count": cand_sig["tls65037_count"],
        },
        "thresholds": {
            "endpoint_url_hit_rate": 1.0,
            "header_set_hit_rate": 0.98,
            "tls_shape_hit_rate": 0.95,
            "tls_combo_l1_distance_max": 0.20,
            "tls_dual_shape_required": True,
        },
        "pass": {
            "endpoint_url_hit_rate": endpoint_rate >= 1.0,
            "header_set_hit_rate": header_rate >= 0.98,
            "tls_shape_hit_rate": tls_rate >= 0.95,
            "tls_combo_l1_distance": tls_combo_l1 <= 0.20,
            "tls_dual_shape_required": has_two_tls_shapes,
        },
        "tls_combo_table": tls_combo_rows,
        "remaining_topN": {
            "endpoint_signatures": top_missing(base_sig["endpoint"], cand_sig["endpoint"], limit=10),
            "tls_shapes": top_missing(base_sig["tls_shape"], cand_sig["tls_shape"], limit=10),
        },
    }

    out_json = REPORT_DIR / "capture_compare_report.json"
    out_md = REPORT_DIR / "capture_compare_report.md"
    out_json.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")

    md = []
    md.append("# Capture Compare Report")
    md.append("")
    md.append(f"- Baseline: `{baseline}`")
    md.append(f"- Candidate: `{candidate}`")
    md.append("")
    md.append("## Scores")
    md.append("")
    md.append(f"- Endpoint/URL hit rate: `{report['scores']['endpoint_url_hit_rate']}` (target: `1.0`)")
    md.append(f"- Header set hit rate: `{report['scores']['header_set_hit_rate']}` (target: `0.98`)")
    md.append(f"- TLS shape hit rate: `{report['scores']['tls_shape_hit_rate']}` (target: `0.95`)")
    md.append(f"- TLS combo L1 distance: `{report['scores']['tls_combo_l1_distance']}` (target: `<=0.20`)")
    md.append(f"- Candidate TLS shape count: `{report['scores']['candidate_tls_shape_count']}` (target: `>=2`)")
    md.append("")
    md.append("## Threshold Checks")
    md.append("")
    md.append(f"- endpoint_url_hit_rate: `{report['pass']['endpoint_url_hit_rate']}`")
    md.append(f"- header_set_hit_rate: `{report['pass']['header_set_hit_rate']}`")
    md.append(f"- tls_shape_hit_rate: `{report['pass']['tls_shape_hit_rate']}`")
    md.append(f"- tls_combo_l1_distance: `{report['pass']['tls_combo_l1_distance']}`")
    md.append(f"- tls_dual_shape_required: `{report['pass']['tls_dual_shape_required']}`")
    md.append("")
    md.append("## TLS Combo Table")
    md.append("")
    md.append("| combo(65037_len,padding21_len) | baseline count/% | candidate count/% |")
    md.append("|---|---:|---:|")
    for row in tls_combo_rows:
        md.append(
            f"| `{row['combo']}` | `{row['baseline_count']}` / `{row['baseline_percent']}` | "
            f"`{row['candidate_count']}` / `{row['candidate_percent']}` |"
        )
    md.append("")
    md.append("## Remaining TopN")
    md.append("")
    md.append("- Endpoint signatures:")
    for item in report["remaining_topN"]["endpoint_signatures"]:
        md.append(f"  - `{item['signature']}` missing `{item['missing']}`")
    md.append("- TLS shapes:")
    for item in report["remaining_topN"]["tls_shapes"]:
        md.append(f"  - `{item['signature']}` missing `{item['missing']}`")
    out_md.write_text("\n".join(md) + "\n", encoding="utf-8")

    print(f"wrote: {out_json}")
    print(f"wrote: {out_md}")
    if args.strict and not all(report["pass"].values()):
        print("threshold check failed in strict mode")
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
