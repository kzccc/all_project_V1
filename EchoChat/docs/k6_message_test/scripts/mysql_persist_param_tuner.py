#!/usr/bin/env python3

from __future__ import annotations

import argparse
import json
import math
import os
import shutil
import socket
import subprocess
import sys
import time
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any

import tomllib

from partition_tuning_runner import (
    PrometheusSnapshot,
    ServerProcessSampler,
    git_commit,
    git_dirty,
    load_json,
    round_or_none,
    write_csv,
    write_json,
)


def detect_repo_root() -> Path:
    env_root = os.environ.get("ECHOCHAT_REPO_ROOT")
    if env_root:
        return Path(env_root).expanduser().resolve()
    current = Path(__file__).resolve()
    for parent in [current.parent, *current.parents]:
        if (parent / "go.mod").exists() and (parent / "configs").exists():
            return parent
    return current.parents[3]


def detect_local_mysql_root(repo_root: Path) -> Path:
    env_root = os.environ.get("ECHOCHAT_LOCAL_MYSQL_ROOT")
    if env_root:
        return Path(env_root).expanduser().resolve()
    return repo_root / "tmp" / "mysql_sys"


ROOT_DIR = detect_repo_root()
TEST_DIR = ROOT_DIR / "docs" / "k6_message_test"
SCRIPT_DIR = TEST_DIR / "scripts"
SINGLE_CHAT_RUNNER = SCRIPT_DIR / "single_chat_stage_runner.py"
DEFAULT_BASE_CONFIG = ROOT_DIR / "configs" / "config_local_singlebroker_part240_mysqlpersist_tune2.toml"
RECORD_ROOT = Path(
    os.environ.get(
        "ECHOCHAT_MYSQL_PERSIST_TUNING_RECORD_ROOT",
        str(TEST_DIR / "mysql_persist_tuning_records"),
    )
)
LOCAL_MYSQL_ROOT = detect_local_mysql_root(ROOT_DIR)
LOCAL_MYSQL_DATA = LOCAL_MYSQL_ROOT / "data"
LOCAL_MYSQL_RUN = LOCAL_MYSQL_ROOT / "run"
LOCAL_MYSQL_TMP = Path("/run/user/0/echochat_mysql_tmp")
LOCAL_MYSQL_LOG = LOCAL_MYSQL_ROOT / "mysql-foreground.log"
GO_BUILD_CACHE_DIR = Path("/run/user/0/echochat_go_build_cache")
GO_MOD_CACHE_DIR = Path("/run/user/0/echochat_go_mod_cache")

DIMENSION_ORDER = [
    "max_open_conns",
    "max_idle_conns",
    "worker_count",
    "batch_size",
    "flush_interval_ms",
    "queue_size",
]

DIMENSION_LABELS = {
    "max_open_conns": "maxOpenConns",
    "max_idle_conns": "maxIdleConns",
    "worker_count": "mysqlPersistWorkerCount",
    "batch_size": "mysqlPersistBatchSize",
    "flush_interval_ms": "mysqlPersistFlushIntervalMs",
    "queue_size": "mysqlPersistQueueSize",
}

DEFAULT_CANDIDATES = {
    "max_open_conns": [300, 400, 500, 600, 700, 800],
    "max_idle_conns": [50, 100, 150, 200, 250],
    "worker_count": [16, 24, 32, 40, 48, 64],
    "batch_size": [128, 192, 256, 320, 384, 512],
    "flush_interval_ms": [1, 2, 3, 5, 7, 10],
    "queue_size": [1024, 2048, 4096, 8192, 16384],
}

REFINE_STEP = {
    "max_open_conns": 25,
    "max_idle_conns": 25,
    "worker_count": 4,
    "batch_size": 32,
    "flush_interval_ms": 1,
    "queue_size": 1024,
}


def parse_int_list(raw: str | None, default: list[int]) -> list[int]:
    if not raw:
        return list(default)
    values = []
    for item in raw.split(","):
        item = item.strip()
        if not item:
            continue
        values.append(int(item))
    return sorted(set(values))


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


def format_ports(ports: list[int]) -> str:
    return ",".join(str(port) for port in ports)


def is_tcp_open(host: str, port: int) -> bool:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.settimeout(1)
        return sock.connect_ex((host, port)) == 0


def wait_for_tcp(host: str, port: int, timeout_sec: int = 30) -> None:
    deadline = time.time() + timeout_sec
    while time.time() < deadline:
        if is_tcp_open(host, port):
            return
        time.sleep(1)
    raise RuntimeError(f"timeout waiting for {host}:{port}")


def wait_for_mysql_ready(timeout_sec: int = 60) -> None:
    deadline = time.time() + timeout_sec
    while time.time() < deadline:
        if mysql_ping():
            return
        time.sleep(1)
    raise RuntimeError("timeout waiting for mysql ping")


def start_local_mysql_if_needed() -> subprocess.Popen[str] | None:
    if is_tcp_open("127.0.0.1", 3306):
        return None
    mysqld_path = shutil.which("mysqld") or "/usr/sbin/mysqld"
    if not Path(mysqld_path).exists() or not LOCAL_MYSQL_DATA.exists():
        return None
    LOCAL_MYSQL_RUN.mkdir(parents=True, exist_ok=True)
    LOCAL_MYSQL_TMP.mkdir(parents=True, exist_ok=True)
    for stale_path in [
        LOCAL_MYSQL_RUN / "mysqld.sock",
        LOCAL_MYSQL_RUN / "mysqld.sock.lock",
        LOCAL_MYSQL_RUN / "mysqld.pid",
    ]:
        try:
            stale_path.unlink()
        except FileNotFoundError:
            pass
    log_fp = LOCAL_MYSQL_LOG.open("a", encoding="utf-8")
    process = subprocess.Popen(
        [
            mysqld_path,
            "--user=root",
            "--port=3306",
            "--bind-address=127.0.0.1",
            "--skip-log-bin",
            f"--datadir={LOCAL_MYSQL_DATA}",
            f"--socket={LOCAL_MYSQL_RUN / 'mysqld.sock'}",
            f"--pid-file={LOCAL_MYSQL_RUN / 'mysqld.pid'}",
            f"--log-error={ROOT_DIR / 'tmp' / 'mysql_sys' / 'mysql.log'}",
            f"--tmpdir={LOCAL_MYSQL_TMP}",
            "--mysqlx=0",
        ],
        cwd=str(ROOT_DIR),
        stdout=log_fp,
        stderr=subprocess.STDOUT,
        text=True,
    )
    try:
        wait_for_tcp("127.0.0.1", 3306, timeout_sec=30)
        wait_for_mysql_ready(timeout_sec=60)
    except Exception:
        process.terminate()
        process.wait(timeout=10)
        raise
    return process


def stop_process(process: subprocess.Popen[str] | None) -> None:
    if process is None or process.poll() is not None:
        return
    process.terminate()
    try:
        process.wait(timeout=10)
    except subprocess.TimeoutExpired:
        process.kill()
        process.wait(timeout=5)


def mysql_ping() -> bool:
    result = subprocess.run(
        ["mysqladmin", "--protocol=TCP", "-h127.0.0.1", "-P3306", "-uroot", "ping"],
        cwd=str(ROOT_DIR),
        text=True,
        capture_output=True,
    )
    return result.returncode == 0


def ensure_local_mysql_running(process: subprocess.Popen[str] | None) -> subprocess.Popen[str] | None:
    if mysql_ping():
        return process
    stop_process(process)
    return start_local_mysql_if_needed()


def redis_cli(*args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["redis-cli", "-p", "6379", *args],
        cwd=str(ROOT_DIR),
        text=True,
        capture_output=True,
    )


def ensure_local_redis_writable() -> None:
    redis_cli("CONFIG", "SET", "stop-writes-on-bgsave-error", "no")
    probe = redis_cli("SET", "codex_mysql_persist_tuner_probe", "ok")
    if probe.returncode != 0 or "OK" not in probe.stdout:
        raise RuntimeError(f"redis write probe failed: {probe.stderr.strip() or probe.stdout.strip()}")


def metric_counter_total(
    snapshot: PrometheusSnapshot,
    name: str,
    *,
    label_filters: dict[str, str] | None = None,
) -> float | None:
    rows = snapshot.series(name, label_filters=label_filters)
    if not rows:
        return None
    return sum(value for _, value in rows)


def stage_passed(args: argparse.Namespace, summary: dict[str, Any], error_count: int) -> bool:
    if error_count > args.max_error_count:
        return False
    success = float(summary.get("delivery_success_rate", 0.0) or 0.0)
    p95_ms = float(summary.get("latency", {}).get("p95_ms", 0.0) or 0.0)
    return success >= args.single_success_threshold and (
        args.single_p95_threshold_ms <= 0 or p95_ms <= args.single_p95_threshold_ms
    )


def coalesce_float(value: Any) -> float | None:
    if value is None:
        return None
    try:
        return float(value)
    except (TypeError, ValueError):
        return None


def scale_above(value: float | None, start: float, step: float, weight: float) -> float:
    if value is None or value <= start or step <= 0:
        return 0.0
    return max(0.0, (value - start) / step) * weight


def scale_below(value: float | None, target: float, step: float, weight: float) -> float:
    if value is None or value >= target or step <= 0:
        return 0.0
    return max(0.0, (target - value) / step) * weight


@dataclass(frozen=True)
class MysqlPersistParams:
    max_open_conns: int
    max_idle_conns: int
    worker_count: int
    batch_size: int
    flush_interval_ms: int
    queue_size: int

    def replace(self, dimension: str, value: int) -> "MysqlPersistParams":
        data = asdict(self)
        data[dimension] = value
        if data["max_idle_conns"] > data["max_open_conns"]:
            data["max_idle_conns"] = data["max_open_conns"]
        return MysqlPersistParams(**data)

    def key(self) -> str:
        return (
            f"open{self.max_open_conns}_idle{self.max_idle_conns}"
            f"_worker{self.worker_count}_batch{self.batch_size}"
            f"_flush{self.flush_interval_ms}_queue{self.queue_size}"
        )

    def to_dict(self) -> dict[str, int]:
        return asdict(self)


@dataclass
class TrialResult:
    profile: str
    params: MysqlPersistParams
    config_path: str
    child_run_dir: str | None
    process_returncode: int
    run_failed: bool
    passed: bool
    error_count: int
    throughput: float | None
    success_rate: float | None
    p95_latency_ms: float | None
    p99_latency_ms: float | None
    target_rate: int | None
    actual_offered_rate: float | None
    duration_sec: float | None
    received_messages: float | None
    mysql_persist_stage_avg_ms: float | None
    mysql_persist_stage_p95_ms: float | None
    mysql_wait_count_total: float | None
    mysql_wait_duration_ms_total: float | None
    mysql_wait_ms_per_1k_msgs: float | None
    mysql_open_conns: float | None
    mysql_in_use_conns: float | None
    mysql_in_use_ratio: float | None
    queue_depth_avg: float | None
    queue_depth_p95: float | None
    queue_depth_ratio_p95: float | None
    enqueue_block_avg_ms: float | None
    enqueue_block_p95_ms: float | None
    flush_avg_batch_size: float | None
    flush_p95_batch_size: float | None
    flush_duration_avg_ms: float | None
    flush_duration_p95_ms: float | None
    flush_total: float | None
    flush_failure_total: float | None
    timer_flush_ratio: float | None
    batch_full_flush_ratio: float | None
    single_flush_ratio: float | None
    rss_peak_mb_total: float | None
    threads_peak_total: float | None
    fd_peak_total: float | None
    health_risk_score: float | None
    summary_path: str | None
    metrics_manifest_path: str | None
    note: str | None

    def to_row(self) -> dict[str, Any]:
        row = {
            "profile": self.profile,
            "param_key": self.params.key(),
            "max_open_conns": self.params.max_open_conns,
            "max_idle_conns": self.params.max_idle_conns,
            "worker_count": self.params.worker_count,
            "batch_size": self.params.batch_size,
            "flush_interval_ms": self.params.flush_interval_ms,
            "queue_size": self.params.queue_size,
            "passed": self.passed,
            "run_failed": self.run_failed,
            "throughput": self.throughput,
            "success_rate": self.success_rate,
            "p95_latency_ms": self.p95_latency_ms,
            "p99_latency_ms": self.p99_latency_ms,
            "mysql_persist_stage_p95_ms": self.mysql_persist_stage_p95_ms,
            "mysql_wait_ms_per_1k_msgs": self.mysql_wait_ms_per_1k_msgs,
            "mysql_in_use_ratio": self.mysql_in_use_ratio,
            "queue_depth_p95": self.queue_depth_p95,
            "queue_depth_ratio_p95": self.queue_depth_ratio_p95,
            "enqueue_block_p95_ms": self.enqueue_block_p95_ms,
            "flush_avg_batch_size": self.flush_avg_batch_size,
            "flush_duration_p95_ms": self.flush_duration_p95_ms,
            "timer_flush_ratio": self.timer_flush_ratio,
            "batch_full_flush_ratio": self.batch_full_flush_ratio,
            "single_flush_ratio": self.single_flush_ratio,
            "flush_failure_total": self.flush_failure_total,
            "rss_peak_mb_total": self.rss_peak_mb_total,
            "fd_peak_total": self.fd_peak_total,
            "health_risk_score": self.health_risk_score,
            "child_run_dir": self.child_run_dir,
            "summary_path": self.summary_path,
            "metrics_manifest_path": self.metrics_manifest_path,
            "note": self.note,
        }
        return row


class MysqlPersistParamTuner:
    def __init__(self, args: argparse.Namespace) -> None:
        self.args = args
        self.timestamp = time.strftime("%Y%m%d_%H%M%S")
        label_suffix = f"_{args.label}" if args.label else ""
        self.run_dir = RECORD_ROOT / f"mysql_persist_tuning_{self.timestamp}{label_suffix}"
        self.config_dir = self.run_dir / "generated_configs"
        self.child_run_root = self.run_dir / "child_runs"
        self.trial_dir = self.run_dir / "trials"
        self.best_config_path = self.run_dir / "best_mysql_persist_config.toml"
        self.dimension_results: dict[str, list[TrialResult]] = {}
        self.all_results: dict[str, TrialResult] = {}
        self.eval_counter = 0
        self.mysql_process: subprocess.Popen[str] | None = None

        with Path(args.base_config).open("rb") as fp:
            self.base_config = tomllib.load(fp)
        mysql_config = self.base_config.get("mysqlConfig", {})
        kafka_config = self.base_config.get("kafkaConfig", {})
        self.baseline_params = MysqlPersistParams(
            max_open_conns=int(mysql_config.get("maxOpenConns", 500)),
            max_idle_conns=int(mysql_config.get("maxIdleConns", 100)),
            worker_count=int(kafka_config.get("mysqlPersistWorkerCount", 32)),
            batch_size=int(kafka_config.get("mysqlPersistBatchSize", 256)),
            flush_interval_ms=int(kafka_config.get("mysqlPersistFlushIntervalMs", 5)),
            queue_size=int(kafka_config.get("mysqlPersistQueueSize", 2048)),
        )

    def log(self, message: str) -> None:
        print(f"[{time.strftime('%H:%M:%S')}] {message}", flush=True)

    def build_config(self, params: MysqlPersistParams, config_path: Path) -> None:
        config_copy = json.loads(json.dumps(self.base_config))
        mysql_config = dict(config_copy.get("mysqlConfig", {}))
        mysql_config["maxOpenConns"] = params.max_open_conns
        mysql_config["maxIdleConns"] = min(params.max_idle_conns, params.max_open_conns)
        config_copy["mysqlConfig"] = mysql_config
        kafka_config = dict(config_copy.get("kafkaConfig", {}))
        kafka_config["mysqlPersistWorkerCount"] = params.worker_count
        kafka_config["mysqlPersistBatchSize"] = params.batch_size
        kafka_config["mysqlPersistFlushIntervalMs"] = params.flush_interval_ms
        kafka_config["mysqlPersistQueueSize"] = params.queue_size
        config_copy["kafkaConfig"] = kafka_config
        write_toml(config_path, config_copy)

    def run_child(
        self,
        *,
        params: MysqlPersistParams,
        profile: str,
        initial_target: int,
        duration_sec: int,
        max_messages: int,
        label_suffix: str,
    ) -> tuple[int, str, str, str | None, dict[str, Any] | None]:
        self.eval_counter += 1
        self.mysql_process = ensure_local_mysql_running(self.mysql_process)
        ensure_local_redis_writable()
        trial_name = f"{self.eval_counter:03d}_{profile}_{params.key()}"
        trial_path = self.trial_dir / trial_name
        trial_path.mkdir(parents=True, exist_ok=True)
        config_path = trial_path / "config.toml"
        self.build_config(params, config_path)
        label = f"{self.args.label}_{profile}_{label_suffix}" if self.args.label else f"{profile}_{label_suffix}"
        command = [
            "python3",
            str(SINGLE_CHAT_RUNNER),
            "--mode",
            "kafka",
            "--label",
            label,
            "--base-config",
            str(config_path),
            "--database",
            self.args.database,
            "--seed-prefix",
            self.args.seed_prefix,
            "--instance-ports",
            format_ports(self.args.instance_ports),
            "--client-instance-ports",
            format_ports(self.args.client_instance_ports),
            "--single-pair-count",
            str(self.args.single_pair_count),
            "--single-initial-target",
            str(initial_target),
            "--single-min-duration-sec",
            str(duration_sec),
            "--single-max-messages",
            str(max_messages),
            "--message-timeout-ms",
            str(self.args.message_timeout_ms),
            "--connection-settle-ms",
            str(self.args.connection_settle_ms),
            "--drain-wait-ms",
            str(self.args.drain_wait_ms),
            "--drain-idle-ms",
            str(self.args.drain_idle_ms),
            "--post-run-settle-ms",
            str(self.args.post_run_settle_ms),
            "--single-success-threshold",
            str(self.args.single_success_threshold),
            "--single-p95-threshold-ms",
            str(self.args.single_p95_threshold_ms),
            "--max-error-count",
            str(self.args.max_error_count),
            "--max-expand-steps",
            str(self.args.max_expand_steps),
            "--max-refine-steps",
            str(self.args.max_refine_steps),
            "--refine-resolution",
            str(self.args.refine_resolution),
        ]
        env = os.environ.copy()
        env["ECHOCHAT_RECORD_ROOT_OVERRIDE"] = str(self.child_run_root)
        env["GOCACHE"] = str(GO_BUILD_CACHE_DIR)
        env["GOMODCACHE"] = str(GO_MOD_CACHE_DIR)
        Path(env["ECHOCHAT_RECORD_ROOT_OVERRIDE"]).mkdir(parents=True, exist_ok=True)
        Path(env["GOCACHE"]).mkdir(parents=True, exist_ok=True)
        Path(env["GOMODCACHE"]).mkdir(parents=True, exist_ok=True)
        self.log(
            f"开始评估 {profile} {params.key()} "
            f"(target={initial_target}, duration={duration_sec}s)"
        )
        process = subprocess.Popen(
            command,
            cwd=str(ROOT_DIR),
            env=env,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        sampler = ServerProcessSampler(process.pid, interval_sec=0.2)
        sampler.start()
        stdout, stderr = process.communicate()
        resource_peak = sampler.stop()
        (trial_path / "stdout.log").write_text(stdout, encoding="utf-8")
        (trial_path / "stderr.log").write_text(stderr, encoding="utf-8")
        child_run_dir = self.extract_run_dir(stdout)
        return process.returncode, stdout, stderr, child_run_dir, resource_peak

    def extract_run_dir(self, stdout: str) -> str | None:
        for line in reversed(stdout.splitlines()):
            candidate = line.strip()
            if not candidate:
                continue
            if candidate.startswith("/"):
                path = Path(candidate)
                if path.exists():
                    return str(path)
        return None

    def recover_best_stage(self, run_dir: Path) -> dict[str, Any] | None:
        summary_all_path = run_dir / "summary.json"
        if summary_all_path.exists():
            summary_all = load_json(summary_all_path)
            scenario_payload = summary_all.get("single")
            if scenario_payload:
                return scenario_payload
        scenario_dir = run_dir / "single"
        if not scenario_dir.exists():
            return None
        best: dict[str, Any] | None = None
        for step_dir in sorted(scenario_dir.glob("step_*")):
            summary_path = step_dir / "summary.json"
            if not summary_path.exists():
                continue
            summary = load_json(summary_path)
            errors_path = step_dir / "errors.json"
            error_count = len(load_json(errors_path)) if errors_path.exists() else 0
            passed = stage_passed(self.args, summary, error_count)
            throughput = float(summary.get("observed_throughput_msg_per_sec", 0.0) or 0.0)
            metrics_manifest_path = step_dir / "metrics_manifest.json"
            if not metrics_manifest_path.exists():
                manifest = {"instances": []}
                metrics_dir = step_dir / "metrics"
                if metrics_dir.exists():
                    for metrics_path in sorted(metrics_dir.glob("*.prom")):
                        manifest["instances"].append(
                            {
                                "path": str(metrics_path.relative_to(run_dir)),
                                "port": metrics_path.stem.replace("metrics_", ""),
                                "error": "",
                            }
                        )
                write_json(metrics_manifest_path, manifest)
            candidate = {
                "passed": passed,
                "throughput": throughput,
                "summary": summary,
                "summary_path": str(summary_path.relative_to(run_dir)),
                "metrics_path": str(metrics_manifest_path.relative_to(run_dir)),
            }
            if not passed:
                continue
            if best is None or throughput > float(best["throughput"]):
                best = candidate
        return best

    def analyze_run(
        self,
        *,
        profile: str,
        params: MysqlPersistParams,
        config_path: Path,
        returncode: int,
        child_run_dir: str | None,
        resource_peak: dict[str, Any] | None,
        note: str | None,
    ) -> TrialResult:
        if returncode != 0 or child_run_dir is None:
            return TrialResult(
                profile=profile,
                params=params,
                config_path=str(config_path),
                child_run_dir=child_run_dir,
                process_returncode=returncode,
                run_failed=True,
                passed=False,
                error_count=self.args.max_error_count + 1,
                throughput=None,
                success_rate=None,
                p95_latency_ms=None,
                p99_latency_ms=None,
                target_rate=None,
                actual_offered_rate=None,
                duration_sec=None,
                received_messages=None,
                mysql_persist_stage_avg_ms=None,
                mysql_persist_stage_p95_ms=None,
                mysql_wait_count_total=None,
                mysql_wait_duration_ms_total=None,
                mysql_wait_ms_per_1k_msgs=None,
                mysql_open_conns=None,
                mysql_in_use_conns=None,
                mysql_in_use_ratio=None,
                queue_depth_avg=None,
                queue_depth_p95=None,
                queue_depth_ratio_p95=None,
                enqueue_block_avg_ms=None,
                enqueue_block_p95_ms=None,
                flush_avg_batch_size=None,
                flush_p95_batch_size=None,
                flush_duration_avg_ms=None,
                flush_duration_p95_ms=None,
                flush_total=None,
                flush_failure_total=None,
                timer_flush_ratio=None,
                batch_full_flush_ratio=None,
                single_flush_ratio=None,
                rss_peak_mb_total=coalesce_float((resource_peak or {}).get("rss_peak_mb_total")),
                threads_peak_total=coalesce_float((resource_peak or {}).get("threads_peak_total")),
                fd_peak_total=coalesce_float((resource_peak or {}).get("fd_peak_total")),
                health_risk_score=None,
                summary_path=None,
                metrics_manifest_path=None,
                note=note or "child runner failed",
            )

        run_dir = Path(child_run_dir)
        best_stage = self.recover_best_stage(run_dir)
        if not best_stage:
            return TrialResult(
                profile=profile,
                params=params,
                config_path=str(config_path),
                child_run_dir=child_run_dir,
                process_returncode=returncode,
                run_failed=True,
                passed=False,
                error_count=self.args.max_error_count + 1,
                throughput=None,
                success_rate=None,
                p95_latency_ms=None,
                p99_latency_ms=None,
                target_rate=None,
                actual_offered_rate=None,
                duration_sec=None,
                received_messages=None,
                mysql_persist_stage_avg_ms=None,
                mysql_persist_stage_p95_ms=None,
                mysql_wait_count_total=None,
                mysql_wait_duration_ms_total=None,
                mysql_wait_ms_per_1k_msgs=None,
                mysql_open_conns=None,
                mysql_in_use_conns=None,
                mysql_in_use_ratio=None,
                queue_depth_avg=None,
                queue_depth_p95=None,
                queue_depth_ratio_p95=None,
                enqueue_block_avg_ms=None,
                enqueue_block_p95_ms=None,
                flush_avg_batch_size=None,
                flush_p95_batch_size=None,
                flush_duration_avg_ms=None,
                flush_duration_p95_ms=None,
                flush_total=None,
                flush_failure_total=None,
                timer_flush_ratio=None,
                batch_full_flush_ratio=None,
                single_flush_ratio=None,
                rss_peak_mb_total=coalesce_float((resource_peak or {}).get("rss_peak_mb_total")),
                threads_peak_total=coalesce_float((resource_peak or {}).get("threads_peak_total")),
                fd_peak_total=coalesce_float((resource_peak or {}).get("fd_peak_total")),
                health_risk_score=None,
                summary_path=None,
                metrics_manifest_path=None,
                note=note or "no passing single stage found",
            )

        summary = best_stage["summary"]
        summary_path = best_stage["summary_path"]
        metrics_manifest_path = best_stage["metrics_path"]
        metrics_manifest = load_json(run_dir / metrics_manifest_path)
        metric_files = []
        for item in metrics_manifest.get("instances", []):
            relative = item.get("path")
            if relative:
                metric_files.append(run_dir / relative)
        snapshot = PrometheusSnapshot.from_files(metric_files)

        total_max_open = params.max_open_conns * max(1, len(self.args.instance_ports))
        mysql_open = snapshot.gauge_sum("echochat_mysql_open_connections")
        mysql_in_use = snapshot.gauge_sum("echochat_mysql_in_use_connections")
        mysql_wait_count = snapshot.gauge_sum("echochat_mysql_wait_count_total")
        mysql_wait_duration_s = snapshot.gauge_sum("echochat_mysql_wait_duration_seconds")
        mysql_in_use_ratio = None
        if mysql_in_use is not None and total_max_open > 0:
            mysql_in_use_ratio = mysql_in_use / float(total_max_open)

        stage_hist = snapshot.histogram_summary(
            "echochat_kafka_consumer_stage_duration_seconds",
            label_filters={"stage": "mysql_persist"},
        )
        queue_hist = snapshot.histogram_summary("echochat_mysql_persist_queue_depth")
        enqueue_hist = snapshot.histogram_summary("echochat_mysql_persist_enqueue_block_duration_seconds")
        flush_batch_hist = snapshot.histogram_summary("echochat_mysql_persist_flush_batch_size")
        flush_duration_hist = snapshot.histogram_summary(
            "echochat_mysql_persist_flush_duration_seconds",
            exclude_labels={"result": "failure"},
        )

        flush_total = metric_counter_total(snapshot, "echochat_mysql_persist_flush_total")
        flush_failure_total = metric_counter_total(
            snapshot,
            "echochat_mysql_persist_flush_total",
            label_filters={"result": "failure"},
        )
        timer_flush_total = metric_counter_total(
            snapshot,
            "echochat_mysql_persist_flush_total",
            label_filters={"reason": "timer", "result": "success"},
        )
        batch_full_flush_total = metric_counter_total(
            snapshot,
            "echochat_mysql_persist_flush_total",
            label_filters={"reason": "batch_full", "result": "success"},
        )
        single_flush_total = metric_counter_total(
            snapshot,
            "echochat_mysql_persist_flush_total",
            label_filters={"reason": "single", "result": "success"},
        )

        success_rate = coalesce_float(summary.get("delivery_success_rate"))
        p95_latency_ms = coalesce_float(summary.get("latency", {}).get("p95_ms"))
        p99_latency_ms = coalesce_float(summary.get("latency", {}).get("p99_ms"))
        throughput = coalesce_float(summary.get("observed_throughput_msg_per_sec"))
        received_messages = coalesce_float(summary.get("received_messages"))
        mysql_wait_duration_ms_total = mysql_wait_duration_s * 1000 if mysql_wait_duration_s is not None else None
        mysql_wait_ms_per_1k_msgs = None
        if mysql_wait_duration_ms_total is not None and received_messages not in (None, 0):
            mysql_wait_ms_per_1k_msgs = mysql_wait_duration_ms_total / float(received_messages) * 1000

        queue_depth_ratio_p95 = None
        if queue_hist["p95"] is not None and params.queue_size > 0:
            queue_depth_ratio_p95 = queue_hist["p95"] / float(params.queue_size)

        timer_flush_ratio = None
        batch_full_flush_ratio = None
        single_flush_ratio = None
        if flush_total not in (None, 0):
            timer_flush_ratio = (timer_flush_total or 0.0) / flush_total
            batch_full_flush_ratio = (batch_full_flush_total or 0.0) / flush_total
            single_flush_ratio = (single_flush_total or 0.0) / flush_total

        error_count = 0
        best_summary_path = run_dir / summary_path
        errors_path = best_summary_path.parent / "errors.json"
        if errors_path.exists():
            error_count = len(load_json(errors_path))

        passed = bool(best_stage.get("passed", stage_passed(self.args, summary, error_count)))
        result = TrialResult(
            profile=profile,
            params=params,
            config_path=str(config_path),
            child_run_dir=child_run_dir,
            process_returncode=returncode,
            run_failed=False,
            passed=passed,
            error_count=error_count,
            throughput=round_or_none(throughput),
            success_rate=round_or_none(success_rate),
            p95_latency_ms=round_or_none(p95_latency_ms),
            p99_latency_ms=round_or_none(p99_latency_ms),
            target_rate=int(best_stage.get("target_rate")) if best_stage.get("target_rate") is not None else None,
            actual_offered_rate=round_or_none(coalesce_float(best_stage.get("actual_offered_rate"))),
            duration_sec=round_or_none(coalesce_float(summary.get("duration_sec"))),
            received_messages=round_or_none(received_messages),
            mysql_persist_stage_avg_ms=round_or_none(stage_hist["avg"] * 1000 if stage_hist["avg"] is not None else None),
            mysql_persist_stage_p95_ms=round_or_none(stage_hist["p95"] * 1000 if stage_hist["p95"] is not None else None),
            mysql_wait_count_total=round_or_none(mysql_wait_count),
            mysql_wait_duration_ms_total=round_or_none(mysql_wait_duration_ms_total),
            mysql_wait_ms_per_1k_msgs=round_or_none(mysql_wait_ms_per_1k_msgs),
            mysql_open_conns=round_or_none(mysql_open),
            mysql_in_use_conns=round_or_none(mysql_in_use),
            mysql_in_use_ratio=round_or_none(mysql_in_use_ratio),
            queue_depth_avg=round_or_none(queue_hist["avg"]),
            queue_depth_p95=round_or_none(queue_hist["p95"]),
            queue_depth_ratio_p95=round_or_none(queue_depth_ratio_p95),
            enqueue_block_avg_ms=round_or_none(enqueue_hist["avg"] * 1000 if enqueue_hist["avg"] is not None else None),
            enqueue_block_p95_ms=round_or_none(enqueue_hist["p95"] * 1000 if enqueue_hist["p95"] is not None else None),
            flush_avg_batch_size=round_or_none(flush_batch_hist["avg"]),
            flush_p95_batch_size=round_or_none(flush_batch_hist["p95"]),
            flush_duration_avg_ms=round_or_none(flush_duration_hist["avg"] * 1000 if flush_duration_hist["avg"] is not None else None),
            flush_duration_p95_ms=round_or_none(flush_duration_hist["p95"] * 1000 if flush_duration_hist["p95"] is not None else None),
            flush_total=round_or_none(flush_total),
            flush_failure_total=round_or_none(flush_failure_total),
            timer_flush_ratio=round_or_none(timer_flush_ratio),
            batch_full_flush_ratio=round_or_none(batch_full_flush_ratio),
            single_flush_ratio=round_or_none(single_flush_ratio),
            rss_peak_mb_total=coalesce_float((resource_peak or {}).get("rss_peak_mb_total")),
            threads_peak_total=coalesce_float((resource_peak or {}).get("threads_peak_total")),
            fd_peak_total=coalesce_float((resource_peak or {}).get("fd_peak_total")),
            health_risk_score=None,
            summary_path=summary_path,
            metrics_manifest_path=metrics_manifest_path,
            note=note,
        )
        result.health_risk_score = round_or_none(self.compute_health_risk(result))
        return result

    def compute_health_risk(self, result: TrialResult) -> float:
        if result.run_failed:
            return 9999.0
        penalty = 0.0
        if not result.passed:
            penalty += 200.0
        penalty += scale_above(result.p95_latency_ms, 650, 100, 1.4)
        penalty += scale_above(result.mysql_persist_stage_p95_ms, 40, 10, 1.0)
        penalty += scale_above(result.mysql_wait_ms_per_1k_msgs, 8, 4, 1.5)
        penalty += scale_above(result.mysql_in_use_ratio, 0.75, 0.05, 1.0)
        penalty += scale_above(result.queue_depth_ratio_p95, 0.35, 0.1, 1.2)
        penalty += scale_above(result.enqueue_block_p95_ms, 1.0, 0.5, 1.0)
        penalty += scale_above(result.flush_duration_p95_ms, 8, 4, 0.8)
        penalty += scale_above(result.timer_flush_ratio, 0.35, 0.1, 0.6)
        penalty += scale_below(result.flush_avg_batch_size, result.params.batch_size * 0.45, max(1.0, result.params.batch_size * 0.1), 0.8)
        penalty += scale_above(result.flush_failure_total, 0, 1, 50.0)
        return penalty

    def cache_key(self, profile: str, params: MysqlPersistParams) -> str:
        return f"{profile}:{params.key()}"

    def evaluate(
        self,
        *,
        params: MysqlPersistParams,
        profile: str,
        initial_target: int,
        duration_sec: int,
        max_messages: int,
        label_suffix: str,
        note: str | None = None,
    ) -> TrialResult:
        key = self.cache_key(profile, params)
        if key in self.all_results:
            return self.all_results[key]
        returncode, stdout, stderr, child_run_dir, resource_peak = self.run_child(
            params=params,
            profile=profile,
            initial_target=initial_target,
            duration_sec=duration_sec,
            max_messages=max_messages,
            label_suffix=label_suffix,
        )
        config_path = self.trial_dir / f"{self.eval_counter:03d}_{profile}_{params.key()}" / "config.toml"
        trial_result = self.analyze_run(
            profile=profile,
            params=params,
            config_path=config_path,
            returncode=returncode,
            child_run_dir=child_run_dir,
            resource_peak=resource_peak,
            note=note or (stderr.strip().splitlines()[-1] if stderr.strip() else None),
        )
        self.all_results[key] = trial_result
        write_json(
            self.trial_dir / f"{self.eval_counter:03d}_{profile}_{params.key()}" / "result.json",
            trial_result.to_row(),
        )
        self.log(
            f"完成 {profile} {params.key()} "
            f"throughput={trial_result.throughput} p95={trial_result.p95_latency_ms} "
            f"risk={trial_result.health_risk_score} passed={trial_result.passed}"
        )
        return trial_result

    def is_hard_bad(self, result: TrialResult) -> bool:
        if result.run_failed or not result.passed:
            return True
        if (result.flush_failure_total or 0) > 0:
            return True
        if (result.queue_depth_ratio_p95 or 0) >= 0.95:
            return True
        if (result.mysql_wait_ms_per_1k_msgs or 0) >= 60:
            return True
        return False

    def better(self, left: TrialResult, right: TrialResult) -> TrialResult:
        left_bad = self.is_hard_bad(left)
        right_bad = self.is_hard_bad(right)
        if left_bad != right_bad:
            return right if not right_bad else left
        left_tp = left.throughput or 0.0
        right_tp = right.throughput or 0.0
        base = max(left_tp, right_tp, 1.0)
        diff_pct = abs(left_tp - right_tp) / base * 100
        if diff_pct > self.args.throughput_tie_pct:
            return left if left_tp >= right_tp else right
        left_risk = left.health_risk_score if left.health_risk_score is not None else math.inf
        right_risk = right.health_risk_score if right.health_risk_score is not None else math.inf
        if abs(left_risk - right_risk) > 0.5:
            return left if left_risk <= right_risk else right
        left_p95 = left.p95_latency_ms or math.inf
        right_p95 = right.p95_latency_ms or math.inf
        if abs(left_p95 - right_p95) > 20:
            return left if left_p95 <= right_p95 else right
        return left if left_tp >= right_tp else right

    def choose_best(self, results: list[TrialResult]) -> TrialResult:
        best = results[0]
        for item in results[1:]:
            best = self.better(best, item)
        return best

    def sorted_unique_values(self, dimension: str, current_value: int) -> list[int]:
        values = set(self.args.candidates[dimension])
        values.add(current_value)
        if dimension == "max_idle_conns":
            values = {value for value in values if value <= self.baseline_params.max_open_conns or value <= max(self.args.candidates["max_open_conns"])}
        return sorted(values)

    def midpoint_value(self, dimension: str, left: int, right: int) -> int | None:
        if right <= left:
            return None
        step = REFINE_STEP[dimension]
        raw = (left + right) / 2.0
        value = int(round(raw / step) * step)
        if value <= left or value >= right:
            return None
        return value

    def evaluate_dimension_value(
        self,
        *,
        base_params: MysqlPersistParams,
        dimension: str,
        value: int,
        bucket: list[TrialResult],
    ) -> TrialResult:
        params = base_params.replace(dimension, value)
        if dimension == "max_idle_conns" and params.max_idle_conns > params.max_open_conns:
            params = params.replace("max_idle_conns", params.max_open_conns)
        result = self.evaluate(
            params=params,
            profile="tune",
            initial_target=self.args.single_initial_target,
            duration_sec=self.args.single_min_duration_sec,
            max_messages=self.args.single_max_messages,
            label_suffix=f"{dimension}_{value}",
            note=f"dimension={dimension}",
        )
        bucket.append(result)
        return result

    def tune_dimension(self, dimension: str, current_best: TrialResult) -> TrialResult:
        current_value = current_best.params.to_dict()[dimension]
        values = self.sorted_unique_values(dimension, current_value)
        if dimension == "max_idle_conns":
            values = [value for value in values if value <= current_best.params.max_open_conns]
            if current_best.params.max_idle_conns not in values:
                values.append(current_best.params.max_idle_conns)
                values = sorted(set(values))
        bucket = self.dimension_results.setdefault(dimension, [])
        self.log(f"开始调 {DIMENSION_LABELS[dimension]} 候选={values}")

        lo = 0
        hi = len(values) - 1
        while hi - lo > 2:
            mid = (lo + hi) // 2
            left_result = self.evaluate_dimension_value(
                base_params=current_best.params,
                dimension=dimension,
                value=values[mid],
                bucket=bucket,
            )
            right_result = self.evaluate_dimension_value(
                base_params=current_best.params,
                dimension=dimension,
                value=values[mid + 1],
                bucket=bucket,
            )
            if self.better(left_result, right_result) is right_result:
                lo = mid + 1
            else:
                hi = mid

        final_results: list[TrialResult] = list(bucket)
        for index in range(lo, hi + 1):
            final_results.append(
                self.evaluate_dimension_value(
                    base_params=current_best.params,
                    dimension=dimension,
                    value=values[index],
                    bucket=bucket,
                )
            )

        winner = self.choose_best(final_results)
        winner_value = winner.params.to_dict()[dimension]
        winner_index = values.index(winner_value) if winner_value in values else -1
        extra_values: set[int] = set()
        if winner_index > 0:
            midpoint = self.midpoint_value(dimension, values[winner_index - 1], winner_value)
            if midpoint is not None:
                extra_values.add(midpoint)
        if winner_index >= 0 and winner_index < len(values) - 1:
            midpoint = self.midpoint_value(dimension, winner_value, values[winner_index + 1])
            if midpoint is not None:
                extra_values.add(midpoint)
        if dimension == "max_idle_conns":
            extra_values = {value for value in extra_values if value <= current_best.params.max_open_conns}
        for value in sorted(extra_values):
            final_results.append(
                self.evaluate_dimension_value(
                    base_params=current_best.params,
                    dimension=dimension,
                    value=value,
                    bucket=bucket,
                )
            )
        winner = self.choose_best(final_results)
        self.log(
            f"{DIMENSION_LABELS[dimension]} 暂定最优={winner.params.to_dict()[dimension]} "
            f"throughput={winner.throughput} risk={winner.health_risk_score}"
        )
        return winner

    def confirm(self, candidate: TrialResult, suffix: str) -> TrialResult:
        initial_target = self.args.confirm_initial_target
        if candidate.throughput is not None:
            initial_target = max(self.args.single_initial_target, int(candidate.throughput * 0.9))
        return self.evaluate(
            params=candidate.params,
            profile="confirm",
            initial_target=initial_target,
            duration_sec=self.args.confirm_duration_sec,
            max_messages=self.args.confirm_max_messages,
            label_suffix=suffix,
            note="final confirmation",
        )

    def build_report(
        self,
        baseline: TrialResult,
        tuned_best: TrialResult,
        confirmed: list[TrialResult],
        final_best: TrialResult,
    ) -> str:
        lines = [
            "# mysql_persist 参数长任务调优报告",
            "",
            f"- 生成时间：`{self.timestamp}`",
            f"- Git commit：`{git_commit()}`",
            f"- Git dirty：`{git_dirty()}`",
            f"- 基线配置：`{self.args.base_config}`",
            f"- 口径：单聊专用 runner，除 mysql_persist 相关参数外全部沿用基线默认值。",
            f"- 评估主轴：吞吐优先；同吞吐或近似吞吐时，再看 p95、MySQL wait、queue 压力、flush 形态与资源副作用。",
            "",
            "## 最终推荐",
            "",
            f"- 参数：`{final_best.params.to_dict()}`",
            f"- 吞吐：`{final_best.throughput}` msg/s",
            f"- p95：`{final_best.p95_latency_ms}` ms",
            f"- MySQL wait：`{final_best.mysql_wait_ms_per_1k_msgs}` ms/1k msgs",
            f"- queue p95：`{final_best.queue_depth_p95}`（占队列 `{final_best.queue_depth_ratio_p95}`）",
            f"- flush avg batch：`{final_best.flush_avg_batch_size}`，timer ratio：`{final_best.timer_flush_ratio}`",
            f"- 确认 run：`{final_best.child_run_dir}`",
            "",
            "## 基线对比",
            "",
            "| 版本 | throughput | p95_ms | mysql_wait_ms_per_1k | queue_ratio_p95 | flush_avg_batch | timer_ratio | risk |",
            "| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |",
            f"| baseline | {baseline.throughput} | {baseline.p95_latency_ms} | {baseline.mysql_wait_ms_per_1k_msgs} | {baseline.queue_depth_ratio_p95} | {baseline.flush_avg_batch_size} | {baseline.timer_flush_ratio} | {baseline.health_risk_score} |",
            f"| tuned_best | {tuned_best.throughput} | {tuned_best.p95_latency_ms} | {tuned_best.mysql_wait_ms_per_1k_msgs} | {tuned_best.queue_depth_ratio_p95} | {tuned_best.flush_avg_batch_size} | {tuned_best.timer_flush_ratio} | {tuned_best.health_risk_score} |",
            f"| final_confirmed | {final_best.throughput} | {final_best.p95_latency_ms} | {final_best.mysql_wait_ms_per_1k_msgs} | {final_best.queue_depth_ratio_p95} | {final_best.flush_avg_batch_size} | {final_best.timer_flush_ratio} | {final_best.health_risk_score} |",
            "",
            "## 维度扫描结果",
            "",
        ]
        for dimension in DIMENSION_ORDER:
            rows = self.dimension_results.get(dimension, [])
            if not rows:
                continue
            lines.append(f"### {DIMENSION_LABELS[dimension]}")
            lines.append("")
            lines.append("| value | throughput | p95_ms | wait_ms_per_1k | queue_ratio_p95 | flush_avg_batch | timer_ratio | risk | passed |")
            lines.append("| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |")
            seen: dict[int, TrialResult] = {}
            for item in rows:
                value = item.params.to_dict()[dimension]
                current = seen.get(value)
                if current is None or self.better(current, item) is item:
                    seen[value] = item
            for value in sorted(seen):
                item = seen[value]
                lines.append(
                    f"| {value} | {item.throughput} | {item.p95_latency_ms} | {item.mysql_wait_ms_per_1k_msgs} | "
                    f"{item.queue_depth_ratio_p95} | {item.flush_avg_batch_size} | {item.timer_flush_ratio} | "
                    f"{item.health_risk_score} | {item.passed} |"
                )
            lines.append("")

        lines.extend(
            [
                "## 最终确认集",
                "",
                "| profile | params | throughput | p95_ms | wait_ms_per_1k | queue_ratio_p95 | risk | run |",
                "| --- | --- | ---: | ---: | ---: | ---: | ---: | --- |",
            ]
        )
        for item in confirmed:
            lines.append(
                f"| {item.profile} | `{item.params.to_dict()}` | {item.throughput} | {item.p95_latency_ms} | "
                f"{item.mysql_wait_ms_per_1k_msgs} | {item.queue_depth_ratio_p95} | {item.health_risk_score} | "
                f"`{item.child_run_dir}` |"
            )
        lines.append("")
        return "\n".join(lines) + "\n"

    def write_checkpoint(self) -> None:
        rows = [item.to_row() for item in self.all_results.values()]
        write_csv(self.run_dir / "trial_scorecard.csv", rows)
        write_json(self.run_dir / "trial_scorecard.json", rows)

    def run(self) -> TrialResult:
        self.run_dir.mkdir(parents=True, exist_ok=True)
        self.config_dir.mkdir(parents=True, exist_ok=True)
        self.child_run_root.mkdir(parents=True, exist_ok=True)
        self.trial_dir.mkdir(parents=True, exist_ok=True)

        write_json(
            self.run_dir / "metadata.json",
            {
                "generated_at": self.timestamp,
                "git_commit": git_commit(),
                "git_dirty": git_dirty(),
                "base_config": str(Path(self.args.base_config).resolve()),
                "baseline_params": self.baseline_params.to_dict(),
                "instance_ports": self.args.instance_ports,
                "client_instance_ports": self.args.client_instance_ports,
                "candidate_sets": self.args.candidates,
                "single_initial_target": self.args.single_initial_target,
                "single_min_duration_sec": self.args.single_min_duration_sec,
                "confirm_duration_sec": self.args.confirm_duration_sec,
                "throughput_tie_pct": self.args.throughput_tie_pct,
            },
        )

        baseline = self.evaluate(
            params=self.baseline_params,
            profile="baseline",
            initial_target=self.args.single_initial_target,
            duration_sec=self.args.single_min_duration_sec,
            max_messages=self.args.single_max_messages,
            label_suffix="baseline",
            note="baseline",
        )
        self.write_checkpoint()

        current_best = baseline
        for dimension in DIMENSION_ORDER:
            current_best = self.tune_dimension(dimension, current_best)
            self.write_checkpoint()

        tuned_best = current_best
        confirm_candidates = [baseline]
        if tuned_best.params != baseline.params:
            confirm_candidates.append(tuned_best)
        confirmed: list[TrialResult] = []
        seen_param_keys: set[str] = set()
        for index, candidate in enumerate(confirm_candidates, start=1):
            if candidate.params.key() in seen_param_keys:
                continue
            seen_param_keys.add(candidate.params.key())
            confirmed.append(self.confirm(candidate, f"confirm_{index}"))
            self.write_checkpoint()

        final_best = self.choose_best(confirmed if confirmed else [tuned_best, baseline])
        self.build_config(final_best.params, self.best_config_path)
        report = self.build_report(baseline, tuned_best, confirmed, final_best)
        (self.run_dir / "report.md").write_text(report, encoding="utf-8")
        write_json(
            self.run_dir / "final_summary.json",
            {
                "baseline": baseline.to_row(),
                "tuned_best": tuned_best.to_row(),
                "confirmed": [item.to_row() for item in confirmed],
                "final_best": final_best.to_row(),
                "best_config_path": str(self.best_config_path),
            },
        )
        self.log(f"调优完成，最终推荐={final_best.params.to_dict()}")
        return final_best


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Tune mysql_persist parameters using the dedicated single-chat runner.")
    parser.add_argument("--label", default="single_chat_mysql_persist")
    parser.add_argument("--base-config", default=str(DEFAULT_BASE_CONFIG))
    parser.add_argument("--database", default="echochat")
    parser.add_argument("--seed-prefix", default="K6")
    parser.add_argument("--instance-ports", type=parse_ports, default=[18082, 18083, 18084, 18085, 18086, 18087, 18088, 18089, 18090, 18091])
    parser.add_argument("--client-instance-ports", type=parse_ports, default=[18082, 18083, 18084, 18085, 18086, 18087, 18088, 18089, 18090, 18091])
    parser.add_argument("--single-pair-count", type=int, default=60)
    parser.add_argument("--single-initial-target", type=int, default=1500)
    parser.add_argument("--single-min-duration-sec", type=int, default=20)
    parser.add_argument("--single-max-messages", type=int, default=5000)
    parser.add_argument("--confirm-initial-target", type=int, default=1500)
    parser.add_argument("--confirm-duration-sec", type=int, default=45)
    parser.add_argument("--confirm-max-messages", type=int, default=10000)
    parser.add_argument("--message-timeout-ms", type=int, default=60000)
    parser.add_argument("--connection-settle-ms", type=int, default=1500)
    parser.add_argument("--drain-wait-ms", type=int, default=5000)
    parser.add_argument("--drain-idle-ms", type=int, default=1000)
    parser.add_argument("--post-run-settle-ms", type=int, default=1000)
    parser.add_argument("--single-success-threshold", type=float, default=0.995)
    parser.add_argument("--single-p95-threshold-ms", type=float, default=1000.0)
    parser.add_argument("--max-error-count", type=int, default=0)
    parser.add_argument("--max-expand-steps", type=int, default=4)
    parser.add_argument("--max-refine-steps", type=int, default=6)
    parser.add_argument("--refine-resolution", type=int, default=20)
    parser.add_argument("--throughput-tie-pct", type=float, default=1.5)
    parser.add_argument("--max-open-conns-candidates", default="")
    parser.add_argument("--max-idle-conns-candidates", default="")
    parser.add_argument("--worker-count-candidates", default="")
    parser.add_argument("--batch-size-candidates", default="")
    parser.add_argument("--flush-interval-candidates", default="")
    parser.add_argument("--queue-size-candidates", default="")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    args.candidates = {
        "max_open_conns": parse_int_list(args.max_open_conns_candidates, DEFAULT_CANDIDATES["max_open_conns"]),
        "max_idle_conns": parse_int_list(args.max_idle_conns_candidates, DEFAULT_CANDIDATES["max_idle_conns"]),
        "worker_count": parse_int_list(args.worker_count_candidates, DEFAULT_CANDIDATES["worker_count"]),
        "batch_size": parse_int_list(args.batch_size_candidates, DEFAULT_CANDIDATES["batch_size"]),
        "flush_interval_ms": parse_int_list(args.flush_interval_candidates, DEFAULT_CANDIDATES["flush_interval_ms"]),
        "queue_size": parse_int_list(args.queue_size_candidates, DEFAULT_CANDIDATES["queue_size"]),
    }
    tuner = MysqlPersistParamTuner(args)
    try:
        final_best = tuner.run()
    finally:
        stop_process(tuner.mysql_process)
    print(json.dumps({"run_dir": str(tuner.run_dir), "final_best": final_best.params.to_dict()}, ensure_ascii=False))


if __name__ == "__main__":
    main()
