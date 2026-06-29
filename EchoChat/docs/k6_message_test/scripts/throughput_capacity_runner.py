#!/usr/bin/env python3

from __future__ import annotations

import argparse
import csv
import json
import math
import os
import shutil
import subprocess
import sys
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import requests
import tomllib


def detect_repo_root() -> Path:
    env_root = os.environ.get("ECHOCHAT_REPO_ROOT")
    if env_root:
        return Path(env_root).expanduser().resolve()
    current = Path(__file__).resolve()
    for parent in [current.parent, *current.parents]:
        if (parent / "go.mod").exists() and (parent / "configs").exists():
            return parent
    return current.parents[3]


ROOT_DIR = detect_repo_root()
TEST_DIR = ROOT_DIR / "docs" / "k6_message_test"
SCRIPT_DIR = TEST_DIR / "scripts"
RECORD_ROOT = Path(os.environ.get("ECHOCHAT_RECORD_ROOT_OVERRIDE", str(TEST_DIR / "records")))


def run_command(command: list[str], *, env: dict[str, str] | None = None, cwd: Path = ROOT_DIR) -> subprocess.CompletedProcess:
    return subprocess.run(
        command,
        cwd=str(cwd),
        env=env,
        check=True,
        text=True,
        capture_output=True,
    )


def wait_for_server(base_url: str, process: subprocess.Popen[str], timeout_sec: int = 60) -> None:
    deadline = time.time() + timeout_sec
    while time.time() < deadline:
        if process.poll() is not None:
            raise RuntimeError(f"test server exited early with code {process.returncode}")
        try:
            response = requests.get(f"{base_url}/readyz", timeout=2)
            if response.status_code == 200:
                return
        except requests.RequestException:
            try:
                response = requests.get(f"{base_url}/metrics", timeout=2)
                if response.status_code == 200:
                    return
            except requests.RequestException:
                pass
        time.sleep(1)
    raise RuntimeError(f"timeout waiting for server at {base_url}")


def stop_server(process: subprocess.Popen[str] | None) -> None:
    if process is None or process.poll() is not None:
        return
    process.terminate()
    try:
        process.wait(timeout=30)
    except subprocess.TimeoutExpired:
        process.kill()
        process.wait(timeout=5)


def ensure_server_binary(binary_path: Path) -> None:
    binary_path.parent.mkdir(parents=True, exist_ok=True)
    run_command(["go", "build", "-o", str(binary_path), "./cmd/echo_chat_server"])


def load_json(path: Path) -> dict[str, Any]:
    return json.loads(path.read_text(encoding="utf-8"))


def write_json(path: Path, data: dict[str, Any]) -> None:
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def write_csv(path: Path, rows: list[dict[str, Any]]) -> None:
    if not rows:
        path.write_text("", encoding="utf-8")
        return
    with path.open("w", encoding="utf-8", newline="") as fp:
        writer = csv.DictWriter(fp, fieldnames=list(rows[0].keys()))
        writer.writeheader()
        for row in rows:
            writer.writerow(row)


def toml_value(value: Any) -> str:
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, int):
        return str(value)
    if isinstance(value, float):
        return str(value)
    escaped = str(value).replace("\\", "\\\\").replace('"', '\\"')
    return f'"{escaped}"'


def write_toml(path: Path, config: dict[str, dict[str, Any]]) -> None:
    lines: list[str] = []
    for section, values in config.items():
        lines.append(f"[{section}]")
        for key, value in values.items():
            lines.append(f"{key} = {toml_value(value)}")
        lines.append("")
    path.write_text("\n".join(lines).rstrip() + "\n", encoding="utf-8")


def parse_ports(value: str) -> list[int]:
    if not value:
        return []
    return [int(item.strip()) for item in value.split(",") if item.strip()]


def git_commit() -> str:
    try:
        result = run_command(["git", "rev-parse", "--short", "HEAD"])
        return result.stdout.strip()
    except Exception:
        return "unknown"


def git_dirty() -> bool:
    try:
        result = run_command(["git", "status", "--porcelain"])
        return bool(result.stdout.strip())
    except Exception:
        return False


@dataclass
class StageResult:
    scenario: str
    phase: str
    step_index: int
    target_rate: int
    actual_offered_rate: float
    interval_ms: int
    messages_per_sender: int
    summary: dict[str, Any]
    error_count: int
    metrics_path: str
    summary_path: str
    passed: bool

    def to_row(self) -> dict[str, Any]:
        row = {
            "scenario": self.scenario,
            "phase": self.phase,
            "step_index": self.step_index,
            "target_rate": self.target_rate,
            "actual_offered_rate": round(self.actual_offered_rate, 3),
            "interval_ms": self.interval_ms,
            "messages_per_sender": self.messages_per_sender,
            "passed": self.passed,
            "error_count": self.error_count,
            "metrics_path": self.metrics_path,
            "summary_path": self.summary_path,
        }
        if self.scenario == "single":
            row["observed_throughput"] = self.summary.get("observed_throughput_msg_per_sec")
            row["success_rate"] = self.summary.get("delivery_success_rate")
            row["p95_latency_ms"] = self.summary.get("latency", {}).get("p95_ms")
            row["p99_latency_ms"] = self.summary.get("latency", {}).get("p99_ms")
        else:
            row["observed_throughput"] = self.summary.get("observed_delivery_per_sec")
            row["success_rate"] = self.summary.get("delivery_coverage_rate")
            row["full_coverage_rate"] = self.summary.get("full_coverage_message_rate")
            row["p95_latency_ms"] = self.summary.get("receipt_latency", {}).get("p95_ms")
            row["p99_latency_ms"] = self.summary.get("receipt_latency", {}).get("p99_ms")
            row["broadcast_p95_ms"] = self.summary.get("broadcast_completion_latency", {}).get("p95_ms")
        return row


class ThroughputCapacityRunner:
    def __init__(self, args: argparse.Namespace) -> None:
        self.args = args
        self.timestamp = time.strftime("%Y%m%d_%H%M%S")
        label_suffix = f"_{args.label}" if args.label else ""
        self.run_dir = RECORD_ROOT / f"throughput_capacity_{args.mode}_{self.timestamp}{label_suffix}"
        self.config_dir = self.run_dir / "configs"
        self.binary_path = ROOT_DIR / "bin" / "echo_chat_server"
        self.server_processes: list[subprocess.Popen[str]] = []
        self.instance_ports = args.instance_ports or [args.port]
        self.client_instance_ports = args.client_instance_ports or self.instance_ports
        self.server_base_urls = [f"http://127.0.0.1:{port}" for port in self.instance_ports]
        self.base_urls = [f"http://127.0.0.1:{port}" for port in self.client_instance_ports]
        self.ws_base_urls = [f"ws://127.0.0.1:{port}" for port in self.client_instance_ports]
        self.base_url = self.base_urls[0]
        self.ws_base_url = self.ws_base_urls[0]
        self.current_config_paths: list[Path] = []
        self.server_log_files: list[Any] = []

    def run(self) -> None:
        self.run_dir.mkdir(parents=True, exist_ok=True)
        ensure_server_binary(self.binary_path)
        self.generate_config()
        self.current_config_path = self.default_config_path

        metadata = {
            "mode": self.args.mode,
            "label": self.args.label,
            "git_commit": git_commit(),
            "git_dirty": git_dirty(),
            "generated_at": self.timestamp,
            "base_config": str(Path(self.args.base_config).resolve()),
            "instance_ports": self.instance_ports,
            "client_instance_ports": self.client_instance_ports,
            "pair_count": self.args.single_pair_count,
            "group_member_limit": self.args.group_member_limit,
            "single_success_threshold": self.args.single_success_threshold,
            "single_p95_threshold_ms": self.args.single_p95_threshold_ms,
            "group_coverage_threshold": self.args.group_coverage_threshold,
            "group_full_coverage_threshold": self.args.group_full_coverage_threshold,
            "group_p95_threshold_ms": self.args.group_p95_threshold_ms,
            "single_only": self.args.single_only,
        }
        write_json(self.run_dir / "metadata.json", metadata)

        single_fixture = self.prepare_fixture(seed_prefix=self.args.seed_prefix)
        single_results = self.search_scenario(
            scenario="single",
            receiver_count=1,
            initial_target=self.args.single_initial_target,
            threshold=self.args.single_success_threshold,
        )
        group_results: list[StageResult] = []
        if not self.args.single_only:
            group_fixture = load_json(single_fixture)
            group_results = self.search_scenario(
                scenario="group",
                receiver_count=max(1, len(group_fixture["group"]["members"]) - 1),
                initial_target=self.args.group_initial_target,
                threshold=self.args.group_coverage_threshold,
            )

        single_best = self.pick_best(single_results)
        group_best = self.pick_best(group_results)

        write_csv(self.run_dir / "single_stage_summary.csv", [item.to_row() for item in single_results])
        if not self.args.single_only:
            write_csv(self.run_dir / "group_stage_summary.csv", [item.to_row() for item in group_results])

        summary = {
            "mode": self.args.mode,
            "label": self.args.label,
            "generated_at": self.timestamp,
            "git_commit": metadata["git_commit"],
            "git_dirty": metadata["git_dirty"],
            "single": self.best_summary_payload(single_best),
            "group": None if self.args.single_only else self.best_summary_payload(group_best),
        }
        write_json(self.run_dir / "summary.json", summary)
        (self.run_dir / "report.md").write_text(self.build_report(single_results, group_results, single_best, group_best), encoding="utf-8")

    def generate_config(self) -> None:
        self.config_dir.mkdir(parents=True, exist_ok=True)
        with Path(self.args.base_config).open("rb") as fp:
            config = tomllib.load(fp)
        self.current_config_paths = []
        for port in self.instance_ports:
            config_copy = json.loads(json.dumps(config))
            config_copy["mainConfig"]["port"] = port
            config_copy["logConfig"]["logPath"] = str(self.run_dir / f"kafka_server_{port}.log")
            config_path = self.config_dir / f"{self.args.mode}_{port}.toml"
            write_toml(config_path, config_copy)
            self.current_config_paths.append(config_path)

    @property
    def default_config_path(self) -> Path:
        return self.current_config_paths[0]

    def prepare_fixture(self, *, seed_prefix: str) -> Path:
        fixture_env = os.environ.copy()
        fixture_env["ECHOCHAT_CONFIG"] = str(self.default_config_path)
        with self.default_config_path.open("rb") as fp:
            config = tomllib.load(fp)
        mysql_config = config.get("mysqlConfig", {})
        fixture_env["ECHOCHAT_MYSQL_HOST"] = str(mysql_config.get("host", "127.0.0.1"))
        fixture_env["ECHOCHAT_MYSQL_PORT"] = str(mysql_config.get("port", 3306))
        fixture_env["ECHOCHAT_MYSQL_USER"] = str(mysql_config.get("user", "root"))
        fixture_env["ECHOCHAT_MYSQL_PASSWORD"] = str(mysql_config.get("password", ""))
        user_count = max(self.args.single_pair_count * 6 + 20, self.args.group_member_limit * 3, 200)
        run_command(
            [
                "go",
                "run",
                "./cmd/echo_chat_seed",
                "--prefix",
                seed_prefix,
                "--user-count",
                str(user_count),
                "--group-size",
                str(max(self.args.group_member_limit, 25)),
                "--reset-prefix=true",
            ],
            env=fixture_env,
        )
        fixture_path = self.run_dir / "fixture.json"
        run_command(
            [
                "python3",
                str(SCRIPT_DIR / "prepare_message_fixtures.py"),
                "--database",
                self.args.database,
                "--user-prefix",
                f"U{seed_prefix}",
                "--group-prefix",
                f"G{seed_prefix}",
                "--pair-count",
                str(self.args.single_pair_count),
                "--group-member-limit",
                str(self.args.group_member_limit),
                "--output",
                str(fixture_path),
            ],
            env=fixture_env,
        )
        return fixture_path

    def scenario_interval(self, scenario: str, target_rate: int, receiver_count: int) -> tuple[int, float]:
        if scenario == "single":
            interval_ms = max(1, round(self.args.single_pair_count * 1000 / target_rate))
            actual_rate = self.args.single_pair_count * 1000 / interval_ms
            return interval_ms, actual_rate
        interval_ms = max(1, round(receiver_count * 1000 / target_rate))
        actual_rate = receiver_count * 1000 / interval_ms
        return interval_ms, actual_rate

    def scenario_messages(self, interval_ms: int, min_duration_sec: int, max_messages: int) -> int:
        return max(1, min(max_messages, math.ceil(min_duration_sec * 1000 / interval_ms)))

    def start_servers(self, log_dir: Path) -> None:
        self.server_processes = []
        self.server_log_files = []
        log_dir.mkdir(parents=True, exist_ok=True)
        for config_path, base_url in zip(self.current_config_paths, self.server_base_urls):
            env = os.environ.copy()
            env["ECHOCHAT_CONFIG"] = str(config_path)
            log_fp = (log_dir / f"server_{config_path.stem}.log").open("w", encoding="utf-8")
            self.server_log_files.append(log_fp)
            process = subprocess.Popen(
                [str(self.binary_path)],
                cwd=str(ROOT_DIR),
                env=env,
                stdout=log_fp,
                stderr=subprocess.STDOUT,
                text=True,
            )
            self.server_processes.append(process)
            wait_for_server(base_url, process)

    def stop_servers(self) -> None:
        for process in self.server_processes:
            stop_server(process)
        self.server_processes = []
        for log_fp in self.server_log_files:
            log_fp.close()
        self.server_log_files = []

    def fetch_metrics(self, output_dir: Path) -> list[dict[str, str]]:
        metric_rows: list[dict[str, str]] = []
        output_dir.mkdir(parents=True, exist_ok=True)
        for port, base_url in zip(self.instance_ports, self.server_base_urls):
            metrics_path = output_dir / f"metrics_{port}.prom"
            try:
                response = requests.get(f"{base_url}/metrics", timeout=5)
                response.raise_for_status()
                metrics_path.write_text(response.text, encoding="utf-8")
                metric_rows.append({"port": str(port), "path": str(metrics_path.relative_to(self.run_dir)), "error": ""})
            except requests.RequestException as exc:
                metric_rows.append({"port": str(port), "path": "", "error": str(exc)})
        return metric_rows

    def build_stage_config(self, *, scenario: str, phase: str, step_index: int, stage_dir: Path) -> Path:
        stage_group = f"chat_{self.args.mode}_{scenario}_{phase}_{step_index}_{int(time.time())}"
        stage_config_paths: list[Path] = []
        for port in self.instance_ports:
            with Path(self.args.base_config).open("rb") as fp:
                config = tomllib.load(fp)
            kafka_config = dict(config.get("kafkaConfig", {}))
            kafka_config["consumerGroup"] = stage_group
            config["kafkaConfig"] = kafka_config
            config["mainConfig"]["port"] = port
            config["logConfig"]["logPath"] = str(stage_dir / f"kafka_server_{port}.log")
            stage_config_path = stage_dir / f"config_{port}.toml"
            write_toml(stage_config_path, config)
            stage_config_paths.append(stage_config_path)
        self.current_config_paths = stage_config_paths
        return stage_config_paths[0]

    def run_stage(self, *, scenario: str, receiver_count: int, target_rate: int, phase: str, step_index: int) -> StageResult:
        interval_ms, actual_offered_rate = self.scenario_interval(scenario, target_rate, receiver_count)
        messages_per_sender = self.scenario_messages(
            interval_ms,
            self.args.single_min_duration_sec if scenario == "single" else self.args.group_min_duration_sec,
            self.args.single_max_messages if scenario == "single" else self.args.group_max_messages,
        )

        stage_dir = self.run_dir / scenario / f"step_{step_index:03d}_{phase}_target_{target_rate}"
        stage_dir.mkdir(parents=True, exist_ok=True)
        stage_config_path = self.build_stage_config(scenario=scenario, phase=phase, step_index=step_index, stage_dir=stage_dir)
        fresh_fixture = self.prepare_fixture(seed_prefix=self.args.seed_prefix)
        shutil.copy2(fresh_fixture, stage_dir / "fixture.json")
        log_dir = stage_dir / "boot_logs"
        metrics_rows: list[dict[str, str]] = []
        try:
            self.start_servers(log_dir)
            run_command(
                [
                    "python3",
                    str(SCRIPT_DIR / "message_latency_runner.py"),
                    "--base-url",
                    self.base_url,
                    "--ws-base-url",
                    self.ws_base_url,
                    "--base-urls",
                    ",".join(self.base_urls),
                    "--ws-base-urls",
                    ",".join(self.ws_base_urls),
                    "--fixture",
                    str(stage_dir / "fixture.json"),
                    "--scenario",
                    scenario,
                    "--output-dir",
                    str(stage_dir),
                    "--messages-per-sender",
                    str(messages_per_sender),
                    "--send-interval-ms",
                    str(interval_ms),
                    "--message-timeout-ms",
                    str(self.args.message_timeout_ms),
                    "--connection-settle-ms",
                    str(self.args.connection_settle_ms),
                    "--drain-wait-ms",
                    str(self.args.drain_wait_ms),
                    "--drain-idle-ms",
                    str(self.args.drain_idle_ms),
                    "--pair-count",
                    str(self.args.single_pair_count),
                    "--group-member-limit",
                    str(self.args.group_member_limit),
                    "--mode-label",
                    self.args.mode,
                    "--server-pid",
                    str(self.server_processes[0].pid),
                ]
            )
            if self.args.post_run_settle_ms > 0:
                time.sleep(self.args.post_run_settle_ms / 1000.0)
            metrics_rows = self.fetch_metrics(stage_dir / "metrics")
        finally:
            self.stop_servers()

        summary = load_json(stage_dir / "summary.json")
        errors = load_json(stage_dir / "errors.json")
        metric_errors = [row for row in metrics_rows if row["error"]]
        if metric_errors:
            for row in metric_errors:
                errors.append({"scenario": scenario, "where": f"fetch_metrics:{row['port']}", "error": row["error"]})
            (stage_dir / "errors.json").write_text(json.dumps(errors, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        write_json(stage_dir / "metrics_manifest.json", {"instances": metrics_rows})
        passed = self.stage_passed(scenario, summary, len(errors))
        return StageResult(
            scenario=scenario,
            phase=phase,
            step_index=step_index,
            target_rate=target_rate,
            actual_offered_rate=actual_offered_rate,
            interval_ms=interval_ms,
            messages_per_sender=messages_per_sender,
            summary=summary,
            error_count=len(errors),
            metrics_path=str((stage_dir / "metrics_manifest.json").relative_to(self.run_dir)),
            summary_path=str((stage_dir / "summary.json").relative_to(self.run_dir)),
            passed=passed,
        )

    def stage_passed(self, scenario: str, summary: dict[str, Any], error_count: int) -> bool:
        if error_count > self.args.max_error_count:
            return False
        if scenario == "single":
            success_ok = float(summary.get("delivery_success_rate", 0.0)) >= self.args.single_success_threshold
            p95_ms = float(summary.get("latency", {}).get("p95_ms", 0.0) or 0.0)
            latency_ok = self.args.single_p95_threshold_ms <= 0 or p95_ms <= self.args.single_p95_threshold_ms
            return success_ok and latency_ok
        receipt_p95_ms = float(summary.get("receipt_latency", {}).get("p95_ms", 0.0) or 0.0)
        latency_ok = self.args.group_p95_threshold_ms <= 0 or receipt_p95_ms <= self.args.group_p95_threshold_ms
        return (
            float(summary.get("delivery_coverage_rate", 0.0)) >= self.args.group_coverage_threshold
            and float(summary.get("full_coverage_message_rate", 0.0)) >= self.args.group_full_coverage_threshold
            and latency_ok
        )

    def search_scenario(self, *, scenario: str, receiver_count: int, initial_target: int, threshold: float) -> list[StageResult]:
        del threshold
        results: list[StageResult] = []
        visited_intervals: set[int] = set()
        low_pass_target: int | None = None
        high_fail_target: int | None = None
        target = initial_target
        step_index = 1

        while step_index <= self.args.max_expand_steps:
            interval_ms, _ = self.scenario_interval(scenario, target, receiver_count)
            if interval_ms in visited_intervals:
                break
            visited_intervals.add(interval_ms)
            stage = self.run_stage(
                scenario=scenario,
                receiver_count=receiver_count,
                target_rate=target,
                phase="expand",
                step_index=step_index,
            )
            results.append(stage)
            if stage.passed:
                low_pass_target = target
                target *= 2
                step_index += 1
                continue
            high_fail_target = target
            break

        if low_pass_target is None:
            return results
        if high_fail_target is None:
            return results

        refine_step = 1
        while refine_step <= self.args.max_refine_steps and high_fail_target-low_pass_target > max(1, self.args.refine_resolution):
            target = (low_pass_target + high_fail_target) // 2
            interval_ms, _ = self.scenario_interval(scenario, target, receiver_count)
            if interval_ms in visited_intervals:
                break
            visited_intervals.add(interval_ms)
            stage = self.run_stage(
                scenario=scenario,
                receiver_count=receiver_count,
                target_rate=target,
                phase="refine",
                step_index=len(results) + 1,
            )
            results.append(stage)
            if stage.passed:
                low_pass_target = target
            else:
                high_fail_target = target
            refine_step += 1
        return results

    def pick_best(self, results: list[StageResult]) -> StageResult | None:
        passed = [item for item in results if item.passed]
        if not passed:
            return None
        if passed[0].scenario == "single":
            return max(passed, key=lambda item: float(item.summary.get("observed_throughput_msg_per_sec", 0.0) or 0.0))
        return max(passed, key=lambda item: float(item.summary.get("observed_delivery_per_sec", 0.0) or 0.0))

    def best_summary_payload(self, result: StageResult | None) -> dict[str, Any] | None:
        if result is None:
            return None
        payload = result.to_row()
        payload["summary"] = result.summary
        return payload

    def build_report(
        self,
        single_results: list[StageResult],
        group_results: list[StageResult],
        single_best: StageResult | None,
        group_best: StageResult | None,
    ) -> str:
        lines = [
            f"# Throughput Capacity Report ({self.args.mode})",
            "",
            f"- Generated at: `{self.timestamp}`",
            f"- Git commit: `{git_commit()}`",
            f"- Git dirty: `{git_dirty()}`",
            f"- Label: `{self.args.label or 'n/a'}`",
            "",
            "## Peak Stable Results",
            "",
        ]
        if single_best is None:
            lines.append("- Single: no passing stage")
        else:
            lines.append(
                f"- Single: `{single_best.summary.get('observed_throughput_msg_per_sec')}` msg/s, "
                f"success `{single_best.summary.get('delivery_success_rate')}`, "
                f"p95 `{single_best.summary.get('latency', {}).get('p95_ms')}` ms, "
                f"stage `{single_best.summary_path}`"
            )
        if self.args.single_only:
            lines.append("- Group: skipped (`--single-only`)")
        elif group_best is None:
            lines.append("- Group: no passing stage")
        else:
            lines.append(
                f"- Group: `{group_best.summary.get('observed_delivery_per_sec')}` deliveries/s, "
                f"coverage `{group_best.summary.get('delivery_coverage_rate')}`, "
                f"full coverage `{group_best.summary.get('full_coverage_message_rate')}`, "
                f"receipt p95 `{group_best.summary.get('receipt_latency', {}).get('p95_ms')}` ms, "
                f"stage `{group_best.summary_path}`"
            )
        lines.extend(["", "## Single Stages", "", "| step | phase | target | offered | observed | success | p95 | pass |", "| --- | --- | ---: | ---: | ---: | ---: | ---: | --- |"])
        for item in single_results:
            lines.append(
                f"| {item.step_index} | {item.phase} | {item.target_rate} | {item.actual_offered_rate:.1f} | "
                f"{item.summary.get('observed_throughput_msg_per_sec')} | {item.summary.get('delivery_success_rate')} | "
                f"{item.summary.get('latency', {}).get('p95_ms')} | {item.passed} |"
            )
        if self.args.single_only:
            lines.extend(["", "## Group Stages", "", "- Skipped (`--single-only`)"])
        else:
            lines.extend(["", "## Group Stages", "", "| step | phase | target | offered | observed | coverage | full coverage | receipt p95 | pass |", "| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |"])
            for item in group_results:
                lines.append(
                    f"| {item.step_index} | {item.phase} | {item.target_rate} | {item.actual_offered_rate:.1f} | "
                    f"{item.summary.get('observed_delivery_per_sec')} | {item.summary.get('delivery_coverage_rate')} | "
                    f"{item.summary.get('full_coverage_message_rate')} | {item.summary.get('receipt_latency', {}).get('p95_ms')} | {item.passed} |"
                )
        lines.extend(
            [
                "",
                "## Notes",
                "",
                "- Single throughput uses `observed_throughput_msg_per_sec`.",
                "- Group throughput uses `observed_delivery_per_sec`.",
                "- Passing criteria are controlled by success / coverage thresholds, p95 thresholds, and max error count.",
            ]
        )
        return "\n".join(lines) + "\n"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Search max stable single/group throughput for the current EchoChat message mode.")
    parser.add_argument("--mode", choices=["channel", "kafka"], default="kafka")
    parser.add_argument("--label", default="")
    parser.add_argument("--base-config", default=str(ROOT_DIR / "configs" / "config_local.toml"))
    parser.add_argument("--database", default="echochat")
    parser.add_argument("--seed-prefix", default="K6")
    parser.add_argument("--port", type=int, default=18082)
    parser.add_argument("--instance-ports", type=parse_ports, default=[])
    parser.add_argument("--client-instance-ports", type=parse_ports, default=[])
    parser.add_argument("--single-pair-count", type=int, default=30)
    parser.add_argument("--group-member-limit", type=int, default=25)
    parser.add_argument("--single-initial-target", type=int, default=120)
    parser.add_argument("--group-initial-target", type=int, default=180)
    parser.add_argument("--single-min-duration-sec", type=int, default=8)
    parser.add_argument("--group-min-duration-sec", type=int, default=8)
    parser.add_argument("--single-max-messages", type=int, default=5000)
    parser.add_argument("--group-max-messages", type=int, default=5000)
    parser.add_argument("--message-timeout-ms", type=int, default=60000)
    parser.add_argument("--connection-settle-ms", type=int, default=1500)
    parser.add_argument("--drain-wait-ms", type=int, default=5000)
    parser.add_argument("--drain-idle-ms", type=int, default=1000)
    parser.add_argument("--post-run-settle-ms", type=int, default=1000)
    parser.add_argument("--single-success-threshold", type=float, default=0.995)
    parser.add_argument("--single-p95-threshold-ms", type=float, default=1000.0)
    parser.add_argument("--group-coverage-threshold", type=float, default=0.995)
    parser.add_argument("--group-full-coverage-threshold", type=float, default=0.99)
    parser.add_argument("--group-p95-threshold-ms", type=float, default=1000.0)
    parser.add_argument("--max-error-count", type=int, default=0)
    parser.add_argument("--max-expand-steps", type=int, default=8)
    parser.add_argument("--max-refine-steps", type=int, default=6)
    parser.add_argument("--refine-resolution", type=int, default=10)
    parser.add_argument("--single-only", action="store_true")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    runner = ThroughputCapacityRunner(args)
    try:
        runner.run()
    except Exception as exc:
        print(f"throughput capacity run failed: {exc}", file=sys.stderr)
        runner.stop_servers()
        raise
    print(runner.run_dir)


if __name__ == "__main__":
    main()
