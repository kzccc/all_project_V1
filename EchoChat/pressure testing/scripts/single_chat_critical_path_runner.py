#!/usr/bin/env python3
from __future__ import annotations

import argparse
from collections import Counter, defaultdict
import json
import math
import statistics
from pathlib import Path
import sys


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


def first_field(fields: dict | None, key: str, default=None):
    if not isinstance(fields, dict):
        return default
    return fields.get(key, default)


def load_json(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def event_time_ms(event: dict) -> float:
    return float(event["occurred_unix_ns"]) / 1_000_000.0


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--trace", required=True)
    parser.add_argument("--summary", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()

    trace_path = Path(args.trace).resolve()
    summary_path = Path(args.summary).resolve()
    output_path = Path(args.output).resolve()

    trace = load_json(trace_path)
    summary = load_json(summary_path)

    meta_by_id: dict[str, dict] = {}
    for meta in trace.get("messages", []):
        bench_id = meta.get("bench_id")
        if bench_id:
            meta_by_id[bench_id] = meta

    events_by_id: dict[str, dict[str, dict]] = defaultdict(dict)
    unique_flushes: dict[tuple[str, int], dict] = {}
    enqueue_queue_depths: list[float] = []

    for event in trace.get("events", []):
        bench_id = event.get("bench_id")
        if not bench_id:
            continue
        name = event.get("event")
        current = events_by_id[bench_id].get(name)
        if current is None:
            events_by_id[bench_id][name] = event
        else:
            if event["occurred_unix_ns"] < current["occurred_unix_ns"]:
                events_by_id[bench_id][name] = event
        if name == "mysql_persist_flush_done":
            fields = event.get("fields")
            flush_worker = str(first_field(fields, "worker_index", "unknown"))
            flush_seq = int(first_field(fields, "flush_seq", 0))
            key = (flush_worker, flush_seq)
            if key not in unique_flushes:
                unique_flushes[key] = {
                    "flush_reason": str(first_field(fields, "flush_reason", "unknown")),
                    "flush_worker": flush_worker,
                    "flush_batch_size": first_field(fields, "flush_batch_size"),
                }
        elif name == "mysql_persist_enqueue_done":
            queue_depth = first_field(event.get("fields"), "queue_depth")
            if isinstance(queue_depth, (int, float)):
                enqueue_queue_depths.append(float(queue_depth))

    flush_reason_counts: Counter[str] = Counter()
    flush_worker_counts: Counter[str] = Counter()
    flush_batch_sizes: list[float] = []
    for flush in unique_flushes.values():
        flush_reason_counts[flush["flush_reason"]] += 1
        flush_worker_counts[flush["flush_worker"]] += 1
        batch_size = flush.get("flush_batch_size")
        if isinstance(batch_size, (int, float)):
            flush_batch_sizes.append(float(batch_size))

    conversation_bucket_enqueue_ms: list[float] = []
    conversation_dispatch_queue_wait_ms: list[float] = []
    conversation_bucket_queue_wait_ms: list[float] = []
    conversation_ready_queue_wait_ms: list[float] = []
    mysql_persist_ms: list[float] = []
    receiver_ws_write_ms: list[float] = []
    server_critical_path_ms: list[float] = []
    end_to_end_ms: list[float] = []

    for bench_id, meta in meta_by_id.items():
        event_map = events_by_id.get(bench_id, {})
        send_ts_ms = meta.get("send_ts_ms")
        decode = event_map.get("consumer_decode_done")
        bucket_enqueue = event_map.get("conversation_bucket_enqueue")
        ready_enqueue = event_map.get("conversation_ready_enqueue")
        ready_dequeue = event_map.get("conversation_ready_dequeue")
        dispatch_start = event_map.get("conversation_dispatch_start")
        persist_enqueue = event_map.get("mysql_persist_enqueue_done")
        persist_flush = event_map.get("mysql_persist_flush_done")
        ws_write = event_map.get("ws_write_done")

        if decode and bucket_enqueue:
            conversation_bucket_enqueue_ms.append(event_time_ms(bucket_enqueue) - event_time_ms(decode))
        if bucket_enqueue and dispatch_start:
            duration = event_time_ms(dispatch_start) - event_time_ms(bucket_enqueue)
            conversation_dispatch_queue_wait_ms.append(duration)
            conversation_bucket_queue_wait_ms.append(duration)
        if ready_enqueue and ready_dequeue:
            conversation_ready_queue_wait_ms.append(event_time_ms(ready_dequeue) - event_time_ms(ready_enqueue))
        if persist_enqueue and persist_flush:
            mysql_persist_ms.append(event_time_ms(persist_flush) - event_time_ms(persist_enqueue))
        if persist_flush and ws_write:
            receiver_ws_write_ms.append(event_time_ms(ws_write) - event_time_ms(persist_flush))
        if decode and ws_write:
            server_critical_path_ms.append(event_time_ms(ws_write) - event_time_ms(decode))
        if send_ts_ms is not None and ws_write:
            end_to_end_ms.append(event_time_ms(ws_write) - float(send_ts_ms))

    result = {
        "summary": summary,
        "time_anchor_ms": (
            float(summary.get("send_window_start_ms"))
            if isinstance(summary.get("send_window_start_ms"), (int, float))
            else (
                float(summary.get("started_at_ms"))
                if isinstance(summary.get("started_at_ms"), (int, float))
                else (
                    float(summary.get("started_at_unix")) * 1000.0
                    if isinstance(summary.get("started_at_unix"), (int, float))
                    else None
                )
            )
        ),
        "trace_counts": {
            "registered_messages": len(trace.get("messages", [])),
            "event_count": len(trace.get("events", [])),
        },
        "stage_metrics": {
            "conversation_bucket_enqueue_ms": summarize(conversation_bucket_enqueue_ms),
            "conversation_dispatch_queue_wait_ms": summarize(conversation_dispatch_queue_wait_ms),
            "conversation_bucket_queue_wait_ms": summarize(conversation_bucket_queue_wait_ms),
            "conversation_ready_queue_wait_ms": summarize(conversation_ready_queue_wait_ms),
            "mysql_persist_ms": summarize(mysql_persist_ms),
            "receiver_ws_write_ms": summarize(receiver_ws_write_ms),
            "server_critical_path_ms": summarize(server_critical_path_ms),
            "end_to_end_ms": summarize(end_to_end_ms),
        },
        "persist": {
            "flush_count": sum(flush_reason_counts.values()),
            "flush_reason_counts": dict(flush_reason_counts),
            "flush_worker_counts": dict(flush_worker_counts),
            "avg_flush_batch_size": round(statistics.fmean(flush_batch_sizes), 3) if flush_batch_sizes else None,
            "avg_enqueue_queue_depth": round(statistics.fmean(enqueue_queue_depths), 3) if enqueue_queue_depths else None,
        },
    }

    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding="utf-8")
    print(output_path)
    return 0


if __name__ == "__main__":
    sys.exit(main())
