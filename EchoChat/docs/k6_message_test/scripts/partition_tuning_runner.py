#!/usr/bin/env python3

from __future__ import annotations

import argparse
import csv
import json
import math
import os
import subprocess
import threading
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any

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
RECORD_ROOT = Path(os.environ.get("ECHOCHAT_PARTITION_TUNING_RECORD_ROOT", str(TEST_DIR / "partition_tuning_records")))


def load_json(path: Path) -> Any:
    return json.loads(path.read_text(encoding="utf-8"))


def write_json(path: Path, data: Any) -> None:
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def write_csv(path: Path, rows: list[dict[str, Any]]) -> None:
    if not rows:
        path.write_text("", encoding="utf-8")
        return
    with path.open("w", encoding="utf-8", newline="") as fp:
        writer = csv.DictWriter(fp, fieldnames=list(rows[0].keys()))
        writer.writeheader()
        writer.writerows(rows)


def git_commit() -> str:
    try:
        result = subprocess.run(
            ["git", "rev-parse", "--short", "HEAD"],
            cwd=str(ROOT_DIR),
            text=True,
            capture_output=True,
            check=True,
        )
        return result.stdout.strip()
    except Exception:
        return "unknown"


def git_dirty() -> bool:
    try:
        result = subprocess.run(
            ["git", "status", "--porcelain"],
            cwd=str(ROOT_DIR),
            text=True,
            capture_output=True,
            check=True,
        )
        return bool(result.stdout.strip())
    except Exception:
        return False


def parse_partition_config(raw: str) -> tuple[int, Path]:
    part, path = raw.split("=", 1)
    return int(part.strip()), Path(path.strip()).resolve()


def parse_partition_run(raw: str) -> tuple[int, Path]:
    part, path = raw.split("=", 1)
    return int(part.strip()), Path(path.strip())


def percentile(values: list[float], p: float) -> float | None:
    if not values:
        return None
    if len(values) == 1:
        return float(values[0])
    ordered = sorted(values)
    rank = (len(ordered) - 1) * p
    low = math.floor(rank)
    high = math.ceil(rank)
    if low == high:
        return float(ordered[low])
    weight = rank - low
    return float(ordered[low] * (1 - weight) + ordered[high] * weight)


def round_or_none(value: float | None, digits: int = 3) -> float | None:
    if value is None:
        return None
    return round(value, digits)


def parse_label_string(raw: str | None) -> dict[str, str]:
    if not raw:
        return {}
    labels: dict[str, str] = {}
    for item in raw.split(","):
        item = item.strip()
        if not item or "=" not in item:
            continue
        key, value = item.split("=", 1)
        labels[key.strip()] = value.strip().strip('"')
    return labels


def parse_metric_line(line: str) -> tuple[str, dict[str, str], float] | None:
    line = line.strip()
    if not line or line.startswith("#"):
        return None
    metric_part, value_part = line.rsplit(" ", 1)
    if "{" in metric_part:
        name, raw_labels = metric_part.split("{", 1)
        labels = parse_label_string(raw_labels[:-1])
    else:
        name = metric_part
        labels = {}
    try:
        value = float(value_part)
    except ValueError:
        return None
    return name, labels, value


class PrometheusSnapshot:
    def __init__(self) -> None:
        self.samples: list[tuple[str, dict[str, str], float]] = []

    @classmethod
    def from_files(cls, paths: list[Path]) -> "PrometheusSnapshot":
        snapshot = cls()
        for path in paths:
            if not path.exists():
                continue
            for raw_line in path.read_text(encoding="utf-8", errors="ignore").splitlines():
                parsed = parse_metric_line(raw_line)
                if parsed is not None:
                    snapshot.samples.append(parsed)
        return snapshot

    def series(
        self,
        name: str,
        *,
        label_filters: dict[str, str] | None = None,
        exclude_labels: dict[str, str] | None = None,
    ) -> list[tuple[dict[str, str], float]]:
        rows: list[tuple[dict[str, str], float]] = []
        for sample_name, labels, value in self.samples:
            if sample_name != name:
                continue
            if label_filters:
                if any(labels.get(key) != expected for key, expected in label_filters.items()):
                    continue
            if exclude_labels:
                if any(labels.get(key) == expected for key, expected in exclude_labels.items()):
                    continue
            rows.append((labels, value))
        return rows

    def gauge_sum(self, name: str) -> float | None:
        rows = self.series(name)
        if not rows:
            return None
        return sum(value for _, value in rows)

    def counter_by_label(self, name: str, label: str, *, label_filters: dict[str, str] | None = None) -> dict[str, float]:
        result: dict[str, float] = {}
        for labels, value in self.series(name, label_filters=label_filters):
            label_value = labels.get(label)
            if label_value is None:
                continue
            result[label_value] = result.get(label_value, 0.0) + value
        return result

    def histogram_summary(
        self,
        metric_base: str,
        *,
        label_filters: dict[str, str] | None = None,
        exclude_labels: dict[str, str] | None = None,
    ) -> dict[str, float | None]:
        buckets: dict[float, float] = {}
        total_sum = 0.0
        total_count = 0.0
        for name, labels, value in self.samples:
            if not name.startswith(metric_base):
                continue
            if label_filters:
                if any(labels.get(key) != expected for key, expected in label_filters.items()):
                    continue
            if exclude_labels:
                if any(labels.get(key) == expected for key, expected in exclude_labels.items()):
                    continue
            if name == f"{metric_base}_sum":
                total_sum += value
            elif name == f"{metric_base}_count":
                total_count += value
            elif name == f"{metric_base}_bucket":
                le_raw = labels.get("le")
                if le_raw is None:
                    continue
                le_value = math.inf if le_raw == "+Inf" else float(le_raw)
                buckets[le_value] = buckets.get(le_value, 0.0) + value
        if total_count <= 0:
            return {"count": None, "avg": None, "p95": None, "p99": None}
        avg = total_sum / total_count
        return {
            "count": total_count,
            "avg": avg,
            "p95": estimate_histogram_quantile(buckets, total_count, 0.95),
            "p99": estimate_histogram_quantile(buckets, total_count, 0.99),
        }


def estimate_histogram_quantile(buckets: dict[float, float], total_count: float, quantile: float) -> float | None:
    if total_count <= 0 or not buckets:
        return None
    ordered = sorted(buckets.items(), key=lambda item: item[0])
    target = total_count * quantile
    previous_upper = 0.0
    previous_count = 0.0
    for upper, cumulative_count in ordered:
        if cumulative_count >= target:
            if math.isinf(upper):
                return previous_upper
            bucket_count = cumulative_count - previous_count
            if bucket_count <= 0:
                return upper
            position = (target - previous_count) / bucket_count
            return previous_upper + (upper - previous_upper) * position
        previous_upper = upper
        previous_count = cumulative_count
    finite = [upper for upper, _ in ordered if not math.isinf(upper)]
    return finite[-1] if finite else None


def read_proc_status(pid: int) -> tuple[int, int]:
    status_path = Path(f"/proc/{pid}/status")
    if not status_path.exists():
        return 0, 0
    rss_bytes = 0
    threads = 0
    for line in status_path.read_text(encoding="utf-8", errors="ignore").splitlines():
        if line.startswith("VmRSS:"):
            rss_bytes = int(line.split()[1]) * 1024
        elif line.startswith("Threads:"):
            threads = int(line.split()[1])
    return rss_bytes, threads


def read_fd_count(pid: int) -> int:
    fd_path = Path(f"/proc/{pid}/fd")
    try:
        return len(list(fd_path.iterdir()))
    except Exception:
        return 0


def read_proc_table() -> dict[int, tuple[int, str]]:
    table: dict[int, tuple[int, str]] = {}
    for proc_dir in Path("/proc").iterdir():
        if not proc_dir.name.isdigit():
            continue
        pid = int(proc_dir.name)
        stat_path = proc_dir / "stat"
        comm_path = proc_dir / "comm"
        try:
            stat_content = stat_path.read_text(encoding="utf-8", errors="ignore")
            first = stat_content.find("(")
            last = stat_content.rfind(")")
            rest = stat_content[last + 2 :].split()
            ppid = int(rest[1])
            comm = comm_path.read_text(encoding="utf-8", errors="ignore").strip()
            table[pid] = (ppid, comm)
        except Exception:
            continue
    return table


class ServerProcessSampler:
    def __init__(self, root_pid: int, interval_sec: float = 0.2) -> None:
        self.root_pid = root_pid
        self.interval_sec = interval_sec
        self.stop_event = threading.Event()
        self.thread: threading.Thread | None = None
        self.samples = 0
        self.rss_peak_bytes = 0
        self.threads_peak = 0
        self.fd_peak = 0
        self.process_count_peak = 0
        self.seen_pids: set[int] = set()

    def _descendants(self, table: dict[int, tuple[int, str]]) -> set[int]:
        children: dict[int, list[int]] = {}
        for pid, (ppid, _) in table.items():
            children.setdefault(ppid, []).append(pid)
        result: set[int] = set()
        stack = [self.root_pid]
        while stack:
            current = stack.pop()
            for child in children.get(current, []):
                if child in result:
                    continue
                result.add(child)
                stack.append(child)
        return result

    def _sample_once(self) -> None:
        table = read_proc_table()
        descendants = self._descendants(table)
        server_pids = [
            pid for pid in descendants if table.get(pid, (0, ""))[1] == "echo_chat_server"
        ]
        total_rss = 0
        total_threads = 0
        total_fd = 0
        for pid in server_pids:
            rss_bytes, threads = read_proc_status(pid)
            total_rss += rss_bytes
            total_threads += threads
            total_fd += read_fd_count(pid)
            self.seen_pids.add(pid)
        self.samples += 1
        self.rss_peak_bytes = max(self.rss_peak_bytes, total_rss)
        self.threads_peak = max(self.threads_peak, total_threads)
        self.fd_peak = max(self.fd_peak, total_fd)
        self.process_count_peak = max(self.process_count_peak, len(server_pids))

    def _loop(self) -> None:
        while not self.stop_event.is_set():
            self._sample_once()
            self.stop_event.wait(self.interval_sec)

    def start(self) -> None:
        self.thread = threading.Thread(target=self._loop, daemon=True)
        self.thread.start()

    def stop(self) -> dict[str, Any]:
        self.stop_event.set()
        if self.thread is not None:
            self.thread.join(timeout=1.0)
        self._sample_once()
        return {
            "samples": self.samples,
            "rss_peak_mb_total": round(self.rss_peak_bytes / 1024 / 1024, 3),
            "threads_peak_total": self.threads_peak,
            "fd_peak_total": self.fd_peak,
            "server_process_peak": self.process_count_peak,
            "server_process_seen": len(self.seen_pids),
        }


@dataclass
class ScenarioMetrics:
    scenario: str
    partition_count: int
    throughput: float | None
    p95_ms: float | None
    p99_ms: float | None
    success: float | None
    duration_sec: float | None
    received_total: float | None
    configured_partitions: int | None
    producer_used_partitions: int | None
    producer_hot_share_pct: float | None
    producer_skew_factor: float | None
    consumer_used_partitions: int | None
    consumer_hot_share_pct: float | None
    consumer_skew_factor: float | None
    consumer_lag_p95: float | None
    consumer_lag_max: float | None
    consumer_total_p95_ms: float | None
    heaviest_stage: str | None
    heaviest_stage_share_pct: float | None
    offset_commit_p95_ms: float | None
    offset_commit_batch_avg: float | None
    mysql_open_conns: float | None
    mysql_in_use_conns: float | None
    mysql_in_use_ratio: float | None
    mysql_wait_count_total: float | None
    mysql_wait_duration_ms_total: float | None
    mysql_wait_ms_per_1k_msgs: float | None
    ws_sendback_queue_p95: float | None
    ws_enqueue_p95_ms: float | None
    ws_write_p95_ms: float | None
    ws_status_update_p95_ms: float | None
    redis_p95_ms: float | None
    active_key_count: int | None
    keys_per_used_partition: float | None
    partitions_per_active_key: float | None
    recommendation: str | None = None
    recommendation_reason: str | None = None
    throughput_gain_vs_prev_pct: float | None = None
    rss_growth_vs_prev_pct: float | None = None
    fd_growth_vs_prev_pct: float | None = None

    def to_row(self) -> dict[str, Any]:
        return self.__dict__.copy()


def load_config_info(config_path: Path) -> dict[str, Any]:
    with config_path.open("rb") as fp:
        config = tomllib.load(fp)
    kafka_config = config.get("kafkaConfig", {})
    mysql_config = config.get("mysqlConfig", {})
    return {
        "topic_partitions": kafka_config.get("topicPartitions"),
        "max_open_conns": mysql_config.get("maxOpenConns"),
    }


def compute_distribution(series: dict[str, float]) -> tuple[int | None, float | None, float | None]:
    if not series:
        return None, None, None
    values = [value for value in series.values() if value > 0]
    if not values:
        return 0, None, None
    total = sum(values)
    hottest = max(values)
    avg = total / len(values)
    hot_share_pct = hottest / total * 100 if total > 0 else None
    skew_factor = hottest / avg if avg > 0 else None
    return len(values), hot_share_pct, skew_factor


def analyze_stage_metrics(
    *,
    scenario: str,
    partition_count: int,
    summary: dict[str, Any],
    metrics_manifest_path: Path,
    run_dir: Path,
    config_path: Path,
    resource_peak: dict[str, Any] | None,
    instance_count: int,
) -> tuple[ScenarioMetrics, dict[str, Any]]:
    manifest = load_json(metrics_manifest_path)
    metrics_paths = []
    for item in manifest.get("instances", []):
        relative = item.get("path")
        if relative:
            metrics_paths.append(run_dir / relative)
    snapshot = PrometheusSnapshot.from_files(metrics_paths)
    config_info = load_config_info(config_path)
    configured_partitions = config_info.get("topic_partitions") or partition_count

    producer_by_partition = snapshot.counter_by_label(
        "echochat_kafka_producer_partition_messages_total",
        "partition",
    )
    producer_used_partitions, producer_hot_share_pct, producer_skew_factor = compute_distribution(producer_by_partition)

    consumer_by_partition = snapshot.counter_by_label(
        "echochat_kafka_consumer_messages_total",
        "partition",
    )
    consumer_used_partitions, consumer_hot_share_pct, consumer_skew_factor = compute_distribution(consumer_by_partition)

    lag_values = [value for _, value in snapshot.series("echochat_kafka_consumer_lag")]
    lag_p95 = percentile(lag_values, 0.95)
    lag_max = max(lag_values) if lag_values else None

    stage_rows: list[dict[str, Any]] = []
    total_stage_sum = 0.0
    stage_names = sorted({labels.get("stage") for labels, _ in snapshot.series("echochat_kafka_consumer_stage_duration_seconds_sum") if labels.get("stage")})
    for stage_name in stage_names:
        hist = snapshot.histogram_summary(
            "echochat_kafka_consumer_stage_duration_seconds",
            label_filters={"stage": stage_name},
        )
        if hist["count"] is None:
            continue
        stage_sum = hist["avg"] * hist["count"] if hist["avg"] is not None else 0.0
        total_stage_sum += stage_sum
        stage_rows.append(
            {
                "stage": stage_name,
                "count": hist["count"],
                "avg_ms": round_or_none(hist["avg"] * 1000 if hist["avg"] is not None else None),
                "p95_ms": round_or_none(hist["p95"] * 1000 if hist["p95"] is not None else None),
                "sum_s": stage_sum,
            }
        )
    heaviest_stage = None
    heaviest_stage_share_pct = None
    if stage_rows:
        heaviest = max(stage_rows, key=lambda item: item["sum_s"])
        heaviest_stage = heaviest["stage"]
        if total_stage_sum > 0:
            heaviest_stage_share_pct = heaviest["sum_s"] / total_stage_sum * 100

    total_hist = snapshot.histogram_summary(
        "echochat_kafka_consumer_stage_duration_seconds",
        label_filters={"stage": "total"},
    )
    offset_hist = snapshot.histogram_summary("echochat_kafka_offset_commit_duration_seconds")
    commit_batch_hist = snapshot.histogram_summary("echochat_kafka_offset_commit_batch_size")

    mysql_open = snapshot.gauge_sum("echochat_mysql_open_connections")
    mysql_in_use = snapshot.gauge_sum("echochat_mysql_in_use_connections")
    mysql_wait_count = snapshot.gauge_sum("echochat_mysql_wait_count_total")
    mysql_wait_duration_s = snapshot.gauge_sum("echochat_mysql_wait_duration_seconds")
    max_open_conns = config_info.get("max_open_conns")
    total_max_open = max_open_conns * instance_count if isinstance(max_open_conns, int) else None
    mysql_in_use_ratio = None
    if mysql_in_use is not None and total_max_open:
        mysql_in_use_ratio = mysql_in_use / total_max_open

    ws_queue_hist = snapshot.histogram_summary(
        "echochat_ws_sendback_queue_length",
        label_filters={"result": "success"},
    )
    ws_enqueue_hist = snapshot.histogram_summary(
        "echochat_ws_sendback_enqueue_duration_seconds",
        label_filters={"result": "success"},
    )
    ws_write_hist = snapshot.histogram_summary(
        "echochat_ws_write_duration_seconds",
        label_filters={"result": "success"},
    )
    ws_status_hist = snapshot.histogram_summary(
        "echochat_ws_status_update_duration_seconds",
        label_filters={"result": "success"},
    )
    redis_hist = snapshot.histogram_summary(
        "echochat_redis_command_duration_seconds",
        exclude_labels={"result": "failure"},
    )

    if scenario == "single":
        throughput = summary.get("observed_throughput_msg_per_sec")
        p95_ms = summary.get("latency", {}).get("p95_ms")
        p99_ms = summary.get("latency", {}).get("p99_ms")
        success = summary.get("delivery_success_rate")
        duration_sec = summary.get("duration_sec")
        received_total = summary.get("received_messages")
        active_key_count = int(summary.get("pair_count", 0) or 0)
    else:
        throughput = summary.get("observed_delivery_per_sec")
        p95_ms = summary.get("receipt_latency", {}).get("p95_ms")
        p99_ms = summary.get("receipt_latency", {}).get("p99_ms")
        success = summary.get("delivery_coverage_rate")
        duration_sec = summary.get("duration_sec")
        received_total = summary.get("received_receipts")
        active_key_count = 1

    keys_per_used_partition = None
    if active_key_count and producer_used_partitions:
        keys_per_used_partition = active_key_count / producer_used_partitions
    partitions_per_active_key = None
    if active_key_count:
        partitions_per_active_key = configured_partitions / active_key_count

    mysql_wait_duration_ms_total = mysql_wait_duration_s * 1000 if mysql_wait_duration_s is not None else None
    mysql_wait_ms_per_1k_msgs = None
    if mysql_wait_duration_ms_total is not None and received_total:
        mysql_wait_ms_per_1k_msgs = mysql_wait_duration_ms_total / float(received_total) * 1000

    metrics = ScenarioMetrics(
        scenario=scenario,
        partition_count=partition_count,
        throughput=throughput,
        p95_ms=p95_ms,
        p99_ms=p99_ms,
        success=success,
        duration_sec=duration_sec,
        received_total=received_total,
        configured_partitions=configured_partitions,
        producer_used_partitions=producer_used_partitions,
        producer_hot_share_pct=round_or_none(producer_hot_share_pct),
        producer_skew_factor=round_or_none(producer_skew_factor),
        consumer_used_partitions=consumer_used_partitions,
        consumer_hot_share_pct=round_or_none(consumer_hot_share_pct),
        consumer_skew_factor=round_or_none(consumer_skew_factor),
        consumer_lag_p95=round_or_none(lag_p95),
        consumer_lag_max=round_or_none(lag_max),
        consumer_total_p95_ms=round_or_none(total_hist["p95"] * 1000 if total_hist["p95"] is not None else None),
        heaviest_stage=heaviest_stage,
        heaviest_stage_share_pct=round_or_none(heaviest_stage_share_pct),
        offset_commit_p95_ms=round_or_none(offset_hist["p95"] * 1000 if offset_hist["p95"] is not None else None),
        offset_commit_batch_avg=round_or_none(commit_batch_hist["avg"]),
        mysql_open_conns=round_or_none(mysql_open),
        mysql_in_use_conns=round_or_none(mysql_in_use),
        mysql_in_use_ratio=round_or_none(mysql_in_use_ratio),
        mysql_wait_count_total=round_or_none(mysql_wait_count),
        mysql_wait_duration_ms_total=round_or_none(mysql_wait_duration_ms_total),
        mysql_wait_ms_per_1k_msgs=round_or_none(mysql_wait_ms_per_1k_msgs),
        ws_sendback_queue_p95=round_or_none(ws_queue_hist["p95"]),
        ws_enqueue_p95_ms=round_or_none(ws_enqueue_hist["p95"] * 1000 if ws_enqueue_hist["p95"] is not None else None),
        ws_write_p95_ms=round_or_none(ws_write_hist["p95"] * 1000 if ws_write_hist["p95"] is not None else None),
        ws_status_update_p95_ms=round_or_none(ws_status_hist["p95"] * 1000 if ws_status_hist["p95"] is not None else None),
        redis_p95_ms=round_or_none(redis_hist["p95"] * 1000 if redis_hist["p95"] is not None else None),
        active_key_count=active_key_count,
        keys_per_used_partition=round_or_none(keys_per_used_partition),
        partitions_per_active_key=round_or_none(partitions_per_active_key),
    )
    detail = {
        "resource_peak": resource_peak,
        "stage_rows": stage_rows,
        "producer_by_partition": producer_by_partition,
        "consumer_by_partition": consumer_by_partition,
    }
    return metrics, detail


def recommend(metrics: ScenarioMetrics, previous: ScenarioMetrics | None, resource_peak: dict[str, Any] | None) -> None:
    reasons: list[str] = []
    throughput_gain = None
    rss_growth = None
    fd_growth = None
    if previous and previous.throughput and metrics.throughput:
        throughput_gain = (metrics.throughput - previous.throughput) / previous.throughput * 100
        metrics.throughput_gain_vs_prev_pct = round_or_none(throughput_gain)
    if previous and resource_peak and previous.partition_count:
        prev_rss = previous_extra.get(previous.partition_count, {}).get("resource_peak", {}).get("rss_peak_mb_total")
        prev_fd = previous_extra.get(previous.partition_count, {}).get("resource_peak", {}).get("fd_peak_total")
        curr_rss = resource_peak.get("rss_peak_mb_total")
        curr_fd = resource_peak.get("fd_peak_total")
        if prev_rss not in (None, 0) and curr_rss is not None:
            rss_growth = (curr_rss - prev_rss) / prev_rss * 100
            metrics.rss_growth_vs_prev_pct = round_or_none(rss_growth)
        if prev_fd not in (None, 0) and curr_fd is not None:
            fd_growth = (curr_fd - prev_fd) / prev_fd * 100
            metrics.fd_growth_vs_prev_pct = round_or_none(fd_growth)

    poor_marginal_gain = throughput_gain is not None and throughput_gain < 5
    resource_growth_high = (rss_growth is not None and rss_growth > 20) or (fd_growth is not None and fd_growth > 20)
    hot_skew = metrics.consumer_skew_factor is not None and metrics.consumer_skew_factor > 2.5
    lag_high = metrics.consumer_lag_p95 is not None and metrics.consumer_lag_p95 > 50
    mysql_pool_tight = (
        (metrics.mysql_in_use_ratio is not None and metrics.mysql_in_use_ratio > 0.85)
        or (metrics.mysql_wait_ms_per_1k_msgs is not None and metrics.mysql_wait_ms_per_1k_msgs > 20)
    )
    ws_backpressure = (
        (metrics.ws_sendback_queue_p95 is not None and metrics.ws_sendback_queue_p95 > 10)
        or (metrics.ws_enqueue_p95_ms is not None and metrics.ws_enqueue_p95_ms > 1.0)
    )
    over_partitioned = (
        metrics.partitions_per_active_key is not None
        and metrics.partitions_per_active_key > 4
        and poor_marginal_gain
    )

    if over_partitioned:
        reasons.append("当前分区数已经远高于活跃会话数，继续加分区的边际收益很低")
    if hot_skew:
        reasons.append("虽然分区变多，但热点分区仍然明显偏热")
    if lag_high:
        reasons.append("consumer lag 的 p95 已经抬高，说明消费侧管理成本或排队在上升")
    if mysql_pool_tight:
        reasons.append("MySQL 连接池等待开始明显出现，后端出口压力在升高")
    if ws_backpressure:
        reasons.append("WS 回推队列或 enqueue 延迟开始变差，实时分发副作用在放大")
    if poor_marginal_gain and resource_growth_high:
        reasons.append("吞吐增长已经放缓，但 RSS/FD 开销还在继续上升")

    if previous is None:
        metrics.recommendation = "基线档"
        metrics.recommendation_reason = "先作为分区扫描的起点，不做增减判断"
        return
    if poor_marginal_gain and (resource_growth_high or mysql_pool_tight or over_partitioned):
        metrics.recommendation = "不建议继续加分区"
    elif hot_skew or lag_high:
        metrics.recommendation = "可以继续试更大分区，但要优先盯热点和 lag"
    else:
        metrics.recommendation = "当前分区副作用可控"
    metrics.recommendation_reason = "；".join(reasons) if reasons else "当前档位的吞吐收益还在，副作用没有明显失控"


previous_extra: dict[int, dict[str, Any]] = {}


class PartitionTuningRunner:
    def __init__(self, args: argparse.Namespace) -> None:
        self.args = args
        self.timestamp = time.strftime("%Y%m%d_%H%M%S")
        label_suffix = f"_{args.label}" if args.label else ""
        self.run_dir = RECORD_ROOT / f"partition_tuning_{args.scenario}_{self.timestamp}{label_suffix}"

    def execute_throughput_run(self, partition_count: int, config_path: Path) -> tuple[Path, dict[str, Any]]:
        label = f"{self.args.label}_part{partition_count}" if self.args.label else f"part{partition_count}"
        command = [
            "python3",
            str(SCRIPT_DIR / "throughput_capacity_runner.py"),
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
            "--port",
            str(self.args.port),
            "--instance-ports",
            self.args.instance_ports,
            "--client-instance-ports",
            self.args.client_instance_ports,
            "--single-pair-count",
            str(self.args.single_pair_count),
            "--group-member-limit",
            str(self.args.group_member_limit),
            "--single-initial-target",
            str(self.args.single_initial_target),
            "--group-initial-target",
            str(self.args.group_initial_target),
            "--single-min-duration-sec",
            str(self.args.single_min_duration_sec),
            "--group-min-duration-sec",
            str(self.args.group_min_duration_sec),
            "--single-max-messages",
            str(self.args.single_max_messages),
            "--group-max-messages",
            str(self.args.group_max_messages),
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
            "--group-coverage-threshold",
            str(self.args.group_coverage_threshold),
            "--group-full-coverage-threshold",
            str(self.args.group_full_coverage_threshold),
            "--group-p95-threshold-ms",
            str(self.args.group_p95_threshold_ms),
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
        env.setdefault("GOCACHE", "/tmp/go-build")
        env["ECHOCHAT_RECORD_ROOT_OVERRIDE"] = str(TEST_DIR / "partition_tuning_records" / "child_runs")
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
        if process.returncode != 0:
            raise RuntimeError(
                f"partition={partition_count} throughput run failed\nstdout:\n{stdout}\nstderr:\n{stderr}"
            )
        run_dir = Path(stdout.strip().splitlines()[-1].strip())
        return run_dir, resource_peak

    def load_existing_run(self, run_dir: Path) -> dict[str, Any]:
        return {
            "run_dir": run_dir.resolve(),
            "resource_peak": None,
        }

    def scenario_stage_passed(self, scenario: str, summary: dict[str, Any], error_count: int) -> bool:
        if error_count > self.args.max_error_count:
            return False
        if scenario == "single":
            success = float(summary.get("delivery_success_rate", 0.0) or 0.0)
            p95_ms = float(summary.get("latency", {}).get("p95_ms", 0.0) or 0.0)
            return success >= self.args.single_success_threshold and (
                self.args.single_p95_threshold_ms <= 0 or p95_ms <= self.args.single_p95_threshold_ms
            )
        coverage = float(summary.get("delivery_coverage_rate", 0.0) or 0.0)
        full_coverage = float(summary.get("full_coverage_message_rate", 0.0) or 0.0)
        p95_ms = float(summary.get("receipt_latency", {}).get("p95_ms", 0.0) or 0.0)
        return (
            coverage >= self.args.group_coverage_threshold
            and full_coverage >= self.args.group_full_coverage_threshold
            and (self.args.group_p95_threshold_ms <= 0 or p95_ms <= self.args.group_p95_threshold_ms)
        )

    def recover_best_stage(self, run_dir: Path, scenario: str) -> dict[str, Any] | None:
        scenario_dir = run_dir / scenario
        if not scenario_dir.exists():
            return None
        candidates: list[dict[str, Any]] = []
        for step_dir in sorted(scenario_dir.glob("step_*")):
            summary_path = step_dir / "summary.json"
            if not summary_path.exists():
                continue
            errors_path = step_dir / "errors.json"
            metrics_manifest_path = step_dir / "metrics_manifest.json"
            if not metrics_manifest_path.exists():
                metrics_dir = step_dir / "metrics"
                manifest = {"instances": []}
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
            summary = load_json(summary_path)
            errors = load_json(errors_path) if errors_path.exists() else []
            passed = self.scenario_stage_passed(scenario, summary, len(errors))
            throughput = (
                float(summary.get("observed_throughput_msg_per_sec", 0.0) or 0.0)
                if scenario == "single"
                else float(summary.get("observed_delivery_per_sec", 0.0) or 0.0)
            )
            candidates.append(
                {
                    "passed": passed,
                    "throughput": throughput,
                    "summary": summary,
                    "summary_path": str(summary_path.relative_to(run_dir)),
                    "metrics_path": str(metrics_manifest_path.relative_to(run_dir)),
                }
            )
        passed_candidates = [item for item in candidates if item["passed"]]
        if not passed_candidates:
            return None
        best = max(passed_candidates, key=lambda item: item["throughput"])
        return best

    def analyze_run(self, partition_count: int, config_path: Path, run_dir: Path, resource_peak: dict[str, Any] | None) -> tuple[ScenarioMetrics | None, dict[str, Any]]:
        summary_all_path = run_dir / "summary.json"
        if summary_all_path.exists():
            summary_all = load_json(summary_all_path)
            scenario_payload = summary_all.get(self.args.scenario)
        else:
            scenario_payload = self.recover_best_stage(run_dir, self.args.scenario)
        if not scenario_payload:
            return None, {"resource_peak": resource_peak}
        summary = scenario_payload["summary"]
        summary_path = run_dir / scenario_payload["summary_path"]
        metrics_manifest_path = run_dir / scenario_payload["metrics_path"]
        metrics, detail = analyze_stage_metrics(
            scenario=self.args.scenario,
            partition_count=partition_count,
            summary=summary,
            metrics_manifest_path=metrics_manifest_path,
            run_dir=run_dir,
            config_path=config_path,
            resource_peak=resource_peak,
            instance_count=len([item for item in self.args.instance_ports.split(",") if item.strip()]) or 1,
        )
        detail["run_dir"] = str(run_dir)
        detail["summary_path"] = str(summary_path)
        detail["metrics_manifest_path"] = str(metrics_manifest_path)
        detail["resource_peak"] = resource_peak
        previous_extra[partition_count] = detail
        return metrics, detail

    def build_report(self, results: list[ScenarioMetrics], details: dict[int, dict[str, Any]]) -> str:
        lines = [
            f"# 调整分区专用测压脚本报告（{self.args.scenario}）",
            "",
            f"- 生成时间：`{self.timestamp}`",
            f"- Git commit：`{git_commit()}`",
            f"- Git dirty：`{git_dirty()}`",
            f"- 场景：`{self.args.scenario}`",
            f"- 说明：这份报告不是只看吞吐，而是同时把 Kafka / consumer / MySQL / WS / 资源成本一起量化。",
            "",
            "## 结果总表",
            "",
            "| partitions | throughput | p95_ms | producer_used | consumer_used | lag_p95 | commit_p95_ms | mysql_in_use_ratio | mysql_wait_ms_per_1k | rss_peak_mb | fd_peak | throughput_gain_pct | recommendation |",
            "| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |",
        ]
        for item in results:
            resource_peak = details.get(item.partition_count, {}).get("resource_peak") or {}
            lines.append(
                f"| {item.partition_count} | {item.throughput} | {item.p95_ms} | {item.producer_used_partitions} | "
                f"{item.consumer_used_partitions} | {item.consumer_lag_p95} | {item.offset_commit_p95_ms} | "
                f"{item.mysql_in_use_ratio} | {item.mysql_wait_ms_per_1k_msgs} | "
                f"{resource_peak.get('rss_peak_mb_total')} | {resource_peak.get('fd_peak_total')} | "
                f"{item.throughput_gain_vs_prev_pct} | {item.recommendation} |"
            )

        lines.extend(
            [
                "",
                "## 评估口径",
                "",
                "- `throughput / p95_ms`：业务侧主指标，先看这两个有没有继续变好。",
                "- `producer_used / consumer_used`：实际被打热、被消费到的分区数，用来判断加分区之后到底有没有换来更多有效车道。",
                "- `lag_p95`：消费积压是否开始抬头，分区管理成本和排队是否在变坏。",
                "- `commit_p95_ms`：offset commit 是否开始成为新的尾部成本。",
                "- `mysql_in_use_ratio / mysql_wait_ms_per_1k`：分区增加后，后端落库出口是否被带坏。",
                "- `rss_peak_mb / fd_peak`：单机上最直接的资源副作用指标。",
                "",
                "## 建议解读",
                "",
                "- `当前分区副作用可控`：说明当前档位还能继续吃到分区红利。",
                "- `可以继续试更大分区，但要优先盯热点和 lag`：说明吞吐还有收益，但热点和消费侧成本已经值得重点关注。",
                "- `不建议继续加分区`：说明边际收益开始不划算，副作用已经比收益更值得担心。",
                "",
                "## 分档结论",
                "",
            ]
        )
        for item in results:
            lines.append(f"### {item.partition_count} partitions")
            lines.append("")
            lines.append(f"- 吞吐：`{item.throughput}`")
            lines.append(f"- p95：`{item.p95_ms} ms`")
            lines.append(f"- 推荐：`{item.recommendation}`")
            lines.append(f"- 原因：{item.recommendation_reason}")
            lines.append(f"- 有效生产分区：`{item.producer_used_partitions}` / `{item.configured_partitions}`")
            lines.append(f"- 有效消费分区：`{item.consumer_used_partitions}` / `{item.configured_partitions}`")
            lines.append(f"- 热点倾斜：producer skew=`{item.producer_skew_factor}`，consumer skew=`{item.consumer_skew_factor}`")
            lines.append(f"- lag：p95=`{item.consumer_lag_p95}`，max=`{item.consumer_lag_max}`")
            lines.append(f"- commit：p95=`{item.offset_commit_p95_ms} ms`，batch avg=`{item.offset_commit_batch_avg}`")
            lines.append(f"- MySQL：in_use_ratio=`{item.mysql_in_use_ratio}`，wait_ms_per_1k=`{item.mysql_wait_ms_per_1k_msgs}`")
            lines.append(f"- WS：queue_p95=`{item.ws_sendback_queue_p95}`，enqueue_p95=`{item.ws_enqueue_p95_ms} ms`")
            resource_peak = details.get(item.partition_count, {}).get("resource_peak") or {}
            lines.append(
                f"- 资源峰值：rss=`{resource_peak.get('rss_peak_mb_total')}` MB，threads=`{resource_peak.get('threads_peak_total')}`，fd=`{resource_peak.get('fd_peak_total')}`"
            )
            lines.append("")
        return "\n".join(lines) + "\n"

    def run(self) -> None:
        self.run_dir.mkdir(parents=True, exist_ok=True)
        write_json(
            self.run_dir / "metadata.json",
            {
                "generated_at": self.timestamp,
                "git_commit": git_commit(),
                "git_dirty": git_dirty(),
                "scenario": self.args.scenario,
                "label": self.args.label,
                "partition_configs": {str(part): str(path) for part, path in self.args.partition_configs},
                "partition_runs": {str(part): str(path) for part, path in self.args.partition_runs},
            },
        )

        combined: list[tuple[int, Path, Path, dict[str, Any] | None]] = []
        for partition_count, run_dir in self.args.partition_runs:
            matching_config = dict(self.args.partition_configs).get(partition_count)
            if matching_config is None:
                raise RuntimeError(f"missing --partition-config for existing run partition={partition_count}")
            combined.append((partition_count, matching_config, run_dir, None))
        existing_parts = {part for part, _ in self.args.partition_runs}
        for partition_count, config_path in self.args.partition_configs:
            if partition_count in existing_parts:
                continue
            run_dir, resource_peak = self.execute_throughput_run(partition_count, config_path)
            combined.append((partition_count, config_path, run_dir, resource_peak))

        combined.sort(key=lambda item: item[0])
        results: list[ScenarioMetrics] = []
        detail_by_partition: dict[int, dict[str, Any]] = {}
        previous: ScenarioMetrics | None = None
        for partition_count, config_path, run_dir, resource_peak in combined:
            metrics, detail = self.analyze_run(partition_count, config_path, run_dir, resource_peak)
            detail_by_partition[partition_count] = detail
            if metrics is None:
                continue
            recommend(metrics, previous, resource_peak)
            results.append(metrics)
            previous = metrics

        write_csv(self.run_dir / f"{self.args.scenario}_partition_scorecard.csv", [item.to_row() for item in results])
        write_json(
            self.run_dir / f"{self.args.scenario}_partition_scorecard.json",
            {
                "scenario": self.args.scenario,
                "results": [item.to_row() for item in results],
                "details": detail_by_partition,
            },
        )
        (self.run_dir / "report.md").write_text(self.build_report(results, detail_by_partition), encoding="utf-8")
        print(self.run_dir)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="专门用于分区调优的测压与副作用评估脚本。")
    parser.add_argument("--scenario", choices=["single", "group"], required=True)
    parser.add_argument("--label", default="")
    parser.add_argument("--database", default="echochat")
    parser.add_argument("--seed-prefix", default="K6")
    parser.add_argument("--port", type=int, default=18082)
    parser.add_argument("--instance-ports", default="")
    parser.add_argument("--client-instance-ports", default="")
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
    parser.add_argument("--partition-config", action="append", default=[], help="格式：240=/abs/path/config.toml")
    parser.add_argument("--partition-run", action="append", default=[], help="格式：240=/abs/path/existing_run_dir")
    args = parser.parse_args()
    args.partition_configs = [parse_partition_config(item) for item in args.partition_config]
    args.partition_runs = [parse_partition_run(item) for item in args.partition_run]
    if not args.partition_configs:
        raise SystemExit("at least one --partition-config is required")
    return args


def main() -> None:
    args = parse_args()
    runner = PartitionTuningRunner(args)
    runner.run()


if __name__ == "__main__":
    main()
