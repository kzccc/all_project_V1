#!/usr/bin/env python3

from __future__ import annotations

import argparse
import json
from pathlib import Path


def load(path: str) -> dict:
    return json.loads(Path(path).read_text(encoding="utf-8"))


def num(v):
    if v is None:
        return "-"
    if isinstance(v, float):
        return f"{v:.3f}"
    return str(v)


def pct_delta(a, b):
    if a in (None, 0) or b is None:
        return "-"
    return f"{((b - a) / a) * 100:.2f}%"


def build_report(channel_single: dict, kafka_single: dict, channel_group: dict, kafka_group: dict) -> str:
    lines = []
    lines.append("# Channel vs Kafka Message Test Report")
    lines.append("")
    lines.append("## Single Chat")
    lines.append("")
    lines.append("| Metric | channel | kafka | kafka vs channel |")
    lines.append("| --- | ---: | ---: | ---: |")
    single_rows = [
        ("success_rate", channel_single["delivery_success_rate"], kafka_single["delivery_success_rate"]),
        ("throughput_msg_per_sec", channel_single["observed_throughput_msg_per_sec"], kafka_single["observed_throughput_msg_per_sec"]),
        ("latency_avg_ms", channel_single["latency"]["avg_ms"], kafka_single["latency"]["avg_ms"]),
        ("latency_p50_ms", channel_single["latency"]["p50_ms"], kafka_single["latency"]["p50_ms"]),
        ("latency_p95_ms", channel_single["latency"]["p95_ms"], kafka_single["latency"]["p95_ms"]),
        ("latency_p99_ms", channel_single["latency"]["p99_ms"], kafka_single["latency"]["p99_ms"]),
    ]
    for name, c_value, k_value in single_rows:
        lines.append(f"| {name} | {num(c_value)} | {num(k_value)} | {pct_delta(c_value, k_value)} |")

    lines.append("")
    lines.append("## Group Chat")
    lines.append("")
    lines.append("| Metric | channel | kafka | kafka vs channel |")
    lines.append("| --- | ---: | ---: | ---: |")
    group_rows = [
        ("coverage_rate", channel_group["delivery_coverage_rate"], kafka_group["delivery_coverage_rate"]),
        ("full_coverage_message_rate", channel_group["full_coverage_message_rate"], kafka_group["full_coverage_message_rate"]),
        ("delivery_per_sec", channel_group["observed_delivery_per_sec"], kafka_group["observed_delivery_per_sec"]),
        ("receipt_avg_ms", channel_group["receipt_latency"]["avg_ms"], kafka_group["receipt_latency"]["avg_ms"]),
        ("receipt_p95_ms", channel_group["receipt_latency"]["p95_ms"], kafka_group["receipt_latency"]["p95_ms"]),
        ("receipt_p99_ms", channel_group["receipt_latency"]["p99_ms"], kafka_group["receipt_latency"]["p99_ms"]),
        ("broadcast_p95_ms", channel_group["broadcast_completion_latency"]["p95_ms"], kafka_group["broadcast_completion_latency"]["p95_ms"]),
        ("broadcast_p99_ms", channel_group["broadcast_completion_latency"]["p99_ms"], kafka_group["broadcast_completion_latency"]["p99_ms"]),
    ]
    for name, c_value, k_value in group_rows:
        lines.append(f"| {name} | {num(c_value)} | {num(k_value)} | {pct_delta(c_value, k_value)} |")

    def resource_lines(title: str, channel_summary: dict, kafka_summary: dict) -> None:
        channel_peak = channel_summary.get("server_resource_peak", {})
        kafka_peak = kafka_summary.get("server_resource_peak", {})
        if not channel_peak and not kafka_peak:
            return
        lines.append("")
        lines.append(f"## {title} Server Resource Peak")
        lines.append("")
        lines.append("| Metric | channel | kafka |")
        lines.append("| --- | ---: | ---: |")
        for key in ("rss_peak_mb", "threads_peak", "fd_peak"):
            lines.append(f"| {key} | {num(channel_peak.get(key))} | {num(kafka_peak.get(key))} |")

    resource_lines("Single Chat", channel_single, kafka_single)
    resource_lines("Group Chat", channel_group, kafka_group)
    lines.append("")
    lines.append("## Notes")
    lines.append("")
    lines.append("- This report only compares the measured business-message path.")
    lines.append("- Login and WebSocket handshake were used to establish test sessions, but latency metrics start at sender send time.")
    lines.append("- Group broadcast completion latency is calculated only on messages that reached full receiver coverage.")
    lines.append("")
    return "\n".join(lines) + "\n"


def main() -> None:
    parser = argparse.ArgumentParser(description="Compare channel and kafka message test summaries.")
    parser.add_argument("--channel-single", required=True)
    parser.add_argument("--kafka-single", required=True)
    parser.add_argument("--channel-group", required=True)
    parser.add_argument("--kafka-group", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()

    report = build_report(
        load(args.channel_single),
        load(args.kafka_single),
        load(args.channel_group),
        load(args.kafka_group),
    )
    output_path = Path(args.output)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(report, encoding="utf-8")
    print(output_path)


if __name__ == "__main__":
    main()
