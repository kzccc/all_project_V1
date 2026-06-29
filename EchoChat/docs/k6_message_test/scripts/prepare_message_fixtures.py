#!/usr/bin/env python3

from __future__ import annotations

import argparse
import json
import os
import subprocess
from pathlib import Path


def mysql_query(database: str, query: str) -> list[list[str]]:
    command = ["mysql"]
    mysql_host = os.environ.get("ECHOCHAT_MYSQL_HOST")
    mysql_port = os.environ.get("ECHOCHAT_MYSQL_PORT")
    mysql_user = os.environ.get("ECHOCHAT_MYSQL_USER")
    mysql_password = os.environ.get("ECHOCHAT_MYSQL_PASSWORD")
    if mysql_host:
        command.extend(["-h", mysql_host])
    if mysql_port:
        command.extend(["-P", mysql_port])
    if mysql_user:
        command.extend(["-u", mysql_user])
    if mysql_password is not None:
        command.append(f"-p{mysql_password}")
    command.extend(["-NBe", query, database])
    result = subprocess.run(
        command,
        check=True,
        capture_output=True,
        text=True,
    )
    rows: list[list[str]] = []
    for raw in result.stdout.splitlines():
        if raw.strip() == "":
            continue
        rows.append(raw.split("\t"))
    return rows


def build_pairs(database: str, user_prefix: str, pair_count: int) -> list[dict]:
    user_rows = mysql_query(
        database,
        f"""
SELECT uuid, telephone, nickname, avatar
FROM user_info
WHERE uuid LIKE '{user_prefix}%'
ORDER BY telephone
LIMIT {pair_count * 6}
""",
    )
    users = [
        {
            "uuid": row[0],
            "telephone": row[1],
            "nickname": row[2],
            "avatar": row[3],
        }
        for row in user_rows
    ]
    contact_rows = mysql_query(
        database,
        f"""
SELECT user_id, contact_id
FROM user_contact
WHERE user_id LIKE '{user_prefix}%'
  AND contact_id LIKE '{user_prefix}%'
  AND status = 0
""",
    )
    contacts = {tuple(sorted((row[0], row[1]))) for row in contact_rows}

    pairs: list[dict] = []
    cursor = 0
    while cursor + 1 < len(users) and len(pairs) < pair_count:
        left = users[cursor]
        right = users[cursor + 1]
        if tuple(sorted((left["uuid"], right["uuid"]))) in contacts:
            pairs.append(
                {
                    "sender_uuid": left["uuid"],
                    "sender_telephone": left["telephone"],
                    "sender_nickname": left["nickname"],
                    "sender_avatar": left["avatar"],
                    "receiver_uuid": right["uuid"],
                    "receiver_telephone": right["telephone"],
                    "receiver_nickname": right["nickname"],
                    "receiver_avatar": right["avatar"],
                }
            )
            cursor += 2
            continue
        cursor += 1
    return pairs


def build_group(database: str, group_prefix: str, group_id: str | None, member_limit: int) -> dict:
    if group_id:
        query = f"SELECT uuid, name, members FROM group_info WHERE uuid = '{group_id}' LIMIT 1"
    else:
        query = f"SELECT uuid, name, members FROM group_info WHERE uuid LIKE '{group_prefix}%' ORDER BY uuid LIMIT 1"
    rows = mysql_query(database, query)
    if not rows:
        raise SystemExit("no group fixture found")

    group_uuid, group_name, members_raw = rows[0]
    member_uuids = json.loads(members_raw)
    member_uuids = member_uuids[:member_limit]
    uuid_list = ",".join(f"'{uuid}'" for uuid in member_uuids)
    members_query = f"""
SELECT uuid, telephone, nickname, avatar
FROM user_info
WHERE uuid IN ({uuid_list})
ORDER BY FIELD(uuid, {uuid_list})
"""
    member_rows = mysql_query(database, members_query)
    members = []
    for row in member_rows:
        members.append(
            {
                "uuid": row[0],
                "telephone": row[1],
                "nickname": row[2],
                "avatar": row[3],
            }
        )
    return {
        "group_id": group_uuid,
        "group_name": group_name,
        "sender_uuid": members[0]["uuid"],
        "sender_telephone": members[0]["telephone"],
        "members": members,
    }


def main() -> None:
    parser = argparse.ArgumentParser(description="Prepare message test fixtures from MySQL seed data.")
    parser.add_argument("--database", default="echochat")
    parser.add_argument("--user-prefix", default="UK6")
    parser.add_argument("--group-prefix", default="GK6")
    parser.add_argument("--pair-count", type=int, default=30)
    parser.add_argument("--group-id")
    parser.add_argument("--group-member-limit", type=int, default=25)
    parser.add_argument("--password", default="123456")
    parser.add_argument("--output", required=True)
    args = parser.parse_args()

    pairs = build_pairs(args.database, args.user_prefix, args.pair_count)
    if not pairs:
        raise SystemExit("no single-chat pairs found")

    group = build_group(args.database, args.group_prefix, args.group_id, args.group_member_limit)
    output = {
        "database": args.database,
        "user_prefix": args.user_prefix,
        "group_prefix": args.group_prefix,
        "default_password": args.password,
        "single_pairs": pairs,
        "group": group,
    }

    output_path = Path(args.output)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(output, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(output_path)


if __name__ == "__main__":
    main()
