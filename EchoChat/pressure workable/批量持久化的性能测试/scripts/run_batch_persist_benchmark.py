#!/usr/bin/env python3
from __future__ import annotations

import argparse
import copy
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


def write_text(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")


def toml_literal(value) -> str:
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, int):
        return str(value)
    if isinstance(value, float):
        return str(value)
    if value is None:
        return '""'
    return json.dumps(str(value), ensure_ascii=False)


def write_toml(path: Path, data: dict) -> None:
    lines: list[str] = []
    for section, values in data.items():
        if not isinstance(values, dict):
            continue
        lines.append(f"[{section}]")
        for key, value in values.items():
            lines.append(f"{key} = {toml_literal(value)}")
        lines.append("")
    write_text(path, "\n".join(lines).rstrip() + "\n")


def load_json(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def get_nested(data: dict, *keys):
    current = data
    for key in keys:
        if not isinstance(current, dict):
            return None
        current = current.get(key)
    return current


def build_experiment_notes(cfg: dict) -> str:
    lines = [
        "# 测试设计",
        "",
        "## 核心问题",
        "",
        "验证 Kafka 主链路中 `mysql_persist` 批量写库，相比单条写库到底带来多少真实收益。",
        "",
        "## 设计原则",
        "",
        "1. 只改 `mysql_persist` 的写库聚合参数，不改 Kafka、Redis、conversation bucket 主体结构。",
        "2. 每个挡位都跑两次：`single_insert` 和 `batched`。",
        "3. 两组共用同一份压测模板，只覆盖少量差异项，保证结论可归因。",
        "",
        "## 评价指标",
        "",
        "1. 吞吐：真实客户端接收吞吐 `observed_throughput_msg_per_sec`。",
        "2. 用户体验：端到端 `avg/p95/p99`。",
        "3. 服务端链路：`server_critical_path_ms`。",
        "4. 数据库链路：`mysql_persist_ms`。",
        "5. 批量化效果：`flush_count`、`avg_flush_batch_size`、`flush_reason_counts`。",
        "6. 背压程度：`avg_enqueue_queue_depth`。",
        "7. 稳定性：成功率、拖尾补完量。",
        "",
        "## 基线与实验组",
        "",
        f"- 基线：`{cfg['baseline']['name']}`，batch=1，模拟逐条刷库。",
        f"- 实验组：`{cfg['candidate']['name']}`，沿用当前批量写库口径。",
        "",
    ]
    return "\n".join(lines) + "\n"


def build_runner_config(template: dict, root_cfg: dict, mode_cfg: dict, stage_cfg: dict, run_root: Path) -> dict:
    cfg = copy.deepcopy(template)
    shared = root_cfg["shared"]
    run_cfg = cfg.setdefault("run", {})
    main_cfg = cfg.setdefault("mainconfig", {})
    scenario_cfg = cfg.setdefault("scenario", {})
    kafka_cfg = cfg.setdefault("kafka", {})
    mysql_cfg = cfg.setdefault("mysql", {})
    redis_cfg = cfg.setdefault("redis", {})
    persist_cfg = cfg.setdefault("persist", {})
    conversation_cfg = cfg.setdefault("conversation_bucket", {})
    consumers_cfg = cfg.setdefault("consumers", {})
    cleanup_cfg = cfg.setdefault("cleanup", {})

    stage_name = str(stage_cfg["name"])
    label = root_cfg["run"]["label"]
    mode_name = str(mode_cfg["name"])

    run_cfg["label_mode"] = "manual"
    run_cfg["label"] = f"{dt.datetime.now().strftime('%Y%m%d_%H%M%S')}_{label}_{stage_name}_{mode_name}"
    run_cfg["record_root"] = str(root_cfg["run"]["record_root"])
    run_cfg["base_config"] = str(shared["base_config"])
    run_cfg["seed_prefix"] = str(shared["seed_prefix"])
    run_cfg["mysql_runtime_root"] = str(shared["mysql_runtime_root"])
    run_cfg["database"] = str(shared["database"])

    main_cfg["target_rate"] = int(stage_cfg.get("target_rate", shared["target_rate"]))
    main_cfg["session_count"] = int(shared["session_count"])
    main_cfg["topic_partitions"] = int(shared["topic_partitions"])
    main_cfg["conversation_bucket_worker_count"] = int(shared["conversation_bucket_worker_count"])
    main_cfg["conversation_bucket_ready_queue_size"] = int(shared["conversation_bucket_ready_queue_size"])
    main_cfg["conversation_bucket_bucket_queue_size"] = int(shared["conversation_bucket_bucket_queue_size"])
    main_cfg["conversation_bucket_max_messages_per_turn"] = int(shared["conversation_bucket_max_messages_per_turn"])
    main_cfg["conversation_bucket_max_run_duration_ms"] = int(shared["conversation_bucket_max_run_duration_ms"])
    main_cfg["mysql_persist_worker_count"] = int(mode_cfg["mysql_persist_worker_count"])
    main_cfg["mysql_persist_batch_size"] = int(mode_cfg["mysql_persist_batch_size"])

    scenario_cfg["session_count"] = int(shared["session_count"])
    scenario_cfg["fixture_pair_count"] = int(shared["fixture_pair_count"])
    scenario_cfg["user_count"] = int(shared["user_count"])
    scenario_cfg["target_rate"] = int(stage_cfg.get("target_rate", shared["target_rate"]))
    scenario_cfg["duration_sec"] = int(stage_cfg.get("duration_sec", shared["duration_sec"]))
    scenario_cfg["partition_selection_mode"] = "balanced"
    scenario_cfg["session_partition_balance_mode"] = "on"
    scenario_cfg["worker_balance_mode"] = "off"

    kafka_cfg["host_port"] = str(shared["kafka_host_port"])
    kafka_cfg["topic_partitions"] = int(shared["topic_partitions"])
    kafka_cfg["chat_topic_prefix"] = f"chat_message_batch_persist_{stage_name}_{mode_name}"
    kafka_cfg["unique_topic_per_run"] = True

    consumers_cfg["count"] = int(shared["consumer_count"])
    consumers_cfg["base_port"] = int(shared["consumer_base_port"])
    consumers_cfg["ports"] = str(shared["consumer_base_port"])
    consumers_cfg["client_ports"] = str(shared["consumer_base_port"])

    mysql_cfg["host"] = str(shared["mysql_host"])
    mysql_cfg["port"] = int(shared["mysql_port"])
    mysql_cfg["database_name"] = str(shared["database"])
    redis_cfg["host"] = str(shared["redis_host"])
    redis_cfg["port"] = int(shared["redis_port"])

    persist_cfg["batch_size"] = int(mode_cfg["mysql_persist_batch_size"])
    persist_cfg["first_job_hold_ms"] = float(mode_cfg["mysql_persist_first_job_hold_ms"])
    persist_cfg["flush_interval_ms"] = int(mode_cfg["mysql_persist_flush_interval_ms"])
    persist_cfg["worker_count"] = int(mode_cfg["mysql_persist_worker_count"])
    persist_cfg["queue_size"] = int(shared["mysql_persist_queue_size"])
    persist_cfg["mysql_persist_noop_experimental"] = False

    conversation_cfg["enabled"] = bool(shared["conversation_bucket_enabled"])
    conversation_cfg["worker_count"] = int(shared["conversation_bucket_worker_count"])
    conversation_cfg["ready_queue_size"] = int(shared["conversation_bucket_ready_queue_size"])
    conversation_cfg["bucket_queue_size"] = int(shared["conversation_bucket_bucket_queue_size"])
    conversation_cfg["max_messages_per_turn"] = int(shared["conversation_bucket_max_messages_per_turn"])
    conversation_cfg["max_run_duration_ms"] = int(shared["conversation_bucket_max_run_duration_ms"])
    conversation_cfg["drain_timeout_ms"] = 3000

    cleanup_cfg["delete_mysql_runtime_dir"] = bool(shared["delete_mysql_runtime_dir"])

    run_root.mkdir(parents=True, exist_ok=True)
    return cfg


def run_runner(runner_script: Path, config_path: Path) -> Path:
    cmd = [sys.executable, str(runner_script), "--config", str(config_path)]
    subprocess.run(cmd, cwd=str(ROOT), check=True)
    cfg = read_toml(config_path)
    record_root = ROOT / str(cfg["run"]["record_root"])
    label = str(cfg["run"]["label"])
    today_dir = dt.datetime.now().strftime("%-m.%-d")
    bundle_dir = record_root / today_dir / label
    if not bundle_dir.exists():
        raise RuntimeError(f"bundle dir not found: {bundle_dir}")
    return bundle_dir


def collect_bundle(bundle_dir: Path) -> dict:
    raw_dir = bundle_dir / "raw_runner"
    return {
        "bundle_dir": str(bundle_dir),
        "summary": load_json(raw_dir / "summary.json"),
        "critical": load_json(raw_dir / "critical_path_summary.json"),
        "dashboard": load_json(bundle_dir / "dashboard_summary.json") if (bundle_dir / "dashboard_summary.json").exists() else {},
    }


def build_highlights(stages: list[dict]) -> list[str]:
    highlights: list[str] = []
    if not stages:
        return highlights
    best_throughput = []
    best_persist = []
    best_tail = []
    for stage in stages:
        base_t = get_nested(stage, "baseline", "summary", "observed_throughput_msg_per_sec")
        cand_t = get_nested(stage, "candidate", "summary", "observed_throughput_msg_per_sec")
        base_p = get_nested(stage, "baseline", "critical", "stage_metrics", "mysql_persist_ms", "avg_ms")
        cand_p = get_nested(stage, "candidate", "critical", "stage_metrics", "mysql_persist_ms", "avg_ms")
        base_p99 = get_nested(stage, "baseline", "summary", "latency", "p99_ms")
        cand_p99 = get_nested(stage, "candidate", "summary", "latency", "p99_ms")
        if all(v is not None for v in [base_t, cand_t]) and float(base_t) > 0:
            best_throughput.append((float(cand_t) - float(base_t)) / float(base_t) * 100.0)
        if all(v is not None for v in [base_p, cand_p]) and float(base_p) > 0:
            best_persist.append((float(base_p) - float(cand_p)) / float(base_p) * 100.0)
        if all(v is not None for v in [base_p99, cand_p99]) and float(base_p99) > 0:
            best_tail.append((float(base_p99) - float(cand_p99)) / float(base_p99) * 100.0)
    if best_throughput:
        highlights.append(f"批量写库相对单条写库，吞吐提升区间约为 `{min(best_throughput):.2f}% ~ {max(best_throughput):.2f}%`。")
    if best_persist:
        highlights.append(f"`mysql_persist` 平均阶段耗时下降区间约为 `{min(best_persist):.2f}% ~ {max(best_persist):.2f}%`。")
    if best_tail:
        highlights.append(f"端到端 `p99` 延迟改善区间约为 `{min(best_tail):.2f}% ~ {max(best_tail):.2f}%`。")
    return highlights


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True)
    args = parser.parse_args()

    config_path = Path(args.config).resolve()
    root_cfg = read_toml(config_path)

    runner_script = ROOT / str(root_cfg["run"]["runner_script"])
    base_pressure_config = ROOT / str(root_cfg["run"]["base_pressure_config"])
    template = read_toml(base_pressure_config)

    work_root = config_path.parent.parent
    generated_dir = work_root / "record" / "generated_configs"
    generated_dir.mkdir(parents=True, exist_ok=True)

    report_root = ROOT / str(root_cfg["run"]["report_root"])
    report_root.mkdir(parents=True, exist_ok=True)
    write_text(report_root / "00_测试设计.md", build_experiment_notes(root_cfg))

    stages_result: list[dict] = []
    run_manifest = {
        "label": root_cfg["run"]["label"],
        "generated_at": dt.datetime.now().isoformat(timespec="seconds"),
        "baseline_name": root_cfg["baseline"]["name"],
        "candidate_name": root_cfg["candidate"]["name"],
        "stages": [],
    }

    for stage_cfg in root_cfg.get("stages", []):
        stage_name = str(stage_cfg["name"])
        stage_result = {"stage_name": stage_name}
        for mode_key in ["baseline", "candidate"]:
            mode_cfg = root_cfg[mode_key]
            runner_cfg = build_runner_config(template, root_cfg, mode_cfg, stage_cfg, generated_dir)
            runner_cfg_path = generated_dir / f"{stage_name}_{mode_cfg['name']}.toml"
            write_toml(runner_cfg_path, runner_cfg)
            print(f"[run] stage={stage_name} mode={mode_cfg['name']} config={runner_cfg_path}", flush=True)
            bundle_dir = run_runner(runner_script, runner_cfg_path)
            stage_result[mode_key] = collect_bundle(bundle_dir)
        stages_result.append(stage_result)
        run_manifest["stages"].append(stage_result)

    run_manifest["overall_summary"] = {
        "highlights": build_highlights(stages_result),
    }

    summary_json = report_root / "batch_persist_comparison_summary.json"
    write_text(summary_json, json.dumps(run_manifest, ensure_ascii=False, indent=2))

    report_script = work_root / "scripts" / "build_batch_persist_report.py"
    final_report = report_root / "01_批量持久化性能对比报告.md"
    subprocess.run(
        [sys.executable, str(report_script), "--input", str(summary_json), "--output", str(final_report)],
        cwd=str(ROOT),
        check=True,
    )
    print(final_report)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
