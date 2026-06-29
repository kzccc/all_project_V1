#!/usr/bin/env python3
import json
import subprocess
import sys


TESTS = [
    (
        "download",
        "cd /userdata/vision/bin && "
        "wget -O lcd_demo http://8.129.31.210/rk/lcd_demo && "
        "wget -O touch_test http://8.129.31.210/rk/touch_test && "
        "wget -O camera_capture http://8.129.31.210/rk/camera_capture && "
        "chmod +x lcd_demo touch_test camera_capture && "
        "./lcd_demo -h && ./touch_test -h && ./camera_capture -h",
        30,
    ),
    (
        "lcd_split",
        "killall -9 rk356x-demo 2>/dev/null || true; /userdata/vision/bin/lcd_demo split",
        15,
    ),
    (
        "touch_3s",
        "/userdata/vision/bin/touch_test --seconds 3 /dev/input/event6 1024 600",
        15,
    ),
    (
        "camera_probe",
        "/userdata/vision/bin/camera_capture --probe /dev/video-camera0",
        20,
    ),
]


def run_test(name, cmd, wait):
    proc = subprocess.run(
        [sys.executable, "scripts/bridge_command.py", "--wait", str(wait), cmd],
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
    )
    print("== %s ==" % name)
    print(proc.stdout.strip())
    return proc.returncode


def main():
    results = {}
    for name, cmd, wait in TESTS:
        results[name] = run_test(name, cmd, wait)
    print(json.dumps(results, ensure_ascii=False, indent=2))
    return 0 if all(rc == 0 for rc in results.values()) else 1


if __name__ == "__main__":
    raise SystemExit(main())
