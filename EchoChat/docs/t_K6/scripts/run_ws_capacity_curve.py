#!/usr/bin/env python3

from __future__ import annotations

import argparse
import csv
import json
import math
import os
import re
import subprocess
import sys
import time
from collections import defaultdict
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from typing import Dict, Iterable, List, Optional, Tuple
from urllib.error import URLError
from urllib.request import urlopen

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import pandas as pd


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
SCRIPT_DIR = ROOT_DIR / "docs" / "t_K6" / "scripts"
RECORD_ROOT = ROOT_DIR / "docs" / "t_K6" / "records"
WS_SCRIPT = SCRIPT_DIR / "ws_online_tokens.js"

PROM_LINE_RE = re.compile(r'^([a-zA-Z_:][a-zA-Z0-9_:]*)(\{([^}]*)\})?\s+([-+0-9.eE]+)$')


@dataclass
class StepDecision:
    zone: str
    reason: str


def run_command(
    args: List[str],
    cwd: Path,
    stdout_path: Optional[Path] = None,
    env: Optional[Dict[str, str]] = None,
) -> subprocess.CompletedProcess:
    if stdout_path is None:
        return subprocess.run(
            args,
            cwd=str(cwd),
            env=env,
            check=False,
            text=True,
            capture_output=True,
        )

    with stdout_path.open("w", encoding="utf-8") as handle:
        return subprocess.run(
            args,
            cwd=str(cwd),
            env=env,
            check=False,
            text=True,
            stdout=handle,
            stderr=subprocess.STDOUT,
        )


def command_output(args: List[str], cwd: Path) -> str:
    result = run_command(args, cwd=cwd)
    if result.returncode != 0:
        raise RuntimeError(
            f"command failed: {' '.join(args)}\nstdout:\n{result.stdout}\nstderr:\n{result.stderr}"
        )
    return (result.stdout or "").strip()


def mysql_scalar(sql: str) -> int:
    output = command_output(["mysql", "-N", "-uroot", "echochat", "-e", sql], cwd=ROOT_DIR)
    return int(output.strip())


def systemd_active(service: str) -> bool:
    result = run_command(["systemctl", "is-active", service], cwd=ROOT_DIR)
    return result.returncode == 0 and (result.stdout or "").strip() == "active"


def systemd_pid(service: str) -> int:
    return int(command_output(["systemctl", "show", "-p", "MainPID", "--value", service], cwd=ROOT_DIR))


def ps_value(pid: int, fmt: str) -> float:
    if pid <= 0:
        return 0.0
    result = run_command(["ps", "-p", str(pid), "-o", fmt], cwd=ROOT_DIR)
    if result.returncode != 0:
        return 0.0
    output = (result.stdout or "").strip()
    try:
        return float(output)
    except ValueError:
        return 0.0


def proc_threads(pid: int) -> int:
    if pid <= 0:
        return 0
    status_path = Path(f"/proc/{pid}/status")
    if not status_path.exists():
        return 0
    for line in status_path.read_text(encoding="utf-8").splitlines():
        if line.startswith("Threads:"):
            return int(line.split(":", 1)[1].strip())
    return 0


def proc_fd_count(pid: int) -> int:
    if pid <= 0:
        return 0
    fd_dir = Path(f"/proc/{pid}/fd")
    if not fd_dir.exists():
        return 0
    try:
        return len(list(fd_dir.iterdir()))
    except OSError:
        return 0


def established_count(port: int) -> int:
    result = run_command(
        ["bash", "-lc", f"ss -tan state established '( sport = :{port} )' | tail -n +2 | wc -l"],
        cwd=ROOT_DIR,
    )
    if result.returncode != 0:
        return 0
    return int((result.stdout or "0").strip())


def fetch_url_text(url: str, timeout: float = 2.0) -> str:
    try:
        with urlopen(url, timeout=timeout) as response:
            return response.read().decode("utf-8", errors="replace")
    except URLError:
        return ""


def parse_labels(raw: str) -> Dict[str, str]:
    if not raw:
        return {}
    labels: Dict[str, str] = {}
    for item in raw.split(","):
        if "=" not in item:
            continue
        key, value = item.split("=", 1)
        labels[key.strip()] = value.strip().strip('"')
    return labels


def parse_prometheus(text: str) -> Dict[str, List[Tuple[Dict[str, str], float]]]:
    result: Dict[str, List[Tuple[Dict[str, str], float]]] = defaultdict(list)
    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        match = PROM_LINE_RE.match(line)
        if not match:
            continue
        name = match.group(1)
        labels = parse_labels(match.group(3) or "")
        try:
            value = float(match.group(4))
        except ValueError:
            continue
        result[name].append((labels, value))
    return result


def prom_value(
    metrics: Dict[str, List[Tuple[Dict[str, str], float]]],
    name: str,
    labels: Optional[Dict[str, str]] = None,
) -> float:
    labels = labels or {}
    for item_labels, value in metrics.get(name, []):
        if all(item_labels.get(key) == expected for key, expected in labels.items()):
            return value
    return 0.0


def prom_labeled_counter(
    metrics: Dict[str, List[Tuple[Dict[str, str], float]]], name: str
) -> Dict[Tuple[Tuple[str, str], ...], float]:
    result: Dict[Tuple[Tuple[str, str], ...], float] = {}
    for labels, value in metrics.get(name, []):
        result[tuple(sorted(labels.items()))] = value
    return result


def diff_counter_map(
    after: Dict[Tuple[Tuple[str, str], ...], float], before: Dict[Tuple[Tuple[str, str], ...], float]
) -> Dict[str, float]:
    diff: Dict[str, float] = {}
    for label_items, after_value in after.items():
        delta = after_value - before.get(label_items, 0.0)
        if delta <= 0:
            continue
        label_map = dict(label_items)
        reason = label_map.get("reason", "unknown")
        source = label_map.get("source", "unknown")
        diff[f"{source}:{reason}"] = delta
    return diff


def metric_field(summary: Dict[str, object], metric_name: str, field_name: str) -> float:
    metrics = summary.get("metrics", {})
    if not isinstance(metrics, dict):
        return 0.0
    metric = metrics.get(metric_name, {})
    if not isinstance(metric, dict):
        return 0.0
    value = metric.get(field_name)
    if value is not None:
        return float(value)
    values = metric.get("values")
    if isinstance(values, dict) and field_name in values:
        return float(values[field_name])
    if field_name == "value" and "value" in metric:
        return float(metric["value"])
    return 0.0


def ensure_ws_users(prefix: str, user_count: int, password: str, telephone_start: int, summary_path: Path) -> None:
    current = mysql_scalar(f"SELECT COUNT(*) FROM user_info WHERE uuid LIKE 'U{prefix}%';")
    if current >= user_count:
        return

    args = [
        "go",
        "run",
        "./cmd/echo_chat_seed",
        "-prefix",
        prefix,
        "-reset-prefix",
        "-user-count",
        str(user_count),
        "-admin-count",
        "1",
        "-group-count",
        "0",
        "-group-size",
        "1",
        "-friend-span",
        "1",
        "-pair-messages",
        "0",
        "-group-messages",
        "0",
        "-apply-count",
        "0",
        "-password",
        password,
        "-telephone-start",
        str(telephone_start),
    ]
    result = run_command(args, cwd=ROOT_DIR, stdout_path=summary_path)
    if result.returncode != 0:
        raise RuntimeError(f"seed users failed, see {summary_path}")


def generate_tokens(prefix: str, count: int, output_path: Path) -> None:
    args = [
        "go",
        "run",
        "./cmd/echo_chat_ws_tokens",
        "-prefix",
        prefix,
        "-count",
        str(count),
        "-output",
        str(output_path),
    ]
    result = run_command(args, cwd=ROOT_DIR)
    if result.returncode != 0:
        raise RuntimeError(
            f"generate tokens failed\nstdout:\n{result.stdout}\nstderr:\n{result.stderr}"
        )


def write_json(path: Path, content: object) -> None:
    path.write_text(json.dumps(content, ensure_ascii=False, indent=2), encoding="utf-8")


def detect_drop_time(samples: pd.DataFrame, target: int) -> str:
    if samples.empty:
        return ""
    seen_target = False
    for _, row in samples.iterrows():
        online = float(row.get("online_connections", 0))
        if online >= target:
            seen_target = True
        if seen_target and online < target:
            return str(row.get("timestamp", ""))
    return ""


def classify_step(row: Dict[str, object], args: argparse.Namespace) -> StepDecision:
    target = int(row["target_vus"])
    handshake_rate = float(row.get("handshake_success_rate", 0.0))
    early_disconnect_rate = float(row.get("early_disconnect_rate", 0.0))
    actual_peak = float(row.get("online_peak", 0.0))
    last_positive = float(row.get("online_last_positive", 0.0))
    connect_p95 = float(row.get("connect_p95_ms", 0.0))
    k6_status = int(row.get("k6_exit_code", 1))

    if k6_status != 0:
        return StepDecision("failure", f"k6_exit_code={k6_status}")
    if handshake_rate < args.fail_handshake_rate:
        return StepDecision("failure", f"handshake_rate={handshake_rate:.4f}")
    if actual_peak < target * (1.0 - args.fail_online_gap):
        return StepDecision("failure", f"online_peak={actual_peak:.0f}")
    if last_positive > 0 and last_positive < target * (1.0 - args.fail_online_gap):
        return StepDecision("failure", f"online_last_positive={last_positive:.0f}")
    if early_disconnect_rate >= args.fail_early_disconnect_rate:
        return StepDecision("failure", f"early_disconnect_rate={early_disconnect_rate:.4f}")

    degradation_reasons: List[str] = []
    if early_disconnect_rate >= args.degrade_early_disconnect_rate:
        degradation_reasons.append(f"early_disconnect_rate={early_disconnect_rate:.4f}")
    if actual_peak < target * (1.0 - args.degrade_online_gap):
        degradation_reasons.append(f"online_peak={actual_peak:.0f}")
    if last_positive > 0 and last_positive < target * (1.0 - args.degrade_online_gap):
        degradation_reasons.append(f"online_last_positive={last_positive:.0f}")
    if connect_p95 >= args.degrade_connect_p95_ms:
        degradation_reasons.append(f"connect_p95_ms={connect_p95:.1f}")

    if degradation_reasons:
        return StepDecision("degradation", "; ".join(degradation_reasons))
    return StepDecision("healthy", "within_threshold")


def sample_metrics(
    metrics_url: str,
    pid: int,
    port: int,
    previous_sample: Optional[Dict[str, float]],
    sample_interval_seconds: float,
) -> Dict[str, float]:
    metrics_text = fetch_url_text(metrics_url)
    metrics = parse_prometheus(metrics_text)

    process_rx_total = prom_value(metrics, "process_network_receive_bytes_total")
    process_tx_total = prom_value(metrics, "process_network_transmit_bytes_total")
    previous_rx_total = (previous_sample or {}).get("process_rx_total", process_rx_total)
    previous_tx_total = (previous_sample or {}).get("process_tx_total", process_tx_total)
    rx_rate = max(0.0, process_rx_total - previous_rx_total) / max(sample_interval_seconds, 1e-6)
    tx_rate = max(0.0, process_tx_total - previous_tx_total) / max(sample_interval_seconds, 1e-6)

    return {
        "process_cpu_percent": ps_value(pid, "%cpu="),
        "process_rss_kb": ps_value(pid, "rss="),
        "process_vsz_kb": ps_value(pid, "vsz="),
        "threads": float(proc_threads(pid)),
        "fd_count_proc": float(proc_fd_count(pid)),
        "established_8081": float(established_count(port)),
        "online_connections": prom_value(
            metrics, "echochat_ws_online_connections", {"route": "bench"}
        ),
        "process_open_fds": prom_value(metrics, "process_open_fds"),
        "process_max_fds": prom_value(metrics, "process_max_fds"),
        "go_goroutines": prom_value(metrics, "go_goroutines"),
        "process_resident_memory_bytes": prom_value(metrics, "process_resident_memory_bytes"),
        "process_virtual_memory_bytes": prom_value(metrics, "process_virtual_memory_bytes"),
        "process_rx_total": process_rx_total,
        "process_tx_total": process_tx_total,
        "process_rx_rate_bps": rx_rate,
        "process_tx_rate_bps": tx_rate,
        "heap_alloc_bytes": prom_value(metrics, "go_memstats_heap_alloc_bytes"),
        "heap_inuse_bytes": prom_value(metrics, "go_memstats_heap_inuse_bytes"),
        "heap_objects": prom_value(metrics, "go_memstats_heap_objects"),
        "go_threads": prom_value(metrics, "go_threads"),
    }


def plot_timeseries(samples_path: Path, steps_path: Path, plots_dir: Path) -> None:
    samples = pd.read_csv(samples_path)
    steps = pd.read_csv(steps_path)
    if samples.empty:
        return

    samples["time_s"] = samples["relative_seconds"]
    samples["resident_mb"] = samples["process_resident_memory_bytes"] / 1024 / 1024
    samples["heap_alloc_mb"] = samples["heap_alloc_bytes"] / 1024 / 1024
    samples["heap_inuse_mb"] = samples["heap_inuse_bytes"] / 1024 / 1024
    samples["rss_mb"] = samples["process_rss_kb"] / 1024
    samples["rx_mbps"] = samples["process_rx_rate_bps"] * 8 / 1024 / 1024
    samples["tx_mbps"] = samples["process_tx_rate_bps"] * 8 / 1024 / 1024

    plt.style.use("seaborn-v0_8-whitegrid")

    # 1. target vs actual
    fig, ax = plt.subplots(figsize=(14, 6))
    ax.plot(samples["time_s"], samples["target_vus"], label="target_connections", linewidth=2)
    ax.plot(samples["time_s"], samples["online_connections"], label="actual_online_connections", linewidth=2)
    ax.set_title("Target vs Actual Online Connections")
    ax.set_xlabel("Elapsed Seconds")
    ax.set_ylabel("Connections")
    ax.legend()
    fig.tight_layout()
    fig.savefig(plots_dir / "01_target_vs_actual.png", dpi=180)
    plt.close(fig)

    # 2. memory
    fig, ax = plt.subplots(figsize=(14, 6))
    ax.plot(samples["time_s"], samples["resident_mb"], label="process_resident_memory_mb", linewidth=2)
    ax.plot(samples["time_s"], samples["heap_alloc_mb"], label="heap_alloc_mb", linewidth=2)
    ax.plot(samples["time_s"], samples["heap_inuse_mb"], label="heap_inuse_mb", linewidth=2)
    ax.set_title("Memory Curves")
    ax.set_xlabel("Elapsed Seconds")
    ax.set_ylabel("MB")
    ax.legend()
    fig.tight_layout()
    fig.savefig(plots_dir / "02_memory.png", dpi=180)
    plt.close(fig)

    # 3. goroutines and fd
    fig, ax1 = plt.subplots(figsize=(14, 6))
    ax1.plot(samples["time_s"], samples["go_goroutines"], color="#2c7fb8", label="go_goroutines", linewidth=2)
    ax1.set_xlabel("Elapsed Seconds")
    ax1.set_ylabel("Goroutines", color="#2c7fb8")
    ax2 = ax1.twinx()
    ax2.plot(samples["time_s"], samples["process_open_fds"], color="#f03b20", label="process_open_fds", linewidth=2)
    ax2.plot(samples["time_s"], samples["fd_count_proc"], color="#fd8d3c", label="proc_fd_count", linewidth=1.5)
    ax2.set_ylabel("FD Count", color="#f03b20")
    ax1.set_title("Goroutines and FD")
    lines = ax1.get_lines() + ax2.get_lines()
    ax1.legend(lines, [line.get_label() for line in lines], loc="upper left")
    fig.tight_layout()
    fig.savefig(plots_dir / "03_goroutines_fds.png", dpi=180)
    plt.close(fig)

    # 4. error / degradation
    fig, ax = plt.subplots(figsize=(14, 6))
    ax.plot(steps["target_vus"], steps["handshake_success_rate"] * 100, marker="o", label="handshake_success_rate")
    ax.plot(steps["target_vus"], steps["early_disconnect_rate"] * 100, marker="o", label="early_disconnect_rate")
    ax.plot(steps["target_vus"], steps["ws_error_rate"] * 100, marker="o", label="ws_error_rate")
    ax.set_title("Handshake / Error / Early Disconnect")
    ax.set_xlabel("Target Connections")
    ax.set_ylabel("Rate (%)")
    ax.legend()
    fig.tight_layout()
    fig.savefig(plots_dir / "04_error_rates.png", dpi=180)
    plt.close(fig)

    # 5. connect latency
    fig, ax = plt.subplots(figsize=(14, 6))
    ax.plot(steps["target_vus"], steps["connect_p95_ms"], marker="o", label="ws_connecting_p95_ms")
    ax.plot(steps["target_vus"], steps["connect_p99_ms"], marker="o", label="ws_connecting_p99_ms")
    ax.set_title("Handshake Latency by Step")
    ax.set_xlabel("Target Connections")
    ax.set_ylabel("Milliseconds")
    ax.legend()
    fig.tight_layout()
    fig.savefig(plots_dir / "05_connect_latency.png", dpi=180)
    plt.close(fig)

    # 6. capacity curves
    fig, ax1 = plt.subplots(figsize=(14, 6))
    ax1.plot(steps["online_peak"], steps["resident_peak_mb"], marker="o", color="#2ca25f", label="resident_peak_mb")
    ax1.plot(steps["online_peak"], steps["heap_alloc_peak_mb"], marker="o", color="#99d8c9", label="heap_alloc_peak_mb")
    ax1.set_xlabel("Peak Online Connections")
    ax1.set_ylabel("Memory (MB)")
    ax2 = ax1.twinx()
    ax2.plot(steps["online_peak"], steps["goroutines_peak"], marker="o", color="#756bb1", label="goroutines_peak")
    ax2.plot(steps["online_peak"], steps["open_fds_peak"], marker="o", color="#e6550d", label="open_fds_peak")
    ax2.set_ylabel("Goroutines / Open FDs")
    ax1.set_title("Capacity Curves")
    lines = ax1.get_lines() + ax2.get_lines()
    ax1.legend(lines, [line.get_label() for line in lines], loc="upper left")
    fig.tight_layout()
    fig.savefig(plots_dir / "06_capacity_curves.png", dpi=180)
    plt.close(fig)

    # 7. close reasons
    close_reason_rows: List[Dict[str, object]] = []
    for _, row in steps.iterrows():
        raw = row.get("close_reasons", "")
        if not isinstance(raw, str) or not raw.strip():
            continue
        try:
            parsed = json.loads(raw)
        except json.JSONDecodeError:
            continue
        if not isinstance(parsed, dict):
            continue
        for key, value in parsed.items():
            close_reason_rows.append(
                {
                    "target_vus": row["target_vus"],
                    "reason": key,
                    "count": float(value),
                }
            )

    if close_reason_rows:
        close_df = pd.DataFrame(close_reason_rows)
        pivot = close_df.pivot_table(
            index="target_vus",
            columns="reason",
            values="count",
            aggfunc="sum",
            fill_value=0.0,
        )
        fig, ax = plt.subplots(figsize=(14, 6))
        pivot.plot(kind="bar", stacked=True, ax=ax, colormap="tab20")
        ax.set_title("WebSocket Close Reasons by Step")
        ax.set_xlabel("Target Connections")
        ax.set_ylabel("Close Count")
        ax.legend(loc="upper left", bbox_to_anchor=(1.01, 1.0))
        fig.tight_layout()
        fig.savefig(plots_dir / "07_close_reasons.png", dpi=180)
        plt.close(fig)

    # 7. dashboard
    fig, axes = plt.subplots(3, 2, figsize=(18, 16))
    axs = axes.flatten()
    axs[0].plot(samples["time_s"], samples["target_vus"], label="target")
    axs[0].plot(samples["time_s"], samples["online_connections"], label="actual")
    axs[0].set_title("Target vs Actual")
    axs[0].legend()

    axs[1].plot(samples["time_s"], samples["resident_mb"], label="resident_mb")
    axs[1].plot(samples["time_s"], samples["heap_alloc_mb"], label="heap_alloc_mb")
    axs[1].set_title("Memory")
    axs[1].legend()

    axs[2].plot(samples["time_s"], samples["go_goroutines"], label="goroutines")
    axs[2].plot(samples["time_s"], samples["process_open_fds"], label="open_fds")
    axs[2].set_title("Goroutines / FDs")
    axs[2].legend()

    axs[3].plot(steps["target_vus"], steps["early_disconnect_rate"] * 100, marker="o", label="early_disconnect_rate")
    axs[3].plot(steps["target_vus"], steps["ws_error_rate"] * 100, marker="o", label="ws_error_rate")
    axs[3].set_title("Error Rates")
    axs[3].legend()

    axs[4].plot(steps["target_vus"], steps["connect_p95_ms"], marker="o", label="p95")
    axs[4].plot(steps["target_vus"], steps["connect_p99_ms"], marker="o", label="p99")
    axs[4].set_title("Connect Latency")
    axs[4].legend()

    axs[5].plot(steps["online_peak"], steps["resident_peak_mb"], marker="o", label="resident_peak_mb")
    axs[5].plot(steps["online_peak"], steps["goroutines_peak"], marker="o", label="goroutines_peak")
    axs[5].plot(steps["online_peak"], steps["open_fds_peak"], marker="o", label="open_fds_peak")
    axs[5].set_title("Capacity Curves")
    axs[5].legend()

    for ax in axs:
        ax.set_xlabel("Time / Connections")
    fig.tight_layout()
    fig.savefig(plots_dir / "dashboard.png", dpi=180)
    plt.close(fig)


def markdown_table(df: pd.DataFrame, columns: Iterable[str]) -> List[str]:
    headers = list(columns)
    lines = [
        "| " + " | ".join(headers) + " |",
        "| " + " | ".join(["---"] * len(headers)) + " |",
    ]
    for _, row in df.iterrows():
        values: List[str] = []
        for column in headers:
            value = row[column]
            if isinstance(value, float):
                if math.isnan(value):
                    values.append("")
                elif column.endswith("_rate"):
                    values.append(f"{value * 100:.2f}%")
                else:
                    values.append(f"{value:.2f}")
            else:
                values.append(str(value))
        lines.append("| " + " | ".join(values) + " |")
    return lines


def write_report(
    report_path: Path,
    args: argparse.Namespace,
    run_dir: Path,
    stage_df: pd.DataFrame,
    first_degradation: Optional[pd.Series],
    failure_step: Optional[pd.Series],
) -> None:
    plots_dir = run_dir / "plots"
    healthy_count = int((stage_df["zone"] == "healthy").sum())
    degradation_count = int((stage_df["zone"] == "degradation").sum())
    failure_count = int((stage_df["zone"] == "failure").sum())
    highest_healthy = stage_df.loc[stage_df["zone"] == "healthy", "target_vus"].max()
    highest_peak_online = stage_df["online_peak"].max()

    lines: List[str] = [
        "# EchoChat WebSocket Capacity Curve Report",
        "",
        "## Summary",
        "",
        f"- Run directory: `{run_dir}`",
        f"- Step mode: `{args.start_vus}` -> `{args.max_vus}` by `{args.step_vus}`",
        f"- Hold seconds per step: `{args.hold_seconds}`",
        f"- Sample interval: `{args.sample_interval}` seconds",
        f"- Healthy steps: `{healthy_count}`",
        f"- Degradation steps: `{degradation_count}`",
        f"- Failure steps: `{failure_count}`",
        f"- Highest healthy target: `{int(highest_healthy) if not math.isnan(highest_healthy) else 0}`",
        f"- Highest observed online peak: `{int(highest_peak_online)}`",
        "",
    ]

    if first_degradation is not None:
        lines.extend(
            [
                "## First Degradation",
                "",
                f"- Step: `{first_degradation['step_name']}`",
                f"- Target connections: `{int(first_degradation['target_vus'])}`",
                f"- Zone reason: `{first_degradation['zone_reason']}`",
                f"- Early disconnect rate: `{first_degradation['early_disconnect_rate'] * 100:.2f}%`",
                f"- Last positive online connections: `{int(first_degradation['online_last_positive'])}`",
                "",
            ]
        )

    if failure_step is not None:
        lines.extend(
            [
                "## Failure Point",
                "",
                f"- Step: `{failure_step['step_name']}`",
                f"- Target connections: `{int(failure_step['target_vus'])}`",
                f"- Zone reason: `{failure_step['zone_reason']}`",
                f"- Handshake success rate: `{failure_step['handshake_success_rate'] * 100:.2f}%`",
                f"- Online peak: `{int(failure_step['online_peak'])}`",
                "",
            ]
        )

    lines.extend(
        [
            "## Stage Summary",
            "",
        ]
    )
    lines.extend(
        markdown_table(
            stage_df[
                [
                    "step_name",
                    "target_vus",
                    "zone",
                    "zone_reason",
                    "handshake_success_rate",
                    "early_disconnect_rate",
                    "connect_p95_ms",
                    "connect_p99_ms",
                    "online_peak",
                    "online_last_positive",
                    "resident_peak_mb",
                    "goroutines_peak",
                    "open_fds_peak",
                ]
            ],
            [
                "step_name",
                "target_vus",
                "zone",
                "zone_reason",
                "handshake_success_rate",
                "early_disconnect_rate",
                "connect_p95_ms",
                "connect_p99_ms",
                "online_peak",
                "online_last_positive",
                "resident_peak_mb",
                "goroutines_peak",
                "open_fds_peak",
            ],
        )
    )
    lines.extend(
        [
            "",
            "## Charts",
            "",
            f"![dashboard]({plots_dir / 'dashboard.png'})",
            "",
            f"![target_vs_actual]({plots_dir / '01_target_vs_actual.png'})",
            "",
            f"![memory]({plots_dir / '02_memory.png'})",
            "",
            f"![goroutines_fds]({plots_dir / '03_goroutines_fds.png'})",
            "",
            f"![error_rates]({plots_dir / '04_error_rates.png'})",
            "",
            f"![connect_latency]({plots_dir / '05_connect_latency.png'})",
            "",
            f"![capacity_curves]({plots_dir / '06_capacity_curves.png'})",
            "",
            f"![close_reasons]({plots_dir / '07_close_reasons.png'})",
            "",
            "## Interpretation",
            "",
            "- `Healthy Zone`: handshake success rate remains high, online peak tracks target, and early disconnect rate stays below the degradation threshold.",
            "- `Degradation Zone`: connections can still build, but retention starts dropping, handshake latency stretches, or early disconnects become visible.",
            "- `Failure Zone`: target online cannot be reached or retained, handshake success rate drops materially, or k6 exits non-zero.",
            "",
            "## Artifacts",
            "",
            f"- Stage CSV: `{run_dir / 'stage_summary.csv'}`",
            f"- Time-series CSV: `{run_dir / 'all_samples.csv'}`",
            f"- Raw step directories: `{run_dir}`",
        ]
    )

    report_path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def build_arg_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Run WebSocket capacity curve benchmark for EchoChat.")
    parser.add_argument("--ws-url", default="ws://127.0.0.1:8081", help="base websocket url without path")
    parser.add_argument("--metrics-url", default="http://127.0.0.1:8081/metrics", help="prometheus metrics url")
    parser.add_argument("--pprof-url", default="http://127.0.0.1:8081/debug/pprof/goroutine?debug=1")
    parser.add_argument("--prefix", default="WS", help="seed/token business prefix without leading U")
    parser.add_argument("--password", default="123456")
    parser.add_argument("--telephone-start", type=int, default=17620000000)
    parser.add_argument("--start-vus", type=int, default=2000)
    parser.add_argument("--step-vus", type=int, default=2000)
    parser.add_argument("--max-vus", type=int, default=16000)
    parser.add_argument("--hold-seconds", type=int, default=120)
    parser.add_argument("--sample-interval", type=float, default=2.0)
    parser.add_argument("--service-name", default="echochat")
    parser.add_argument("--service-port", type=int, default=8081)
    parser.add_argument("--degrade-early-disconnect-rate", dest="degrade_early_disconnect_rate", type=float, default=0.01)
    parser.add_argument("--degrade-online-gap", type=float, default=0.01)
    parser.add_argument("--degrade-connect-p95-ms", type=float, default=250.0)
    parser.add_argument("--fail-handshake-rate", type=float, default=0.95)
    parser.add_argument("--fail-early-disconnect-rate", dest="fail_early_disconnect_rate", type=float, default=0.05)
    parser.add_argument("--fail-online-gap", type=float, default=0.10)
    parser.add_argument("--user-padding", type=int, default=2000)
    parser.add_argument("--summary-trend-stats", default="avg,min,med,max,p(90),p(95),p(99)")
    return parser


def main() -> int:
    parser = build_arg_parser()
    args = parser.parse_args()

    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    run_dir = RECORD_ROOT / f"ws_capacity_curve_{timestamp}"
    plots_dir = run_dir / "plots"
    run_dir.mkdir(parents=True, exist_ok=True)
    plots_dir.mkdir(parents=True, exist_ok=True)

    seed_summary_path = run_dir / "seed_ws_users.json"
    token_file = run_dir / "ws_tokens.json"
    config_snapshot_path = run_dir / "config_snapshot.json"

    config_snapshot = {
        "ws_url": args.ws_url,
        "metrics_url": args.metrics_url,
        "prefix": args.prefix,
        "start_vus": args.start_vus,
        "step_vus": args.step_vus,
        "max_vus": args.max_vus,
        "hold_seconds": args.hold_seconds,
        "sample_interval": args.sample_interval,
        "degrade_early_disconnect_rate": args.degrade_early_disconnect_rate,
        "degrade_online_gap": args.degrade_online_gap,
        "degrade_connect_p95_ms": args.degrade_connect_p95_ms,
        "fail_handshake_rate": args.fail_handshake_rate,
        "fail_early_disconnect_rate": args.fail_early_disconnect_rate,
        "fail_online_gap": args.fail_online_gap,
    }
    write_json(config_snapshot_path, config_snapshot)

    if not systemd_active(args.service_name):
        raise RuntimeError(f"service {args.service_name} is not active")

    ensure_ws_users(
        prefix=args.prefix,
        user_count=args.max_vus + args.user_padding,
        password=args.password,
        telephone_start=args.telephone_start,
        summary_path=seed_summary_path,
    )
    generate_tokens(args.prefix, args.max_vus, token_file)

    service_pid = systemd_pid(args.service_name)
    all_samples_path = run_dir / "all_samples.csv"
    stage_rows: List[Dict[str, object]] = []
    all_samples_file = all_samples_path.open("w", newline="", encoding="utf-8")
    sample_writer = csv.DictWriter(
        all_samples_file,
        fieldnames=[
            "timestamp",
            "relative_seconds",
            "step_name",
            "target_vus",
            "step_elapsed_seconds",
            "process_cpu_percent",
            "process_rss_kb",
            "process_vsz_kb",
            "threads",
            "fd_count_proc",
            "established_8081",
            "online_connections",
            "process_open_fds",
            "process_max_fds",
            "go_goroutines",
            "process_resident_memory_bytes",
            "process_virtual_memory_bytes",
            "process_rx_total",
            "process_tx_total",
            "process_rx_rate_bps",
            "process_tx_rate_bps",
            "heap_alloc_bytes",
            "heap_inuse_bytes",
            "heap_objects",
            "go_threads",
        ],
    )
    sample_writer.writeheader()

    run_start_monotonic = time.monotonic()

    try:
        for target_vus in range(args.start_vus, args.max_vus + 1, args.step_vus):
            if not systemd_active(args.service_name):
                break

            step_name = f"step_{target_vus:05d}"
            step_dir = run_dir / step_name
            step_dir.mkdir(parents=True, exist_ok=True)
            summary_path = step_dir / "summary.json"
            stdout_path = step_dir / "stdout.txt"
            metrics_before = parse_prometheus(fetch_url_text(args.metrics_url))
            close_before = prom_labeled_counter(metrics_before, "echochat_ws_close_total")

            env = os.environ.copy()
            env["WS_URL"] = args.ws_url
            env["TOKEN_FILE"] = str(token_file)
            env["HOLD_SECONDS"] = str(args.hold_seconds)
            k6_args = [
                "k6",
                "run",
                str(WS_SCRIPT),
                "--address",
                "127.0.0.1:0",
                "-u",
                str(target_vus),
                "-i",
                str(target_vus),
                "--summary-export",
                str(summary_path),
                "--summary-trend-stats",
                args.summary_trend_stats,
            ]

            with stdout_path.open("w", encoding="utf-8") as handle:
                process = subprocess.Popen(
                    k6_args,
                    cwd=str(ROOT_DIR),
                    env=env,
                    stdout=handle,
                    stderr=subprocess.STDOUT,
                    text=True,
                )

                previous_sample: Optional[Dict[str, float]] = None
                step_start_monotonic = time.monotonic()
                while True:
                    now_monotonic = time.monotonic()
                    if systemd_active(args.service_name):
                        service_pid = systemd_pid(args.service_name)
                    else:
                        service_pid = 0
                    row = sample_metrics(
                        metrics_url=args.metrics_url,
                        pid=service_pid,
                        port=args.service_port,
                        previous_sample=previous_sample,
                        sample_interval_seconds=args.sample_interval,
                    )
                    row["timestamp"] = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
                    row["relative_seconds"] = round(now_monotonic - run_start_monotonic, 3)
                    row["step_name"] = step_name
                    row["target_vus"] = target_vus
                    row["step_elapsed_seconds"] = round(now_monotonic - step_start_monotonic, 3)
                    sample_writer.writerow(row)
                    all_samples_file.flush()
                    previous_sample = row

                    if process.poll() is not None:
                        break
                    time.sleep(args.sample_interval)

                exit_code = process.wait()

            (step_dir / "exit_code.txt").write_text(f"{exit_code}\n", encoding="utf-8")
            metrics_text = fetch_url_text(args.metrics_url)
            (step_dir / "metrics.prom").write_text(metrics_text, encoding="utf-8")
            (step_dir / "goroutine.txt").write_text(fetch_url_text(args.pprof_url, timeout=4.0), encoding="utf-8")

            if summary_path.exists():
                summary = json.loads(summary_path.read_text(encoding="utf-8"))
            else:
                summary = {"metrics": {}}
            step_samples = pd.read_csv(all_samples_path)
            step_samples = step_samples[step_samples["step_name"] == step_name].copy()
            step_samples.to_csv(step_dir / "samples.csv", index=False)

            metrics_after = parse_prometheus(metrics_text)
            close_after = prom_labeled_counter(metrics_after, "echochat_ws_close_total")
            close_diff = diff_counter_map(close_after, close_before)
            write_json(step_dir / "close_reasons.json", close_diff)

            online_peak = float(step_samples["online_connections"].max()) if not step_samples.empty else 0.0
            positive_online = step_samples.loc[step_samples["online_connections"] > 0, "online_connections"]
            online_last_positive = float(positive_online.iloc[-1]) if not positive_online.empty else 0.0

            stage_row: Dict[str, object] = {
                "step_name": step_name,
                "target_vus": target_vus,
                "k6_exit_code": exit_code,
                "handshake_success_rate": metric_field(summary, "ws_upgrade_success_rate", "value"),
                "handshake_success_passes": metric_field(summary, "ws_upgrade_success_rate", "passes"),
                "early_disconnect_rate": metric_field(summary, "ws_early_disconnect_rate", "value"),
                "early_disconnect_count": metric_field(summary, "ws_early_disconnect_count", "count"),
                "ws_error_rate": metric_field(summary, "ws_error_rate", "value"),
                "connect_avg_ms": metric_field(summary, "ws_connecting", "avg"),
                "connect_p95_ms": metric_field(summary, "ws_connecting", "p(95)"),
                "connect_p99_ms": metric_field(summary, "ws_connecting", "p(99)"),
                "session_p95_ms": metric_field(summary, "ws_session_duration_ms", "p(95)"),
                "online_peak": online_peak,
                "online_last_positive": online_last_positive,
                "online_drop_started_at": detect_drop_time(step_samples, target_vus),
                "resident_peak_mb": float(step_samples["process_resident_memory_bytes"].max() / 1024 / 1024)
                if not step_samples.empty
                else 0.0,
                "heap_alloc_peak_mb": float(step_samples["heap_alloc_bytes"].max() / 1024 / 1024)
                if not step_samples.empty
                else 0.0,
                "rss_peak_mb": float(step_samples["process_rss_kb"].max() / 1024) if not step_samples.empty else 0.0,
                "goroutines_peak": float(step_samples["go_goroutines"].max()) if not step_samples.empty else 0.0,
                "open_fds_peak": float(step_samples["process_open_fds"].max()) if not step_samples.empty else 0.0,
                "cpu_peak_percent": float(step_samples["process_cpu_percent"].max()) if not step_samples.empty else 0.0,
                "rx_peak_mbps": float(step_samples["process_rx_rate_bps"].max() * 8 / 1024 / 1024)
                if not step_samples.empty
                else 0.0,
                "tx_peak_mbps": float(step_samples["process_tx_rate_bps"].max() * 8 / 1024 / 1024)
                if not step_samples.empty
                else 0.0,
                "close_reasons": json.dumps(close_diff, ensure_ascii=False, sort_keys=True),
            }
            decision = classify_step(stage_row, args)
            stage_row["zone"] = decision.zone
            stage_row["zone_reason"] = decision.reason
            stage_rows.append(stage_row)

            if decision.zone == "failure":
                break
            if not systemd_active(args.service_name):
                break

        stage_df = pd.DataFrame(stage_rows)
        stage_summary_path = run_dir / "stage_summary.csv"
        stage_df.to_csv(stage_summary_path, index=False)

        plot_timeseries(all_samples_path, stage_summary_path, plots_dir)

        first_degradation = None
        if not stage_df.empty and (stage_df["zone"] == "degradation").any():
            first_degradation = stage_df.loc[stage_df["zone"] == "degradation"].iloc[0]
        failure_step = None
        if not stage_df.empty and (stage_df["zone"] == "failure").any():
            failure_step = stage_df.loc[stage_df["zone"] == "failure"].iloc[0]

        write_report(
            report_path=run_dir / "report.md",
            args=args,
            run_dir=run_dir,
            stage_df=stage_df,
            first_degradation=first_degradation,
            failure_step=failure_step,
        )

        print(f"ws capacity curve records written to {run_dir}")
        return 0
    finally:
        all_samples_file.close()


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:  # pragma: no cover
        print(f"capacity curve runner failed: {exc}", file=sys.stderr)
        raise
