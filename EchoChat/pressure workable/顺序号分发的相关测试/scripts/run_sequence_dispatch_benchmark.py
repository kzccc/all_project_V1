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


def build_runner_config(root_cfg: dict, mode: str, attempt: int) -> dict:
    shared = root_cfg["shared"]
    benchmark = root_cfg["benchmark"]
    template = read_toml(ROOT / str(root_cfg["run"]["base_pressure_config"]))
    cfg = copy.deepcopy(template)

    label = f"{dt.datetime.now().strftime('%Y%m%d_%H%M%S')}_{root_cfg['run']['label']}_{mode}_attempt{attempt}"
    seed_prefix_map = {
        "cold_start": f"SEQC{attempt}",
        "hot_path": f"SEQH{attempt}",
        "redis_floor_recovery": f"SEQR{attempt}",
    }
    cfg.setdefault("run", {})
    cfg["run"]["label_mode"] = "manual"
    cfg["run"]["label"] = label
    cfg["run"]["record_root"] = str(root_cfg["run"]["record_root"])
    cfg["run"]["base_config"] = str(shared["base_config"])
    cfg["run"]["seed_prefix"] = seed_prefix_map[mode]
    cfg["run"]["mysql_runtime_root"] = str(shared["mysql_runtime_root"])
    cfg["run"]["database"] = str(shared["database"])

    cfg.setdefault("mainconfig", {})
    cfg["mainconfig"]["target_rate"] = int(shared["target_rate"])
    cfg["mainconfig"]["session_count"] = int(shared["session_count"])
    cfg["mainconfig"]["topic_partitions"] = int(shared["topic_partitions"])
    cfg["mainconfig"]["conversation_bucket_worker_count"] = int(shared["conversation_bucket_worker_count"])
    cfg["mainconfig"]["conversation_bucket_ready_queue_size"] = int(shared["conversation_bucket_ready_queue_size"])
    cfg["mainconfig"]["conversation_bucket_bucket_queue_size"] = int(shared["conversation_bucket_bucket_queue_size"])
    cfg["mainconfig"]["conversation_bucket_max_messages_per_turn"] = int(shared["conversation_bucket_max_messages_per_turn"])
    cfg["mainconfig"]["conversation_bucket_max_run_duration_ms"] = int(shared["conversation_bucket_max_run_duration_ms"])
    cfg["mainconfig"]["mysql_persist_worker_count"] = int(shared["mysql_persist_worker_count"])
    cfg["mainconfig"]["mysql_persist_batch_size"] = int(shared["mysql_persist_batch_size"])

    cfg.setdefault("scenario", {})
    cfg["scenario"]["session_count"] = int(shared["session_count"])
    cfg["scenario"]["fixture_pair_count"] = int(shared["fixture_pair_count"])
    cfg["scenario"]["user_count"] = int(shared["user_count"])
    cfg["scenario"]["target_rate"] = int(shared["target_rate"])
    cfg["scenario"]["duration_sec"] = int(shared["duration_sec"])
    cfg["scenario"]["partition_selection_mode"] = "balanced"
    cfg["scenario"]["session_partition_balance_mode"] = "on"
    cfg["scenario"]["worker_balance_mode"] = "off"

    cfg.setdefault("consumers", {})
    cfg["consumers"]["count"] = int(shared["consumer_count"])
    cfg["consumers"]["base_port"] = int(shared["consumer_base_port"])
    cfg["consumers"]["ports"] = str(shared["consumer_base_port"])
    cfg["consumers"]["client_ports"] = str(shared["consumer_base_port"])

    cfg.setdefault("kafka", {})
    cfg["kafka"]["host_port"] = str(shared["kafka_host_port"])
    cfg["kafka"]["topic_partitions"] = int(shared["topic_partitions"])
    cfg["kafka"]["chat_topic_prefix"] = f"chat_message_sequence_dispatch_{mode}"
    cfg["kafka"]["unique_topic_per_run"] = True

    cfg.setdefault("mysql", {})
    cfg["mysql"]["host"] = str(shared["mysql_host"])
    cfg["mysql"]["port"] = int(shared["mysql_port"])
    cfg["mysql"]["database_name"] = str(shared["database"])

    cfg.setdefault("redis", {})
    cfg["redis"]["host"] = str(shared["redis_host"])
    cfg["redis"]["port"] = int(shared["redis_port"])

    cfg.setdefault("persist", {})
    cfg["persist"]["batch_size"] = int(shared["mysql_persist_batch_size"])
    cfg["persist"]["first_job_hold_ms"] = 0.5
    cfg["persist"]["flush_interval_ms"] = 7
    cfg["persist"]["worker_count"] = int(shared["mysql_persist_worker_count"])
    cfg["persist"]["queue_size"] = int(shared["mysql_persist_queue_size"])

    cfg.setdefault("conversation_bucket", {})
    cfg["conversation_bucket"]["enabled"] = bool(shared["conversation_bucket_enabled"])
    cfg["conversation_bucket"]["worker_count"] = int(shared["conversation_bucket_worker_count"])
    cfg["conversation_bucket"]["ready_queue_size"] = int(shared["conversation_bucket_ready_queue_size"])
    cfg["conversation_bucket"]["bucket_queue_size"] = int(shared["conversation_bucket_bucket_queue_size"])
    cfg["conversation_bucket"]["max_messages_per_turn"] = int(shared["conversation_bucket_max_messages_per_turn"])
    cfg["conversation_bucket"]["max_run_duration_ms"] = int(shared["conversation_bucket_max_run_duration_ms"])
    cfg["conversation_bucket"]["drain_timeout_ms"] = 3000

    cfg.setdefault("benchmark", {})
    cfg["benchmark"]["message_timeout_ms"] = int(benchmark["message_timeout_ms"])
    cfg["benchmark"]["setup_workers"] = int(benchmark["setup_workers"])
    cfg["benchmark"]["setup_http_timeout_ms"] = int(benchmark["setup_http_timeout_ms"])
    cfg["benchmark"]["connection_settle_ms"] = int(benchmark["connection_settle_ms"])
    cfg["benchmark"]["drain_wait_ms"] = int(benchmark["drain_wait_ms"])
    cfg["benchmark"]["drain_idle_ms"] = int(benchmark["drain_idle_ms"])
    cfg["benchmark"]["post_run_settle_ms"] = int(benchmark["post_run_settle_ms"])
    cfg["benchmark"]["ws_path"] = str(benchmark["ws_path"])
    cfg["benchmark"]["seed_reset_prefix"] = True

    if mode == "cold_start":
        cfg["benchmark"]["session_seq_warmup_enabled"] = False
        cfg["benchmark"]["session_seq_reset_state_before_run"] = True
        cfg["benchmark"]["session_seq_force_flush_after_warmup"] = False
        cfg["benchmark"]["session_seq_prepare_recovery_after_warmup"] = False
    elif mode == "hot_path":
        cfg["benchmark"]["session_seq_warmup_enabled"] = True
        cfg["benchmark"]["session_seq_warmup_interval_ms"] = 10
        cfg["benchmark"]["session_seq_warmup_timeout_ms"] = 15000
        cfg["benchmark"]["session_seq_warmup_settle_ms"] = 1000
        cfg["benchmark"]["session_seq_reset_state_before_run"] = True
        cfg["benchmark"]["session_seq_force_flush_after_warmup"] = True
        cfg["benchmark"]["session_seq_prepare_recovery_after_warmup"] = False
    elif mode == "redis_floor_recovery":
        cfg["benchmark"]["session_seq_warmup_enabled"] = True
        cfg["benchmark"]["session_seq_warmup_interval_ms"] = 10
        cfg["benchmark"]["session_seq_warmup_timeout_ms"] = 15000
        cfg["benchmark"]["session_seq_warmup_settle_ms"] = 1000
        cfg["benchmark"]["session_seq_reset_state_before_run"] = True
        cfg["benchmark"]["session_seq_force_flush_after_warmup"] = True
        cfg["benchmark"]["session_seq_prepare_recovery_after_warmup"] = True
        cfg["benchmark"]["session_seq_recovery_delete_pair_count"] = int(shared["session_count"])
    else:
        raise ValueError(f"unsupported mode: {mode}")

    cfg.setdefault("cleanup", {})
    cfg["cleanup"]["delete_mysql_runtime_dir"] = bool(shared["delete_mysql_runtime_dir"])
    return cfg


def run_runner(runner_script: Path, config_path: Path) -> Path:
    subprocess.run([sys.executable, str(runner_script), "--config", str(config_path)], cwd=str(ROOT), check=True)
    cfg = read_toml(config_path)
    record_root = ROOT / str(cfg["run"]["record_root"])
    today_dir = dt.datetime.now().strftime("%-m.%-d")
    bundle_dir = record_root / today_dir / str(cfg["run"]["label"])
    if not bundle_dir.exists():
        raise RuntimeError(f"bundle dir not found: {bundle_dir}")
    return bundle_dir


def analyze_bundle(work_root: Path, bundle_dir: Path, root_cfg: dict) -> dict:
    raw_dir = bundle_dir / "raw_runner"
    output_path = bundle_dir / "session_seq_analysis.json"
    subprocess.run(
        [
            sys.executable,
            str(work_root / "scripts" / "analyze_sequence_dispatch.py"),
            "--trace",
            str(raw_dir / "trace.json"),
            "--output",
            str(output_path),
            "--outlier-sigma",
            str(root_cfg["analysis"]["outlier_sigma"]),
            "--min-expected-hot-count",
            str(root_cfg["analysis"]["min_expected_hot_count"]),
            "--min-expected-recovery-count",
            str(root_cfg["analysis"]["min_expected_recovery_count"]),
        ],
        cwd=str(ROOT),
        check=True,
    )
    return json.loads(output_path.read_text(encoding="utf-8"))


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


def merge_results(stage_results: dict[str, dict]) -> dict:
    mode_summary = {}
    total_samples = 0
    raw_mode_counts = {}
    top_outliers = {}
    stage_reasonableness = stage_results.get("_stage_reasonableness", {})

    for mode, item in stage_results.items():
        if mode.startswith("_"):
            continue
        analysis = item["analysis"]
        mode_data = analysis.get("mode_summary", {}).get(mode, {})
        mode_summary[mode] = mode_data
        total_samples += int(mode_data.get("count") or 0)
        raw_mode_counts[mode] = int(mode_data.get("count") or 0)
        top_outliers[mode] = analysis.get("top_outliers", {}).get(mode, [])

    return {
        "total_session_seq_samples": total_samples,
        "mode_summary": mode_summary,
        "mode_order": ["cold_start", "hot_path", "redis_floor_recovery"],
        "reasonableness": {
            "unreasonable": bool(stage_reasonableness.get("unreasonable", False)),
            "findings": stage_reasonableness.get("findings", ["三类顺序号分发耗时关系基本符合预期"]),
        },
        "top_outliers": top_outliers,
        "raw_mode_counts": raw_mode_counts,
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True)
    args = parser.parse_args()

    config_path = Path(args.config).resolve()
    root_cfg = read_toml(config_path)
    work_root = config_path.parent.parent
    generated_dir = work_root / "record" / "generated_configs"
    generated_dir.mkdir(parents=True, exist_ok=True)
    report_root = ROOT / str(root_cfg["run"]["report_root"])
    report_root.mkdir(parents=True, exist_ok=True)

    runner_script = ROOT / str(root_cfg["run"]["base_runner_script"])
    max_retry_rounds = int(root_cfg["analysis"]["max_retry_rounds"])
    final_summary: dict | None = None

    for attempt in range(1, max_retry_rounds + 2):
        stage_results: dict[str, dict] = {}
        for mode in ["cold_start", "hot_path", "redis_floor_recovery"]:
            runner_cfg = build_runner_config(root_cfg, mode, attempt)
            cfg_path = generated_dir / f"{mode}_attempt{attempt}.toml"
            write_toml(cfg_path, runner_cfg)
            print(f"[run] mode={mode} attempt={attempt} config={cfg_path}", flush=True)
            bundle_dir = run_runner(runner_script, cfg_path)
            analysis = analyze_bundle(work_root, bundle_dir, root_cfg)
            stage_results[mode] = {
                "bundle_dir": str(bundle_dir),
                "analysis": analysis,
            }

        stage_results["_stage_reasonableness"] = evaluate_stage_results(stage_results, root_cfg)
        merged = merge_results(stage_results)
        final_summary = {
            "label": root_cfg["run"]["label"],
            "generated_at": dt.datetime.now().isoformat(timespec="seconds"),
            "attempt": attempt,
            "scenarios": stage_results,
            "analysis": merged,
        }

        if not merged.get("reasonableness", {}).get("unreasonable", False):
            break

    if final_summary is None:
        raise RuntimeError("no sequence dispatch summary produced")

    summary_json = report_root / "sequence_dispatch_summary.json"
    write_text(summary_json, json.dumps(final_summary, ensure_ascii=False, indent=2))
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
