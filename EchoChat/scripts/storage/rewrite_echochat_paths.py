#!/usr/bin/env python3

from __future__ import annotations

import argparse
from pathlib import Path


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Rewrite EchoChat absolute paths for a relocated repo/runtime root.")
    parser.add_argument("--repo-root", required=True, help="Relocated EchoChat repo root.")
    parser.add_argument("--runtime-root", required=True, help="Runtime root on the big disk.")
    parser.add_argument(
        "--old-root",
        default="/workspace/czk/Personal/EchoChat",
        help="Legacy repo root to replace.",
    )
    return parser.parse_args()


def rewrite_file(path: Path, replacements: list[tuple[str, str]]) -> bool:
    original = path.read_text(encoding="utf-8")
    updated = original
    for old, new in replacements:
        updated = updated.replace(old, new)
    if updated == original:
        return False
    path.write_text(updated, encoding="utf-8")
    return True


def main() -> None:
    args = parse_args()
    repo_root = Path(args.repo_root).expanduser().resolve()
    runtime_root = Path(args.runtime_root).expanduser().resolve()
    old_root = Path(args.old_root).expanduser().resolve()

    replacements = [
        (str(old_root / "logs"), str(runtime_root / "logs")),
        (str(old_root / "bin"), str(runtime_root / "bin")),
        (str(old_root / "tmp"), str(runtime_root / "tmp")),
        (str(old_root / "docs" / "k6_message_test" / "records"), str(runtime_root / "records" / "k6_message_test")),
        (
            str(old_root / "docs" / "k6_message_test" / "partition_tuning_records"),
            str(runtime_root / "records" / "partition_tuning"),
        ),
        (
            str(old_root / "docs" / "k6_message_test" / "mysql_persist_tuning_records"),
            str(runtime_root / "records" / "mysql_persist_tuning"),
        ),
        (str(old_root / "docs" / "t_K6" / "records"), str(runtime_root / "records" / "t_K6")),
        (str(old_root), str(repo_root)),
    ]

    touched = []
    candidate_dirs = [
        repo_root / "configs",
        repo_root / "docs" / "k6_message_test" / "generated" / "configs",
    ]
    for base_dir in candidate_dirs:
        if not base_dir.exists():
            continue
        for path in sorted(base_dir.rglob("*.toml")):
            if rewrite_file(path, replacements):
                touched.append(path)

    print(f"rewritten_files={len(touched)}")
    for path in touched:
        print(path)


if __name__ == "__main__":
    main()
