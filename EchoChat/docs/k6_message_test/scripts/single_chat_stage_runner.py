#!/usr/bin/env python3

from __future__ import annotations

import argparse
import os
import shutil
import socket
import subprocess
import sys
import time
from pathlib import Path


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
SCRIPT_DIR = ROOT_DIR / "docs" / "k6_message_test" / "scripts"
CAPACITY_RUNNER = SCRIPT_DIR / "throughput_capacity_runner.py"
DEFAULT_BASE_CONFIG = ROOT_DIR / "configs" / "config_local_singlebroker_part240_mysqlpersist_tune2.toml"
DEFAULT_INSTANCE_PORTS = [18082, 18083, 18084, 18085, 18086, 18087, 18088, 18089, 18090, 18091]
LOCAL_MYSQL_ROOT = detect_local_mysql_root(ROOT_DIR)
LOCAL_MYSQL_DATA = LOCAL_MYSQL_ROOT / "data"
LOCAL_MYSQL_RUN = LOCAL_MYSQL_ROOT / "run"
LOCAL_MYSQL_TMP = Path("/run/user/0/echochat_mysql_tmp")
LOCAL_MYSQL_LOG = LOCAL_MYSQL_ROOT / "mysql-foreground.log"


def parse_ports(value: str) -> list[int]:
    if not value:
        return []
    return [int(item.strip()) for item in value.split(",") if item.strip()]


def format_ports(ports: list[int]) -> str:
    return ",".join(str(port) for port in ports)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Dedicated single-chat throughput entry using the historical capacity-search methodology."
    )
    parser.add_argument("--mode", choices=["channel", "kafka"], default="kafka")
    parser.add_argument("--label", default="single_dedicated_1broker_10consumer_tune2_part240")
    parser.add_argument("--base-config", default=str(DEFAULT_BASE_CONFIG))
    parser.add_argument("--database", default="echochat")
    parser.add_argument("--seed-prefix", default="K6")
    parser.add_argument("--instance-ports", type=parse_ports, default=DEFAULT_INSTANCE_PORTS)
    parser.add_argument("--client-instance-ports", type=parse_ports, default=DEFAULT_INSTANCE_PORTS)
    parser.add_argument("--single-pair-count", type=int, default=60)
    parser.add_argument("--single-initial-target", type=int, default=120)
    parser.add_argument("--single-min-duration-sec", type=int, default=8)
    parser.add_argument("--single-max-messages", type=int, default=5000)
    parser.add_argument("--message-timeout-ms", type=int, default=60000)
    parser.add_argument("--connection-settle-ms", type=int, default=1500)
    parser.add_argument("--drain-wait-ms", type=int, default=5000)
    parser.add_argument("--drain-idle-ms", type=int, default=1000)
    parser.add_argument("--post-run-settle-ms", type=int, default=1000)
    parser.add_argument("--single-success-threshold", type=float, default=0.995)
    parser.add_argument("--single-p95-threshold-ms", type=float, default=1000.0)
    parser.add_argument("--max-error-count", type=int, default=0)
    parser.add_argument("--max-expand-steps", type=int, default=8)
    parser.add_argument("--max-refine-steps", type=int, default=6)
    parser.add_argument("--refine-resolution", type=int, default=10)
    parser.add_argument("--skip-auto-mysql", action="store_true")
    return parser.parse_args()


def build_command(args: argparse.Namespace) -> list[str]:
    instance_ports = args.instance_ports or DEFAULT_INSTANCE_PORTS
    client_instance_ports = args.client_instance_ports or instance_ports
    return [
        "python3",
        str(CAPACITY_RUNNER),
        "--mode",
        args.mode,
        "--label",
        args.label,
        "--base-config",
        args.base_config,
        "--database",
        args.database,
        "--seed-prefix",
        args.seed_prefix,
        "--port",
        str(instance_ports[0]),
        "--instance-ports",
        format_ports(instance_ports),
        "--client-instance-ports",
        format_ports(client_instance_ports),
        "--single-pair-count",
        str(args.single_pair_count),
        "--single-initial-target",
        str(args.single_initial_target),
        "--single-min-duration-sec",
        str(args.single_min_duration_sec),
        "--single-max-messages",
        str(args.single_max_messages),
        "--message-timeout-ms",
        str(args.message_timeout_ms),
        "--connection-settle-ms",
        str(args.connection_settle_ms),
        "--drain-wait-ms",
        str(args.drain_wait_ms),
        "--drain-idle-ms",
        str(args.drain_idle_ms),
        "--post-run-settle-ms",
        str(args.post_run_settle_ms),
        "--single-success-threshold",
        str(args.single_success_threshold),
        "--single-p95-threshold-ms",
        str(args.single_p95_threshold_ms),
        "--max-error-count",
        str(args.max_error_count),
        "--max-expand-steps",
        str(args.max_expand_steps),
        "--max-refine-steps",
        str(args.max_refine_steps),
        "--refine-resolution",
        str(args.refine_resolution),
        "--single-only",
    ]


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


def mysql_ping() -> bool:
    result = subprocess.run(
        ["mysqladmin", "--protocol=TCP", "-h127.0.0.1", "-P3306", "-uroot", "ping"],
        cwd=str(ROOT_DIR),
        text=True,
        capture_output=True,
    )
    return result.returncode == 0


def wait_for_mysql_ready(timeout_sec: int = 60) -> None:
    deadline = time.time() + timeout_sec
    while time.time() < deadline:
        if mysql_ping():
            return
        time.sleep(1)
    raise RuntimeError("timeout waiting for mysql ping")


def start_local_mysql_if_needed(args: argparse.Namespace) -> subprocess.Popen[str] | None:
    if args.skip_auto_mysql or is_tcp_open("127.0.0.1", 3306):
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


def main() -> None:
    args = parse_args()
    command = build_command(args)
    env = os.environ.copy()
    env.setdefault("ECHOCHAT_RECORD_ROOT_OVERRIDE", str(ROOT_DIR / "tmp" / "k6_records"))
    env.setdefault("GOCACHE", str(ROOT_DIR / "tmp" / "go-build-cache"))
    env.setdefault("GOMODCACHE", str(ROOT_DIR / "tmp" / "go-mod-cache"))
    Path(env["ECHOCHAT_RECORD_ROOT_OVERRIDE"]).mkdir(parents=True, exist_ok=True)
    Path(env["GOCACHE"]).mkdir(parents=True, exist_ok=True)
    Path(env["GOMODCACHE"]).mkdir(parents=True, exist_ok=True)
    mysql_process = start_local_mysql_if_needed(args)
    try:
        result = subprocess.run(command, cwd=str(ROOT_DIR), env=env, text=True)
    finally:
        stop_process(mysql_process)
    if result.returncode != 0:
        raise SystemExit(result.returncode)


if __name__ == "__main__":
    main()
