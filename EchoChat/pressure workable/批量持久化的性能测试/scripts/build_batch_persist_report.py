#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import math
from pathlib import Path


def load_json(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def fmt_num(value) -> str:
    if value is None:
        return "-"
    if isinstance(value, int):
        return str(value)
    if isinstance(value, float):
        if math.isfinite(value):
            return f"{value:.3f}"
        return str(value)
    return str(value)


def pct_improve(baseline: float | int | None, candidate: float | int | None, lower_is_better: bool) -> str:
    if baseline in (None, 0) or candidate is None:
        return "-"
    baseline_f = float(baseline)
    candidate_f = float(candidate)
    if lower_is_better:
        value = (baseline_f - candidate_f) / baseline_f * 100.0
    else:
        value = (candidate_f - baseline_f) / baseline_f * 100.0
    return f"{value:.2f}%"


def get_nested(data: dict, *keys):
    current = data
    for key in keys:
        if not isinstance(current, dict):
            return None
        current = current.get(key)
    return current


def build_metric_row(name: str, baseline, candidate, lower_is_better: bool) -> str:
    return (
        f"| {name} | {fmt_num(baseline)} | {fmt_num(candidate)} | "
        f"{pct_improve(baseline, candidate, lower_is_better)} |"
    )


def build_report(result: dict) -> str:
    lines: list[str] = []
    lines.append("# 批量持久化性能对比报告")
    lines.append("")
    lines.append("## 实验范围")
    lines.append("")
    lines.append(f"- 标签：`{result.get('label')}`")
    lines.append(f"- 生成时间：`{result.get('generated_at')}`")
    lines.append(f"- 基线组：`{result.get('baseline_name')}`")
    lines.append(f"- 实验组：`{result.get('candidate_name')}`")
    lines.append("")
    lines.append("## 结论摘要")
    lines.append("")

    summary = result.get("overall_summary", {})
    for line in summary.get("highlights", []):
        lines.append(f"- {line}")

    lines.append("")
    lines.append("## 分挡位对比")
    lines.append("")
    for stage in result.get("stages", []):
        stage_name = stage.get("stage_name")
        base = stage.get("baseline", {})
        cand = stage.get("candidate", {})
        lines.append(f"### {stage_name}")
        lines.append("")
        lines.append("| 指标 | 单条写库 | 批量写库 | 提升 |")
        lines.append("| --- | --- | --- | --- |")
        lines.append(build_metric_row("客户端吞吐 msg/s", get_nested(base, "summary", "observed_throughput_msg_per_sec"), get_nested(cand, "summary", "observed_throughput_msg_per_sec"), False))
        lines.append(build_metric_row("端到端 avg ms", get_nested(base, "summary", "latency", "avg_ms"), get_nested(cand, "summary", "latency", "avg_ms"), True))
        lines.append(build_metric_row("端到端 p95 ms", get_nested(base, "summary", "latency", "p95_ms"), get_nested(cand, "summary", "latency", "p95_ms"), True))
        lines.append(build_metric_row("端到端 p99 ms", get_nested(base, "summary", "latency", "p99_ms"), get_nested(cand, "summary", "latency", "p99_ms"), True))
        lines.append(build_metric_row("server_critical_path avg ms", get_nested(base, "critical", "stage_metrics", "server_critical_path_ms", "avg_ms"), get_nested(cand, "critical", "stage_metrics", "server_critical_path_ms", "avg_ms"), True))
        lines.append(build_metric_row("mysql_persist avg ms", get_nested(base, "critical", "stage_metrics", "mysql_persist_ms", "avg_ms"), get_nested(cand, "critical", "stage_metrics", "mysql_persist_ms", "avg_ms"), True))
        lines.append(build_metric_row("flush 总次数", get_nested(base, "critical", "persist", "flush_count"), get_nested(cand, "critical", "persist", "flush_count"), True))
        lines.append(build_metric_row("平均 flush batch", get_nested(base, "critical", "persist", "avg_flush_batch_size"), get_nested(cand, "critical", "persist", "avg_flush_batch_size"), False))
        lines.append(build_metric_row("平均 enqueue 队列深度", get_nested(base, "critical", "persist", "avg_enqueue_queue_depth"), get_nested(cand, "critical", "persist", "avg_enqueue_queue_depth"), True))
        lines.append(build_metric_row("成功率", get_nested(base, "summary", "delivery_success_rate"), get_nested(cand, "summary", "delivery_success_rate"), False))
        lines.append("")
        lines.append(f"- 单条写库 flush reason：`{json.dumps(get_nested(base, 'critical', 'persist', 'flush_reason_counts') or {}, ensure_ascii=False)}`")
        lines.append(f"- 批量写库 flush reason：`{json.dumps(get_nested(cand, 'critical', 'persist', 'flush_reason_counts') or {}, ensure_ascii=False)}`")
        lines.append("")

    lines.append("## 指标解释")
    lines.append("")
    lines.append("- 吞吐更高，说明同样业务链路下系统可持续处理的消息速率更高。")
    lines.append("- `mysql_persist avg ms` 更低，说明落库阶段自身开销更小。")
    lines.append("- `flush_count` 更低且 `avg_flush_batch_size` 更高，说明批量聚合生效，单次 SQL 携带消息更多。")
    lines.append("- `avg_enqueue_queue_depth` 更低，说明 MySQL 持久化出口不容易形成积压。")
    lines.append("- 端到端和 `server_critical_path` 尾延迟下降，说明收益不只是数据库局部优化，而是对整体消息链路有正反馈。")
    lines.append("")
    return "\n".join(lines) + "\n"


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()

    input_path = Path(args.input).resolve()
    output_path = Path(args.output).resolve()
    result = load_json(input_path)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(build_report(result), encoding="utf-8")
    print(output_path)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
