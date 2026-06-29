#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import math
import statistics
from pathlib import Path


VALID_MODES = ("cold_start", "hot_path", "redis_floor_recovery")


def load_json(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def percentile(values: list[float], p: float) -> float | None:
    if not values:
        return None
    if len(values) == 1:
        return float(values[0])
    ordered = sorted(values)
    rank = (len(ordered) - 1) * p
    low = math.floor(rank)
    high = math.ceil(rank)
    if low == high:
        return float(ordered[low])
    weight = rank - low
    return float(ordered[low] * (1 - weight) + ordered[high] * weight)


def summarize(values: list[float]) -> dict[str, float | int | None]:
    return {
        "count": len(values),
        "avg_ms": round(statistics.fmean(values), 3) if values else None,
        "p50_ms": round(percentile(values, 0.50), 3) if values else None,
        "p95_ms": round(percentile(values, 0.95), 3) if values else None,
        "p99_ms": round(percentile(values, 0.99), 3) if values else None,
        "max_ms": round(max(values), 3) if values else None,
    }


def mode_rank(mode: str) -> int:
    try:
        return VALID_MODES.index(mode)
    except ValueError:
        return len(VALID_MODES)


def extract_session_seq_rows(trace: dict) -> list[dict]:
    start_events: dict[str, dict] = {}
    rows: list[dict] = []
    for event in trace.get("events", []):
        bench_id = event.get("bench_id")
        if not bench_id:
            continue
        name = event.get("event")
        if name == "session_seq_start":
            current = start_events.get(bench_id)
            if current is None or event.get("occurred_unix_ns", 0) < current.get("occurred_unix_ns", 0):
                start_events[bench_id] = event
        elif name == "session_seq_done":
            start = start_events.get(bench_id)
            if not start:
                continue
            end_ns = event.get("occurred_unix_ns")
            start_ns = start.get("occurred_unix_ns")
            if not isinstance(end_ns, int) or not isinstance(start_ns, int) or end_ns < start_ns:
                continue
            fields = event.get("fields") or {}
            rows.append(
                {
                    "bench_id": bench_id,
                    "seq_mode": str(fields.get("seq_mode", "unknown")),
                    "duration_ms": round((end_ns - start_ns) / 1_000_000.0, 6),
                    "partition": fields.get("partition"),
                    "offset": fields.get("offset"),
                    "topic": fields.get("topic"),
                }
            )
    return rows


def sigma_outliers(rows: list[dict], sigma: float) -> list[dict]:
    durations = [row["duration_ms"] for row in rows]
    if len(durations) < 3:
        return []
    mean = statistics.fmean(durations)
    stddev = statistics.pstdev(durations)
    if stddev <= 0:
        return []
    threshold = mean + sigma * stddev
    return [row for row in rows if row["duration_ms"] > threshold]


def analyze_reasonableness(mode_summary: dict, min_expected_hot_count: int, min_expected_recovery_count: int) -> dict:
    findings: list[str] = []
    unreasonable = False

    cold_avg = mode_summary.get("cold_start", {}).get("avg_ms")
    hot_avg = mode_summary.get("hot_path", {}).get("avg_ms")
    recovery_avg = mode_summary.get("redis_floor_recovery", {}).get("avg_ms")
    hot_count = int(mode_summary.get("hot_path", {}).get("count") or 0)
    recovery_count = int(mode_summary.get("redis_floor_recovery", {}).get("count") or 0)

    if hot_count < min_expected_hot_count:
        unreasonable = True
        findings.append(f"hot_path 样本数偏少，仅 {hot_count}")
    if recovery_count < min_expected_recovery_count:
        unreasonable = True
        findings.append(f"redis_floor_recovery 样本数偏少，仅 {recovery_count}")
    if cold_avg is not None and hot_avg is not None and hot_avg > cold_avg * 1.25:
        unreasonable = True
        findings.append("hot_path 平均耗时显著高于 cold_start，不符合常识")
    if recovery_avg is not None and hot_avg is not None and recovery_avg < hot_avg * 0.9:
        unreasonable = True
        findings.append("redis_floor_recovery 平均耗时显著低于 hot_path，不符合常识")

    if not findings:
        findings.append("三类顺序号分发耗时关系基本符合预期")
    return {"unreasonable": unreasonable, "findings": findings}


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--trace", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--outlier-sigma", type=float, default=4.0)
    parser.add_argument("--min-expected-hot-count", type=int, default=5000)
    parser.add_argument("--min-expected-recovery-count", type=int, default=200)
    args = parser.parse_args()

    trace = load_json(Path(args.trace).resolve())
    rows = extract_session_seq_rows(trace)

    by_mode: dict[str, list[dict]] = {mode: [] for mode in VALID_MODES}
    by_mode["unknown"] = []
    for row in rows:
        by_mode.setdefault(row["seq_mode"], []).append(row)

    mode_summary = {mode: summarize([row["duration_ms"] for row in mode_rows]) for mode, mode_rows in by_mode.items()}
    outliers = {mode: sigma_outliers(mode_rows, args.outlier_sigma)[:20] for mode, mode_rows in by_mode.items() if mode_rows}
    reasonableness = analyze_reasonableness(
        mode_summary,
        min_expected_hot_count=args.min_expected_hot_count,
        min_expected_recovery_count=args.min_expected_recovery_count,
    )

    result = {
        "total_session_seq_samples": len(rows),
        "mode_summary": mode_summary,
        "mode_order": list(VALID_MODES),
        "reasonableness": reasonableness,
        "top_outliers": outliers,
        "raw_mode_counts": {key: len(value) for key, value in by_mode.items()},
        "sorted_mode_rows_preview": sorted(rows, key=lambda item: (mode_rank(item["seq_mode"]), item["duration_ms"]), reverse=False)[:50],
    }

    output_path = Path(args.output).resolve()
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding="utf-8")
    print(output_path)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
