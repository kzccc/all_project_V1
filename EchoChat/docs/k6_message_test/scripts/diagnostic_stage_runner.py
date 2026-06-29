#!/usr/bin/env python3

from __future__ import annotations

import argparse
import json
import math
import os
import shutil
import subprocess
import sys
import threading
import time
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
RECORD_ROOT = TEST_DIR / "records"


def run_command(command: list[str], *, env: dict[str, str] | None = None, cwd: Path = ROOT_DIR) -> subprocess.CompletedProcess:
    return subprocess.run(
        command,
        cwd=str(cwd),
        env=env,
        check=True,
        text=True,
        capture_output=True,
    )


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


def load_json(path: Path) -> dict[str, Any]:
    return json.loads(path.read_text(encoding="utf-8"))


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


class DiagnosticStageRunner:
    def __init__(self, args: argparse.Namespace) -> None:
        self.args = args
        self.timestamp = time.strftime("%Y%m%d_%H%M%S")
        label_suffix = f"_{args.label}" if args.label else ""
        self.run_dir = RECORD_ROOT / f"diagnostic_stages_{args.mode}_{self.timestamp}{label_suffix}"
        self.config_dir = self.run_dir / "configs"
        self.binary_path = ROOT_DIR / "bin" / "echo_chat_server"
        self.server_process: subprocess.Popen[str] | None = None
        self.base_url = f"http://127.0.0.1:{args.port}"
        self.ws_base_url = f"ws://127.0.0.1:{args.port}"
        self.current_config_path: Path | None = None

    @property
    def default_config_path(self) -> Path:
        return self.config_dir / f"{self.args.mode}_test.toml"

    def generate_config(self) -> None:
        self.config_dir.mkdir(parents=True, exist_ok=True)
        run_command(
            [
                "python3",
                str(SCRIPT_DIR / "make_test_configs.py"),
                "--base-config",
                self.args.base_config,
                "--output-dir",
                str(self.config_dir),
                "--channel-port",
                str(self.args.port),
                "--kafka-port",
                str(self.args.port),
                "--channel-log",
                str(self.run_dir / "channel_server.log"),
                "--kafka-log",
                str(self.run_dir / "kafka_server.log"),
            ]
        )

    def prepare_fixture(self, stage_dir: Path) -> Path:
        user_count = max(self.args.single_pair_count * 6 + 20, self.args.group_member_limit * 3, 200)
        run_command(
            [
                "go",
                "run",
                "./cmd/echo_chat_seed",
                "--prefix",
                self.args.seed_prefix,
                "--user-count",
                str(user_count),
                "--group-size",
                str(max(self.args.group_member_limit, 25)),
                "--reset-prefix=true",
            ]
        )
        fixture_path = stage_dir / "fixture.json"
        run_command(
            [
                "python3",
                str(SCRIPT_DIR / "prepare_message_fixtures.py"),
                "--database",
                self.args.database,
                "--user-prefix",
                f"U{self.args.seed_prefix}",
                "--group-prefix",
                f"G{self.args.seed_prefix}",
                "--pair-count",
                str(self.args.single_pair_count),
                "--group-member-limit",
                str(self.args.group_member_limit),
                "--output",
                str(fixture_path),
            ]
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

    def build_stage_config(self, stage_dir: Path, scenario: str, target_rate: int) -> Path:
        with self.default_config_path.open("rb") as fp:
            config = tomllib.load(fp)
        kafka_config = dict(config.get("kafkaConfig", {}))
        kafka_config["consumerGroup"] = f"diag_{self.args.mode}_{scenario}_{target_rate}_{int(time.time())}"
        config["kafkaConfig"] = kafka_config
        path = stage_dir / "config.toml"
        write_toml(path, config)
        return path

    def start_server(self, log_path: Path) -> None:
        env = os.environ.copy()
        env["ECHOCHAT_CONFIG"] = str(self.current_config_path or self.default_config_path)
        log_fp = log_path.open("w", encoding="utf-8")
        self.server_process = subprocess.Popen(
            [str(self.binary_path)],
            cwd=str(ROOT_DIR),
            env=env,
            stdout=log_fp,
            stderr=subprocess.STDOUT,
            text=True,
        )
        wait_for_server(self.base_url, self.server_process)

    def fetch_metrics(self, output_path: Path) -> None:
        response = requests.get(f"{self.base_url}/metrics", timeout=10)
        response.raise_for_status()
        output_path.write_text(response.text, encoding="utf-8")

    def fetch_pprof_text(self, endpoint: str, output_path: Path) -> None:
        response = requests.get(f"{self.base_url}/debug/pprof/{endpoint}", timeout=30)
        response.raise_for_status()
        output_path.write_bytes(response.content)

    def fetch_pprof_cpu(self, seconds: int, output_path: Path) -> None:
        response = requests.get(f"{self.base_url}/debug/pprof/profile", params={"seconds": seconds}, timeout=seconds + 20)
        response.raise_for_status()
        output_path.write_bytes(response.content)

    def collect_pprof(self, stage_dir: Path) -> None:
        pprof_dir = stage_dir / "pprof"
        pprof_dir.mkdir(parents=True, exist_ok=True)
        cpu_thread = threading.Thread(
            target=self.fetch_pprof_cpu,
            args=(self.args.pprof_seconds, pprof_dir / "cpu.pb.gz"),
            daemon=True,
        )
        cpu_thread.start()
        cpu_thread.join()
        self.fetch_pprof_text("goroutine?debug=1", pprof_dir / "goroutine.txt")
        self.fetch_pprof_text("block?debug=1", pprof_dir / "block.txt")
        self.fetch_pprof_text("mutex?debug=1", pprof_dir / "mutex.txt")
        self.fetch_pprof_text("heap?debug=1", pprof_dir / "heap.txt")

    def run_stage(self, scenario: str, target_rate: int) -> dict[str, Any]:
        stage_dir = self.run_dir / scenario / f"target_{target_rate}"
        stage_dir.mkdir(parents=True, exist_ok=True)
        fixture_path = self.prepare_fixture(stage_dir)
        stage_config_path = self.build_stage_config(stage_dir, scenario, target_rate)
        fixture = load_json(fixture_path)
        receiver_count = 1 if scenario == "single" else max(1, len(fixture["group"]["members"]) - 1)
        interval_ms, actual_rate = self.scenario_interval(scenario, target_rate, receiver_count)
        messages_per_sender = self.scenario_messages(
            interval_ms,
            self.args.single_min_duration_sec if scenario == "single" else self.args.group_min_duration_sec,
            self.args.single_max_messages if scenario == "single" else self.args.group_max_messages,
        )
        metadata = {
            "scenario": scenario,
            "target_rate": target_rate,
            "actual_offered_rate": round(actual_rate, 3),
            "send_interval_ms": interval_ms,
            "messages_per_sender": messages_per_sender,
            "receiver_count": receiver_count,
        }
        (stage_dir / "stage_meta.json").write_text(json.dumps(metadata, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

        log_path = stage_dir / "server.log"
        try:
            self.current_config_path = stage_config_path
            self.start_server(log_path)
            run_command(
                [
                    "python3",
                    str(SCRIPT_DIR / "message_latency_runner.py"),
                    "--base-url",
                    self.base_url,
                    "--ws-base-url",
                    self.ws_base_url,
                    "--fixture",
                    str(fixture_path),
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
                    str(self.server_process.pid),
                ]
            )
            if self.args.post_run_settle_ms > 0:
                time.sleep(self.args.post_run_settle_ms / 1000.0)
            self.fetch_metrics(stage_dir / "metrics.prom")
            self.collect_pprof(stage_dir)
        finally:
            stop_server(self.server_process)
            self.server_process = None
            self.current_config_path = None

        summary = load_json(stage_dir / "summary.json")
        result = {
            "scenario": scenario,
            "target_rate": target_rate,
            "actual_offered_rate": round(actual_rate, 3),
            "send_interval_ms": interval_ms,
            "messages_per_sender": messages_per_sender,
            "summary_path": str((stage_dir / "summary.json").relative_to(self.run_dir)),
            "metrics_path": str((stage_dir / "metrics.prom").relative_to(self.run_dir)),
            "pprof_dir": str((stage_dir / "pprof").relative_to(self.run_dir)),
        }
        if scenario == "single":
            result["observed"] = summary.get("observed_throughput_msg_per_sec")
            result["success"] = summary.get("delivery_success_rate")
            result["p95"] = summary.get("latency", {}).get("p95_ms")
        else:
            result["observed"] = summary.get("observed_delivery_per_sec")
            result["coverage"] = summary.get("delivery_coverage_rate")
            result["full_coverage"] = summary.get("full_coverage_message_rate")
            result["p95"] = summary.get("receipt_latency", {}).get("p95_ms")
        return result

    def run(self) -> None:
        self.run_dir.mkdir(parents=True, exist_ok=True)
        ensure_server_binary(self.binary_path)
        self.generate_config()
        metadata = {
            "mode": self.args.mode,
            "label": self.args.label,
            "generated_at": self.timestamp,
            "git_commit": git_commit(),
            "git_dirty": git_dirty(),
            "single_targets": self.args.single_targets,
            "group_targets": self.args.group_targets,
        }
        (self.run_dir / "metadata.json").write_text(json.dumps(metadata, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

        results: dict[str, list[dict[str, Any]]] = {"single": [], "group": []}
        for target in self.args.single_targets:
            results["single"].append(self.run_stage("single", target))
        for target in self.args.group_targets:
            results["group"].append(self.run_stage("group", target))
        (self.run_dir / "summary.json").write_text(json.dumps(results, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

        report_lines = [
            f"# Diagnostic Stage Report ({self.args.mode})",
            "",
            f"- Generated at: `{self.timestamp}`",
            f"- Git commit: `{metadata['git_commit']}`",
            f"- Git dirty: `{metadata['git_dirty']}`",
            f"- Label: `{self.args.label}`",
            "",
            "## Single",
            "",
            "| target | offered | interval_ms | messages_per_sender | observed | success | p95 |",
            "| ---: | ---: | ---: | ---: | ---: | ---: | ---: |",
        ]
        for item in results["single"]:
            report_lines.append(
                f"| {item['target_rate']} | {item['actual_offered_rate']:.1f} | {item['send_interval_ms']} | {item['messages_per_sender']} | "
                f"{item['observed']} | {item['success']} | {item['p95']} |"
            )
        report_lines.extend(
            [
                "",
                "## Group",
                "",
                "| target | offered | interval_ms | messages_per_sender | observed | coverage | full coverage | p95 |",
                "| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |",
            ]
        )
        for item in results["group"]:
            report_lines.append(
                f"| {item['target_rate']} | {item['actual_offered_rate']:.1f} | {item['send_interval_ms']} | {item['messages_per_sender']} | "
                f"{item['observed']} | {item['coverage']} | {item['full_coverage']} | {item['p95']} |"
            )
        (self.run_dir / "report.md").write_text("\n".join(report_lines) + "\n", encoding="utf-8")
        print(self.run_dir)


def parse_targets(value: str) -> list[int]:
    return [int(item.strip()) for item in value.split(",") if item.strip()]


def main() -> None:
    parser = argparse.ArgumentParser(description="Run fixed diagnostic throughput stages with metrics and pprof capture.")
    parser.add_argument("--mode", default="kafka")
    parser.add_argument("--label", default="diag")
    parser.add_argument("--base-config", default=str(ROOT_DIR / "configs" / "config_local.toml"))
    parser.add_argument("--database", default="echochat")
    parser.add_argument("--seed-prefix", default="K6")
    parser.add_argument("--port", type=int, default=18082)
    parser.add_argument("--single-pair-count", type=int, default=30)
    parser.add_argument("--group-member-limit", type=int, default=25)
    parser.add_argument("--single-targets", type=parse_targets, default=parse_targets("240,960,2880"))
    parser.add_argument("--group-targets", type=parse_targets, default=parse_targets("1440,5760,11520"))
    parser.add_argument("--single-min-duration-sec", type=int, default=8)
    parser.add_argument("--group-min-duration-sec", type=int, default=8)
    parser.add_argument("--single-max-messages", type=int, default=5000)
    parser.add_argument("--group-max-messages", type=int, default=5000)
    parser.add_argument("--message-timeout-ms", type=int, default=60000)
    parser.add_argument("--connection-settle-ms", type=int, default=1500)
    parser.add_argument("--drain-wait-ms", type=int, default=5000)
    parser.add_argument("--drain-idle-ms", type=int, default=1000)
    parser.add_argument("--post-run-settle-ms", type=int, default=1000)
    parser.add_argument("--pprof-seconds", type=int, default=8)
    args = parser.parse_args()

    runner = DiagnosticStageRunner(args)
    runner.run()


if __name__ == "__main__":
    try:
        main()
    except subprocess.CalledProcessError as exc:
        sys.stderr.write(exc.stdout or "")
        sys.stderr.write(exc.stderr or "")
        raise
