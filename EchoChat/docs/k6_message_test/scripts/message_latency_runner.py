#!/usr/bin/env python3

from __future__ import annotations

import argparse
import asyncio
import csv
import hashlib
import json
import math
import statistics
import threading
import time
from pathlib import Path
from typing import Any
from urllib.parse import quote

import requests
import websockets
from tqdm import tqdm


BENCH_PREFIX = "BENCH:"


def parse_url_list(raw: str | None) -> list[str]:
    if not raw:
        return []
    return [item.strip().rstrip("/") for item in raw.split(",") if item.strip()]


def endpoint_for_key(urls: list[str], key: str, fallback: str) -> str:
    if not urls:
        return fallback.rstrip("/")
    if len(urls) == 1:
        return urls[0]
    digest = hashlib.sha1(key.encode("utf-8")).hexdigest()
    index = int(digest[:8], 16) % len(urls)
    return urls[index]


def now_ms() -> int:
    return time.time_ns() // 1_000_000


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


def summarize_latencies(values: list[float]) -> dict[str, float | None]:
    return {
        "count": len(values),
        "avg_ms": round(statistics.fmean(values), 3) if values else None,
        "p50_ms": round(percentile(values, 0.50), 3) if values else None,
        "p95_ms": round(percentile(values, 0.95), 3) if values else None,
        "p99_ms": round(percentile(values, 0.99), 3) if values else None,
        "max_ms": round(max(values), 3) if values else None,
    }


def resolve_setup_workers(requested: int, item_count: int) -> int:
    if requested > 0:
        return requested
    return max(1, min(128, max(16, item_count // 4 if item_count > 0 else 16)))


def resolve_setup_http_timeout_sec(requested_ms: int, item_count: int) -> float:
    if requested_ms > 0:
        return max(float(requested_ms) / 1000.0, 1.0)
    derived_ms = max(90_000, min(300_000, max(1, item_count) * 250))
    return float(derived_ms) / 1000.0


class TqdmGroup:
    def __init__(self, enabled: bool = True) -> None:
        self.enabled = enabled
        self._bars: list[tqdm] = []
        self._next_position = 0

    def add(self, total: int, desc: str, unit: str) -> tqdm | None:
        if not self.enabled:
            return None
        bar = tqdm(
            total=total,
            desc=desc,
            unit=unit,
            dynamic_ncols=True,
            position=self._next_position,
            mininterval=0.2,
        )
        self._next_position += 1
        self._bars.append(bar)
        return bar

    def write(self, message: str) -> None:
        if self.enabled:
            tqdm.write(message)
        else:
            print(message)

    def close(self) -> None:
        for bar in reversed(self._bars):
            bar.close()
        self._bars.clear()


class PlainProgress:
    def __init__(self, enabled: bool = True) -> None:
        self.enabled = enabled
        self._state: dict[str, dict[str, int | str]] = {}

    def add(self, total: int, desc: str, unit: str) -> str | None:
        if not self.enabled:
            return None
        key = desc
        self._state[key] = {"total": max(0, total), "current": 0, "unit": unit}
        return key

    def update(self, key: str | None, step: int = 1) -> None:
        if not self.enabled or key is None or key not in self._state:
            return
        state = self._state[key]
        current = int(state["current"]) + step
        total = int(state["total"])
        state["current"] = min(current, total) if total > 0 else current
        unit = str(state["unit"])
        print(f"[progress] {key}: {state['current']}/{total} {unit}", flush=True)

    def write(self, message: str) -> None:
        if self.enabled:
            print(message, flush=True)

    def close(self) -> None:
        return


class SinglePhaseSummaryProgress:
    def __init__(
        self,
        *,
        total_accounts: int,
        total_pairs: int,
        total_messages: int,
        min_interval_sec: float = 2.0,
        min_message_step: int = 2000,
        min_setup_step: int = 25,
    ) -> None:
        self.total_accounts = max(0, total_accounts)
        self.total_pairs = max(0, total_pairs)
        self.total_messages = max(0, total_messages)
        self.accounts = 0
        self.pairs = 0
        self.sent = 0
        self.received = 0
        self.min_interval_sec = max(0.05, float(min_interval_sec))
        self.min_message_step = max(1, int(min_message_step))
        self.min_setup_step = max(1, int(min_setup_step))
        self._last_emit_time = 0.0
        self._last_accounts = 0
        self._last_pairs = 0
        self._last_sent = 0
        self._last_received = 0

    def _should_emit_setup(self, *, current: int, last: int) -> bool:
        return (current - last) >= self.min_setup_step

    def _should_emit_message(self, *, current: int, last: int) -> bool:
        return (current - last) >= self.min_message_step

    def _emit(self, message: str, *, force: bool = False) -> None:
        now = time.monotonic()
        if not force and (now - self._last_emit_time) < self.min_interval_sec:
            return
        print(message, flush=True)
        self._last_emit_time = now

    def update_accounts(self, step: int = 1) -> None:
        self.accounts = min(self.total_accounts, self.accounts + step)
        if self._should_emit_setup(current=self.accounts, last=self._last_accounts) or self.accounts == self.total_accounts:
            self._last_accounts = self.accounts
            self._emit(
                f"[progress] 登录 {self.accounts}/{self.total_accounts}，建连 {self.pairs}/{self.total_pairs}，发送 {self.sent}/{self.total_messages}，回执 {self.received}/{self.total_messages}"
            )

    def update_pairs(self, step: int = 1) -> None:
        self.pairs = min(self.total_pairs, self.pairs + step)
        if self._should_emit_setup(current=self.pairs, last=self._last_pairs) or self.pairs == self.total_pairs:
            self._last_pairs = self.pairs
            self._emit(
                f"[progress] 登录 {self.accounts}/{self.total_accounts}，建连 {self.pairs}/{self.total_pairs}，发送 {self.sent}/{self.total_messages}，回执 {self.received}/{self.total_messages}"
            )

    def update_sent(self, step: int = 1) -> None:
        self.sent = min(self.total_messages, self.sent + step)
        if self._should_emit_message(current=self.sent, last=self._last_sent) or self.sent == self.total_messages:
            self._last_sent = self.sent
            self._emit(f"[progress] 发送 {self.sent}/{self.total_messages}，回执 {self.received}/{self.total_messages}")

    def update_received(self, step: int = 1) -> None:
        self.received = min(self.total_messages, self.received + step)
        if self._should_emit_message(current=self.received, last=self._last_received) or self.received == self.total_messages:
            self._last_received = self.received
            self._emit(f"[progress] 发送 {self.sent}/{self.total_messages}，回执 {self.received}/{self.total_messages}")

    def write(self, message: str) -> None:
        self._emit(message, force=True)

    def close(self) -> None:
        self._emit(
            f"[progress] 登录 {self.accounts}/{self.total_accounts}，建连 {self.pairs}/{self.total_pairs}，发送 {self.sent}/{self.total_messages}，回执 {self.received}/{self.total_messages}",
            force=True,
        )


async def drain_until_stable(
    *,
    is_complete,
    progress_value,
    max_wait_ms: int,
    idle_wait_ms: int,
) -> dict[str, Any]:
    progress_before = progress_value()
    if max_wait_ms <= 0 or is_complete():
        return {
            "applied": False,
            "waited_ms": 0,
            "reason": "not_needed",
            "progress_before": progress_before,
            "progress_after": progress_before,
        }

    started = time.monotonic()
    last_progress_value = progress_before
    stable_since = started
    reason = "complete"

    while True:
        current_progress = progress_value()
        if current_progress != last_progress_value:
            last_progress_value = current_progress
            stable_since = time.monotonic()
        if is_complete():
            reason = "complete"
            break

        now = time.monotonic()
        waited_ms = int((now - started) * 1000)
        if waited_ms >= max_wait_ms:
            reason = "max_wait"
            break
        if int((now - stable_since) * 1000) >= idle_wait_ms:
            reason = "idle"
            break
        await asyncio.sleep(min(0.1, max(0.01, (max_wait_ms - waited_ms) / 1000.0)))

    return {
        "applied": True,
        "waited_ms": int((time.monotonic() - started) * 1000),
        "reason": reason,
        "progress_before": progress_before,
        "progress_after": progress_value(),
    }


class ResourceSampler:
    def __init__(self, pid: int, interval_sec: float = 0.2) -> None:
        self.pid = pid
        self.interval_sec = interval_sec
        self.stop_event = threading.Event()
        self.thread: threading.Thread | None = None
        self.samples = 0
        self.rss_peak_bytes = 0
        self.thread_peak = 0
        self.fd_peak = 0

    def _sample_once(self) -> None:
        status_path = Path(f"/proc/{self.pid}/status")
        fd_path = Path(f"/proc/{self.pid}/fd")
        if not status_path.exists():
            return
        rss_bytes = 0
        thread_count = 0
        for line in status_path.read_text(encoding="utf-8", errors="ignore").splitlines():
            if line.startswith("VmRSS:"):
                rss_kb = int(line.split()[1])
                rss_bytes = rss_kb * 1024
            elif line.startswith("Threads:"):
                thread_count = int(line.split()[1])
        try:
            fd_count = len(list(fd_path.iterdir()))
        except FileNotFoundError:
            fd_count = 0

        self.samples += 1
        self.rss_peak_bytes = max(self.rss_peak_bytes, rss_bytes)
        self.thread_peak = max(self.thread_peak, thread_count)
        self.fd_peak = max(self.fd_peak, fd_count)

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
            "rss_peak_mb": round(self.rss_peak_bytes / 1024 / 1024, 3),
            "threads_peak": self.thread_peak,
            "fd_peak": self.fd_peak,
        }


def http_json(method: str, url: str, *, timeout_sec: float = 10.0, **kwargs) -> dict:
    response = requests.request(method, url, timeout=timeout_sec, **kwargs)
    response.raise_for_status()
    data = response.json()
    if data.get("code") != 200:
        raise RuntimeError(f"http api failed: {url} -> {data}")
    return data


def login_user(base_url: str, telephone: str, password: str, *, timeout_sec: float = 10.0) -> dict:
    data = http_json(
        "POST",
        f"{base_url}/login",
        timeout_sec=timeout_sec,
        headers={"Content-Type": "application/json"},
        data=json.dumps({"telephone": telephone, "password": password}),
    )
    return data["data"]


def open_session(base_url: str, access_token: str, receive_id: str, *, timeout_sec: float = 10.0) -> str:
    data = http_json(
        "POST",
        f"{base_url}/session/openSession",
        timeout_sec=timeout_sec,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {access_token}",
        },
        data=json.dumps({"receive_id": receive_id}),
    )
    return data["data"]


def bench_payload(run_id: str, scenario: str, bench_id: str, send_ts_ms: int) -> str:
    return BENCH_PREFIX + json.dumps(
        {
            "run_id": run_id,
            "scenario": scenario,
            "bench_id": bench_id,
            "send_ts_ms": send_ts_ms,
        },
        separators=(",", ":"),
        ensure_ascii=False,
    )


def parse_bench_content(content: str) -> dict | None:
    if not isinstance(content, str) or not content.startswith(BENCH_PREFIX):
        return None
    try:
        return json.loads(content[len(BENCH_PREFIX) :])
    except json.JSONDecodeError:
        return None


def parse_ws_message(raw: Any) -> dict | None:
    if not isinstance(raw, str):
        return None
    try:
        data = json.loads(raw)
    except json.JSONDecodeError:
        return None
    if not isinstance(data, dict):
        return None
    return data


async def ws_connect(ws_base_url: str, ws_path: str, access_token: str, *, open_timeout_sec: float = 10.0):
    url = f"{ws_base_url}{ws_path}?token={quote(access_token)}"
    return await websockets.connect(
        url,
        open_timeout=open_timeout_sec,
        close_timeout=3,
        ping_interval=20,
        ping_timeout=20,
    )


def build_text_message(session_id: str, sender: dict, receive_id: str, content: str) -> str:
    payload = {
        "session_id": session_id,
        "type": 0,
        "content": content,
        "url": "",
        "send_id": sender["uuid"],
        "send_name": sender["nickname"],
        "send_avatar": sender["avatar"],
        "receive_id": receive_id,
        "file_size": "",
        "file_type": "",
        "file_name": "",
        "av_data": "",
    }
    return json.dumps(payload, ensure_ascii=False, separators=(",", ":"))


async def login_fixture_accounts(
    *,
    needed: dict[str, str],
    password: str,
    base_urls: list[str],
    ws_base_urls: list[str],
    default_base_url: str,
    default_ws_base_url: str,
    timeout_sec: float,
    concurrency: int,
) -> dict[str, dict]:
    semaphore = asyncio.Semaphore(max(1, concurrency))

    async def login_one(uuid: str, telephone: str) -> tuple[str, dict]:
        base_url = endpoint_for_key(base_urls, uuid, default_base_url)
        ws_base_url = endpoint_for_key(ws_base_urls, uuid, default_ws_base_url)
        async with semaphore:
            login_data = await asyncio.to_thread(login_user, base_url, telephone, password, timeout_sec=timeout_sec)
        return uuid, {
            "uuid": login_data["uuid"],
            "telephone": telephone,
            "nickname": login_data["nickname"],
            "avatar": login_data["avatar"],
            "access_token": login_data["access_token"],
            "base_url": base_url,
            "ws_base_url": ws_base_url,
        }

    return dict(await asyncio.gather(*(login_one(uuid, telephone) for uuid, telephone in needed.items())))


async def run_single_scenario(args, fixture: dict, run_id: str) -> tuple[dict, list[dict], list[dict]]:
    pairs = fixture["single_pairs"][: args.pair_count]
    password = fixture["default_password"]

    needed = {}
    for pair in pairs:
        needed[pair["sender_uuid"]] = pair["sender_telephone"]
        needed[pair["receiver_uuid"]] = pair["receiver_telephone"]
    setup_workers = resolve_setup_workers(args.setup_workers, max(len(needed), len(pairs)))
    http_timeout_sec = resolve_setup_http_timeout_sec(args.setup_http_timeout_ms, max(len(needed), len(pairs)))
    ws_open_timeout_sec = max(float(args.ws_open_timeout_ms) / 1000.0, 1.0)
    setup_semaphore = asyncio.Semaphore(setup_workers)
    use_tqdm_progress = not bool(getattr(args, "plain_progress", False))
    total_messages = len(pairs) * args.messages_per_sender
    if use_tqdm_progress:
        progress = TqdmGroup()
        login_bar = progress.add(len(needed), "[single] 账号登录", "acct")
        prepare_bar = progress.add(len(pairs), "[single] openSession+建连", "pair")
        send_bar = progress.add(total_messages, "[single] 发送消息", "msg")
        recv_bar = progress.add(total_messages, "[single] 接收回执", "msg")
    else:
        progress = SinglePhaseSummaryProgress(
            total_accounts=len(needed),
            total_pairs=len(pairs),
            total_messages=total_messages,
        )
        login_bar = prepare_bar = send_bar = recv_bar = None
    progress.write(
        f"[single] 启动: pair_count={len(pairs)} unique_accounts={len(needed)} "
        f"setup_workers={setup_workers} http_timeout={http_timeout_sec:.1f}s "
        f"ws_open_timeout={ws_open_timeout_sec:.1f}s messages_per_sender={args.messages_per_sender} "
        f"send_interval={args.send_interval_ms}ms"
    )

    async def login_accounts_with_progress() -> dict[str, dict]:
        semaphore = asyncio.Semaphore(max(1, setup_workers))

        async def login_one(uuid: str, telephone: str) -> tuple[str, dict]:
            base_url = endpoint_for_key(args.base_urls, uuid, args.base_url)
            ws_base_url = endpoint_for_key(args.ws_base_urls, uuid, args.ws_base_url)
            async with semaphore:
                login_data = await asyncio.to_thread(login_user, base_url, telephone, password, timeout_sec=http_timeout_sec)
            if use_tqdm_progress:
                if login_bar is not None:
                    login_bar.update(1)
            else:
                progress.update_accounts(1)
            return uuid, {
                "uuid": login_data["uuid"],
                "telephone": telephone,
                "nickname": login_data["nickname"],
                "avatar": login_data["avatar"],
                "access_token": login_data["access_token"],
                "base_url": base_url,
                "ws_base_url": ws_base_url,
            }

        return dict(await asyncio.gather(*(login_one(uuid, telephone) for uuid, telephone in needed.items())))

    accounts = await login_accounts_with_progress()
    progress.write(f"[single] 登录完成: accounts={len(accounts)}")

    deliveries: list[dict] = []
    errors: list[dict] = []
    send_window_start_ms: int | None = None
    send_window_end_ms: int | None = None
    start_event = asyncio.Event()
    all_ready_event = asyncio.Event()
    scheduled_send_start_perf: float | None = None
    ready_pairs = 0

    async def pair_worker(index: int, pair: dict) -> dict:
        nonlocal send_window_start_ms, send_window_end_ms, ready_pairs
        sender = accounts[pair["sender_uuid"]]
        receiver = accounts[pair["receiver_uuid"]]
        pending: dict[str, dict] = {}
        received_count = 0
        connection_errors = 0
        stop_event = asyncio.Event()
        receiver_ws = None
        sender_ws = None
        receiver_task = None
        sender_task = None
        try:
            async with setup_semaphore:
                session_id = await asyncio.to_thread(
                    open_session,
                    sender["base_url"],
                    sender["access_token"],
                    receiver["uuid"],
                    timeout_sec=http_timeout_sec,
                )
                receiver_ws = await ws_connect(
                    receiver["ws_base_url"],
                    args.ws_path,
                    receiver["access_token"],
                    open_timeout_sec=ws_open_timeout_sec,
                )
                try:
                    sender_ws = await ws_connect(
                        sender["ws_base_url"],
                        args.ws_path,
                        sender["access_token"],
                        open_timeout_sec=ws_open_timeout_sec,
                    )
                except Exception:
                    await receiver_ws.close()
                    raise
                if use_tqdm_progress:
                    if prepare_bar is not None:
                        prepare_bar.update(1)
                else:
                    progress.update_pairs(1)

            async def receiver_loop() -> None:
                nonlocal received_count, connection_errors
                try:
                    async for raw in receiver_ws:
                        message = parse_ws_message(raw)
                        if not message:
                            continue
                        meta = parse_bench_content(message.get("content", ""))
                        if not meta:
                            continue
                        if meta.get("run_id") != run_id:
                            continue
                        bench_id = meta["bench_id"]
                        if bench_id not in pending or pending[bench_id].get("received_ms") is not None:
                            continue
                        recv_ms = now_ms()
                        send_ms = pending[bench_id]["send_ts_ms"]
                        latency_ms = recv_ms - send_ms
                        pending[bench_id]["received_ms"] = recv_ms
                        received_count += 1
                        if use_tqdm_progress:
                            if recv_bar is not None:
                                recv_bar.update(1)
                        else:
                            progress.update_received(1)
                        deliveries.append(
                            {
                                "scenario": "single",
                                "conversation_id": f"{sender['uuid']}->{receiver['uuid']}",
                                "bench_id": bench_id,
                                "target_uuid": receiver["uuid"],
                                "send_ts_ms": send_ms,
                                "receive_ts_ms": recv_ms,
                                "latency_ms": latency_ms,
                            }
                        )
                        if received_count >= args.messages_per_sender:
                            stop_event.set()
                except Exception as exc:  # pragma: no cover
                    connection_errors += 1
                    errors.append({"scenario": "single", "pair_index": index, "where": "receiver_loop", "error": str(exc)})
                    stop_event.set()

            async def sender_loop() -> None:
                nonlocal connection_errors
                try:
                    async for raw in sender_ws:
                        if isinstance(raw, str) and "消息发送失败" in raw:
                            errors.append({"scenario": "single", "pair_index": index, "where": "sender_loop", "error": raw})
                except Exception as exc:  # pragma: no cover
                    connection_errors += 1
                    errors.append({"scenario": "single", "pair_index": index, "where": "sender_loop", "error": str(exc)})
                    stop_event.set()

            receiver_task = asyncio.create_task(receiver_loop())
            sender_task = asyncio.create_task(sender_loop())
            if args.connection_settle_ms > 0:
                await asyncio.sleep(args.connection_settle_ms / 1000.0)
            ready_pairs += 1
            if ready_pairs >= len(pairs):
                all_ready_event.set()
            await start_event.wait()
            interval_sec = max(float(args.send_interval_ms) / 1000.0, 0.0)
            phase_sec = (interval_sec / max(len(pairs), 1)) * index if interval_sec > 0 else 0.0
            for message_index in range(args.messages_per_sender):
                if scheduled_send_start_perf is not None and interval_sec > 0:
                    target_perf = scheduled_send_start_perf + phase_sec + (message_index * interval_sec)
                    wait_sec = target_perf - time.perf_counter()
                    if wait_sec > 0:
                        await asyncio.sleep(wait_sec)
                bench_id = f"{run_id}-single-{index:03d}-{message_index:03d}"
                send_ts_ms = now_ms()
                if send_window_start_ms is None or send_ts_ms < send_window_start_ms:
                    send_window_start_ms = send_ts_ms
                pending[bench_id] = {
                    "send_ts_ms": send_ts_ms,
                    "received_ms": None,
                }
                payload = build_text_message(
                    session_id,
                    sender,
                    receiver["uuid"],
                    bench_payload(run_id, "single", bench_id, send_ts_ms),
                )
                await sender_ws.send(payload)
                send_window_end_ms = now_ms()
                if use_tqdm_progress:
                    if send_bar is not None:
                        send_bar.update(1)
                else:
                    progress.update_sent(1)

            received_before_drain = received_count
            timeout_hit = False
            try:
                await asyncio.wait_for(stop_event.wait(), timeout=args.message_timeout_ms / 1000.0)
            except asyncio.TimeoutError:
                timeout_hit = True
            drain_result = await drain_until_stable(
                is_complete=lambda: received_count >= args.messages_per_sender,
                progress_value=lambda: received_count,
                max_wait_ms=args.drain_wait_ms,
                idle_wait_ms=args.drain_idle_ms,
            )
        finally:
            if receiver_task is not None:
                receiver_task.cancel()
            if sender_task is not None:
                sender_task.cancel()
            if receiver_task is not None or sender_task is not None:
                await asyncio.gather(*(task for task in [receiver_task, sender_task] if task is not None), return_exceptions=True)
            if receiver_ws is not None:
                await receiver_ws.close()
            if sender_ws is not None:
                await sender_ws.close()

        return {
            "sent": args.messages_per_sender,
            "received": received_count,
            "received_before_drain": received_before_drain,
            "connection_errors": connection_errors,
            "timeout_hit": timeout_hit,
            "drain": drain_result,
        }

    started_at = time.time()
    try:
        pair_tasks = [asyncio.create_task(pair_worker(i, pair)) for i, pair in enumerate(pairs)]
        await all_ready_event.wait()
        scheduled_send_start_perf = time.perf_counter() + 0.2
        progress.write("[single] 全会话建连完成，统一开跑")
        start_event.set()
        pair_results = await asyncio.gather(*pair_tasks)
    finally:
        progress.close()
    duration_sec = time.time() - started_at

    latency_values = [entry["latency_ms"] for entry in deliveries]
    expected = len(pairs) * args.messages_per_sender
    received = sum(result["received"] for result in pair_results)
    received_before_drain = sum(result["received_before_drain"] for result in pair_results)
    pairs_timeout = sum(1 for result in pair_results if result["timeout_hit"])
    pairs_drain_applied = sum(1 for result in pair_results if result["drain"]["applied"])
    summary = {
        "scenario": "single",
        "mode": args.mode_label,
        "run_id": run_id,
        "pair_count": len(pairs),
        "messages_per_sender": args.messages_per_sender,
        "send_interval_ms": args.send_interval_ms,
        "expected_messages": expected,
        "received_messages": received,
        "received_messages_before_drain": received_before_drain,
        "drain_recovered_messages": received - received_before_drain,
        "delivery_success_rate": round(received / expected, 6) if expected else 0.0,
        "duration_sec": round(duration_sec, 3),
        "observed_throughput_msg_per_sec": round(received / duration_sec, 3) if duration_sec > 0 else None,
        "send_window_start_ms": send_window_start_ms,
        "send_window_end_ms": send_window_end_ms or send_window_start_ms,
        "send_window_duration_ms": (
            max(0, int((send_window_end_ms or send_window_start_ms) - send_window_start_ms))
            if send_window_start_ms is not None
            else None
        ),
        "latency": summarize_latencies(latency_values),
        "drain": {
            "enabled": args.drain_wait_ms > 0,
            "max_wait_ms": args.drain_wait_ms,
            "idle_wait_ms": args.drain_idle_ms,
            "pairs_timeout": pairs_timeout,
            "pairs_applied": pairs_drain_applied,
        },
    }
    progress.write(
        f"[single] 完成: expected={expected} received={received} "
        f"success_rate={summary['delivery_success_rate']} duration={summary['duration_sec']}s "
        f"throughput={summary['observed_throughput_msg_per_sec']} msg/s"
    )
    return summary, deliveries, errors


async def run_group_scenario(args, fixture: dict, run_id: str) -> tuple[dict, list[dict], list[dict]]:
    group = fixture["group"]
    members = group["members"][: args.group_member_limit]
    if len(members) < 2:
        raise RuntimeError("group fixture needs at least two members")
    password = fixture["default_password"]

    needed = {member["uuid"]: member["telephone"] for member in members}
    setup_workers = resolve_setup_workers(args.setup_workers, len(needed))
    http_timeout_sec = resolve_setup_http_timeout_sec(args.setup_http_timeout_ms, len(needed))
    ws_open_timeout_sec = max(float(args.ws_open_timeout_ms) / 1000.0, 1.0)
    setup_semaphore = asyncio.Semaphore(setup_workers)
    accounts = await login_fixture_accounts(
        needed=needed,
        password=password,
        base_urls=args.base_urls,
        ws_base_urls=args.ws_base_urls,
        default_base_url=args.base_url,
        default_ws_base_url=args.ws_base_url,
        timeout_sec=http_timeout_sec,
        concurrency=setup_workers,
    )

    sender_fixture_uuid = group["sender_uuid"]
    sender = accounts[sender_fixture_uuid]
    receivers = [accounts[member["uuid"]] for member in members if member["uuid"] != sender_fixture_uuid]
    deliveries: list[dict] = []
    errors: list[dict] = []
    pending: dict[str, dict] = {}
    send_window_start_ms: int | None = None
    send_window_end_ms: int | None = None
    receiver_connections = []
    receiver_tasks = []
    sender_connection = None
    sender_task = None
    completion_event = asyncio.Event()
    scheduled_send_start_perf: float | None = None

    def total_receipts() -> int:
        return sum(len(item["receipts"]) for item in pending.values())

    def full_coverage_messages() -> int:
        return sum(1 for item in pending.values() if len(item["receipts"]) == len(receivers))

    async def receiver_loop(receiver: dict, ws_conn) -> None:
        try:
            async for raw in ws_conn:
                message = parse_ws_message(raw)
                if not message:
                    continue
                meta = parse_bench_content(message.get("content", ""))
                if not meta or meta.get("run_id") != run_id:
                    continue
                bench_id = meta["bench_id"]
                if bench_id not in pending:
                    continue
                record = pending[bench_id]
                if receiver["uuid"] in record["receipts"]:
                    continue
                recv_ms = now_ms()
                send_ms = record["send_ts_ms"]
                latency_ms = recv_ms - send_ms
                record["receipts"][receiver["uuid"]] = recv_ms
                deliveries.append(
                    {
                        "scenario": "group",
                        "conversation_id": group["group_id"],
                        "bench_id": bench_id,
                        "target_uuid": receiver["uuid"],
                        "send_ts_ms": send_ms,
                        "receive_ts_ms": recv_ms,
                        "latency_ms": latency_ms,
                    }
                )
                if sum(len(item["receipts"]) for item in pending.values()) >= args.messages_per_sender * len(receivers):
                    completion_event.set()
        except Exception as exc:  # pragma: no cover
            errors.append({"scenario": "group", "where": f"receiver_loop:{receiver['uuid']}", "error": str(exc)})
            completion_event.set()

    async def sender_loop(ws_conn) -> None:
        try:
            async for raw in ws_conn:
                if isinstance(raw, str) and "消息发送失败" in raw:
                    errors.append({"scenario": "group", "where": "sender_loop", "error": raw})
        except Exception as exc:  # pragma: no cover
            errors.append({"scenario": "group", "where": "sender_loop", "error": str(exc)})
            completion_event.set()

    started_at = time.time()
    try:
        async with setup_semaphore:
            session_id = await asyncio.to_thread(
                open_session,
                sender["base_url"],
                sender["access_token"],
                group["group_id"],
                timeout_sec=http_timeout_sec,
            )
            for receiver in receivers:
                conn = await ws_connect(
                    receiver["ws_base_url"],
                    args.ws_path,
                    receiver["access_token"],
                    open_timeout_sec=ws_open_timeout_sec,
                )
                receiver_connections.append(conn)
            sender_connection = await ws_connect(
                sender["ws_base_url"],
                args.ws_path,
                sender["access_token"],
                open_timeout_sec=ws_open_timeout_sec,
            )
            await asyncio.sleep(args.connection_settle_ms / 1000.0)

        for receiver, conn in zip(receivers, receiver_connections):
            receiver_tasks.append(asyncio.create_task(receiver_loop(receiver, conn)))
        sender_task = asyncio.create_task(sender_loop(sender_connection))

        scheduled_send_start_perf = time.perf_counter() + 0.2
        for message_index in range(args.messages_per_sender):
            interval_sec = max(float(args.send_interval_ms) / 1000.0, 0.0)
            if scheduled_send_start_perf is not None and interval_sec > 0:
                target_perf = scheduled_send_start_perf + (message_index * interval_sec)
                wait_sec = target_perf - time.perf_counter()
                if wait_sec > 0:
                    await asyncio.sleep(wait_sec)
            bench_id = f"{run_id}-group-{message_index:03d}"
            send_ts_ms = now_ms()
            if send_window_start_ms is None or send_ts_ms < send_window_start_ms:
                send_window_start_ms = send_ts_ms
            pending[bench_id] = {"send_ts_ms": send_ts_ms, "receipts": {}}
            payload = build_text_message(
                session_id,
                sender,
                group["group_id"],
                bench_payload(run_id, "group", bench_id, send_ts_ms),
            )
            await sender_connection.send(payload)
            send_window_end_ms = now_ms()

        try:
            await asyncio.wait_for(completion_event.wait(), timeout=args.message_timeout_ms / 1000.0)
        except asyncio.TimeoutError:
            timeout_hit = True
        else:
            timeout_hit = False
        receipts_before_drain = total_receipts()
        full_coverage_before_drain = full_coverage_messages()
        drain_result = await drain_until_stable(
            is_complete=lambda: total_receipts() >= args.messages_per_sender * len(receivers),
            progress_value=total_receipts,
            max_wait_ms=args.drain_wait_ms,
            idle_wait_ms=args.drain_idle_ms,
        )
    finally:
        for task in receiver_tasks:
            task.cancel()
        if sender_task is not None:
            sender_task.cancel()
        await asyncio.gather(*receiver_tasks, return_exceptions=True)
        if sender_task is not None:
            await asyncio.gather(sender_task, return_exceptions=True)
        for conn in receiver_connections:
            await conn.close()
        if sender_connection is not None:
            await sender_connection.close()

    duration_sec = time.time() - started_at
    receipt_latencies = [entry["latency_ms"] for entry in deliveries]
    broadcast_completion = []
    full_coverage_message_count = 0
    for record in pending.values():
        if len(record["receipts"]) == len(receivers):
            full_coverage_message_count += 1
            completion_ms = max(record["receipts"].values()) - record["send_ts_ms"]
            broadcast_completion.append(completion_ms)
    expected_receipts = args.messages_per_sender * len(receivers)
    summary = {
        "scenario": "group",
        "mode": args.mode_label,
        "run_id": run_id,
        "group_id": group["group_id"],
        "group_name": group["group_name"],
        "group_member_count": len(members),
        "receiver_count": len(receivers),
        "messages_per_sender": args.messages_per_sender,
        "send_interval_ms": args.send_interval_ms,
        "expected_receipts": expected_receipts,
        "received_receipts": len(deliveries),
        "received_receipts_before_drain": receipts_before_drain,
        "drain_recovered_receipts": len(deliveries) - receipts_before_drain,
        "delivery_coverage_rate": round(len(deliveries) / expected_receipts, 6) if expected_receipts else 0.0,
        "full_coverage_message_rate": round(full_coverage_message_count / args.messages_per_sender, 6) if args.messages_per_sender else 0.0,
        "full_coverage_messages_before_drain": full_coverage_before_drain,
        "drain_recovered_full_coverage_messages": full_coverage_message_count - full_coverage_before_drain,
        "duration_sec": round(duration_sec, 3),
        "observed_delivery_per_sec": round(len(deliveries) / duration_sec, 3) if duration_sec > 0 else None,
        "send_window_start_ms": send_window_start_ms,
        "send_window_end_ms": send_window_end_ms or send_window_start_ms,
        "send_window_duration_ms": (
            max(0, int((send_window_end_ms or send_window_start_ms) - send_window_start_ms))
            if send_window_start_ms is not None
            else None
        ),
        "receipt_latency": summarize_latencies(receipt_latencies),
        "broadcast_completion_latency": summarize_latencies(broadcast_completion),
        "drain": {
            "enabled": args.drain_wait_ms > 0,
            "max_wait_ms": args.drain_wait_ms,
            "idle_wait_ms": args.drain_idle_ms,
            "timeout_hit": timeout_hit,
            "applied": drain_result["applied"],
            "waited_ms": drain_result["waited_ms"],
            "reason": drain_result["reason"],
        },
    }
    return summary, deliveries, errors


def write_outputs(output_dir: Path, summary: dict, deliveries: list[dict], errors: list[dict]) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    (output_dir / "summary.json").write_text(json.dumps(summary, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    with (output_dir / "deliveries.csv").open("w", encoding="utf-8", newline="") as fp:
        writer = csv.DictWriter(
            fp,
            fieldnames=["scenario", "conversation_id", "bench_id", "target_uuid", "send_ts_ms", "receive_ts_ms", "latency_ms"],
        )
        writer.writeheader()
        for row in deliveries:
            writer.writerow(row)
    (output_dir / "errors.json").write_text(json.dumps(errors, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser(description="Run EchoChat message latency scenarios without changing business code.")
    parser.add_argument("--base-url", required=True)
    parser.add_argument("--ws-base-url", required=True)
    parser.add_argument("--base-urls", default="")
    parser.add_argument("--ws-base-urls", default="")
    parser.add_argument("--ws-path", default="/wss")
    parser.add_argument("--fixture", required=True)
    parser.add_argument("--scenario", choices=["single", "group"], required=True)
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--messages-per-sender", type=int, required=True)
    parser.add_argument("--send-interval-ms", type=int, required=True)
    parser.add_argument("--message-timeout-ms", type=int, default=10000)
    parser.add_argument("--connection-settle-ms", type=int, default=1000)
    parser.add_argument("--setup-workers", type=int, default=16)
    parser.add_argument("--setup-http-timeout-ms", type=int, default=90000)
    parser.add_argument("--ws-open-timeout-ms", type=int, default=30000)
    parser.add_argument("--drain-wait-ms", type=int, default=5000)
    parser.add_argument("--drain-idle-ms", type=int, default=1000)
    parser.add_argument("--pair-count", type=int, default=30)
    parser.add_argument("--group-member-limit", type=int, default=25)
    parser.add_argument("--mode-label", required=True)
    parser.add_argument("--server-pid", type=int)
    parser.add_argument("--plain-progress", action="store_true")
    args = parser.parse_args()
    args.base_url = args.base_url.rstrip("/")
    args.ws_base_url = args.ws_base_url.rstrip("/")
    args.base_urls = parse_url_list(args.base_urls) or [args.base_url]
    args.ws_base_urls = parse_url_list(args.ws_base_urls) or [args.ws_base_url]

    fixture = json.loads(Path(args.fixture).read_text(encoding="utf-8"))
    run_id = f"{args.mode_label}-{args.scenario}-{int(time.time())}"
    sampler = ResourceSampler(args.server_pid) if args.server_pid else None
    if sampler:
        sampler.start()
    started_at = time.time()
    if args.scenario == "single":
        summary, deliveries, errors = asyncio.run(run_single_scenario(args, fixture, run_id))
    else:
        summary, deliveries, errors = asyncio.run(run_group_scenario(args, fixture, run_id))
    if sampler:
        summary["server_resource_peak"] = sampler.stop()
    summary["started_at_unix"] = int(started_at)
    summary["started_at_ms"] = int(started_at * 1000)
    summary["finished_at_unix"] = int(time.time())
    summary["finished_at_ms"] = now_ms()
    write_outputs(Path(args.output_dir), summary, deliveries, errors)
    print(Path(args.output_dir) / "summary.json")


if __name__ == "__main__":
    main()
