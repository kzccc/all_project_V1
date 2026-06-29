#!/usr/bin/env python3
from __future__ import annotations

import argparse
import csv
import datetime as dt
import json
import os
import re
import shutil
import statistics
import subprocess
import sys
import time
from pathlib import Path
import tomllib


ROOT = Path(__file__).resolve().parents[3]


def read_toml(path: Path) -> dict:
    with path.open("rb") as fp:
        return tomllib.load(fp)


def write_text(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")


def load_json(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def run_cmd(cmd: list[str], cwd: Path, env: dict | None = None, timeout: int | None = None) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        cmd,
        cwd=str(cwd),
        env=env,
        text=True,
        capture_output=True,
        timeout=timeout,
        check=True,
    )


def shell_join(parts: list[str]) -> str:
    return " ".join(subprocess.list2cmdline([part]) for part in parts)


def parse_producer_output(text: str) -> dict:
    lines = [line.strip() for line in text.splitlines() if line.strip()]
    summary_line = ""
    for line in reversed(lines):
        if "records sent" in line and "records/sec" in line:
            summary_line = line
            break
    if not summary_line:
        raise RuntimeError("producer perf output parse failed")

    pattern = re.compile(
        r"(?P<records>\d+) records sent, "
        r"(?P<records_per_sec>[0-9.]+) records/sec "
        r"\((?P<mb_per_sec>[0-9.]+) MB/sec\), "
        r"(?P<avg_latency_ms>[0-9.]+) ms avg latency, "
        r"(?P<max_latency_ms>[0-9.]+) ms max latency, "
        r"(?P<p50_latency_ms>[0-9.]+) ms 50th, "
        r"(?P<p95_latency_ms>[0-9.]+) ms 95th, "
        r"(?P<p99_latency_ms>[0-9.]+) ms 99th, "
        r"(?P<p999_latency_ms>[0-9.]+) ms 99.9th\."
    )
    match = pattern.search(summary_line)
    if not match:
        raise RuntimeError(f"producer perf summary line parse failed: {summary_line}")
    data = {key: float(value) if "." in value else int(value) for key, value in match.groupdict().items()}
    data["raw_summary_line"] = summary_line
    data["total_time_sec"] = round(float(data["records"]) / float(data["records_per_sec"]), 3)
    return data


def parse_consumer_output(text: str) -> dict:
    lines = [line.strip() for line in text.splitlines() if line.strip()]
    summary_line = ""
    for line in reversed(lines):
        if re.match(r"^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}:\d{3},", line):
            summary_line = line
            break
    if not summary_line:
        raise RuntimeError("consumer perf output parse failed")

    cols = [part.strip() for part in summary_line.split(",")]
    if len(cols) < 6:
        raise RuntimeError(f"consumer perf summary line parse failed: {summary_line}")

    start_ts = dt.datetime.strptime(cols[0], "%Y-%m-%d %H:%M:%S:%f")
    end_ts = dt.datetime.strptime(cols[1], "%Y-%m-%d %H:%M:%S:%f")
    data_mb = float(cols[2])
    mb_per_sec = float(cols[3])
    messages = int(cols[4])
    messages_per_sec = float(cols[5])
    return {
        "start_time": cols[0],
        "end_time": cols[1],
        "data_mb": data_mb,
        "mb_per_sec": mb_per_sec,
        "messages": messages,
        "messages_per_sec": messages_per_sec,
        "total_time_sec": round((end_ts - start_ts).total_seconds(), 3),
        "raw_summary_line": summary_line,
    }


def parse_e2e_output(text: str) -> dict:
    values: list[float] = []
    avg_latency_ms = None
    p50_latency_ms = None
    p99_latency_ms = None
    p999_latency_ms = None
    for line in text.splitlines():
        sample_match = re.match(r"^\d+\s+([0-9.]+)$", line.strip())
        if sample_match:
            values.append(float(sample_match.group(1)))
            continue
        avg_match = re.search(r"Avg latency: ([0-9.]+) ms", line)
        if avg_match:
            avg_latency_ms = float(avg_match.group(1))
            continue
        pct_match = re.search(r"Percentiles: 50th = ([0-9.]+), 99th = ([0-9.]+), 99.9th = ([0-9.]+)", line)
        if pct_match:
            p50_latency_ms = float(pct_match.group(1))
            p99_latency_ms = float(pct_match.group(2))
            p999_latency_ms = float(pct_match.group(3))
    if avg_latency_ms is None and not values:
        raise RuntimeError("e2e latency output parse failed")
    values_sorted = sorted(values) if values else []
    return {
        "sample_points": len(values),
        "avg_latency_ms": round(avg_latency_ms if avg_latency_ms is not None else (sum(values) / len(values)), 3),
        "p50_latency_ms": round(p50_latency_ms if p50_latency_ms is not None else statistics.median(values_sorted), 3),
        "p95_latency_ms": None,
        "p99_latency_ms": round(p99_latency_ms if p99_latency_ms is not None else percentile(values_sorted, 99), 3),
        "p999_latency_ms": round(p999_latency_ms, 3) if p999_latency_ms is not None else None,
        "max_latency_ms": round(max(values_sorted), 3) if values_sorted else None,
    }


def percentile(values: list[float], q: float) -> float:
    if not values:
        return float("nan")
    if len(values) == 1:
        return values[0]
    rank = (len(values) - 1) * (q / 100.0)
    lo = int(rank)
    hi = min(lo + 1, len(values) - 1)
    frac = rank - lo
    return values[lo] * (1.0 - frac) + values[hi] * frac


def wait_for_broker(kafka_home: Path, bootstrap_server: str, env: dict, timeout_sec: int = 30) -> None:
    script = kafka_home / "bin" / "kafka-broker-api-versions.sh"
    deadline = time.time() + timeout_sec
    last_error = ""
    while time.time() < deadline:
        proc = subprocess.run(
            [str(script), "--bootstrap-server", bootstrap_server],
            cwd=str(kafka_home),
            env=env,
            text=True,
            capture_output=True,
        )
        if proc.returncode == 0:
            return
        last_error = (proc.stderr or proc.stdout).strip()
        time.sleep(1)
    raise RuntimeError(f"broker not ready: {last_error}")


def ensure_clean_topic(kafka_home: Path, bootstrap_server: str, topic: str, partitions: int, replication_factor: int, env: dict) -> None:
    topics = kafka_home / "bin" / "kafka-topics.sh"
    delete_proc = subprocess.run(
        [str(topics), "--bootstrap-server", bootstrap_server, "--delete", "--if-exists", "--topic", topic],
        cwd=str(kafka_home),
        env=env,
        text=True,
        capture_output=True,
    )
    _ = delete_proc
    time.sleep(1)
    run_cmd(
        [
            str(topics),
            "--bootstrap-server",
            bootstrap_server,
            "--create",
            "--if-not-exists",
            "--topic",
            topic,
            "--partitions",
            str(partitions),
            "--replication-factor",
            str(replication_factor),
        ],
        cwd=kafka_home,
        env=env,
        timeout=30,
    )


def build_notes(cfg: dict) -> str:
    return "\n".join(
        [
            "# 测试设计",
            "",
            "## 核心问题",
            "",
            "补齐 Kafka 官方自带性能工具的三项基准，得到一个不经过 EchoChat 业务链路的 Kafka 自身上限参考值。",
            "",
            "## 三项测试",
            "",
            "1. `kafka-producer-perf-test.sh`：只测写入吞吐上限。",
            "2. `kafka-consumer-perf-test.sh`：只测读取吞吐上限。",
            "3. `kafka-e2e-latency.sh`：测单条消息从生产到消费确认的端到端往返时延。",
            "",
            "## 当前口径",
            "",
            f"- broker：`{cfg['broker']['host']}:{cfg['broker']['port']}` 单节点 KRaft",
            f"- producer：`{cfg['producer']['num_records']}` 条，`{cfg['producer']['record_size']}` bytes，`{cfg['producer']['partitions']}` partitions",
            f"- consumer：预灌入 `consumer.seed_records={cfg['consumer']['seed_records']}` 条后再消费 `messages={cfg['consumer']['messages']}` 条",
            f"- e2e：`{cfg['e2e']['num_messages']}` 条，`{cfg['e2e']['message_size']}` bytes",
            "",
            "## 说明",
            "",
            "1. 这不是业务链路压测，而是 Kafka 工具自测基线。",
            "2. 单节点结果更适合做本机可重复对比，不适合直接等价成集群业务吞吐结论。",
            "3. 报告里业务阶段字段不存在时统一标记 `本轮未采到`。",
            "",
        ]
    ) + "\n"


class KafkaSingleNodeRuntime:
    def __init__(self, cfg: dict, run_dir: Path):
        self.cfg = cfg
        self.run_dir = run_dir
        self.kafka_home = ROOT / cfg["run"]["kafka_home"]
        self.bootstrap_server = f"{cfg['broker']['host']}:{cfg['broker']['port']}"
        self.runtime_dir = run_dir / "runtime"
        self.config_path = self.runtime_dir / "server.properties"
        self.data_dir = self.runtime_dir / "data"
        self.log_path = self.runtime_dir / "broker.log"
        self.pid_path = self.runtime_dir / "broker.pid"
        self.env = os.environ.copy()
        self.env["KAFKA_HEAP_OPTS"] = cfg["broker"]["heap_opts"]
        self.process: subprocess.Popen[str] | None = None

    def setup(self) -> None:
        if not self.kafka_home.exists():
            raise RuntimeError(f"kafka home not found: {self.kafka_home}")
        if self.runtime_dir.exists():
            shutil.rmtree(self.runtime_dir)
        self.data_dir.mkdir(parents=True, exist_ok=True)
        config = "\n".join(
            [
                "process.roles=broker,controller",
                "node.id=1",
                f"controller.quorum.voters=1@{self.cfg['broker']['host']}:{self.cfg['broker']['controller_port']}",
                f"listeners=PLAINTEXT://{self.cfg['broker']['host']}:{self.cfg['broker']['port']},CONTROLLER://{self.cfg['broker']['host']}:{self.cfg['broker']['controller_port']}",
                f"advertised.listeners=PLAINTEXT://{self.cfg['broker']['host']}:{self.cfg['broker']['port']}",
                "listener.security.protocol.map=CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT",
                "inter.broker.listener.name=PLAINTEXT",
                "controller.listener.names=CONTROLLER",
                f"log.dirs={self.data_dir}",
                f"num.partitions={self.cfg['broker']['num_partitions']}",
                "default.replication.factor=1",
                "min.insync.replicas=1",
                "offsets.topic.replication.factor=1",
                "transaction.state.log.replication.factor=1",
                "transaction.state.log.min.isr=1",
                "auto.create.topics.enable=false",
                "delete.topic.enable=true",
                f"log.retention.hours={self.cfg['broker']['log_retention_hours']}",
                "group.initial.rebalance.delay.ms=0",
                f"socket.request.max.bytes={self.cfg['broker']['socket_request_max_bytes']}",
                "",
            ]
        )
        write_text(self.config_path, config)
        run_cmd(
            [
                str(self.kafka_home / "bin" / "kafka-storage.sh"),
                "format",
                "-t",
                self.cfg["broker"]["cluster_id"],
                "-c",
                str(self.config_path),
            ],
            cwd=self.kafka_home,
            env=self.env,
            timeout=30,
        )

    def start(self) -> None:
        self.setup()
        log_fp = self.log_path.open("w", encoding="utf-8")
        self.process = subprocess.Popen(
            [str(self.kafka_home / "bin" / "kafka-server-start.sh"), str(self.config_path)],
            cwd=str(self.kafka_home),
            env=self.env,
            stdout=log_fp,
            stderr=subprocess.STDOUT,
            text=True,
        )
        self.pid_path.write_text(str(self.process.pid), encoding="utf-8")
        wait_for_broker(self.kafka_home, self.bootstrap_server, self.env, timeout_sec=45)

    def stop(self) -> None:
        if self.process is None:
            return
        if self.process.poll() is None:
            self.process.terminate()
            try:
                self.process.wait(timeout=20)
            except subprocess.TimeoutExpired:
                self.process.kill()
                self.process.wait(timeout=10)


def run_producer_test(runtime: KafkaSingleNodeRuntime, cfg: dict, raw_dir: Path) -> dict:
    producer = cfg["producer"]
    ensure_clean_topic(
        runtime.kafka_home,
        runtime.bootstrap_server,
        producer["topic"],
        producer["partitions"],
        producer["replication_factor"],
        runtime.env,
    )
    cmd = [
        str(runtime.kafka_home / "bin" / "kafka-producer-perf-test.sh"),
        "--topic",
        producer["topic"],
        "--num-records",
        str(producer["num_records"]),
        "--throughput",
        str(producer["throughput"]),
        "--record-size",
        str(producer["record_size"]),
        "--producer-props",
        f"bootstrap.servers={runtime.bootstrap_server}",
        f"acks={producer['acks']}",
        f"batch.size={producer['batch_size']}",
        f"linger.ms={producer['linger_ms']}",
        f"compression.type={producer['compression_type']}",
        f"buffer.memory={producer['buffer_memory']}",
        f"client.id={producer['client_id']}",
    ]
    result = run_cmd(cmd, cwd=runtime.kafka_home, env=runtime.env, timeout=300)
    write_text(raw_dir / "producer_perf_stdout.txt", result.stdout)
    write_text(raw_dir / "producer_perf_stderr.txt", result.stderr)
    metrics = parse_producer_output(result.stdout)
    metrics["topic"] = producer["topic"]
    metrics["partitions"] = producer["partitions"]
    metrics["record_size"] = producer["record_size"]
    metrics["num_records"] = producer["num_records"]
    metrics["command"] = cmd
    return metrics


def seed_topic_for_consumer(runtime: KafkaSingleNodeRuntime, cfg: dict, raw_dir: Path) -> None:
    consumer = cfg["consumer"]
    cmd = [
        str(runtime.kafka_home / "bin" / "kafka-producer-perf-test.sh"),
        "--topic",
        consumer["topic"],
        "--num-records",
        str(consumer["seed_records"]),
        "--throughput",
        "-1",
        "--record-size",
        str(consumer["record_size"]),
        "--producer-props",
        f"bootstrap.servers={runtime.bootstrap_server}",
        "acks=1",
        "batch.size=131072",
        "linger.ms=5",
        "compression.type=lz4",
        "buffer.memory=268435456",
        "client.id=seed-producer",
    ]
    result = run_cmd(cmd, cwd=runtime.kafka_home, env=runtime.env, timeout=300)
    write_text(raw_dir / "consumer_seed_stdout.txt", result.stdout)
    write_text(raw_dir / "consumer_seed_stderr.txt", result.stderr)


def run_consumer_test(runtime: KafkaSingleNodeRuntime, cfg: dict, raw_dir: Path) -> dict:
    consumer = cfg["consumer"]
    ensure_clean_topic(
        runtime.kafka_home,
        runtime.bootstrap_server,
        consumer["topic"],
        consumer["partitions"],
        consumer["replication_factor"],
        runtime.env,
    )
    seed_topic_for_consumer(runtime, cfg, raw_dir)
    cmd = [
        str(runtime.kafka_home / "bin" / "kafka-consumer-perf-test.sh"),
        "--bootstrap-server",
        runtime.bootstrap_server,
        "--topic",
        consumer["topic"],
        "--messages",
        str(consumer["messages"]),
        "--fetch-size",
        str(consumer["fetch_size"]),
        "--timeout",
        str(consumer["timeout"]),
        "--threads",
        str(consumer["threads"]),
        "--group",
        consumer["group"],
        "--hide-header",
    ]
    result = run_cmd(cmd, cwd=runtime.kafka_home, env=runtime.env, timeout=300)
    write_text(raw_dir / "consumer_perf_stdout.txt", result.stdout)
    write_text(raw_dir / "consumer_perf_stderr.txt", result.stderr)
    metrics = parse_consumer_output(result.stdout)
    metrics["topic"] = consumer["topic"]
    metrics["partitions"] = consumer["partitions"]
    metrics["messages"] = consumer["messages"]
    metrics["record_size"] = consumer["record_size"]
    metrics["group"] = consumer["group"]
    metrics["command"] = cmd
    return metrics


def run_e2e_test(runtime: KafkaSingleNodeRuntime, cfg: dict, raw_dir: Path) -> dict:
    e2e = cfg["e2e"]
    ensure_clean_topic(
        runtime.kafka_home,
        runtime.bootstrap_server,
        e2e["topic"],
        e2e["partitions"],
        e2e["replication_factor"],
        runtime.env,
    )
    cmd = [
        str(runtime.kafka_home / "bin" / "kafka-e2e-latency.sh"),
        runtime.bootstrap_server,
        e2e["topic"],
        str(e2e["num_messages"]),
        str(e2e["acks"]),
        str(e2e["message_size"]),
    ]
    result = run_cmd(cmd, cwd=runtime.kafka_home, env=runtime.env, timeout=300)
    write_text(raw_dir / "e2e_latency_stdout.txt", result.stdout)
    write_text(raw_dir / "e2e_latency_stderr.txt", result.stderr)
    metrics = parse_e2e_output(result.stdout)
    metrics["topic"] = e2e["topic"]
    metrics["partitions"] = e2e["partitions"]
    metrics["message_size"] = e2e["message_size"]
    metrics["num_messages"] = e2e["num_messages"]
    metrics["command"] = cmd
    return metrics


def write_manifest(run_dir: Path, summary: dict) -> None:
    write_text(run_dir / "summary.json", json.dumps(summary, ensure_ascii=False, indent=2))

    rows = [
        ["test_name", "primary_metric", "secondary_metric", "unit"],
        ["producer", summary["producer"]["records_per_sec"], summary["producer"]["mb_per_sec"], "records/s,MB/s"],
        ["consumer", summary["consumer"]["messages_per_sec"], summary["consumer"]["mb_per_sec"], "records/s,MB/s"],
        ["e2e", summary["e2e"]["avg_latency_ms"], summary["e2e"]["p95_latency_ms"], "ms"],
    ]
    with (run_dir / "metrics_overview.csv").open("w", encoding="utf-8", newline="") as fp:
        writer = csv.writer(fp)
        writer.writerows(rows)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True)
    args = parser.parse_args()

    config_path = Path(args.config).resolve()
    cfg = read_toml(config_path)

    work_root = config_path.parent.parent
    report_root = ROOT / cfg["run"]["report_root"]
    record_root = ROOT / cfg["run"]["record_root"]
    report_root.mkdir(parents=True, exist_ok=True)
    record_root.mkdir(parents=True, exist_ok=True)
    write_text(report_root / "00_测试设计.md", build_notes(cfg))

    timestamp = dt.datetime.now().strftime("%Y%m%d_%H%M%S")
    day_dir = dt.datetime.now().strftime("%-m.%-d")
    run_label = f"{timestamp}_{cfg['run']['label']}"
    run_dir = record_root / day_dir / run_label
    raw_dir = run_dir / "raw"
    raw_dir.mkdir(parents=True, exist_ok=True)

    runtime = KafkaSingleNodeRuntime(cfg, run_dir)
    try:
        runtime.start()
        producer = run_producer_test(runtime, cfg, raw_dir)
        consumer = run_consumer_test(runtime, cfg, raw_dir)
        e2e = run_e2e_test(runtime, cfg, raw_dir)
        summary = {
            "label": cfg["run"]["label"],
            "generated_at": dt.datetime.now().isoformat(timespec="seconds"),
            "run_label": run_label,
            "broker": {
                "host": cfg["broker"]["host"],
                "port": cfg["broker"]["port"],
                "controller_port": cfg["broker"]["controller_port"],
                "mode": "single-node-kraft",
            },
            "producer": producer,
            "consumer": consumer,
            "e2e": e2e,
        }
        write_manifest(run_dir, summary)
        summary_json = report_root / "kafka_tools_throughput_summary.json"
        write_text(summary_json, json.dumps(summary, ensure_ascii=False, indent=2))
        report_script = work_root / "scripts" / "build_kafka_throughput_report.py"
        final_report = report_root / "01_Kafka官方工具吞吐极限报告.md"
        run_cmd(
            [sys.executable, str(report_script), "--input", str(summary_json), "--output", str(final_report)],
            cwd=ROOT,
            timeout=30,
        )
        print(final_report)
    finally:
        runtime.stop()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
