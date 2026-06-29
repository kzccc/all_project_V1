#!/usr/bin/env python3

from __future__ import annotations

import argparse
import os
import tomllib
from pathlib import Path


def toml_value(value) -> str:
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, int):
        return str(value)
    if isinstance(value, float):
        return str(value)
    escaped = str(value).replace("\\", "\\\\").replace('"', '\\"')
    return f'"{escaped}"'


def write_toml(path: Path, config: dict) -> None:
    lines: list[str] = []
    for section, values in config.items():
        lines.append(f"[{section}]")
        for key, value in values.items():
            lines.append(f"{key} = {toml_value(value)}")
        lines.append("")
    path.write_text("\n".join(lines).rstrip() + "\n", encoding="utf-8")


def clone_section(base: dict, name: str) -> dict:
    return dict(base.get(name, {}))


def main() -> None:
    parser = argparse.ArgumentParser(description="Generate dedicated test configs for channel and kafka message tests.")
    parser.add_argument("--base-config", required=True)
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--channel-port", type=int, default=18081)
    parser.add_argument("--kafka-port", type=int, default=18082)
    parser.add_argument("--channel-log", default="/workspace/czk/Personal/EchoChat/docs/k6_message_test/records/logs/channel_server.log")
    parser.add_argument("--kafka-log", default="/workspace/czk/Personal/EchoChat/docs/k6_message_test/records/logs/kafka_server.log")
    args = parser.parse_args()

    base_config_path = Path(args.base_config)
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    with base_config_path.open("rb") as fp:
        base = tomllib.load(fp)

    def build(mode: str, port: int, log_path: str) -> dict:
        config = {
            "mainConfig": clone_section(base, "mainConfig"),
            "mysqlConfig": clone_section(base, "mysqlConfig"),
            "redisConfig": clone_section(base, "redisConfig"),
            "authCodeConfig": clone_section(base, "authCodeConfig"),
            "logConfig": clone_section(base, "logConfig"),
            "jwtConfig": clone_section(base, "jwtConfig"),
            "kafkaConfig": clone_section(base, "kafkaConfig"),
            "staticSrcConfig": clone_section(base, "staticSrcConfig"),
            "pressureTestConfig": clone_section(base, "pressureTestConfig"),
            "observabilityConfig": clone_section(base, "observabilityConfig"),
        }
        config["mainConfig"]["port"] = port
        config["logConfig"]["logPath"] = log_path
        config["logConfig"]["disableStdout"] = True
        config["kafkaConfig"]["messageMode"] = mode
        config["kafkaConfig"].setdefault("messageKey", "chat")
        config["kafkaConfig"].setdefault("topicPartitions", 3)
        config["kafkaConfig"].setdefault("producerRetryMax", 3)
        config["kafkaConfig"].setdefault("producerRetryBackoffMs", 200)
        config["kafkaConfig"].setdefault("minInsyncReplicas", 1)
        return config

    logs_dir = output_dir.parent / "logs"
    logs_dir.mkdir(parents=True, exist_ok=True)

    channel_path = output_dir / "channel_test.toml"
    kafka_path = output_dir / "kafka_test.toml"
    write_toml(channel_path, build("channel", args.channel_port, args.channel_log))
    write_toml(kafka_path, build("kafka", args.kafka_port, args.kafka_log))

    print(channel_path)
    print(kafka_path)


if __name__ == "__main__":
    main()
