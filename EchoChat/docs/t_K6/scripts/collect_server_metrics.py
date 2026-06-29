#!/usr/bin/env python3

from __future__ import annotations

import argparse
import csv
import json
import os
import re
import signal
import subprocess
import time
from datetime import datetime
from pathlib import Path
from typing import Dict, List, Optional, Tuple
from urllib.error import URLError
from urllib.request import urlopen


PROM_LINE_RE = re.compile(r"^([a-zA-Z_:][a-zA-Z0-9_:]*)(\{([^}]*)\})?\s+([-+0-9.eE]+)$")
RUNNING = True


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Collect EchoChat server metrics during a benchmark run.")
    parser.add_argument("--output-dir", required=True, help="directory to write collector artifacts into")
    parser.add_argument("--service-name", default="echochat", help="systemd service name")
    parser.add_argument("--service-port", type=int, default=8081, help="service port for TCP established checks")
    parser.add_argument("--metrics-url", default="http://127.0.0.1:8081/metrics", help="prometheus metrics endpoint")
    parser.add_argument(
        "--pprof-url",
        default="http://127.0.0.1:8081/debug/pprof/goroutine?debug=1",
        help="goroutine pprof endpoint",
    )
    parser.add_argument("--interval", type=float, default=2.0, help="sampling interval in seconds")
    parser.add_argument("--label", default="", help="optional label written into metadata")
    return parser.parse_args()


def handle_stop(_signum, _frame) -> None:
    global RUNNING
    RUNNING = False


def run_command(args: List[str]) -> subprocess.CompletedProcess:
    return subprocess.run(args, check=False, text=True, capture_output=True)


def command_output(args: List[str]) -> str:
    result = run_command(args)
    if result.returncode != 0:
        return ""
    return (result.stdout or "").strip()


def service_pid(service_name: str) -> int:
    output = command_output(["systemctl", "show", "-p", "MainPID", "--value", service_name])
    if output.isdigit():
        return int(output)
    return 0


def service_active(service_name: str) -> bool:
    result = run_command(["systemctl", "is-active", service_name])
    return result.returncode == 0 and (result.stdout or "").strip() == "active"


def read_proc_status(pid: int) -> Dict[str, int]:
    result = {"rss_kb": 0, "vsz_kb": 0, "threads": 0}
    if pid <= 0:
        return result
    status_path = Path(f"/proc/{pid}/status")
    if not status_path.exists():
        return result
    for line in status_path.read_text(encoding="utf-8", errors="replace").splitlines():
        if line.startswith("VmRSS:"):
            result["rss_kb"] = int(line.split(":", 1)[1].strip().split()[0])
        elif line.startswith("VmSize:"):
            result["vsz_kb"] = int(line.split(":", 1)[1].strip().split()[0])
        elif line.startswith("Threads:"):
            result["threads"] = int(line.split(":", 1)[1].strip())
    return result


def read_proc_ticks(pid: int) -> int:
    if pid <= 0:
        return 0
    stat_path = Path(f"/proc/{pid}/stat")
    if not stat_path.exists():
        return 0
    raw = stat_path.read_text(encoding="utf-8", errors="replace").strip()
    parts = raw.split()
    if len(parts) < 17:
        return 0
    return int(parts[13]) + int(parts[14])


def read_total_cpu_ticks() -> int:
    stat_path = Path("/proc/stat")
    for line in stat_path.read_text(encoding="utf-8", errors="replace").splitlines():
        if line.startswith("cpu "):
            return sum(int(item) for item in line.split()[1:])
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
        ["bash", "-lc", f"ss -tan state established '( sport = :{port} )' | tail -n +2 | wc -l"]
    )
    if result.returncode != 0:
        return 0
    output = (result.stdout or "0").strip()
    return int(output) if output.isdigit() else 0


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
    result: Dict[str, List[Tuple[Dict[str, str], float]]] = {}
    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        match = PROM_LINE_RE.match(line)
        if not match:
            continue
        name = match.group(1)
        labels = parse_labels(match.group(3) or "")
        value = float(match.group(4))
        result.setdefault(name, []).append((labels, value))
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


def cpu_percent(proc_delta: int, total_delta: int, cpu_count: int) -> float:
    if proc_delta <= 0 or total_delta <= 0 or cpu_count <= 0:
        return 0.0
    return proc_delta / total_delta * 100.0 * cpu_count


def main() -> int:
    args = parse_args()
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    signal.signal(signal.SIGTERM, handle_stop)
    signal.signal(signal.SIGINT, handle_stop)

    metadata = {
        "label": args.label,
        "service_name": args.service_name,
        "service_port": args.service_port,
        "metrics_url": args.metrics_url,
        "pprof_url": args.pprof_url,
        "interval": args.interval,
        "started_at": datetime.now().isoformat(),
    }
    (output_dir / "metadata.json").write_text(json.dumps(metadata, ensure_ascii=False, indent=2), encoding="utf-8")

    csv_path = output_dir / "samples.csv"
    fieldnames = [
        "timestamp",
        "relative_seconds",
        "service_active",
        "pid",
        "cpu_percent_inst",
        "rss_kb",
        "vsz_kb",
        "threads",
        "fd_count",
        "established_conn",
        "online_connections",
        "process_open_fds",
        "process_max_fds",
        "go_goroutines",
        "go_threads",
        "process_resident_memory_bytes",
        "process_virtual_memory_bytes",
        "heap_alloc_bytes",
        "heap_inuse_bytes",
        "heap_objects",
        "net_rx_bytes_total",
        "net_tx_bytes_total",
        "net_rx_rate_bps",
        "net_tx_rate_bps",
        "bench_handshake_success_total",
        "bench_handshake_failure_total",
        "bench_auth_reject_total",
    ]

    start_monotonic = time.monotonic()
    prev_proc_ticks = 0
    prev_total_ticks = 0
    prev_rx_total = 0.0
    prev_tx_total = 0.0
    cpu_count = os.cpu_count() or 1

    with csv_path.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.DictWriter(handle, fieldnames=fieldnames)
        writer.writeheader()

        while RUNNING:
            now = datetime.now()
            active = service_active(args.service_name)
            pid = service_pid(args.service_name) if active else 0

            proc_ticks = read_proc_ticks(pid)
            total_ticks = read_total_cpu_ticks()
            cpu_inst = cpu_percent(proc_ticks - prev_proc_ticks, total_ticks - prev_total_ticks, cpu_count)
            prev_proc_ticks = proc_ticks
            prev_total_ticks = total_ticks

            status = read_proc_status(pid)
            fd_count = proc_fd_count(pid)
            established = established_count(args.service_port)

            metrics_text = fetch_url_text(args.metrics_url)
            metrics = parse_prometheus(metrics_text)

            rx_total = prom_value(metrics, "process_network_receive_bytes_total")
            tx_total = prom_value(metrics, "process_network_transmit_bytes_total")
            rx_rate = 0.0
            tx_rate = 0.0
            if prev_rx_total > 0:
                rx_rate = max(0.0, rx_total - prev_rx_total) / max(args.interval, 1e-6)
            if prev_tx_total > 0:
                tx_rate = max(0.0, tx_total - prev_tx_total) / max(args.interval, 1e-6)
            prev_rx_total = rx_total
            prev_tx_total = tx_total

            row = {
                "timestamp": now.strftime("%Y-%m-%d %H:%M:%S"),
                "relative_seconds": round(time.monotonic() - start_monotonic, 3),
                "service_active": 1 if active else 0,
                "pid": pid,
                "cpu_percent_inst": round(cpu_inst, 3),
                "rss_kb": status["rss_kb"],
                "vsz_kb": status["vsz_kb"],
                "threads": status["threads"],
                "fd_count": fd_count,
                "established_conn": established,
                "online_connections": prom_value(metrics, "echochat_ws_online_connections", {"route": "bench"}),
                "process_open_fds": prom_value(metrics, "process_open_fds"),
                "process_max_fds": prom_value(metrics, "process_max_fds"),
                "go_goroutines": prom_value(metrics, "go_goroutines"),
                "go_threads": prom_value(metrics, "go_threads"),
                "process_resident_memory_bytes": prom_value(metrics, "process_resident_memory_bytes"),
                "process_virtual_memory_bytes": prom_value(metrics, "process_virtual_memory_bytes"),
                "heap_alloc_bytes": prom_value(metrics, "go_memstats_heap_alloc_bytes"),
                "heap_inuse_bytes": prom_value(metrics, "go_memstats_heap_inuse_bytes"),
                "heap_objects": prom_value(metrics, "go_memstats_heap_objects"),
                "net_rx_bytes_total": rx_total,
                "net_tx_bytes_total": tx_total,
                "net_rx_rate_bps": round(rx_rate, 3),
                "net_tx_rate_bps": round(tx_rate, 3),
                "bench_handshake_success_total": prom_value(
                    metrics, "echochat_ws_handshake_result_total", {"route": "bench", "result": "success"}
                ),
                "bench_handshake_failure_total": prom_value(
                    metrics, "echochat_ws_handshake_result_total", {"route": "bench", "result": "failure"}
                ),
                "bench_auth_reject_total": sum(
                    value
                    for labels, value in metrics.get("echochat_auth_reject_total", [])
                    if labels.get("route") == "bench"
                ),
            }
            writer.writerow(row)
            handle.flush()
            time.sleep(args.interval)

    (output_dir / "metrics.prom").write_text(fetch_url_text(args.metrics_url, timeout=4.0), encoding="utf-8")
    (output_dir / "goroutine.txt").write_text(fetch_url_text(args.pprof_url, timeout=4.0), encoding="utf-8")
    status = {
        "finished_at": datetime.now().isoformat(),
        "service_active": service_active(args.service_name),
    }
    (output_dir / "collector_status.json").write_text(json.dumps(status, ensure_ascii=False, indent=2), encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
