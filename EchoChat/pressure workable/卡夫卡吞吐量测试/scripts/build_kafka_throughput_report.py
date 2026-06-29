#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import math
from pathlib import Path


def load_json(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def fmt_num(value, digits: int = 3) -> str:
    if value is None:
        return "本轮未采到"
    if isinstance(value, int):
        return str(value)
    if isinstance(value, float):
        if math.isfinite(value):
            return f"{value:.{digits}f}"
        return str(value)
    return str(value)


def build_report(summary: dict) -> str:
    broker = summary.get("broker", {})
    producer = summary.get("producer", {})
    consumer = summary.get("consumer", {})
    e2e = summary.get("e2e", {})

    lines: list[str] = []
    lines.append("# Kafka 官方工具吞吐极限报告")
    lines.append("")
    lines.append("口径")
    lines.append(f"- 运行标签：`{summary.get('label')}`")
    lines.append(f"- 生成时间：`{summary.get('generated_at')}`")
    lines.append(f"- 测试拓扑：`single broker / single controller / KRaft`")
    lines.append(
        f"- Broker：`{broker.get('host')}:{broker.get('port')}`，"
        f"分区默认数 `producer={producer.get('partitions')}` / "
        f"`consumer={consumer.get('partitions')}` / `e2e={e2e.get('partitions')}`"
    )
    lines.append("- 测试范围：`producer-perf-test`、`consumer-perf-test`、`kafka-e2e-latency`")
    lines.append("")
    lines.append("吞吐量")
    lines.append(
        f"- producer 极限吞吐：`{fmt_num(producer.get('records_per_sec'))} records/s`，"
        f"`{fmt_num(producer.get('mb_per_sec'))} MB/s`"
    )
    lines.append(
        f"- consumer 极限吞吐：`{fmt_num(consumer.get('mb_per_sec'))} MB/s`，"
        f"`{fmt_num(consumer.get('messages_per_sec'))} records/s`"
    )
    lines.append(
        f"- e2e 请求消息数：`{fmt_num(e2e.get('num_messages'), 0)}` 条，"
        f"采样点 `sample_points={fmt_num(e2e.get('sample_points'), 0)}`"
    )
    lines.append("")
    lines.append("全链路各模块平均耗时")
    lines.append("- ingress_to_produce_ack_ms：本轮未采到")
    lines.append("- kafka_queue_wait_ms：本轮未采到")
    lines.append("- deserialize_ms：本轮未采到")
    lines.append("- session_seq_ms：本轮未采到")
    lines.append("- mysql_persist_ms：本轮未采到")
    lines.append("- dispatch_after_persist_ms：本轮未采到")
    lines.append("- receiver_queue_wait_ms：本轮未采到")
    lines.append("- receiver_ws_write_ms：本轮未采到")
    lines.append("- server_critical_path_ms：本轮未采到")
    lines.append(
        f"- end_to_end_ms：avg `{fmt_num(e2e.get('avg_latency_ms'))}` / "
        f"p50 `{fmt_num(e2e.get('p50_latency_ms'))}` / "
        f"p95 `{fmt_num(e2e.get('p95_latency_ms'))}` / "
        f"p99 `{fmt_num(e2e.get('p99_latency_ms'))}` / "
        f"max `{fmt_num(e2e.get('max_latency_ms'))}`"
    )
    lines.append("")
    lines.append("mysql_persist 细分")
    lines.append("- enqueue_block：本轮未采到")
    lines.append("- worker_queue_wait：本轮未采到")
    lines.append("- batch_collect_wait：本轮未采到")
    lines.append("- sql_exec：本轮未采到")
    lines.append("- flush：本轮未采到")
    lines.append("")
    lines.append("每秒读写")
    lines.append(
        f"- producer 总耗时 `{fmt_num(producer.get('total_time_sec'))}` 秒，"
        f"对应平均写入 `{fmt_num(producer.get('records_per_sec'))}` records/s"
    )
    lines.append(
        f"- consumer 总耗时 `{fmt_num(consumer.get('total_time_sec'))}` 秒，"
        f"对应平均读取 `{fmt_num(consumer.get('messages_per_sec'))}` records/s"
    )
    lines.append("- 第 1 秒：read 本轮未采到，write 本轮未采到")
    lines.append("")
    lines.append("分区热度")
    lines.append(f"- active partitions：`{fmt_num(producer.get('partitions'), 0)}`")
    lines.append("- hottest partition：本轮未采到")
    lines.append("- hottest share：本轮未采到")
    lines.append("- heat-shape：官方 perf 工具不输出分区级明细，本轮未采到")
    lines.append("")
    lines.append("consumer 分配")
    lines.append("- consumer 实例数：`1`")
    lines.append(f"- 总分配分区：`{fmt_num(consumer.get('partitions'), 0)}`")
    lines.append(f"- 实际活跃分区：`{fmt_num(consumer.get('partitions'), 0)}`")
    lines.append(
        f"- {consumer.get('group')}：assigned `{fmt_num(consumer.get('partitions'), 0)}`，"
        f"active `{fmt_num(consumer.get('partitions'), 0)}`，"
        f"consumed `{fmt_num(consumer.get('messages'), 0)}`"
    )
    lines.append("")
    lines.append("persist batch")
    lines.append("- flush reason：本轮未采到")
    lines.append("- batch 分布：本轮未采到")
    lines.append("- per reason average batch size：本轮未采到")
    lines.append("- flush duration：本轮未采到")
    lines.append("")
    lines.append("总结")
    lines.append(
        f"- 本轮 Kafka 官方工具三项基准里，producer 上限约为 "
        f"`{fmt_num(producer.get('records_per_sec'))} records/s`，"
        f"consumer 上限约为 `{fmt_num(consumer.get('messages_per_sec'))} records/s`。"
    )
    lines.append(
        f"- 端到端单条往返平均 `{fmt_num(e2e.get('avg_latency_ms'))} ms`，"
        f"p95 `{fmt_num(e2e.get('p95_latency_ms'))}`，当前瓶颈口径更偏向 broker 自身读写与刷盘能力。"
    )
    lines.append(
        "- 这份结果是 Kafka 工具自测基线，不包含 EchoChat 业务链路，所以适合拿来和后续业务压测结果做上限差对比。"
    )
    lines.append("")
    return "\n".join(lines)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()

    summary = load_json(Path(args.input).resolve())
    output_path = Path(args.output).resolve()
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(build_report(summary), encoding="utf-8")
    print(output_path)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
