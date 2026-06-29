#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
from pathlib import Path


def load_json(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def fmt(value) -> str:
    if value is None:
        return "-"
    if isinstance(value, float):
        return f"{value:.3f}"
    return str(value)


def build_report(summary: dict) -> str:
    analysis = summary.get("analysis", {})
    mode_summary = analysis.get("mode_summary", {})
    reasonableness = analysis.get("reasonableness", {})
    lines: list[str] = []
    lines.append("# 顺序号分发测试报告")
    lines.append("")
    lines.append("## 口径")
    lines.append("")
    lines.append(f"- 标签：`{summary.get('label')}`")
    lines.append(f"- 生成时间：`{summary.get('generated_at')}`")
    lines.append(f"- 测试轮次：`{summary.get('attempt')}`")
    lines.append(f"- 总样本数：`{analysis.get('total_session_seq_samples')}`")
    lines.append("")
    lines.append("## 平均值")
    lines.append("")
    lines.append("| 指标 | count | avg_ms | p50_ms | p95_ms | p99_ms | max_ms |")
    lines.append("| --- | --- | --- | --- | --- | --- | --- |")
    for mode in ["cold_start", "hot_path", "redis_floor_recovery"]:
        item = mode_summary.get(mode, {})
        lines.append(
            f"| {mode} | {fmt(item.get('count'))} | {fmt(item.get('avg_ms'))} | {fmt(item.get('p50_ms'))} | "
            f"{fmt(item.get('p95_ms'))} | {fmt(item.get('p99_ms'))} | {fmt(item.get('max_ms'))} |"
        )
    lines.append("")
    lines.append("## 合理性分析")
    lines.append("")
    for finding in reasonableness.get("findings", []):
        lines.append(f"- {finding}")
    lines.append(f"- 是否判定异常：`{reasonableness.get('unreasonable')}`")
    lines.append("")
    lines.append("## 异常样本")
    lines.append("")
    top_outliers = analysis.get("top_outliers", {})
    for mode in ["cold_start", "hot_path", "redis_floor_recovery"]:
        rows = top_outliers.get(mode, [])
        lines.append(f"### {mode}")
        lines.append("")
        if not rows:
            lines.append("- 本轮无显著异常样本")
            lines.append("")
            continue
        for row in rows[:10]:
            lines.append(
                f"- bench_id=`{row.get('bench_id')}` duration=`{fmt(row.get('duration_ms'))}ms` "
                f"partition=`{row.get('partition')}` offset=`{row.get('offset')}`"
            )
        lines.append("")
    lines.append("## 总结")
    lines.append("")
    if reasonableness.get("unreasonable"):
        lines.append("- 本轮结果存在不合理信号，已在原始样本中给出异常点，需继续迭代框架或重跑。")
    else:
        lines.append("- 本轮三类顺序号分发耗时关系基本稳定，可作为当前口径下的参考结果。")
    return "\n".join(lines) + "\n"


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()

    result = load_json(Path(args.input).resolve())
    output_path = Path(args.output).resolve()
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(build_report(result), encoding="utf-8")
    print(output_path)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
