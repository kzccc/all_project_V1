#!/usr/bin/env python3
from __future__ import annotations

import argparse
from collections import Counter
import copy
import datetime as dt
import json
import os
from pathlib import Path
import random
import shutil
import socket
import subprocess
import sys
import time
import signal
import re
import tempfile
import tomllib
from typing import Callable

import requests
from tqdm import tqdm


ROOT = Path(__file__).resolve().parents[2]


def load_report_helpers():
    from report_helpers import analyze as analyze_reports
    from report_helpers import build_reports as build_extended_reports
    from report_helpers import export_analysis as export_report_data
    from report_helpers import load_json as load_report_json

    return analyze_reports, build_extended_reports, export_report_data, load_report_json


def configure_live_output() -> None:
    for stream_name in ("stdout", "stderr"):
        stream = getattr(sys, stream_name, None)
        reconfigure = getattr(stream, "reconfigure", None)
        if callable(reconfigure):
            reconfigure(line_buffering=True, write_through=True)


def startup_notice(config_arg: str | None) -> None:
    now_text = dt.datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    message = (
        f"[startup] run_single_chat_pressure 启动"
        f" time={now_text}"
        f" cwd={ROOT}"
        f" config={config_arg or ''}"
    )
    print(message, flush=True)


class StageProgress:
    def __init__(self, total: int = 11) -> None:
        self.total = total
        self.bar = tqdm(total=total, desc="单聊压测进度", unit="step", dynamic_ncols=True)
        self.current_stage = 0

    def set(self, stage: int, title: str, detail: str = "") -> None:
        stage = max(0, min(stage, self.total))
        self.current_stage = stage
        self.bar.n = stage - 1 if stage > 0 else 0
        self.bar.set_description_str(f"[{stage}/{self.total}] {title}")
        if detail:
            self.bar.set_postfix_str(detail)
            tqdm.write(f"[{stage}/{self.total}] {title}：{detail}")
        else:
            self.bar.set_postfix_str("")
            tqdm.write(f"[{stage}/{self.total}] {title}")
        self.bar.refresh()

    def advance(self, title: str, detail: str = "") -> None:
        next_stage = min(self.total, self.current_stage + 1)
        self.set(next_stage, title, detail)

    def close(self) -> None:
        self.bar.n = self.total
        self.bar.refresh()
        self.bar.close()

    def heartbeat(self, title: str, detail: str) -> None:
        stage = self.current_stage or 0
        self.bar.set_description_str(f"[{stage}/{self.total}] {title}")
        self.bar.set_postfix_str(detail)
        tqdm.write(f"[{stage}/{self.total}] {title}：{detail}")
        self.bar.refresh()


class ChildProgressBar:
    def __init__(self, desc: str) -> None:
        self.desc = desc
        self.bar = None
        self.total = None

    def update(self, current: int, total: int, detail: str = "") -> None:
        total = max(total, 1)
        if self.bar is None or self.total != total:
            if self.bar is not None:
                self.bar.close()
            self.total = total
            self.bar = tqdm(total=total, desc=desc_safe(self.desc), unit="step", dynamic_ncols=True, leave=False)
        self.bar.n = max(0, min(current, total))
        if detail:
            self.bar.set_postfix_str(detail)
        self.bar.refresh()

    def message(self, detail: str) -> None:
        if self.bar is not None:
            self.bar.set_postfix_str(detail)
            self.bar.refresh()
        tqdm.write(detail)

    def close(self) -> None:
        if self.bar is not None:
            self.bar.n = self.total or self.bar.n
            self.bar.refresh()
            self.bar.close()
            self.bar = None


def desc_safe(value: str) -> str:
    return value.replace("\n", " ").strip()


def read_toml(path: Path) -> dict:
    with path.open("rb") as fp:
        return tomllib.load(fp)


def write_text(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")


def read_json(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


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


def write_toml(path: Path, config_data: dict) -> None:
    lines: list[str] = []
    for section, values in config_data.items():
        if not isinstance(values, dict):
            continue
        lines.append(f"[{section}]")
        for key, value in values.items():
            lines.append(f"{key} = {toml_literal(value)}")
        lines.append("")
    write_text(path, "\n".join(lines).rstrip() + "\n")


def bool_mode_is_on(value) -> bool:
    if isinstance(value, bool):
        return value
    if value is None:
        return False
    return str(value).strip().lower() in {"on", "true", "1", "yes"}


def sanitize_seed_prefix(raw: str) -> str:
    normalized = "".join(ch for ch in str(raw).strip().upper() if ("A" <= ch <= "Z") or ("0" <= ch <= "9"))
    return normalized[:6]


def is_tcp_open(host: str, port: int, timeout_sec: float = 0.5) -> bool:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.settimeout(timeout_sec)
        try:
            sock.connect((host, port))
            return True
        except OSError:
            return False


def find_listener_pids(port: int) -> list[int]:
    try:
        result = subprocess.run(
            ["ss", "-ltnp"],
            cwd=str(ROOT),
            text=True,
            capture_output=True,
            check=False,
        )
    except Exception:
        return []
    pids: list[int] = []
    pattern = re.compile(r"pid=(\d+)")
    for line in result.stdout.splitlines():
        if f":{port} " not in line and not line.rstrip().endswith(f":{port}"):
            continue
        for match in pattern.findall(line):
            try:
                pids.append(int(match))
            except ValueError:
                continue
    return sorted(set(pids))


def stop_listener_processes(port: int, grace_sec: float = 8.0) -> None:
    pids = find_listener_pids(port)
    if not pids:
        return
    for pid in pids:
        try:
            os.kill(pid, signal.SIGTERM)
        except ProcessLookupError:
            continue
    deadline = time.time() + grace_sec
    while time.time() < deadline:
        if not find_listener_pids(port):
            return
        time.sleep(0.25)
    for pid in find_listener_pids(port):
        try:
            os.kill(pid, signal.SIGKILL)
        except ProcessLookupError:
            continue


def wait_for_tcp(host: str, port: int, timeout_sec: int = 60) -> None:
    deadline = time.time() + timeout_sec
    while time.time() < deadline:
        if is_tcp_open(host, port):
            return
        time.sleep(1)
    raise RuntimeError(f"timeout waiting for {host}:{port}")


def wait_for_tcp_with_progress(
    host: str,
    port: int,
    timeout_sec: int,
    *,
    progress_cb: Callable[[str], None] | None = None,
    label: str = "等待端口就绪",
) -> None:
    deadline = time.time() + timeout_sec
    next_report_at = 0.0
    while time.time() < deadline:
        if is_tcp_open(host, port):
            if progress_cb:
                progress_cb(f"{label}完成：{host}:{port}")
            return
        now = time.time()
        if progress_cb and now >= next_report_at:
            waited = int(timeout_sec - max(0, deadline - now))
            progress_cb(f"{label}中：{host}:{port}，已等待 {waited}s / {timeout_sec}s")
            next_report_at = now + 5
        time.sleep(1)
    raise RuntimeError(f"timeout waiting for {host}:{port}")


def ensure_go_runtime_env(env: dict[str, str]) -> dict[str, str]:
    runtime_env = dict(env)
    go_root = Path("/tmp/echochat-go")
    gopath = Path(runtime_env.get("GOPATH", "")).expanduser() if runtime_env.get("GOPATH") else go_root / "gopath"
    gomodcache = (
        Path(runtime_env.get("GOMODCACHE", "")).expanduser()
        if runtime_env.get("GOMODCACHE")
        else go_root / "gomodcache"
    )
    gocache = Path(runtime_env.get("GOCACHE", "")).expanduser() if runtime_env.get("GOCACHE") else go_root / "gocache"
    for path in [gopath, gomodcache, gocache]:
        path.mkdir(parents=True, exist_ok=True)
    runtime_env["GOPATH"] = str(gopath)
    runtime_env["GOMODCACHE"] = str(gomodcache)
    runtime_env["GOCACHE"] = str(gocache)
    return runtime_env


def wait_for_server_ready(
    process: subprocess.Popen[str],
    host: str,
    port: int,
    base_url: str,
    log_path: Path,
    timeout_sec: int = 900,
) -> None:
    deadline = time.time() + timeout_sec
    last_bench_error = None
    next_report_at = 0.0
    while time.time() < deadline:
        exit_code = process.poll()
        if exit_code is not None:
            log_tail = tail_text(log_path, max_lines=120)
            details = f"server exited with code {exit_code}"
            if log_tail:
                details += f"\n--- server.log tail ---\n{log_tail}"
            raise RuntimeError(details)

        if is_tcp_open(host, port):
            try:
                fetch_bench_json(base_url, "/bench/admin/metrics_snapshot")
                return
            except Exception as exc:  # pragma: no cover
                last_bench_error = exc
        now = time.time()
        if now >= next_report_at:
            waited = int(timeout_sec - max(0, deadline - now))
            status = f"等待服务 ready：{host}:{port}，已等待 {waited}s / {timeout_sec}s"
            if last_bench_error is not None:
                status += f"，bench={last_bench_error}"
            tqdm.write(status)
            next_report_at = now + 5
        time.sleep(1)

    details = f"timeout waiting for server ready on {host}:{port}"
    if last_bench_error is not None:
        details += f": {last_bench_error}"
    log_tail = tail_text(log_path, max_lines=120)
    if log_tail:
        details += f"\n--- server.log tail ---\n{log_tail}"
    raise RuntimeError(details)


def tail_text(path: Path, max_lines: int = 120) -> str:
    try:
        text = path.read_text(encoding="utf-8", errors="replace")
    except Exception:
        return ""
    lines = text.splitlines()
    if len(lines) <= max_lines:
        return text
    return "\n".join(lines[-max_lines:])


def tail_lines(path: Path, max_lines: int = 3) -> list[str]:
    try:
        text = path.read_text(encoding="utf-8", errors="replace")
    except Exception:
        return []
    lines = [line for line in text.splitlines() if line.strip()]
    if not lines:
        return []
    return lines[-max_lines:]


def go_int32(value: int) -> int:
    value &= 0xFFFFFFFF
    return value if value < 0x80000000 else value - 0x100000000


def fnv1a32(value: str) -> int:
    result = 0x811C9DC5
    for byte in value.encode("utf-8"):
        result ^= byte
        result = (result * 0x01000193) & 0xFFFFFFFF
    return result


def sarama_hash_partition(key: str, num_partitions: int) -> int:
    if num_partitions <= 0:
        return 0
    return abs(go_int32(fnv1a32(key))) % num_partitions


def build_conversation_key(send_id: str, receive_id: str) -> str:
    if receive_id.startswith("G"):
        return f"group:{receive_id}"
    if send_id < receive_id:
        return f"user:{send_id}:{receive_id}"
    return f"user:{receive_id}:{send_id}"


def persist_worker_index_for_pair(send_id: str, receive_id: str, worker_count: int) -> int:
    if worker_count <= 0:
        return 0
    scope = build_conversation_key(send_id, receive_id)
    return fnv1a32(scope) % worker_count


def build_balanced_targets(total: int, bucket_count: int) -> dict[int, int]:
    if bucket_count <= 0:
        return {}
    base = total // bucket_count
    extra = total % bucket_count
    return {idx: base + (1 if idx < extra else 0) for idx in range(bucket_count)}


def derive_runtime_service_config(
    cfg: dict,
    raw_dir: Path,
    bundle_label: str,
    mysql_runtime_layout: dict[str, Path | str | int] | None = None,
) -> tuple[Path, dict]:
    run_cfg = cfg.get("run", {})
    base_config_path = ROOT / str(run_cfg.get("base_config", "configs/config_local.toml"))
    base_config = read_toml(base_config_path) if base_config_path.exists() else {}

    main_cfg = base_config.setdefault("mainConfig", {})
    mysql_cfg = base_config.setdefault("mysqlConfig", {})
    redis_cfg = base_config.setdefault("redisConfig", {})
    kafka_cfg = base_config.setdefault("kafkaConfig", {})
    log_cfg = base_config.setdefault("logConfig", {})
    pressure_cfg = base_config.setdefault("pressureTestConfig", {})
    observability_cfg = base_config.setdefault("observabilityConfig", {})

    consumer_port = int(cfg.get("consumers", {}).get("base_port", 18082))
    kafka_input = cfg.get("kafka", {})
    mysql_input = cfg.get("mysql", {})
    redis_input = cfg.get("redis", {})
    persist = cfg.get("persist", {})
    commit = cfg.get("commit", {})
    partition_async = cfg.get("partition_async", {})
    conversation_bucket = cfg.get("conversation_bucket", {})
    logging_cfg = cfg.get("logging", {})

    main_cfg["host"] = str(main_cfg.get("host", "127.0.0.1") or "127.0.0.1")
    main_cfg["port"] = consumer_port
    mysql_cfg["host"] = str(mysql_input.get("host", mysql_cfg.get("host", "127.0.0.1")))
    mysql_cfg["port"] = int(
        mysql_runtime_layout["port"] if mysql_runtime_layout is not None else mysql_input.get("port", mysql_cfg.get("port", 3306))
    )
    runtime_database = str(run_cfg.get("database", "")).strip() or str(
        mysql_input.get("database_name", mysql_cfg.get("databaseName", "echochat"))
    )
    mysql_cfg["databaseName"] = runtime_database
    mysql_cfg["user"] = str(mysql_cfg.get("user", "root"))
    mysql_cfg["password"] = str(mysql_cfg.get("password", ""))
    redis_cfg["host"] = str(redis_input.get("host", redis_cfg.get("host", "127.0.0.1")))
    redis_cfg["port"] = int(redis_input.get("port", redis_cfg.get("port", 6379)))
    redis_cfg["db"] = int(redis_input.get("db", redis_cfg.get("db", 0)))

    kafka_cfg["messageMode"] = "kafka"
    kafka_cfg["hostPort"] = str(kafka_input.get("host_port", kafka_cfg.get("hostPort", "127.0.0.1:9092")))
    topic_prefix = str(kafka_input.get("chat_topic_prefix", "chat_pressure"))
    if bool_mode_is_on(kafka_input.get("unique_topic_per_run", True)):
        kafka_cfg["chatTopic"] = f"{topic_prefix}_{bundle_label}"
    else:
        kafka_cfg["chatTopic"] = topic_prefix
    kafka_cfg["consumerGroup"] = f"chat_pressure_{bundle_label}"
    kafka_cfg["topicPartitions"] = int(kafka_input.get("topic_partitions", kafka_cfg.get("topicPartitions", 1)))
    kafka_cfg["consumerCommitBatchSize"] = int(commit.get("batch_size", kafka_cfg.get("consumerCommitBatchSize", 100)))
    kafka_cfg["consumerCommitIntervalMs"] = int(commit.get("interval_ms", kafka_cfg.get("consumerCommitIntervalMs", 250)))
    kafka_cfg["mysqlPersistBatchSize"] = int(persist.get("batch_size", kafka_cfg.get("mysqlPersistBatchSize", 64)))
    kafka_cfg["mysqlPersistFirstJobHoldMs"] = float(persist.get("first_job_hold_ms", kafka_cfg.get("mysqlPersistFirstJobHoldMs", 0)))
    kafka_cfg["mysqlPersistFlushIntervalMs"] = int(persist.get("flush_interval_ms", kafka_cfg.get("mysqlPersistFlushIntervalMs", 5)))
    kafka_cfg["mysqlPersistWorkerCount"] = int(persist.get("worker_count", kafka_cfg.get("mysqlPersistWorkerCount", 8)))
    kafka_cfg["mysqlPersistQueueSize"] = int(persist.get("queue_size", kafka_cfg.get("mysqlPersistQueueSize", 2048)))
    kafka_cfg["sessionSeqRedisOnlyExperimental"] = bool(persist.get("session_seq_redis_only_experimental", kafka_cfg.get("sessionSeqRedisOnlyExperimental", False)))
    kafka_cfg["mysqlPersistNoopExperimental"] = bool(persist.get("mysql_persist_noop_experimental", kafka_cfg.get("mysqlPersistNoopExperimental", False)))
    kafka_cfg["partitionAsyncEnabled"] = bool(partition_async.get("enabled", kafka_cfg.get("partitionAsyncEnabled", False)))
    kafka_cfg["partitionAsyncShardCount"] = int(partition_async.get("shard_count", kafka_cfg.get("partitionAsyncShardCount", 4)))
    kafka_cfg["partitionAsyncQueueSize"] = int(partition_async.get("queue_size", kafka_cfg.get("partitionAsyncQueueSize", 512)))
    kafka_cfg["partitionAsyncDrainTimeoutMs"] = int(partition_async.get("drain_timeout_ms", kafka_cfg.get("partitionAsyncDrainTimeoutMs", 3000)))
    kafka_cfg["conversationBucketEnabled"] = bool(conversation_bucket.get("enabled", kafka_cfg.get("conversationBucketEnabled", False)))
    kafka_cfg["conversationBucketWorkerCount"] = int(conversation_bucket.get("worker_count", kafka_cfg.get("conversationBucketWorkerCount", 8)))
    kafka_cfg["conversationBucketReadyQueueSize"] = int(conversation_bucket.get("ready_queue_size", kafka_cfg.get("conversationBucketReadyQueueSize", 512)))
    kafka_cfg["conversationBucketQueueSize"] = int(conversation_bucket.get("bucket_queue_size", kafka_cfg.get("conversationBucketQueueSize", 256)))
    kafka_cfg["conversationBucketMaxMessagesPerTurn"] = int(conversation_bucket.get("max_messages_per_turn", kafka_cfg.get("conversationBucketMaxMessagesPerTurn", 32)))
    kafka_cfg["conversationBucketMaxRunDurationMs"] = int(conversation_bucket.get("max_run_duration_ms", kafka_cfg.get("conversationBucketMaxRunDurationMs", 5)))
    kafka_cfg["conversationBucketDrainTimeoutMs"] = int(conversation_bucket.get("drain_timeout_ms", kafka_cfg.get("conversationBucketDrainTimeoutMs", 3000)))

    pressure_cfg["enableBenchmarkRoutes"] = True
    pressure_cfg["disableBenchmarkRequestLog"] = bool(logging_cfg.get("disable_benchmark_request_log", True))
    pressure_cfg["disableBenchmarkHotPathLog"] = bool(logging_cfg.get("disable_benchmark_hot_path_log", True))
    observability_cfg["enableMetrics"] = True
    log_cfg["level"] = str(logging_cfg.get("level", log_cfg.get("level", "info")))
    log_cfg["disableStdout"] = bool(logging_cfg.get("disable_stdout", log_cfg.get("disableStdout", True)))

    runtime_config_path = raw_dir / "runtime_config.toml"
    write_toml(runtime_config_path, base_config)
    return runtime_config_path, base_config


def runtime_mysql_settings(service_cfg: dict) -> dict[str, str | int]:
    mysql_cfg = service_cfg.get("mysqlConfig", {})
    return {
        "host": str(mysql_cfg.get("host", "127.0.0.1")),
        "port": int(mysql_cfg.get("port", 3306)),
        "user": str(mysql_cfg.get("user", "root")),
        "password": str(mysql_cfg.get("password", "")),
        "database": str(mysql_cfg.get("databaseName", "echochat")),
    }


def resolve_runtime_urls(run_cfg: dict, runtime_service_cfg: dict) -> tuple[str, str, str]:
    main_cfg = runtime_service_cfg.get("mainConfig", {})
    host = str(main_cfg.get("host", "127.0.0.1") or "127.0.0.1").strip()
    if host in {"0.0.0.0", ""}:
        host = "127.0.0.1"
    port = int(main_cfg.get("port", 18082))
    default_base_url = f"http://{host}:{port}"
    default_ws_base_url = f"ws://{host}:{port}"
    base_url = str(run_cfg.get("base_url", "")).rstrip("/") or default_base_url
    ws_base_url = str(run_cfg.get("ws_base_url", "")).rstrip("/") or default_ws_base_url
    bench_admin_base_url = str(run_cfg.get("bench_admin_base_url", "")).rstrip("/") or base_url
    return base_url, ws_base_url, bench_admin_base_url


def build_mysql_runtime_layout(cfg: dict, bundle_label: str) -> dict[str, Path | str | int]:
    run_cfg = cfg.get("run", {})
    mysql_cfg = cfg.get("mysql", {})
    root = Path(str(run_cfg.get("mysql_runtime_root", "/tmp/echochat_mysql_isolated"))).resolve()
    runtime_dir = root / bundle_label
    data_dir = runtime_dir / "data"
    tmp_dir = runtime_dir / "tmp"
    port = int(mysql_cfg.get("port", 33306))
    short_name = f"echo_mysql_{port}_{bundle_label[:24]}"
    short_root = Path("/tmp") / short_name
    short_root.mkdir(parents=True, exist_ok=True)
    socket_path = short_root / "mysql.sock"
    pid_path = short_root / "mysqld.pid"
    init_log_path = runtime_dir / "mysql_init.log"
    runtime_log_path = runtime_dir / "mysql_runtime.log"
    return {
        "root": runtime_dir,
        "data_dir": data_dir,
        "tmp_dir": tmp_dir,
        "short_root": short_root,
        "socket_path": socket_path,
        "pid_path": pid_path,
        "init_log_path": init_log_path,
        "runtime_log_path": runtime_log_path,
        "port": port,
    }


def is_mysql_process(pid: int) -> bool:
    try:
        result = subprocess.run(
            ["ps", "-p", str(pid), "-o", "comm="],
            cwd=str(ROOT),
            text=True,
            capture_output=True,
            check=False,
        )
    except Exception:
        return False
    command = result.stdout.strip().lower()
    return command in {"mysqld", "mariadbd"}


def stop_mysql_listener_processes(port: int, grace_sec: float = 15.0) -> None:
    pids = [pid for pid in find_listener_pids(port) if is_mysql_process(pid)]
    if not pids:
        return
    for pid in pids:
        try:
            os.kill(pid, signal.SIGTERM)
        except ProcessLookupError:
            continue
    deadline = time.time() + grace_sec
    while time.time() < deadline:
        active = [pid for pid in find_listener_pids(port) if is_mysql_process(pid)]
        if not active:
            return
        time.sleep(0.25)
    for pid in [pid for pid in find_listener_pids(port) if is_mysql_process(pid)]:
        try:
            os.kill(pid, signal.SIGKILL)
        except ProcessLookupError:
            continue


def mysql_query(settings: dict[str, str | int], query: str) -> list[list[str]]:
    command = [
        "mysql",
        "--protocol=TCP",
        "-h",
        str(settings["host"]),
        "-P",
        str(settings["port"]),
        "-u",
        str(settings["user"]),
    ]
    password = str(settings.get("password", ""))
    if password:
        command.append(f"-p{password}")
    command.extend(["-NBe", query, str(settings["database"])])
    result = subprocess.run(command, check=True, capture_output=True, text=True)
    rows: list[list[str]] = []
    for raw in result.stdout.splitlines():
        if raw.strip():
            rows.append(raw.split("\t"))
    return rows


def mysql_exec_without_database(settings: dict[str, str | int], query: str) -> None:
    command = [
        "mysql",
        "--protocol=TCP",
        "-h",
        str(settings["host"]),
        "-P",
        str(settings["port"]),
        "-u",
        str(settings["user"]),
    ]
    password = str(settings.get("password", ""))
    if password:
        command.append(f"-p{password}")
    command.extend(["-e", query])
    subprocess.run(command, check=True, capture_output=True, text=True)


def count_available_single_pairs(mysql_settings: dict[str, str | int], user_prefix: str) -> int:
    rows = mysql_query(
        mysql_settings,
        f"""
SELECT COUNT(*)
FROM session s
WHERE s.send_id LIKE '{user_prefix}%'
  AND s.receive_id LIKE '{user_prefix}%'
  AND s.send_id < s.receive_id
  AND s.deleted_at IS NULL
""",
    )
    return int(rows[0][0]) if rows else 0


def load_single_pair_candidates(cfg: dict, mysql_settings: dict[str, str | int], limit: int) -> list[dict]:
    user_prefix = f"U{sanitize_seed_prefix(cfg.get('run', {}).get('seed_prefix', 'PT'))}"
    rows = mysql_query(
        mysql_settings,
        f"""
SELECT
  s.uuid,
  s.send_id,
  su.telephone,
  su.nickname,
  su.avatar,
  s.receive_id,
  ru.telephone,
  ru.nickname,
  ru.avatar
FROM session s
JOIN user_info su ON su.uuid = s.send_id
JOIN user_info ru ON ru.uuid = s.receive_id
WHERE s.send_id LIKE '{user_prefix}%'
  AND s.receive_id LIKE '{user_prefix}%'
  AND s.send_id < s.receive_id
  AND s.deleted_at IS NULL
ORDER BY s.send_id, s.receive_id
LIMIT {limit}
""",
    )
    topic_partitions = int(cfg.get("kafka", {}).get("topic_partitions", 1))
    worker_count = int(cfg.get("persist", {}).get("worker_count", 1))
    candidates: list[dict] = []
    for row in rows:
        session_id, sender_uuid, sender_tel, sender_name, sender_avatar, receiver_uuid, receiver_tel, receiver_name, receiver_avatar = row
        candidates.append(
            {
                "session_id": session_id,
                "sender_uuid": sender_uuid,
                "sender_telephone": sender_tel,
                "sender_nickname": sender_name,
                "sender_avatar": sender_avatar,
                "receiver_uuid": receiver_uuid,
                "receiver_telephone": receiver_tel,
                "receiver_nickname": receiver_name,
                "receiver_avatar": receiver_avatar,
                "partition": sarama_hash_partition(session_id, topic_partitions),
                "worker_index": persist_worker_index_for_pair(sender_uuid, receiver_uuid, worker_count),
            }
        )
    return candidates


def estimate_candidate_limit(
    *,
    pair_count: int,
    initial_candidates: int,
    total_available: int,
    partition_targets: dict[int, int] | None,
    worker_targets: dict[int, int] | None,
) -> int:
    if total_available <= 0:
        return 0
    constrained = bool(partition_targets) or bool(worker_targets)
    min_reasonable = max(pair_count * 8, initial_candidates, pair_count)
    if constrained:
        # 在“不重复用户 + 均衡”约束下，前缀样本很容易偏到少数用户。
        # 这里直接放宽到全量候选池，避免反复扩容。
        return total_available
    return min(total_available, min_reasonable)


def try_select_disjoint_pairs(
    candidates: list[dict],
    pair_count: int,
    partition_targets: dict[int, int] | None,
    worker_targets: dict[int, int] | None,
    attempts: int = 64,
) -> list[dict] | None:
    if pair_count <= 0:
        return []
    user_frequency: Counter[str] = Counter()
    for item in candidates:
        user_frequency[item["sender_uuid"]] += 1
        user_frequency[item["receiver_uuid"]] += 1

    for attempt in range(attempts):
        ordered = list(candidates)
        random.Random(17 + attempt).shuffle(ordered)
        ordered.sort(
            key=lambda item: (
                user_frequency[item["sender_uuid"]] + user_frequency[item["receiver_uuid"]],
                item["partition"],
                item["worker_index"],
                item["session_id"],
            )
        )
        used_users: set[str] = set()
        remaining_partitions = dict(partition_targets or {})
        remaining_workers = dict(worker_targets or {})
        selected: list[dict] = []

        while len(selected) < pair_count:
            best = None
            best_score = None
            for item in ordered:
                if item["sender_uuid"] in used_users or item["receiver_uuid"] in used_users:
                    continue
                if remaining_partitions and remaining_partitions.get(item["partition"], 0) <= 0:
                    continue
                if remaining_workers and remaining_workers.get(item["worker_index"], 0) <= 0:
                    continue
                score = (
                    -(remaining_partitions.get(item["partition"], 1) if remaining_partitions else 1),
                    -(remaining_workers.get(item["worker_index"], 1) if remaining_workers else 1),
                    user_frequency[item["sender_uuid"]] + user_frequency[item["receiver_uuid"]],
                    item["session_id"],
                )
                if best_score is None or score < best_score:
                    best = item
                    best_score = score
            if best is None:
                break
            selected.append(best)
            used_users.add(best["sender_uuid"])
            used_users.add(best["receiver_uuid"])
            if remaining_partitions:
                remaining_partitions[best["partition"]] -= 1
            if remaining_workers:
                remaining_workers[best["worker_index"]] -= 1

        if len(selected) == pair_count and all(v == 0 for v in remaining_partitions.values()) and all(v == 0 for v in remaining_workers.values()):
            return selected
    return None


def build_fixture_with_constraints(cfg: dict, raw_dir: Path, mysql_settings: dict[str, str | int]) -> Path:
    run_cfg = cfg.get("run", {})
    scenario = cfg.get("scenario", {})
    persist = cfg.get("persist", {})
    pair_count = int(scenario.get("session_count", 0))
    initial_candidates = int(scenario.get("fixture_pair_count", pair_count))
    user_prefix = f"U{sanitize_seed_prefix(run_cfg.get('seed_prefix', 'PT'))}"
    total_available = count_available_single_pairs(mysql_settings, user_prefix)
    if total_available <= 0:
        raise RuntimeError("fixture 候选池为空，当前前缀下没有可用单聊会话")

    partition_targets = None
    if bool_mode_is_on(scenario.get("session_partition_balance_mode")):
        partition_targets = build_balanced_targets(pair_count, int(cfg.get("kafka", {}).get("topic_partitions", 1)))
    worker_targets = None
    worker_count = int(persist.get("worker_count", 1))
    if bool_mode_is_on(scenario.get("worker_balance_mode")) and worker_count > 1:
        worker_targets = build_balanced_targets(pair_count, worker_count)

    current_limit = estimate_candidate_limit(
        pair_count=pair_count,
        initial_candidates=initial_candidates,
        total_available=total_available,
        partition_targets=partition_targets,
        worker_targets=worker_targets,
    )
    fixture_path = raw_dir / "fixture.json"
    while True:
        candidates = load_single_pair_candidates(cfg, mysql_settings, current_limit)
        selected = try_select_disjoint_pairs(candidates, pair_count, partition_targets, worker_targets)
        if selected is not None:
            payload = {
                "database": mysql_settings["database"],
                "user_prefix": user_prefix,
                "default_password": str(run_cfg.get("default_password", "123456")),
                "single_pairs": selected,
                "selection_meta": {
                    "pair_count": pair_count,
                    "candidate_pairs": len(candidates),
                    "total_available_pairs": total_available,
                    "partition_targets": partition_targets,
                    "worker_targets": worker_targets,
                },
            }
            write_text(fixture_path, json.dumps(payload, ensure_ascii=False, indent=2) + "\n")
            return fixture_path

        unique_users = len({value for item in candidates for value in (item["sender_uuid"], item["receiver_uuid"])})
        reason = (
            "unable to select disjoint single-chat pairs under current large-scale constraints; "
            f"pair_count={pair_count} candidate_pairs={len(candidates)} unique_users={unique_users} "
            f"partition_targets={partition_targets} worker_targets={worker_targets}"
        )
        if current_limit >= total_available:
            raise RuntimeError(
                "fixture 候选池不足，无法满足当前会话/分区/worker 约束；"
                f"最后一次候选会话数={current_limit}，错误={reason}"
            )
        next_limit = min(max(current_limit * 2, current_limit + 1), total_available)
        tqdm.write(
            f"[fixture] 候选池不足：当前 {current_limit} 对仍不满足约束，准备扩容到 {next_limit} 对重试；原因={reason}"
        )
        current_limit = next_limit


def mysql_ping(mysql_settings: dict[str, str | int]) -> tuple[bool, str]:
    command = [
        "mysql",
        "--protocol=TCP",
        f"-h{mysql_settings['host']}",
        f"-P{mysql_settings['port']}",
        f"-u{mysql_settings['user']}",
        "-NBe",
        "SELECT 1",
    ]
    password = str(mysql_settings.get("password", ""))
    if password:
        command.insert(-1, f"-p{password}")
    result = subprocess.run(command, cwd=str(ROOT), text=True, capture_output=True, check=False)
    detail = (result.stderr or result.stdout or "").strip()
    if result.returncode == 0 and result.stdout.strip() == "1":
        return True, detail
    return False, detail


def wait_for_mysql_ready(mysql_settings: dict[str, str | int], timeout_sec: int = 60) -> None:
    deadline = time.time() + timeout_sec
    while time.time() < deadline:
        ok, _ = mysql_ping(mysql_settings)
        if ok:
            return
        time.sleep(1)
    raise RuntimeError("timeout waiting for mysql ping")


def wait_for_mysql_ready_with_progress(
    mysql_settings: dict[str, str | int],
    timeout_sec: int = 60,
    *,
    progress_cb: Callable[[str], None] | None = None,
    process: subprocess.Popen[str] | None = None,
    log_path: Path | None = None,
) -> None:
    deadline = time.time() + timeout_sec
    next_report_at = 0.0
    host = str(mysql_settings["host"])
    port = int(mysql_settings["port"])
    database = str(mysql_settings.get("database", ""))
    last_detail = ""
    while time.time() < deadline:
        if process is not None:
            exit_code = process.poll()
            if exit_code is not None:
                log_tail = tail_text(log_path, max_lines=120) if log_path is not None else ""
                message = f"mysqld exited before ready with code {exit_code}"
                if log_tail:
                    message += f"\n--- mysql_runtime.log tail ---\n{log_tail}"
                raise RuntimeError(message)
        ok, detail = mysql_ping(mysql_settings)
        if ok:
            if progress_cb:
                progress_cb(f"MySQL 已就绪：{host}:{port} db={database}")
            return
        if detail:
            last_detail = detail
        now = time.time()
        if progress_cb and now >= next_report_at:
            waited = int(timeout_sec - max(0, deadline - now))
            message = f"等待 MySQL ping：{host}:{port} db={database}，已等待 {waited}s / {timeout_sec}s"
            if last_detail:
                message += f"，detail={last_detail}"
            progress_cb(message)
            next_report_at = now + 5
        time.sleep(1)
    raise RuntimeError(
        "timeout waiting for mysql ping"
        + (f": {last_detail}" if last_detail else "")
    )


def run_command_streaming(
    command: list[str],
    *,
    cwd: Path,
    env: dict[str, str] | None = None,
    progress_cb: Callable[[str], None] | None = None,
    prefix: str = "",
    progress_event_handler: Callable[[dict], None] | None = None,
) -> None:
    process = subprocess.Popen(
        command,
        cwd=str(cwd),
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        bufsize=1,
    )
    assert process.stdout is not None
    last_lines: list[str] = []
    suppressed_noisy_lines = 0

    def should_suppress(line: str) -> bool:
        if not line:
            return True
        if len(line) > 800:
            return True
        if line.count("seed direct message") >= 2:
            return True
        if line.startswith("INSERT INTO ") or line.startswith("REPLACE INTO "):
            return True
        if line.startswith("('MPT") or line.startswith("(\"MPT") or line.startswith("001518:UPT"):
            return True
        if "VALUES('MPT" in line or "VALUES ('MPT" in line:
            return True
        return False

    def normalize_progress_line(line: str) -> str:
        if "\r" in line:
            line = line.split("\r")[-1]
        return line.strip()

    try:
        for raw_line in process.stdout:
            line = normalize_progress_line(raw_line.rstrip("\n"))
            if not line:
                continue
            if progress_event_handler is not None and line.startswith("{") and line.endswith("}"):
                try:
                    payload = json.loads(line)
                except json.JSONDecodeError:
                    payload = None
                if isinstance(payload, dict) and payload.get("type") == "seed_progress":
                    progress_event_handler(payload)
                    continue
            if should_suppress(line):
                suppressed_noisy_lines += 1
                if suppressed_noisy_lines == 1:
                    summary = f"{prefix}检测到大批量 SQL/seed 明细输出，后续同类内容已折叠"
                    if progress_cb:
                        progress_cb(summary)
                    else:
                        tqdm.write(summary)
                elif suppressed_noisy_lines % 200 == 0:
                    summary = f"{prefix}已折叠 {suppressed_noisy_lines} 行大批量 SQL/seed 明细"
                    if progress_cb:
                        progress_cb(summary)
                    else:
                        tqdm.write(summary)
                continue
            last_lines.append(line)
            if len(last_lines) > 40:
                last_lines.pop(0)
            if progress_cb:
                progress_cb(f"{prefix}{line}")
            else:
                tqdm.write(f"{prefix}{line}")
    finally:
        process.stdout.close()
    code = process.wait()
    if suppressed_noisy_lines:
        summary = f"{prefix}已折叠 {suppressed_noisy_lines} 行大批量 SQL/seed 明细"
        if progress_cb:
            progress_cb(summary)
        else:
            tqdm.write(summary)
    if code != 0:
        detail = "\n".join(last_lines[-20:])
        raise RuntimeError(
            f"command failed with exit code {code}: {' '.join(command)}"
            + (f"\n--- recent output ---\n{detail}" if detail else "")
        )


def run_command_with_heartbeat(
    command: list[str],
    *,
    cwd: Path,
    env: dict[str, str] | None = None,
    progress_cb: Callable[[str], None] | None = None,
    label: str,
    log_path: Path | None = None,
    heartbeat_sec: float = 3.0,
) -> None:
    process = subprocess.Popen(
        command,
        cwd=str(cwd),
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )
    collected: list[str] = []
    next_report_at = 0.0
    try:
        while True:
            exit_code = process.poll()
            line = None
            if process.stdout is not None:
                line = process.stdout.readline()
            if line:
                line = line.rstrip()
                if line:
                    collected.append(line)
                    if len(collected) > 40:
                        collected.pop(0)
                    message = f"{label}：{line}"
                    if progress_cb:
                        progress_cb(message)
                    else:
                        tqdm.write(message)
            elif exit_code is not None:
                break

            now = time.time()
            if now >= next_report_at:
                detail = f"{label}进行中"
                if log_path is not None:
                    tails = tail_lines(log_path, max_lines=2)
                    if tails:
                        detail += "；log tail=" + " | ".join(tails)
                if progress_cb:
                    progress_cb(detail)
                else:
                    tqdm.write(detail)
                next_report_at = now + heartbeat_sec
            time.sleep(0.2)
    finally:
        if process.stdout is not None:
            process.stdout.close()
    code = process.wait()
    if code != 0:
        detail = "\n".join(collected[-20:])
        log_tail = tail_text(log_path, max_lines=120) if log_path is not None else ""
        message = f"command failed with exit code {code}: {' '.join(command)}"
        if detail:
            message += f"\n--- recent output ---\n{detail}"
        if log_tail:
            message += f"\n--- log tail ---\n{log_tail}"
        raise RuntimeError(message)


def ensure_mysql_database(mysql_settings: dict[str, str | int]) -> None:
    database = str(mysql_settings["database"])
    if not database:
        raise RuntimeError("mysql database name cannot be empty")
    mysql_exec_without_database(
        mysql_settings,
        f"CREATE DATABASE IF NOT EXISTS `{database}` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;",
    )


def resolve_mysql_binaries() -> tuple[str, Path, Path]:
    preferred_candidates = [
        Path("/root/anaconda3/bin/mysqld"),
        Path("/my_storage/root_home/anaconda3/bin/mysqld"),
        Path("/usr/sbin/mysqld"),
    ]
    preferred = next((path for path in preferred_candidates if path.exists()), None)
    mysqld_path = str(preferred if preferred is not None else (shutil.which("mysqld") or "/usr/sbin/mysqld"))
    basedir = Path("/root/anaconda3")
    if not basedir.exists():
        basedir = Path("/my_storage/root_home/anaconda3")
    lc_messages_dir = basedir / "share" / "mysql"
    return mysqld_path, basedir, lc_messages_dir


def initialize_isolated_mysql_runtime(
    runtime_layout: dict[str, Path | str | int],
    *,
    progress_cb: Callable[[str], None] | None = None,
) -> None:
    root = Path(runtime_layout["root"])
    data_dir = Path(runtime_layout["data_dir"])
    tmp_dir = Path(runtime_layout["tmp_dir"])
    init_log_path = Path(runtime_layout["init_log_path"])
    root.mkdir(parents=True, exist_ok=True)
    data_dir.mkdir(parents=True, exist_ok=True)
    tmp_dir.mkdir(parents=True, exist_ok=True)
    mysqld_path, basedir, _ = resolve_mysql_binaries()
    if not Path(mysqld_path).exists():
        raise RuntimeError(f"mysqld not found: {mysqld_path}")
    command = [
        mysqld_path,
        "--initialize-insecure",
        "--user=root",
        f"--datadir={data_dir}",
        f"--basedir={basedir}",
        f"--tmpdir={tmp_dir}",
        f"--log-error={init_log_path}",
        "--skip-log-bin",
        "--mysqlx=0",
    ]
    run_command_with_heartbeat(
        command,
        cwd=ROOT,
        progress_cb=progress_cb,
        label="初始化隔离 MySQL 数据目录",
        log_path=init_log_path,
    )


def stop_mysql_runtime(runtime_layout: dict[str, Path | str | int]) -> None:
    pid_path = Path(runtime_layout["pid_path"])
    if not pid_path.exists():
        return
    try:
        pid = int(pid_path.read_text(encoding="utf-8").strip())
    except Exception:
        return
    try:
        os.kill(pid, signal.SIGTERM)
    except ProcessLookupError:
        return
    deadline = time.time() + 10
    while time.time() < deadline:
        try:
            os.kill(pid, 0)
        except ProcessLookupError:
            return
        time.sleep(0.2)
    try:
        os.kill(pid, signal.SIGKILL)
    except ProcessLookupError:
        return


def start_local_mysql_if_needed(
    mysql_settings: dict[str, str | int],
    runtime_layout: dict[str, Path | str | int],
    *,
    progress_cb: Callable[[str], None] | None = None,
) -> subprocess.Popen[str] | None:
    host = str(mysql_settings["host"])
    port = int(mysql_settings["port"])
    stop_mysql_listener_processes(port)
    if is_tcp_open(host, port):
        raise RuntimeError(f"mysql port {port} is already in use and could not be reclaimed")
    mysqld_path, basedir, lc_messages_dir = resolve_mysql_binaries()
    data_dir = Path(runtime_layout["data_dir"])
    tmp_dir = Path(runtime_layout["tmp_dir"])
    socket_path = Path(runtime_layout["socket_path"])
    pid_path = Path(runtime_layout["pid_path"])
    log_path = Path(runtime_layout["runtime_log_path"])
    if not Path(mysqld_path).exists() or not data_dir.exists():
        return None
    for stale_path in [socket_path, pid_path]:
        try:
            stale_path.unlink()
        except FileNotFoundError:
            pass
    log_fp = log_path.open("a", encoding="utf-8")
    process = subprocess.Popen(
        [
            mysqld_path,
            "--user=root",
            f"--port={port}",
            f"--bind-address={host}",
            "--skip-log-bin",
            f"--datadir={data_dir}",
            f"--socket={socket_path}",
            f"--pid-file={pid_path}",
            f"--log-error={log_path}",
            f"--tmpdir={tmp_dir}",
            f"--basedir={basedir}",
            f"--lc-messages-dir={lc_messages_dir}",
            "--mysqlx=0",
        ],
        cwd=str(ROOT),
        stdout=log_fp,
        stderr=subprocess.STDOUT,
        text=True,
    )
    try:
        wait_for_tcp_with_progress(host, port, timeout_sec=30, progress_cb=progress_cb, label="等待 MySQL 端口")
        wait_for_mysql_ready_with_progress(
            mysql_settings,
            timeout_sec=60,
            progress_cb=progress_cb,
            process=process,
            log_path=log_path,
        )
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


def cleanup_mysql_runtime(cfg: dict, runtime_layout: dict[str, Path | str | int] | None) -> None:
    if runtime_layout is None:
        return
    stop_mysql_runtime(runtime_layout)
    short_root = runtime_layout.get("short_root")
    if short_root:
        shutil.rmtree(Path(short_root), ignore_errors=True)
    cleanup_cfg = cfg.get("cleanup", {})
    if bool(cleanup_cfg.get("delete_mysql_runtime_dir", False)):
        shutil.rmtree(Path(runtime_layout["root"]), ignore_errors=True)


def wait_for_bench_admin(base_url: str, timeout_sec: int = 90) -> None:
    deadline = time.time() + timeout_sec
    last_error = None
    while time.time() < deadline:
        try:
            fetch_bench_json(base_url, "/bench/admin/metrics_snapshot")
            return
        except Exception as exc:  # pragma: no cover
            last_error = exc
            time.sleep(1)
    raise RuntimeError(f"timeout waiting for bench admin {base_url}: {last_error}")


def start_local_server_if_needed(runtime_config_path: Path, base_url: str, raw_dir: Path) -> subprocess.Popen[str] | None:
    host = "127.0.0.1"
    try:
        port = int(base_url.rsplit(":", 1)[1])
    except Exception:
        runtime_cfg = read_toml(runtime_config_path)
        consumers_cfg = runtime_cfg.get("consumers", {})
        fallback_port = consumers_cfg.get("base_port", 18082)
        client_ports = str(consumers_cfg.get("client_ports", "")).strip()
        ports = str(consumers_cfg.get("ports", "")).strip()
        candidate = ""
        if client_ports:
            candidate = client_ports.split(",")[0].strip()
        elif ports:
            candidate = ports.split(",")[0].strip()
        if candidate:
            try:
                fallback_port = int(candidate)
            except ValueError:
                pass
        port = int(fallback_port)
    stop_listener_processes(port)
    if is_tcp_open(host, port):
        raise RuntimeError(f"port {port} is already in use and could not be reclaimed")
    log_path = raw_dir / "server.log"
    log_fp = log_path.open("a", encoding="utf-8")
    env = ensure_go_runtime_env(os.environ.copy())
    env["ECHOCHAT_CONFIG"] = str(runtime_config_path)
    env["ECHOCHAT_REPO_ROOT"] = str(ROOT)
    process = subprocess.Popen(
        ["go", "run", "./cmd/echo_chat_server"],
        cwd=str(ROOT),
        env=env,
        stdout=log_fp,
        stderr=subprocess.STDOUT,
        text=True,
    )
    try:
        tqdm.write(f"[server] 已启动，等待端口与 bench admin ready：{host}:{port}")
        wait_for_server_ready(process, host, port, base_url, log_path, timeout_sec=900)
    except Exception as exc:
        exit_code = process.poll()
        log_tail = tail_text(log_path, max_lines=120)
        stop_process(process)
        details = str(exc)
        if exit_code is not None:
            details += f"\nserver exited with code {exit_code}"
        if log_tail:
            details += f"\n--- server.log tail ---\n{log_tail}"
        raise RuntimeError(details)
    return process


def apply_mainconfig_overrides(cfg: dict) -> dict:
    cfg = copy.deepcopy(cfg)
    main = cfg.get("mainconfig", {})
    scenario = cfg.setdefault("scenario", {})
    kafka = cfg.setdefault("kafka", {})
    persist = cfg.setdefault("persist", {})
    partition_async = cfg.setdefault("partition_async", {})
    conversation_bucket = cfg.setdefault("conversation_bucket", {})

    if "target_rate" in main:
        scenario["target_rate"] = main["target_rate"]
    if "session_count" in main:
        scenario["session_count"] = main["session_count"]
    if "topic_partitions" in main:
        kafka["topic_partitions"] = main["topic_partitions"]
    if "mysql_persist_worker_count" in main:
        persist["worker_count"] = main["mysql_persist_worker_count"]
    if "mysql_persist_batch_size" in main:
        persist["batch_size"] = main["mysql_persist_batch_size"]
    if "partition_async_shard_count" in main:
        partition_async["shard_count"] = main["partition_async_shard_count"]
    if "conversation_bucket_worker_count" in main:
        conversation_bucket["worker_count"] = main["conversation_bucket_worker_count"]
    if "conversation_bucket_ready_queue_size" in main:
        conversation_bucket["ready_queue_size"] = main["conversation_bucket_ready_queue_size"]
    if "conversation_bucket_bucket_queue_size" in main:
        conversation_bucket["bucket_queue_size"] = main["conversation_bucket_bucket_queue_size"]
    if "conversation_bucket_max_messages_per_turn" in main:
        conversation_bucket["max_messages_per_turn"] = main["conversation_bucket_max_messages_per_turn"]
    if "conversation_bucket_max_run_duration_ms" in main:
        conversation_bucket["max_run_duration_ms"] = main["conversation_bucket_max_run_duration_ms"]
    return cfg


def resolve_record_root(cfg: dict) -> Path:
    run_cfg = cfg.get("run", {})
    layout = run_cfg.get("report_layout", "bundle")
    if layout != "bundle":
        raise ValueError(f"unsupported report_layout={layout!r}, only 'bundle' is allowed")
    root = ROOT / run_cfg.get("record_root", "pressure testing/record")
    today = dt.datetime.now().strftime("%-m.%-d")
    return root / today


def build_bundle_label(cfg: dict) -> str:
    run_cfg = cfg.get("run", {})
    label_mode = str(run_cfg.get("label_mode", "auto")).strip().lower()
    if label_mode == "manual":
        label = str(run_cfg.get("label", "")).strip()
        if not label:
            raise ValueError("run.label_mode=manual requires non-empty run.label")
        return label

    scenario = cfg.get("scenario", {})
    kafka = cfg.get("kafka", {})
    persist = cfg.get("persist", {})
    ts = dt.datetime.now().strftime("%Y%m%d_%H%M%S")
    label = (
        f"{ts}_single_{scenario.get('target_rate', 0)}_"
        f"{kafka.get('topic_partitions', 0)}partitions_"
        f"{persist.get('worker_count', 0)}workers_"
        f"batch{persist.get('batch_size', 0)}"
    )
    suffix = str(run_cfg.get("label_suffix", "")).strip()
    if suffix:
        label = f"{label}_{suffix}"
    return label


def build_scope_report(cfg: dict) -> str:
    scenario = cfg.get("scenario", {})
    kafka = cfg.get("kafka", {})
    persist = cfg.get("persist", {})
    commit = cfg.get("commit", {})
    conversation = cfg.get("conversation_bucket", {})
    partition_async = cfg.get("partition_async", {})

    messages_per_sender = resolve_messages_per_sender(cfg)
    offered_actual = resolve_offered_actual(cfg)
    rows = [
        ("场景", "single"),
        ("会话数", scenario.get("session_count")),
        ("目标总速率", f"{scenario.get('target_rate')}/s"),
        ("持续时间", f"{scenario.get('duration_sec')}s"),
        ("候选会话数", scenario.get("fixture_pair_count")),
        ("用户数", scenario.get("user_count")),
        ("分区选择模式", scenario.get("partition_selection_mode")),
        ("会话分区均衡控制", scenario.get("session_partition_balance_mode")),
        ("特殊 worker 均衡模式", scenario.get("worker_balance_mode")),
        ("offered 实际值", f"{offered_actual}/s" if offered_actual is not None else None),
        ("每会话消息数", messages_per_sender),
        ("broker 地址", kafka.get("host_port")),
        ("topic 分区总数", kafka.get("topic_partitions")),
        ("commit batch", commit.get("batch_size")),
        ("commit interval", f"{commit.get('interval_ms')}ms"),
        ("mysqlPersist batch", persist.get("batch_size")),
        ("firstJobHold", f"{persist.get('first_job_hold_ms')}ms"),
        ("flushInterval", f"{persist.get('flush_interval_ms')}ms"),
        ("worker 数量", persist.get("worker_count")),
        ("worker 队列长度", persist.get("queue_size")),
        ("分区内二次并发", partition_async.get("enabled")),
        ("会话桶调度", conversation.get("enabled")),
        ("会话桶 worker 数", conversation.get("worker_count")),
        ("ready 队列长度", conversation.get("ready_queue_size")),
        ("bucket 队列长度", conversation.get("bucket_queue_size")),
        ("单轮最大消息数", conversation.get("max_messages_per_turn")),
        ("单轮最大运行时长", f"{conversation.get('max_run_duration_ms')}ms"),
    ]
    lines = ["# 本次口径", "", "| 指标 | 数值 |", "| --- | --- |"]
    for key, value in rows:
        lines.append(f"| {key} | {value} |")
    return "\n".join(lines) + "\n"


def fetch_bench_json(base_url: str, path: str) -> dict:
    url = base_url.rstrip("/") + path
    response = requests.get(url, timeout=30)
    response.raise_for_status()
    payload = response.json()
    if payload.get("code") != 200:
        raise RuntimeError(f"bench admin request failed: {url} -> {payload}")
    return payload["data"]


def post_bench_admin_reset(base_url: str) -> None:
    url = base_url.rstrip("/") + "/bench/admin/reset"
    response = requests.post(url, timeout=30)
    response.raise_for_status()
    payload = response.json()
    if payload.get("code") != 200:
        raise RuntimeError(f"bench admin reset failed: {url} -> {payload}")


def post_bench_admin_json(base_url: str, path: str, payload: dict) -> dict:
    url = base_url.rstrip("/") + path
    response = requests.post(url, timeout=30, json=payload)
    response.raise_for_status()
    result = response.json()
    if result.get("code") != 200:
        raise RuntimeError(f"bench admin post failed: {url} -> {result}")
    return result.get("data", {})


def load_fixture_json(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def prepare_session_seq_recovery(
    bench_admin_base_url: str,
    fixture_path: Path,
    *,
    delete_pair_count: int,
    progress_cb=None,
) -> dict:
    fixture = load_fixture_json(fixture_path)
    pairs = list(fixture.get("single_pairs", []))
    if not pairs:
        raise RuntimeError("fixture single_pairs is empty, cannot prepare session_seq recovery")
    target_pairs = pairs[: max(0, delete_pair_count)]
    if not target_pairs:
        raise RuntimeError("session_seq recovery delete_pair_count resolved to 0")

    if progress_cb:
        progress_cb("recovery 准备：flush session_seq 高水位")
    post_bench_admin_json(bench_admin_base_url, "/bench/admin/session_seq/flush_state", {})

    deleted_keys: list[str] = []
    for pair in target_pairs:
        conversation_key = build_conversation_key(pair["sender_uuid"], pair["receiver_uuid"])
        post_bench_admin_json(
            bench_admin_base_url,
            "/bench/admin/session_seq/redis_key/delete",
            {"conversation_key": conversation_key},
        )
        deleted_keys.append(conversation_key)

    if progress_cb:
        progress_cb(f"recovery 准备：已删除 {len(deleted_keys)} 个 session_seq redis key")
    return {
        "deleted_pair_count": len(deleted_keys),
        "conversation_keys": deleted_keys,
    }


def build_throughput_report(summary: dict) -> str:
    expected = summary.get("expected_messages")
    received = summary.get("received_messages")
    received_before_drain = summary.get("received_messages_before_drain", received)
    recovered = summary.get("drain_recovered_messages")
    if recovered is None and isinstance(received, int) and isinstance(received_before_drain, int):
        recovered = received - received_before_drain
    lines = [
        "# 吞吐情况",
        "",
        f"- offered：`{summary.get('pair_count', 'unknown')} 对 / {summary.get('messages_per_sender', 'unknown')} 条/发送者 / send_interval={summary.get('send_interval_ms', 'unknown')}ms`",
        f"- 总消息：`{expected}`",
        f"- 结束前写回：`{received_before_drain}`",
        f"- 拖尾补完：`{recovered}`",
        f"- 客户端最终收到：`{received}`",
        f"- 发送窗口：`{summary.get('duration_sec')}s`",
        f"- 客户端 observed 吞吐（窗口内平均）：`{summary.get('observed_throughput_msg_per_sec')} msg/s`",
        f"- 成功率：`{summary.get('delivery_success_rate')}`",
    ]
    latency = summary.get("latency", {})
    if latency:
        lines.extend(
            [
                f"- p50：`{latency.get('p50_ms')}ms`",
                f"- p95：`{latency.get('p95_ms')}ms`",
                f"- p99：`{latency.get('p99_ms')}ms`",
                f"- max：`{latency.get('max_ms')}ms`",
            ]
        )
    return "\n".join(lines) + "\n"


def format_stage_line(name: str, stage_summary: dict) -> str:
    avg = stage_summary.get("avg_ms")
    if avg is None:
        return f"- {name}：`本轮未采到`"
    return f"- {name}：`{avg}ms`"


def build_stage_report(critical: dict) -> str:
    stages = critical.get("stage_metrics", {})
    ordered = [
        "ingress_to_produce_ack_ms",
        "kafka_queue_wait_ms",
        "deserialize_ms",
        "conversation_bucket_enqueue_ms",
        "conversation_dispatch_queue_wait_ms",
        "conversation_bucket_queue_wait_ms",
        "conversation_ready_queue_wait_ms",
        "mysql_persist_ms",
        "dispatch_after_persist_ms",
        "receiver_queue_wait_ms",
        "receiver_ws_write_ms",
        "server_critical_path_ms",
        "end_to_end_ms",
    ]
    lines = [
        "# 全链路分段",
        "",
        "## 摘要",
        "",
    ]
    for name in ordered:
        lines.append(format_stage_line(name, stages.get(name, {})))
    lines.extend(
        [
            "",
        "## 细粒度表",
        "",
        "| stage | count | min_ms | p10_ms | p25_ms | avg_ms | median_ms | p75_ms | p90_ms | p95_ms | p99_ms | p99_9_ms | max_ms | stddev_ms | p99/p50 |",
        "| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |",
        ]
    )
    for name in ordered:
        item = stages.get(name, {})
        if not item or item.get("count", 0) == 0:
            lines.append(f"| {name} | 0 | - | - | - | - | - | - | - | - | - | - | - | - | - |")
            continue
        lines.append(
            "| "
            + " | ".join(
                [
                    name,
                    str(item.get("count")),
                    str(item.get("min_ms")),
                    str(item.get("p10_ms")),
                    str(item.get("p25_ms")),
                    str(item.get("avg_ms")),
                    str(item.get("median_ms")),
                    str(item.get("p75_ms")),
                    str(item.get("p90_ms")),
                    str(item.get("p95_ms")),
                    str(item.get("p99_ms")),
                    str(item.get("p99_9_ms")),
                    str(item.get("max_ms")),
                    str(item.get("stddev_ms")),
                    str(item.get("p99_div_p50")),
                ]
            )
            + " |"
        )
    return "\n".join(lines) + "\n"


def build_persist_report(critical: dict) -> str:
    persist = critical.get("persist", {})
    stages = critical.get("stage_metrics", {})
    lines = [
        "# mysql_persist情况",
        "",
        f"- flush 总次数：`{persist.get('flush_count')}`",
        f"- flush reason：`{json.dumps(persist.get('flush_reason_counts', {}), ensure_ascii=False)}`",
        f"- worker 分布：`{json.dumps(persist.get('flush_worker_counts', {}), ensure_ascii=False)}`",
        f"- 平均 flush batch：`{persist.get('avg_flush_batch_size')}`",
        f"- 平均 enqueue queue depth：`{persist.get('avg_enqueue_queue_depth')}`",
        "",
        "## mysql_persist 细分",
        "",
        format_stage_line("enqueue_block", stages.get("mysql_persist_enqueue_block_ms", {})),
        format_stage_line("worker_queue_wait", stages.get("mysql_persist_worker_queue_wait_ms", {})),
        format_stage_line("batch_collect_wait", stages.get("mysql_persist_batch_collect_wait_ms", {})),
        format_stage_line("sql_exec", stages.get("mysql_persist_sql_exec_ms", {})),
        format_stage_line("flush", stages.get("mysql_persist_flush_ms", {})),
    ]
    return "\n".join(lines) + "\n"


def build_stage_report_from_analysis(analysis: dict) -> str:
    return build_stage_report({"stage_metrics": analysis.get("stage_metrics", {})})


def build_persist_report_from_analysis(analysis: dict) -> str:
    return build_persist_report({"persist": analysis.get("persist", {}), "stage_metrics": analysis.get("stage_metrics", {})})


def resolve_messages_per_sender(cfg: dict) -> int | None:
    scenario = cfg.get("scenario", {})
    target_rate = scenario.get("target_rate")
    duration_sec = scenario.get("duration_sec")
    session_count = scenario.get("session_count")
    if not all(isinstance(v, int) and v > 0 for v in [target_rate, duration_sec, session_count]):
        return None
    return max(1, round(target_rate * duration_sec / session_count))


def resolve_send_interval_ms(cfg: dict) -> int | None:
    scenario = cfg.get("scenario", {})
    target_rate = scenario.get("target_rate")
    session_count = scenario.get("session_count")
    if not all(isinstance(v, int) and v > 0 for v in [target_rate, session_count]):
        return None
    return max(1, round(session_count * 1000 / target_rate))


def resolve_offered_actual(cfg: dict) -> int | None:
    session_count = cfg.get("scenario", {}).get("session_count")
    send_interval_ms = resolve_send_interval_ms(cfg)
    if not isinstance(session_count, int) or session_count <= 0 or not isinstance(send_interval_ms, int) or send_interval_ms <= 0:
        return None
    return round(session_count * 1000 / send_interval_ms)


def run_critical_path_runner(trace_path: Path, summary_path: Path, output_path: Path) -> Path:
    script_path = ROOT / "pressure testing/scripts/single_chat_critical_path_runner.py"
    subprocess.run(
        [
            sys.executable,
            str(script_path),
            "--trace",
            str(trace_path),
            "--summary",
            str(summary_path),
            "--output",
            str(output_path),
        ],
        check=True,
    )
    return output_path


def package_reports(bundle_dir: Path, raw_dir: Path, cfg: dict) -> bool:
    summary_path = raw_dir / "summary.json"
    trace_path = raw_dir / "trace.json"
    if not summary_path.exists() or not trace_path.exists():
        return False
    analyze_reports, build_extended_reports, export_report_data, load_report_json = load_report_helpers()

    critical_path = raw_dir / "critical_path_summary.json"
    run_critical_path_runner(trace_path, summary_path, critical_path)
    critical = read_json(critical_path)
    summary = critical.get("summary", {})

    write_text(bundle_dir / "01_吞吐情况.md", build_throughput_report(summary))
    trace = load_report_json(trace_path)
    analysis = analyze_reports(summary, trace, cfg)
    write_text(bundle_dir / "02_全链路分段.md", build_stage_report_from_analysis(analysis))
    write_text(bundle_dir / "03_mysql_persist情况.md", build_persist_report_from_analysis(analysis))
    export_report_data(bundle_dir, analysis)
    build_extended_reports(bundle_dir, analysis, cfg)
    return True


def resolve_optional_path(cli_value: str | None, cfg_value: str | None) -> Path | None:
    raw = cli_value or cfg_value
    if not raw:
        return None
    return Path(raw).resolve()


def maybe_prepare_fixture(
    cfg: dict,
    raw_dir: Path,
    runtime_config_path: Path,
    runtime_service_cfg: dict,
    progress_cb=None,
) -> Path | None:
    run_cfg = cfg.get("run", {})
    benchmark_cfg = cfg.get("benchmark", {})
    fixture_path_raw = run_cfg.get("fixture_path")
    if fixture_path_raw:
        fixture_path = Path(str(fixture_path_raw)).resolve()
        if not fixture_path.exists():
            raise FileNotFoundError(f"fixture_path does not exist: {fixture_path}")
        return fixture_path

    seed_prefix = sanitize_seed_prefix(run_cfg.get("seed_prefix", ""))
    if not seed_prefix:
        return None

    scenario = cfg.get("scenario", {})
    user_count = int(scenario.get("user_count", 200))
    group_size = max(25, min(user_count, 25))
    env = ensure_go_runtime_env(os.environ.copy())
    env["ECHOCHAT_CONFIG"] = str(runtime_config_path)
    env["ECHOCHAT_REPO_ROOT"] = str(ROOT)
    if progress_cb:
        progress_cb(f"seed prefix={seed_prefix} user_count={user_count} fixture_pair_count={scenario.get('fixture_pair_count')}")

    seed_child_bar = ChildProgressBar("seed 造数")
    seed_batch_bar = ChildProgressBar("seed 当前批次")

    def handle_seed_progress(payload: dict) -> None:
        current = int(payload.get("current", 0))
        total = int(payload.get("total", 0))
        step = str(payload.get("step", "seed"))
        detail = str(payload.get("detail", "")).strip()
        scope = str(payload.get("scope", "stage")).strip().lower()
        label = f"{step}"
        if detail:
            label = f"{step} | {detail}"
        if scope == "batch":
            seed_batch_bar.update(current, total, label)
        else:
            seed_child_bar.update(current, total, label)

    try:
        run_command_streaming(
            [
                "go",
                "run",
                "./cmd/echo_chat_seed",
                "--prefix",
                seed_prefix,
                "--user-count",
                str(user_count),
                "--group-size",
                str(group_size),
                "--password",
                str(run_cfg.get("default_password", "123456")),
                f"--reset-prefix={'true' if bool(benchmark_cfg.get('seed_reset_prefix', True)) else 'false'}",
            ],
            cwd=ROOT,
            env=env,
            progress_cb=progress_cb,
            prefix="[seed] ",
            progress_event_handler=handle_seed_progress,
        )
    finally:
        seed_batch_bar.close()
        seed_child_bar.close()

    mysql_settings = runtime_mysql_settings(runtime_service_cfg)
    if progress_cb:
        progress_cb("构建 fixture（会话数自适应候选池 + 用户不重复 + 分区均衡）")
    return build_fixture_with_constraints(cfg, raw_dir, mysql_settings)


def maybe_run_message_latency_runner(
    cfg: dict,
    raw_dir: Path,
    bench_admin_base_url: str | None,
    runtime_config_path: Path,
    runtime_service_cfg: dict,
    server_pid: int | None = None,
    progress_cb=None,
) -> tuple[Path | None, bool]:
    run_cfg = cfg.get("run", {})
    benchmark = cfg.get("benchmark", {})
    fixture_path = maybe_prepare_fixture(
        cfg,
        raw_dir,
        runtime_config_path,
        runtime_service_cfg,
        progress_cb=progress_cb,
    )
    if fixture_path is None:
        return None, False

    base_url, ws_base_url, default_bench_admin_base_url = resolve_runtime_urls(run_cfg, runtime_service_cfg)
    bench_admin_base_url = (bench_admin_base_url or "").rstrip("/") or default_bench_admin_base_url

    if bench_admin_base_url:
        if progress_cb:
            progress_cb(f"重置 bench admin：{bench_admin_base_url}")
        post_bench_admin_reset(bench_admin_base_url)

    messages_per_sender = resolve_messages_per_sender(cfg)
    send_interval_ms = resolve_send_interval_ms(cfg)
    if messages_per_sender is None or send_interval_ms is None:
        raise ValueError("failed to derive messages_per_sender or send_interval_ms from config")

    scenario = cfg.get("scenario", {})
    script_path = ROOT / "docs/k6_message_test/scripts/message_latency_runner.py"
    output_dir = raw_dir / "runner"
    output_dir.mkdir(parents=True, exist_ok=True)
    def build_runner_command(
        *,
        command_output_dir: Path,
        command_messages_per_sender: int,
        command_send_interval_ms: int,
        command_message_timeout_ms: int,
        command_mode_label: str,
    ) -> list[str]:
        command = [
            sys.executable,
            str(script_path),
            "--base-url",
            base_url,
            "--ws-base-url",
            ws_base_url,
            "--ws-path",
            str(benchmark.get("ws_path", "/bench/wss")),
            "--fixture",
            str(fixture_path),
            "--scenario",
            "single",
            "--output-dir",
            str(command_output_dir),
            "--messages-per-sender",
            str(command_messages_per_sender),
            "--send-interval-ms",
            str(command_send_interval_ms),
            "--message-timeout-ms",
            str(command_message_timeout_ms),
            "--connection-settle-ms",
            str(benchmark.get("connection_settle_ms", 1500)),
            "--setup-workers",
            str(benchmark.get("setup_workers", 16)),
            "--setup-http-timeout-ms",
            str(benchmark.get("setup_http_timeout_ms", 90000)),
            "--ws-open-timeout-ms",
            str(benchmark.get("ws_open_timeout_ms", 30000)),
            "--drain-wait-ms",
            str(benchmark.get("drain_wait_ms", 5000)),
            "--drain-idle-ms",
            str(benchmark.get("drain_idle_ms", 1000)),
            "--pair-count",
            str(scenario.get("session_count", 0)),
            "--mode-label",
            command_mode_label,
            "--plain-progress",
        ]
        if server_pid:
            command.extend(["--server-pid", str(server_pid)])
        return command

    if bool(benchmark.get("session_seq_warmup_enabled", False)):
        warmup_output_dir = raw_dir / "warmup_runner"
        warmup_output_dir.mkdir(parents=True, exist_ok=True)
        warmup_interval_ms = int(benchmark.get("session_seq_warmup_interval_ms", 10))
        warmup_timeout_ms = int(benchmark.get("session_seq_warmup_timeout_ms", 15000))
        warmup_settle_ms = int(benchmark.get("session_seq_warmup_settle_ms", 1000))
        if bench_admin_base_url:
            if progress_cb:
                progress_cb("预热前重置 bench admin")
            post_bench_admin_reset(bench_admin_base_url)
        if progress_cb:
            progress_cb(
                f"全会话预热 1 条：pair_count={scenario.get('session_count')} interval={warmup_interval_ms}ms"
            )
        run_command_streaming(
            build_runner_command(
                command_output_dir=warmup_output_dir,
                command_messages_per_sender=1,
                command_send_interval_ms=warmup_interval_ms,
                command_message_timeout_ms=warmup_timeout_ms,
                command_mode_label="warmup",
            ),
            cwd=ROOT,
            progress_cb=progress_cb,
            prefix="[warmup] ",
        )
        if warmup_settle_ms > 0:
            if progress_cb:
                progress_cb(f"预热完成，等待链路稳定 {warmup_settle_ms}ms")
            time.sleep(warmup_settle_ms / 1000.0)
        if bool(benchmark.get("session_seq_force_flush_after_warmup", False)):
            if progress_cb:
                progress_cb("预热后 flush session_seq 高水位")
            post_bench_admin_json(bench_admin_base_url, "/bench/admin/session_seq/flush_state", {})
        if bool(benchmark.get("session_seq_prepare_recovery_after_warmup", False)):
            delete_pair_count = int(benchmark.get("session_seq_recovery_delete_pair_count", scenario.get("session_count", 0)))
            recovery_context = prepare_session_seq_recovery(
                bench_admin_base_url,
                fixture_path,
                delete_pair_count=delete_pair_count,
                progress_cb=progress_cb,
            )
            write_text(raw_dir / "session_seq_recovery_context.json", json.dumps(recovery_context, ensure_ascii=False, indent=2))
        if bench_admin_base_url:
            if progress_cb:
                progress_cb("正式压测前重置 bench admin")
            post_bench_admin_reset(bench_admin_base_url)
    else:
        if bench_admin_base_url:
            if progress_cb:
                progress_cb(f"重置 bench admin：{bench_admin_base_url}")
            post_bench_admin_reset(bench_admin_base_url)

    if progress_cb:
        progress_cb(
            f"启动 message_latency_runner：pair_count={scenario.get('session_count')} duration={scenario.get('duration_sec')}s"
        )

    run_command_streaming(
        build_runner_command(
            command_output_dir=output_dir,
            command_messages_per_sender=int(messages_per_sender),
            command_send_interval_ms=int(send_interval_ms),
            command_message_timeout_ms=int(benchmark.get("message_timeout_ms", 60000)),
            command_mode_label="pressure",
        ),
        cwd=ROOT,
        progress_cb=progress_cb,
        prefix="[pressure] ",
    )
    return output_dir / "summary.json", True


def main() -> int:
    configure_live_output()
    config_arg = None
    for idx, arg in enumerate(sys.argv[:-1]):
        if arg == "--config":
            config_arg = sys.argv[idx + 1]
            break
    startup_notice(config_arg)
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True)
    parser.add_argument("--summary-json")
    parser.add_argument("--trace-json")
    parser.add_argument("--bench-admin-base-url")
    args = parser.parse_args()

    config_path = Path(args.config).resolve()
    cfg = apply_mainconfig_overrides(read_toml(config_path))

    bundle_root = resolve_record_root(cfg)
    bundle_label = build_bundle_label(cfg)
    bundle_dir = bundle_root / bundle_label
    raw_dir = bundle_dir / "raw_runner"
    raw_dir.mkdir(parents=True, exist_ok=True)

    progress = StageProgress(total=11)
    progress.set(1, "读取配置", str(config_path))
    progress.set(2, "创建产物目录", str(bundle_dir))

    write_text(bundle_dir / "00_本次口径.md", build_scope_report(cfg))
    write_text(bundle_dir / "config_used.toml", config_path.read_text(encoding="utf-8"))

    summary_path = resolve_optional_path(args.summary_json, cfg.get("run", {}).get("summary_json"))
    trace_path = resolve_optional_path(args.trace_json, cfg.get("run", {}).get("trace_json"))
    bench_admin_base_url = args.bench_admin_base_url or cfg.get("run", {}).get("bench_admin_base_url")
    mysql_process = None
    mysql_runtime_layout = None
    server_process = None
    runtime_config_path = None

    def substep(detail: str) -> None:
        progress.set(progress.current_stage or 7, "执行中", detail)

    try:
        if summary_path and summary_path.exists():
            shutil.copy2(summary_path, raw_dir / "summary.json")
            progress.set(3, "接收外部 summary", str(summary_path))
        if trace_path and trace_path.exists():
            shutil.copy2(trace_path, raw_dir / "trace.json")
            progress.set(4, "接收外部 trace", str(trace_path))
        elif not summary_path and not trace_path:
            progress.set(3, "派生运行配置")
            mysql_runtime_layout = build_mysql_runtime_layout(cfg, bundle_label)
            runtime_config_path, runtime_service_cfg = derive_runtime_service_config(
                cfg,
                raw_dir,
                bundle_label,
                mysql_runtime_layout=mysql_runtime_layout,
            )
            run_cfg = cfg.get("run", {})
            _, _, default_bench_admin_base_url = resolve_runtime_urls(run_cfg, runtime_service_cfg)
            bench_admin_base_url = (bench_admin_base_url or "").rstrip("/") or default_bench_admin_base_url
            progress.set(3, "派生运行配置", str(runtime_config_path))
            mysql_settings = runtime_mysql_settings(runtime_service_cfg)
            progress.set(4, "初始化隔离 MySQL", str(mysql_runtime_layout["root"]))
            initialize_isolated_mysql_runtime(mysql_runtime_layout, progress_cb=substep)
            mysql_process = start_local_mysql_if_needed(
                mysql_settings,
                mysql_runtime_layout,
                progress_cb=substep,
            )
            progress.set(5, "确保 MySQL 数据库", str(mysql_settings.get("database", "")))
            ensure_mysql_database(mysql_settings)
            progress.set(6, "启动后端服务", bench_admin_base_url or "bench-admin")
            server_process = start_local_server_if_needed(runtime_config_path, bench_admin_base_url, raw_dir)
            if bool(cfg.get("benchmark", {}).get("session_seq_reset_state_before_run", False)):
                progress.set(6, "重置 session_seq 初始化状态", bench_admin_base_url)
                post_bench_admin_json(bench_admin_base_url, "/bench/admin/session_seq/reset_state", {})
            progress.set(7, "准备 fixture 与执行压测")
            generated_summary_path, ran_latency_runner = maybe_run_message_latency_runner(
                cfg,
                raw_dir,
                bench_admin_base_url,
                runtime_config_path,
                runtime_service_cfg,
                server_pid=server_process.pid if server_process else None,
                progress_cb=substep,
            )
            if ran_latency_runner:
                progress.set(7, "执行压测完成", str(generated_summary_path))
                if generated_summary_path and generated_summary_path.exists():
                    shutil.copy2(generated_summary_path, raw_dir / "summary.json")
            progress.set(8, "拉取 trace 与 metrics", bench_admin_base_url)
            trace = fetch_bench_json(bench_admin_base_url, "/bench/admin/trace")
            metrics_snapshot = fetch_bench_json(bench_admin_base_url, "/bench/admin/metrics_snapshot")
            write_text(raw_dir / "trace.json", json.dumps(trace, ensure_ascii=False, indent=2))
            write_text(raw_dir / "metrics_snapshot.json", json.dumps(metrics_snapshot, ensure_ascii=False, indent=2))
            progress.set(8, "拉取 trace 与 metrics 完成", bench_admin_base_url)
        elif bench_admin_base_url:
            progress.set(8, "拉取 trace 与 metrics", bench_admin_base_url)
            trace = fetch_bench_json(bench_admin_base_url, "/bench/admin/trace")
            metrics_snapshot = fetch_bench_json(bench_admin_base_url, "/bench/admin/metrics_snapshot")
            write_text(raw_dir / "trace.json", json.dumps(trace, ensure_ascii=False, indent=2))
            write_text(raw_dir / "metrics_snapshot.json", json.dumps(metrics_snapshot, ensure_ascii=False, indent=2))
            progress.set(8, "拉取 trace 与 metrics 完成", bench_admin_base_url)

        progress.set(9, "生成汇总与中间数据")
        packaged = package_reports(bundle_dir, raw_dir, cfg)
        bootstrap = {
            "config_path": str(config_path),
            "bundle_dir": str(bundle_dir),
            "raw_dir": str(raw_dir),
            "runtime_config_path": str(runtime_config_path) if runtime_config_path else None,
            "mysql_runtime_root": str(mysql_runtime_layout["root"]) if mysql_runtime_layout else None,
            "status": "packaged_00_13" if packaged else "runner_skeleton_restored",
        }
        write_text(bundle_dir / "bootstrap_context.json", json.dumps(bootstrap, ensure_ascii=False, indent=2))

        if packaged:
            progress.set(10, "生成 00~13 全量报告")
        else:
            progress.set(10, "仅生成部分报告", "缺少 summary.json 或 trace.json")
            tqdm.write("[10/11] 可通过 --summary-json/--trace-json 或 --bench-admin-base-url 补齐原始数据")
    finally:
        progress.set(11, "清理进程与运行时目录")
        stop_process(server_process)
        stop_process(mysql_process)
        cleanup_mysql_runtime(cfg, mysql_runtime_layout)
        progress.close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
