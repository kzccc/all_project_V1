#!/usr/bin/env python3
from __future__ import annotations

import argparse
import datetime as dt
import json
import subprocess
import sys
from pathlib import Path
import tomllib


ROOT = Path(__file__).resolve().parents[3]


def read_toml(path: Path) -> dict:
    with path.open("rb") as fp:
        return tomllib.load(fp)


def load_json(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def write_text(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")


def evaluate_stage_results(stage_results: dict[str, dict], root_cfg: dict) -> dict:
    findings: list[str] = []
    unreasonable = False

    cold = stage_results["cold_start"]["analysis"]["mode_summary"].get("cold_start", {})
    hot = stage_results["hot_path"]["analysis"]["mode_summary"].get("hot_path", {})
    recovery = stage_results["redis_floor_recovery"]["analysis"]["mode_summary"].get("redis_floor_recovery", {})

    cold_count = int(cold.get("count") or 0)
    hot_count = int(hot.get("count") or 0)
    recovery_count = int(recovery.get("count") or 0)
    cold_avg = cold.get("avg_ms")
    hot_avg = hot.get("avg_ms")
    recovery_avg = recovery.get("avg_ms")

    if cold_count <= 0:
        unreasonable = True
        findings.append("cold_start 场景未采到 cold_start 样本")
    if hot_count < int(root_cfg["analysis"]["min_expected_hot_count"]):
        unreasonable = True
        findings.append(f"hot_path 场景样本数偏少，仅 {hot_count}")
    if recovery_count < int(root_cfg["analysis"]["min_expected_recovery_count"]):
        unreasonable = True
        findings.append(f"redis_floor_recovery 场景样本数偏少，仅 {recovery_count}")
    if cold_avg is not None and hot_avg is not None and hot_avg > cold_avg * 1.25:
        unreasonable = True
        findings.append("hot_path 平均耗时显著高于 cold_start，不符合常识")
    if recovery_avg is not None and hot_avg is not None and recovery_avg < hot_avg * 0.9:
        unreasonable = True
        findings.append("redis_floor_recovery 平均耗时显著低于 hot_path，不符合常识")

    if not findings:
        findings.append("三类顺序号分发耗时关系基本符合预期")
    return {"unreasonable": unreasonable, "findings": findings}


def merge_results(stage_results: dict[str, dict], reasonableness: dict) -> dict:
    mode_summary = {}
    total_samples = 0
    raw_mode_counts = {}
    top_outliers = {}
    for mode in ["cold_start", "hot_path", "redis_floor_recovery"]:
        analysis = stage_results[mode]["analysis"]
        mode_data = analysis.get("mode_summary", {}).get(mode, {})
        mode_summary[mode] = mode_data
        total_samples += int(mode_data.get("count") or 0)
        raw_mode_counts[mode] = int(mode_data.get("count") or 0)
        top_outliers[mode] = analysis.get("top_outliers", {}).get(mode, [])
    return {
        "total_session_seq_samples": total_samples,
        "mode_summary": mode_summary,
        "mode_order": ["cold_start", "hot_path", "redis_floor_recovery"],
        "reasonableness": reasonableness,
        "top_outliers": top_outliers,
        "raw_mode_counts": raw_mode_counts,
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True)
    parser.add_argument("--cold", required=True)
    parser.add_argument("--hot", required=True)
    parser.add_argument("--recovery", required=True)
    args = parser.parse_args()

    config_path = Path(args.config).resolve()
    root_cfg = read_toml(config_path)
    work_root = config_path.parent.parent
    report_root = ROOT / str(root_cfg["run"]["report_root"])
    report_root.mkdir(parents=True, exist_ok=True)

    stage_results = {
        "cold_start": {"analysis": load_json(Path(args.cold).resolve())},
        "hot_path": {"analysis": load_json(Path(args.hot).resolve())},
        "redis_floor_recovery": {"analysis": load_json(Path(args.recovery).resolve())},
    }
    reasonableness = evaluate_stage_results(stage_results, root_cfg)
    summary = {
        "label": root_cfg["run"]["label"],
        "generated_at": dt.datetime.now().isoformat(timespec="seconds"),
        "attempt": 1,
        "scenarios": stage_results,
        "analysis": merge_results(stage_results, reasonableness),
    }

    summary_json = report_root / "sequence_dispatch_summary.json"
    write_text(summary_json, json.dumps(summary, ensure_ascii=False, indent=2))
    report_path = report_root / "01_顺序号分发测试报告.md"
    subprocess.run(
        [
            sys.executable,
            str(work_root / "scripts" / "build_sequence_dispatch_report.py"),
            "--input",
            str(summary_json),
            "--output",
            str(report_path),
        ],
        cwd=str(ROOT),
        check=True,
    )
    print(report_path)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
