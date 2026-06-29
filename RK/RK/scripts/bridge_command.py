#!/usr/bin/env python3
import json
import os
from pathlib import Path
import sys
import time
import urllib.request

BRIDGE = "http://127.0.0.1:19090"
LOG_PATH = Path("/root/board_bridge_events.log")


def post(path, payload):
    data = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        BRIDGE + path,
        data=data,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=10) as resp:
        return json.loads(resp.read().decode("utf-8"))


def find_report(task_id):
    if not LOG_PATH.exists():
        return None
    with LOG_PATH.open("r", encoding="utf-8") as fp:
        for line in reversed(fp.readlines()[-500:]):
            try:
                event = json.loads(line)
            except json.JSONDecodeError:
                continue
            if event.get("kind") != "report":
                continue
            payload = event.get("payload", {})
            if payload.get("task_id") == task_id:
                return payload
    return None


def main():
    if len(sys.argv) < 2:
        print("usage: bridge_command.py [--wait seconds] 'shell command' [board_id]", file=sys.stderr)
        return 2
    wait_sec = 0
    args = sys.argv[1:]
    if args and args[0] == "--wait":
        if len(args) < 3:
            print("missing wait seconds or shell command", file=sys.stderr)
            return 2
        wait_sec = int(args[1])
        args = args[2:]
    cmd = args[0]
    board_id = args[1] if len(args) > 1 else "RK356X"
    task_id = "rk-%d-%d" % (int(time.time() * 1000), os.getpid())
    result = post("/command", {"board_id": board_id, "task_id": task_id, "cmd": cmd})
    print(json.dumps(result, ensure_ascii=False))
    print(task_id)
    if wait_sec <= 0:
        return 0
    deadline = time.time() + wait_sec
    while time.time() < deadline:
        report = find_report(task_id)
        if report:
            print(json.dumps(report, ensure_ascii=False, indent=2))
            return int(report.get("rc", 0))
        time.sleep(1)
    print("timeout waiting for report", file=sys.stderr)
    return 124


if __name__ == "__main__":
    raise SystemExit(main())
