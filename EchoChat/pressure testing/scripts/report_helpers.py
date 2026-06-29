from __future__ import annotations

import csv
import json
import math
import statistics
from collections import Counter, defaultdict
from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
from matplotlib import font_manager, rcParams
from matplotlib.ticker import FuncFormatter


def _configure_plot_fonts() -> bool:
    preferred_keywords = [
        "noto sans cjk",
        "source han sans",
        "source han serif",
        "wenquanyi",
        "wqy",
        "microsoft yahei",
        "simhei",
        "pingfang",
        "heiti",
        "songti",
    ]
    for font in font_manager.fontManager.ttflist:
        name = font.name.strip()
        lower = name.lower()
        if any(keyword in lower for keyword in preferred_keywords):
            rcParams["font.sans-serif"] = [name, "DejaVu Sans"]
            rcParams["axes.unicode_minus"] = False
            return True
    rcParams["font.sans-serif"] = ["DejaVu Sans"]
    rcParams["axes.unicode_minus"] = False
    return False


HAS_CJK_FONT = _configure_plot_fonts()

FIG_BG = "#f6f1e8"
PANEL_BG = "#fffdf8"
PANEL_ALT = "#f2ebe0"
GRID = "#d9cfbf"
TEXT = "#2d241b"
SUBTEXT = "#6f6254"
GREEN = "#2f7f5f"
GREEN_LIGHT = "#a9d8c3"
RED = "#c95d3a"
BLUE = "#2a6f97"
BLUE_LIGHT = "#8ecae6"
GOLD = "#d4a017"
SLATE = "#6b7c93"
PURPLE = "#7a5c99"
TEAL = "#1f8a70"
ORANGE = "#ef8f35"
ROSE = "#b85c6d"

rcParams.update(
    {
        "figure.facecolor": FIG_BG,
        "axes.facecolor": PANEL_BG,
        "axes.edgecolor": GRID,
        "axes.labelcolor": TEXT,
        "axes.titlecolor": TEXT,
        "xtick.color": SUBTEXT,
        "ytick.color": SUBTEXT,
        "grid.color": GRID,
        "grid.alpha": 0.45,
        "axes.grid": False,
        "axes.spines.top": False,
        "axes.spines.right": False,
        "axes.titleweight": "bold",
        "savefig.facecolor": FIG_BG,
        "savefig.bbox": "tight",
    }
)


def plot_text(cjk: str, ascii_text: str) -> str:
    if HAS_CJK_FONT:
        return cjk
    return ascii_text


def nice_number(value, digits: int = 3) -> str:
    if value is None:
        return "-"
    if isinstance(value, int):
        return f"{value:,}"
    if isinstance(value, float):
        if abs(value) >= 1000 and value.is_integer():
            return f"{int(value):,}"
        text = f"{value:.{digits}f}".rstrip("0").rstrip(".")
        return text
    return str(value)


def setup_axis(ax, *, title: str, subtitle: str | None = None, xlabel: str | None = None, ylabel: str | None = None) -> None:
    ax.set_title(title, loc="left", fontsize=12, pad=14)
    if subtitle:
        ax.text(0.0, 1.02, subtitle, transform=ax.transAxes, ha="left", va="bottom", fontsize=9, color=SUBTEXT)
    if xlabel:
        ax.set_xlabel(xlabel, fontsize=9, color=SUBTEXT)
    if ylabel:
        ax.set_ylabel(ylabel, fontsize=9, color=SUBTEXT)
    ax.grid(True, axis="y", linestyle="--", linewidth=0.8, alpha=0.35)
    for side in ("left", "bottom"):
        ax.spines[side].set_color(GRID)
        ax.spines[side].set_linewidth(0.9)


def add_metric_card(ax, lines: list[str], *, loc: tuple[float, float] = (0.98, 0.98), ha: str = "right") -> None:
    ax.text(
        loc[0],
        loc[1],
        "\n".join(lines),
        transform=ax.transAxes,
        ha=ha,
        va="top",
        fontsize=9,
        color=TEXT,
        bbox={"boxstyle": "round,pad=0.45", "facecolor": PANEL_ALT, "alpha": 0.98, "edgecolor": GRID},
    )


def save_figure(fig, path: Path, *, dpi: int = 180) -> None:
    fig.savefig(path, dpi=dpi, facecolor=FIG_BG)


def human_count_axis():
    return FuncFormatter(lambda value, _: nice_number(value, digits=0))


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


def summarize(values: list[float]) -> dict[str, float | int | None]:
    p50 = percentile(values, 0.50)
    p99 = percentile(values, 0.99)
    return {
        "count": len(values),
        "avg_ms": round(statistics.fmean(values), 3) if values else None,
        "median_ms": round(percentile(values, 0.50), 3) if values else None,
        "min_ms": round(min(values), 3) if values else None,
        "p10_ms": round(percentile(values, 0.10), 3) if values else None,
        "p25_ms": round(percentile(values, 0.25), 3) if values else None,
        "p50_ms": round(percentile(values, 0.50), 3) if values else None,
        "p75_ms": round(percentile(values, 0.75), 3) if values else None,
        "p90_ms": round(percentile(values, 0.90), 3) if values else None,
        "p95_ms": round(percentile(values, 0.95), 3) if values else None,
        "p99_ms": round(percentile(values, 0.99), 3) if values else None,
        "p99_9_ms": round(percentile(values, 0.999), 3) if values else None,
        "max_ms": round(max(values), 3) if values else None,
        "stddev_ms": round(statistics.pstdev(values), 3) if len(values) > 1 else (0.0 if values else None),
        "p99_div_p50": round((float(p99) / float(p50)), 3) if p99 is not None and p50 not in (None, 0) else None,
    }


def table_value(value) -> str:
    if value is None:
        return "-"
    return str(value)


def event_time_ms(event: dict) -> float:
    return float(event["occurred_unix_ns"]) / 1_000_000.0


def first_field(fields: dict | None, key: str, default=None):
    if not isinstance(fields, dict):
        return default
    return fields.get(key, default)


def nested_field(data: dict | None, path: tuple[str, ...], default=None):
    current = data
    for key in path:
        if not isinstance(current, dict):
            return default
        current = current.get(key)
    return current if current is not None else default


def first_nested(data: dict | None, paths: list[tuple[str, ...]], default=None):
    for path in paths:
        value = nested_field(data, path)
        if value is not None:
            return value
    return default


def load_json(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def write_json(path: Path, data: dict) -> None:
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2), encoding="utf-8")


def write_csv(path: Path, fieldnames: list[str], rows: list[dict]) -> None:
    with path.open("w", encoding="utf-8", newline="") as fp:
        writer = csv.DictWriter(fp, fieldnames=fieldnames)
        writer.writeheader()
        for row in rows:
            writer.writerow(row)


def summary_time_anchor_ms(summary: dict) -> float | None:
    send_window_start_ms = summary.get("send_window_start_ms")
    if isinstance(send_window_start_ms, (int, float)):
        return float(send_window_start_ms)
    started_at_ms = summary.get("started_at_ms")
    if isinstance(started_at_ms, (int, float)):
        return float(started_at_ms)
    started_at_unix = summary.get("started_at_unix")
    if isinstance(started_at_unix, (int, float)):
        return float(started_at_unix) * 1000.0
    return None


def configured_pressure_window_ms(cfg: dict, summary: dict, send_times_ms: list[float]) -> float | None:
    duration_sec = first_field(cfg.get("scenario"), "duration_sec")
    if isinstance(duration_sec, (int, float)) and duration_sec > 0:
        return float(duration_sec) * 1000.0
    send_window_duration_ms = summary.get("send_window_duration_ms")
    if isinstance(send_window_duration_ms, (int, float)) and send_window_duration_ms > 0:
        return float(send_window_duration_ms)
    if send_times_ms:
        return max(0.0, max(send_times_ms) - min(send_times_ms))
    return None


def window_observed_throughput(summary: dict, pressure_window_ms: float | None) -> float | None:
    received_before_drain = summary.get("received_messages_before_drain", summary.get("received_messages"))
    if not isinstance(received_before_drain, (int, float)):
        return None
    if not isinstance(pressure_window_ms, (int, float)) or pressure_window_ms <= 0:
        return None
    return float(received_before_drain) / (float(pressure_window_ms) / 1000.0)


def _build_meta_and_events(trace: dict) -> tuple[dict[str, dict], dict[str, dict[str, list[dict]]], list[dict]]:
    meta_by_id: dict[str, dict] = {}
    for meta in trace.get("messages", []):
        bench_id = meta.get("bench_id")
        if bench_id:
            meta_by_id[bench_id] = meta

    events_by_id: dict[str, dict[str, list[dict]]] = defaultdict(lambda: defaultdict(list))
    all_events: list[dict] = []
    for event in trace.get("events", []):
        bench_id = event.get("bench_id")
        if not bench_id:
            continue
        events_by_id[bench_id][event.get("event", "")].append(event)
        all_events.append(event)
    for event_map in events_by_id.values():
        for event_list in event_map.values():
            event_list.sort(key=lambda item: item["occurred_unix_ns"])
    all_events.sort(key=lambda item: item["occurred_unix_ns"])
    return meta_by_id, events_by_id, all_events


def _ready_timeline(
    all_events: list[dict],
    partitions: int,
    *,
    anchor_ms: float | None,
    window_ms: float | None = None,
    bin_ms: float = 5.0,
) -> tuple[list[dict], dict]:
    relevant = [
        event
        for event in all_events
        if event.get("event") in {"conversation_ready_enqueue", "conversation_ready_dequeue"}
    ]
    if not relevant or partitions <= 0:
        return [], {"avg_depth": None, "max_depth": None}
    start_ms = anchor_ms if anchor_ms is not None else event_time_ms(relevant[0])
    if window_ms is not None and window_ms > 0:
        end_ms = start_ms + window_ms
    else:
        end_ms = max(start_ms, event_time_ms(relevant[-1]))
    bins = max(1, int(math.ceil((end_ms - start_ms) / bin_ms)) + 1)
    rows: list[dict] = []
    depths = {partition: 0 for partition in range(partitions)}
    index = 0
    avg_values: list[float] = []
    max_values: list[float] = []
    for bucket_idx in range(bins):
        bucket_time = start_ms + bucket_idx * bin_ms
        while index < len(relevant) and event_time_ms(relevant[index]) <= bucket_time:
            event = relevant[index]
            partition = int(first_field(event.get("fields"), "partition", 0))
            if partition not in depths:
                depths[partition] = 0
            if event.get("event") == "conversation_ready_enqueue":
                depths[partition] += 1
            else:
                depths[partition] = max(0, depths[partition] - 1)
            index += 1
        avg_depth = sum(depths.values()) / max(1, partitions)
        max_depth = max(depths.values()) if depths else 0
        avg_values.append(avg_depth)
        max_values.append(max_depth)
        rows.append(
            {
                "time_ms": round(bucket_time - start_ms, 3),
                "avg_ready_depth_per_partition": round(avg_depth, 6),
                "max_ready_depth_single_partition": max_depth,
            }
        )
    return rows, {
        "avg_depth": round(statistics.fmean(avg_values), 3) if avg_values else None,
        "max_depth": round(max(max_values), 3) if max_values else None,
        "p95_depth": round(percentile(avg_values, 0.95), 3) if avg_values else None,
    }


def _bucketize_times(
    timestamps: list[float],
    *,
    start_ms: float,
    window_ms: float,
    bin_ms: float,
    reducer: str = "count",
) -> list[float]:
    bins = max(1, int(math.ceil(window_ms / bin_ms)))
    values: list[list[float]] = [[] for _ in range(bins)]
    for ts in timestamps:
        offset = ts - start_ms
        if offset < 0 or offset >= window_ms:
            continue
        bucket = int(offset // bin_ms)
        values[bucket].append(ts)
    if reducer == "count":
        return [float(len(bucket)) for bucket in values]
    raise ValueError(f"unsupported reducer: {reducer}")



def _bucketize_samples(
    samples: list[tuple[float, float]],
    *,
    start_ms: float,
    window_ms: float,
    bin_ms: float,
    agg: str = "avg",
) -> list[float | None]:
    bins = max(1, int(math.ceil(window_ms / bin_ms)))
    values: list[list[float]] = [[] for _ in range(bins)]
    for ts, value in samples:
        offset = ts - start_ms
        if offset < 0 or offset >= window_ms:
            continue
        bucket = int(offset // bin_ms)
        values[bucket].append(float(value))
    out: list[float | None] = []
    for bucket in values:
        if not bucket:
            out.append(None)
        elif agg == "avg":
            out.append(statistics.fmean(bucket))
        elif agg == "sum":
            out.append(sum(bucket))
        elif agg == "max":
            out.append(max(bucket))
        else:
            raise ValueError(f"unsupported agg: {agg}")
    return out



def _smooth_series(values: list[float | None], window: int = 5) -> list[float | None]:
    if window <= 1:
        return values[:]
    radius = window // 2
    out: list[float | None] = []
    for idx in range(len(values)):
        chunk = [
            v
            for v in values[max(0, idx - radius) : min(len(values), idx + radius + 1)]
            if isinstance(v, (int, float))
        ]
        out.append(statistics.fmean(chunk) if chunk else None)
    return out



def _worker_ingress(
    events: list[dict],
    *,
    anchor_ms: float | None,
    window_ms: float = 20000.0,
    bin_ms: float = 1.0,
) -> tuple[list[dict], list[dict], dict]:
    enqueue_events = [event for event in events if event.get("event") == "mysql_persist_enqueue_done"]
    if not enqueue_events:
        return [], [], {"window_ms": window_ms, "bin_ms": bin_ms, "worker_count": 0}
    enqueue_events.sort(key=lambda item: item["occurred_unix_ns"])
    start_ms = anchor_ms if anchor_ms is not None else event_time_ms(enqueue_events[0])
    bins = max(1, int(math.ceil(window_ms / bin_ms)))
    grouped: dict[int, list[float]] = defaultdict(list)
    for event in enqueue_events:
        grouped[int(first_field(event.get("fields"), "worker_index", 0))].append(event_time_ms(event))

    series_rows: list[dict] = []
    summary_rows: list[dict] = []
    for worker_index, timestamps in sorted(grouped.items()):
        counts = [0] * bins
        for ts in timestamps:
            offset = ts - start_ms
            if offset < 0 or offset >= window_ms:
                continue
            bucket = int(offset // bin_ms)
            counts[bucket] += 1
        peak = max(counts) if counts else 0
        avg = statistics.fmean(counts) if counts else 0.0
        non_zero = sum(1 for value in counts if value > 0)
        summary_rows.append(
            {
                "worker_index": worker_index,
                "total_events": len(timestamps),
                "events_in_window": sum(counts),
                "peak_count_per_bin": peak,
                "avg_count_per_bin": round(avg, 6),
                "non_zero_bins": non_zero,
            }
        )
        for idx, count in enumerate(counts):
            series_rows.append(
                {
                    "worker_index": worker_index,
                    "time_ms": round(idx * bin_ms, 3),
                    "count": count,
                }
            )
    return series_rows, summary_rows, {
        "window_ms": window_ms,
        "bin_ms": bin_ms,
        "worker_count": len(grouped),
        "total_series_rows": len(series_rows),
        "time_anchor_ms": round(start_ms, 3),
    }


def _flush_groups(all_events: list[dict]) -> dict[tuple[int, int], dict]:
    groups: dict[tuple[int, int], dict] = {}
    for event in all_events:
        if event.get("event") != "mysql_persist_flush_done":
            continue
        worker = int(first_field(event.get("fields"), "worker_index", 0))
        flush_seq = int(first_field(event.get("fields"), "flush_seq", 0))
        key = (worker, flush_seq)
        entry = groups.setdefault(
            key,
            {
                "worker_index": worker,
                "flush_seq": flush_seq,
                "flush_reason": first_field(event.get("fields"), "flush_reason", "unknown"),
                "flush_batch_size": int(first_field(event.get("fields"), "flush_batch_size", 0)),
                "flush_done_at_ms": event_time_ms(event),
                "bench_ids": [],
            },
        )
        entry["bench_ids"].append(event.get("bench_id"))
    return groups


def _unique_flush_rows(all_events: list[dict]) -> list[dict]:
    groups = _flush_groups(all_events)
    rows: list[dict] = []
    for group in groups.values():
        rows.append(
            {
                "worker_index": group["worker_index"],
                "flush_seq": group["flush_seq"],
                "flush_reason": group["flush_reason"],
                "flush_batch_size": group["flush_batch_size"],
                "flush_done_at_ms": round(group["flush_done_at_ms"], 6),
                "message_count": len(group["bench_ids"]),
            }
        )
    rows.sort(key=lambda item: (item["worker_index"], item["flush_seq"]))
    return rows


def _flush_detailed_rows(all_events: list[dict], events_by_id: dict[str, dict[str, list[dict]]]) -> list[dict]:
    groups = _flush_groups(all_events)
    rows: list[dict] = []
    for key, group in sorted(groups.items()):
        enqueue_times: list[float] = []
        worker_start_times: list[float] = []
        batch_collect_times: list[float] = []
        sql_exec_start_times: list[float] = []
        sql_exec_done_times: list[float] = []
        for bench_id in group["bench_ids"]:
            event_map = events_by_id.get(bench_id, {})
            enqueue_times.extend(event_time_ms(ev) for ev in event_map.get("mysql_persist_enqueue_done", []))
            worker_start_times.extend(event_time_ms(ev) for ev in event_map.get("mysql_persist_worker_start", []))
            batch_collect_times.extend(event_time_ms(ev) for ev in event_map.get("mysql_persist_batch_collect_start", []))
            sql_exec_start_times.extend(event_time_ms(ev) for ev in event_map.get("mysql_persist_sql_exec_start", []))
            sql_exec_done_times.extend(event_time_ms(ev) for ev in event_map.get("mysql_persist_sql_exec_done", []))

        enqueue_start_ms = min(enqueue_times) if enqueue_times else None
        enqueue_end_ms = max(enqueue_times) if enqueue_times else None
        worker_start_ms = min(worker_start_times) if worker_start_times else None
        batch_collect_start_ms = min(batch_collect_times) if batch_collect_times else None
        sql_exec_start_ms = min(sql_exec_start_times) if sql_exec_start_times else None
        sql_exec_done_ms = max(sql_exec_done_times) if sql_exec_done_times else None
        flush_done_ms = group["flush_done_at_ms"]

        formation_wait_ms = None
        if enqueue_start_ms is not None and sql_exec_start_ms is not None:
            formation_wait_ms = max(0.0, sql_exec_start_ms - enqueue_start_ms)
        exec_ms = None
        if sql_exec_start_ms is not None and sql_exec_done_ms is not None:
            exec_ms = max(0.0, sql_exec_done_ms - sql_exec_start_ms)
        total_wait_plus_exec_ms = None
        if formation_wait_ms is not None and exec_ms is not None:
            total_wait_plus_exec_ms = formation_wait_ms + exec_ms
        batch_size = int(group.get("flush_batch_size", 0) or 0)
        exec_rate = None
        if exec_ms and exec_ms > 0:
            exec_rate = batch_size / exec_ms
        effective_rate = None
        if total_wait_plus_exec_ms and total_wait_plus_exec_ms > 0:
            effective_rate = batch_size / total_wait_plus_exec_ms

        rows.append(
            {
                "worker_index": group["worker_index"],
                "flush_seq": group["flush_seq"],
                "flush_reason": group["flush_reason"],
                "flush_batch_size": batch_size,
                "message_count": len(group["bench_ids"]),
                "enqueue_start_ms": round(enqueue_start_ms, 6) if enqueue_start_ms is not None else None,
                "enqueue_end_ms": round(enqueue_end_ms, 6) if enqueue_end_ms is not None else None,
                "worker_start_ms": round(worker_start_ms, 6) if worker_start_ms is not None else None,
                "batch_collect_start_ms": round(batch_collect_start_ms, 6) if batch_collect_start_ms is not None else None,
                "sql_exec_start_ms": round(sql_exec_start_ms, 6) if sql_exec_start_ms is not None else None,
                "sql_exec_done_ms": round(sql_exec_done_ms, 6) if sql_exec_done_ms is not None else None,
                "flush_done_at_ms": round(flush_done_ms, 6),
                "formation_wait_ms": round(formation_wait_ms, 6) if formation_wait_ms is not None else None,
                "exec_ms": round(exec_ms, 6) if exec_ms is not None else None,
                "total_wait_plus_exec_ms": round(total_wait_plus_exec_ms, 6) if total_wait_plus_exec_ms is not None else None,
                "exec_rate_msgs_per_ms": round(exec_rate, 9) if exec_rate is not None else None,
                "effective_rate_msgs_per_ms": round(effective_rate, 9) if effective_rate is not None else None,
            }
        )
    return rows



def _flush_type_time_rows(
    unique_flush_rows: list[dict],
    *,
    anchor_ms: float | None,
    window_ms: float,
    bin_ms: float = 50.0,
) -> tuple[list[dict], dict]:
    if not unique_flush_rows:
        return [], {"window_ms": window_ms, "bin_ms": bin_ms, "flush_types": []}
    start_ms = anchor_ms if anchor_ms is not None else min(float(row["flush_done_at_ms"]) for row in unique_flush_rows)
    bins = max(1, int(math.ceil(window_ms / bin_ms)))
    reasons = ["timer", "batch_full", "single"]
    counts: dict[str, list[int]] = {reason: [0] * bins for reason in reasons}
    for row in unique_flush_rows:
        reason = str(row.get("flush_reason", "unknown"))
        if reason not in counts:
            continue
        ts = float(row.get("flush_done_at_ms", 0.0))
        offset = ts - start_ms
        if offset < 0 or offset >= window_ms:
            continue
        bucket = int(offset // bin_ms)
        counts[reason][bucket] += 1
    rows: list[dict] = []
    for reason in reasons:
        for idx, count in enumerate(counts[reason]):
            rows.append({"flush_reason": reason, "time_ms": round(idx * bin_ms, 3), "count": count})
    return rows, {
        "window_ms": window_ms,
        "bin_ms": bin_ms,
        "time_anchor_ms": round(start_ms, 3),
        "flush_types": reasons,
    }



def _worker_persist_rate_rows(
    events: list[dict],
    flush_detailed_rows: list[dict],
    *,
    anchor_ms: float | None,
    window_ms: float,
    bin_ms: float = 50.0,
    smooth_window: int = 5,
) -> tuple[list[dict], dict]:
    start_ms = anchor_ms
    if start_ms is None:
        candidates = []
        if flush_detailed_rows:
            candidates.extend(float(row["flush_done_at_ms"]) for row in flush_detailed_rows if row.get("flush_done_at_ms") is not None)
        enqueue_events = [event_time_ms(ev) for ev in events if ev.get("event") == "mysql_persist_enqueue_done"]
        candidates.extend(enqueue_events)
        if not candidates:
            return [], {"window_ms": window_ms, "bin_ms": bin_ms, "worker_count": 0}
        start_ms = min(candidates)

    enqueue_grouped: dict[int, list[float]] = defaultdict(list)
    for ev in events:
        if ev.get("event") != "mysql_persist_enqueue_done":
            continue
        worker_index = int(first_field(ev.get("fields"), "worker_index", 0))
        enqueue_grouped[worker_index].append(event_time_ms(ev))

    worker_start_grouped: dict[int, list[float]] = defaultdict(list)
    for ev in events:
        if ev.get("event") != "mysql_persist_worker_start":
            continue
        worker_index = int(first_field(ev.get("fields"), "worker_index", 0))
        worker_start_grouped[worker_index].append(event_time_ms(ev))

    flush_grouped: dict[int, list[dict]] = defaultdict(list)
    for row in flush_detailed_rows:
        flush_grouped[int(row.get("worker_index", 0))].append(row)

    worker_indices = sorted(set(enqueue_grouped) | set(worker_start_grouped) | set(flush_grouped))
    if not worker_indices:
        return [], {"window_ms": window_ms, "bin_ms": bin_ms, "worker_count": 0}

    rows: list[dict] = []
    summary_rows: list[dict] = []
    for worker_index in worker_indices:
        ingress_counts = _bucketize_times(enqueue_grouped.get(worker_index, []), start_ms=start_ms, window_ms=window_ms, bin_ms=bin_ms, reducer="count")
        ingress_rate = [value * (1000.0 / bin_ms) for value in ingress_counts]

        exec_samples = []
        effective_samples = []
        for row in flush_grouped.get(worker_index, []):
            ts = row.get("sql_exec_done_ms") or row.get("flush_done_at_ms")
            exec_rate = row.get("exec_rate_msgs_per_ms")
            effective_rate = row.get("effective_rate_msgs_per_ms")
            if ts is not None and exec_rate is not None:
                exec_samples.append((float(ts), float(exec_rate) * 1000.0))
            if ts is not None and effective_rate is not None:
                effective_samples.append((float(ts), float(effective_rate) * 1000.0))
        exec_rate_bins = _bucketize_samples(exec_samples, start_ms=start_ms, window_ms=window_ms, bin_ms=bin_ms, agg="avg")
        effective_rate_bins = _bucketize_samples(effective_samples, start_ms=start_ms, window_ms=window_ms, bin_ms=bin_ms, agg="avg")

        gap_samples = []
        starts = sorted(worker_start_grouped.get(worker_index, []))
        for left, right in zip(starts, starts[1:]):
            gap_samples.append((right, right - left))
        gap_bins = _bucketize_samples(gap_samples, start_ms=start_ms, window_ms=window_ms, bin_ms=bin_ms, agg="avg")

        ingress_rate = _smooth_series(ingress_rate, window=smooth_window)
        exec_rate_bins = _smooth_series(exec_rate_bins, window=smooth_window)
        effective_rate_bins = _smooth_series(effective_rate_bins, window=smooth_window)
        gap_bins = _smooth_series(gap_bins, window=smooth_window)

        summary_rows.append({
            "worker_index": worker_index,
            "peak_ingress_rate": round(max(v for v in ingress_rate if isinstance(v, (int, float))), 6) if any(isinstance(v, (int, float)) for v in ingress_rate) else None,
            "avg_ingress_rate": round(statistics.fmean(v for v in ingress_rate if isinstance(v, (int, float))), 6) if any(isinstance(v, (int, float)) for v in ingress_rate) else None,
        })

        bins = max(1, int(math.ceil(window_ms / bin_ms)))
        for idx in range(bins):
            rows.append({
                "worker_index": worker_index,
                "time_ms": round(idx * bin_ms, 3),
                "ingress_rate_msgs_per_sec": round(float(ingress_rate[idx]), 6) if isinstance(ingress_rate[idx], (int, float)) else None,
                "exec_rate_msgs_per_sec": round(float(exec_rate_bins[idx]), 6) if isinstance(exec_rate_bins[idx], (int, float)) else None,
                "effective_rate_msgs_per_sec": round(float(effective_rate_bins[idx]), 6) if isinstance(effective_rate_bins[idx], (int, float)) else None,
                "worker_gap_ms": round(float(gap_bins[idx]), 6) if isinstance(gap_bins[idx], (int, float)) else None,
            })
    return rows, {
        "window_ms": window_ms,
        "bin_ms": bin_ms,
        "time_anchor_ms": round(start_ms, 3),
        "worker_count": len(worker_indices),
        "smooth_window": smooth_window,
    }


def _batch_fill_quantiles_rows(flush_detailed_rows: list[dict]) -> list[dict]:
    values = [float(row["formation_wait_ms"]) for row in flush_detailed_rows if row.get("flush_reason") == "batch_full" and isinstance(row.get("formation_wait_ms"), (int, float, str)) and str(row.get("formation_wait_ms")) not in ("", "None")]
    if not values:
        return []
    quantiles = list(range(10, 100, 10)) + list(range(90, 100, 1)) + [99]
    uniq = []
    seen = set()
    for q in quantiles:
        if q not in seen:
            uniq.append(q)
            seen.add(q)
    rows = []
    for q in uniq:
        value = percentile(values, q / 100.0)
        rows.append({"quantile": q, "fill_time_ms": round(value, 6) if value is not None else None})
    return rows



def _batch_fill_curve_rows(
    all_events: list[dict],
    events_by_id: dict[str, dict[str, list[dict]]],
) -> list[dict]:
    flush_groups = _flush_groups(all_events)
    groups: list[tuple[int, list[float]]] = []
    max_batch = 0
    for group in flush_groups.values():
        if group.get("flush_reason") != "batch_full":
            continue
        enqueue_times: list[float] = []
        for bench_id in group.get("bench_ids", []):
            enqueue_times.extend(
                event_time_ms(ev)
                for ev in events_by_id.get(bench_id, {}).get("mysql_persist_enqueue_done", [])
            )
        if not enqueue_times:
            continue
        enqueue_times.sort()
        batch_size = min(int(group.get("flush_batch_size", 0) or 0), len(enqueue_times))
        if batch_size <= 0:
            continue
        groups.append((batch_size, enqueue_times[:batch_size]))
        max_batch = max(max_batch, batch_size)

    if not groups or max_batch <= 0:
        return []

    out = []
    for batch_index in range(1, max_batch + 1):
        samples: list[float] = []
        for batch_size, enqueue_times in groups:
            if batch_index > batch_size:
                continue
            fill_time = enqueue_times[batch_index - 1] - enqueue_times[0]
            samples.append(max(0.0, fill_time))
        out.append(
            {
                "batch_index": batch_index,
                "avg_fill_time_ms": round(statistics.fmean(samples), 6) if samples else None,
                "p50_fill_time_ms": round(percentile(samples, 0.50), 6) if samples else None,
                "p95_fill_time_ms": round(percentile(samples, 0.95), 6) if samples else None,
            }
        )
    return out


def _full_batch_flush_gap(all_events: list[dict], events_by_id: dict[str, dict[str, list[dict]]]) -> tuple[list[dict], list[dict], dict]:
    groups = _flush_groups(all_events)
    gap_rows: list[dict] = []
    gap_values: list[float] = []
    for group in groups.values():
        if group["flush_reason"] != "batch_full":
            continue
        enqueue_times: list[float] = []
        for bench_id in group["bench_ids"]:
            for event in events_by_id.get(bench_id, {}).get("mysql_persist_enqueue_done", []):
                enqueue_times.append(event_time_ms(event))
        if not enqueue_times:
            continue
        gap_ms = max(enqueue_times) - min(enqueue_times)
        gap_values.append(gap_ms)
        gap_rows.append(
            {
                "worker_index": group["worker_index"],
                "flush_seq": group["flush_seq"],
                "flush_batch_size": group["flush_batch_size"],
                "gap_ms": round(gap_ms, 6),
            }
        )
    hist_counter: Counter[float] = Counter()
    for value in gap_values:
        bucket = round(math.floor(value * 10) / 10, 1)
        hist_counter[bucket] += 1
    hist_rows = [{"gap_bucket_ms": key, "count": hist_counter[key]} for key in sorted(hist_counter.keys())]
    summary = {
        "full_batch_flush_count": len(gap_rows),
        "avg_gap_ms": round(statistics.fmean(gap_values), 3) if gap_values else None,
        "p50_gap_ms": round(percentile(gap_values, 0.50), 3) if gap_values else None,
        "p95_gap_ms": round(percentile(gap_values, 0.95), 3) if gap_values else None,
        "p99_gap_ms": round(percentile(gap_values, 0.99), 3) if gap_values else None,
    }
    return gap_rows, hist_rows, summary


def analyze(summary: dict, trace: dict, cfg: dict) -> dict:
    meta_by_id, events_by_id, all_events = _build_meta_and_events(trace)
    stage_metrics: dict[str, list[float]] = defaultdict(list)
    partition_counts: Counter[int] = Counter()
    conversation_dispatch_times: dict[str, list[float]] = defaultdict(list)
    ready_wait_values: list[float] = []
    end_to_end_values: list[float] = []
    enqueue_queue_depths: list[float] = []
    send_second_counts: Counter[int] = Counter()
    read_second_counts: Counter[int] = Counter()
    write_second_counts: Counter[int] = Counter()

    for event in all_events:
        if event.get("event") == "consumer_decode_done":
            partition_counts[int(first_field(event.get("fields"), "partition", 0))] += 1
        if event.get("event") == "mysql_persist_enqueue_done":
            queue_depth = first_field(event.get("fields"), "queue_depth")
            if isinstance(queue_depth, (int, float)):
                enqueue_queue_depths.append(float(queue_depth))

    send_times_ms = [
        float(meta.get("send_ts_ms"))
        for meta in meta_by_id.values()
        if isinstance(meta.get("send_ts_ms"), (int, float))
    ]
    write_times_ms = [
        event_time_ms(event)
        for event in all_events
        if event.get("event") == "ws_write_done"
    ]
    base_time_ms: float | None = summary_time_anchor_ms(summary)
    if not isinstance(summary.get("send_window_start_ms"), (int, float)) and send_times_ms:
        base_time_ms = min(send_times_ms)
    elif base_time_ms is None:
        candidates = [*send_times_ms, *write_times_ms]
        if candidates:
            base_time_ms = min(candidates)

    if base_time_ms is not None:
        for ts_ms in send_times_ms:
            send_second_counts[int(max(0, (ts_ms - base_time_ms) // 1000))] += 1
        for event in all_events:
            if event.get("event") == "ws_read_done":
                read_second_counts[int(max(0, (event_time_ms(event) - base_time_ms) // 1000))] += 1
        for ts_ms in write_times_ms:
            write_second_counts[int(max(0, (ts_ms - base_time_ms) // 1000))] += 1

    for bench_id, meta in meta_by_id.items():
        event_map = events_by_id.get(bench_id, {})
        ws_read = event_map.get("ws_read_done", [None])[0]
        producer_send = event_map.get("kafka_producer_send_start", [None])[0]
        producer_ack = event_map.get("kafka_producer_ack_done", [None])[0]
        consumer_fetch = event_map.get("consumer_fetch_done", [None])[0]
        decode_start = event_map.get("consumer_decode_start", [None])[0]
        decode = event_map.get("consumer_decode_done", [None])[0]
        bucket_enqueue = event_map.get("conversation_bucket_enqueue", [None])[0]
        ready_enqueue = event_map.get("conversation_ready_enqueue", [None])[0]
        ready_dequeue = event_map.get("conversation_ready_dequeue", [None])[0]
        dispatch_start = event_map.get("conversation_dispatch_start", [None])[0]
        dispatch_after_persist = event_map.get("dispatch_after_persist_start", [None])[0]
        persist_enqueue_start = event_map.get("mysql_persist_enqueue_start", [None])[0]
        persist_enqueue = event_map.get("mysql_persist_enqueue_done", [None])[0]
        persist_worker_start = event_map.get("mysql_persist_worker_start", [None])[0]
        batch_collect_start = event_map.get("mysql_persist_batch_collect_start", [None])[0]
        sql_exec_start = event_map.get("mysql_persist_sql_exec_start", [None])[0]
        sql_exec_done = event_map.get("mysql_persist_sql_exec_done", [None])[0]
        persist_flush = event_map.get("mysql_persist_flush_done", [None])[0]
        receiver_queue_enqueue = event_map.get("receiver_queue_enqueue", [None])[0]
        receiver_queue_dequeue = event_map.get("receiver_queue_dequeue", [None])[0]
        ws_write = event_map.get("ws_write_done", [None])[0]
        send_ts_ms = meta.get("send_ts_ms")

        if ws_read and producer_ack:
            stage_metrics["ingress_to_produce_ack_ms"].append(event_time_ms(producer_ack) - event_time_ms(ws_read))
        if producer_ack and consumer_fetch:
            stage_metrics["kafka_queue_wait_ms"].append(event_time_ms(consumer_fetch) - event_time_ms(producer_ack))
        if decode_start and decode:
            stage_metrics["deserialize_ms"].append(event_time_ms(decode) - event_time_ms(decode_start))
        if decode and bucket_enqueue:
            stage_metrics["conversation_bucket_enqueue_ms"].append(event_time_ms(bucket_enqueue) - event_time_ms(decode))
        if bucket_enqueue and dispatch_start:
            value = event_time_ms(dispatch_start) - event_time_ms(bucket_enqueue)
            stage_metrics["conversation_dispatch_queue_wait_ms"].append(value)
            stage_metrics["conversation_bucket_queue_wait_ms"].append(value)
        if ready_enqueue and ready_dequeue:
            value = event_time_ms(ready_dequeue) - event_time_ms(ready_enqueue)
            stage_metrics["conversation_ready_queue_wait_ms"].append(value)
            ready_wait_values.append(value)
        if persist_enqueue_start and persist_enqueue:
            stage_metrics["mysql_persist_enqueue_block_ms"].append(event_time_ms(persist_enqueue) - event_time_ms(persist_enqueue_start))
        if persist_enqueue and persist_worker_start:
            stage_metrics["mysql_persist_worker_queue_wait_ms"].append(event_time_ms(persist_worker_start) - event_time_ms(persist_enqueue))
        if persist_worker_start and batch_collect_start:
            stage_metrics["mysql_persist_batch_collect_wait_ms"].append(event_time_ms(batch_collect_start) - event_time_ms(persist_worker_start))
        if sql_exec_start and sql_exec_done:
            stage_metrics["mysql_persist_sql_exec_ms"].append(event_time_ms(sql_exec_done) - event_time_ms(sql_exec_start))
        if sql_exec_done and persist_flush:
            stage_metrics["mysql_persist_flush_ms"].append(event_time_ms(persist_flush) - event_time_ms(sql_exec_done))
        if persist_enqueue and persist_flush:
            stage_metrics["mysql_persist_ms"].append(event_time_ms(persist_flush) - event_time_ms(persist_enqueue))
        if persist_flush and dispatch_after_persist:
            stage_metrics["dispatch_after_persist_ms"].append(event_time_ms(dispatch_after_persist) - event_time_ms(persist_flush))
        if receiver_queue_enqueue and receiver_queue_dequeue:
            stage_metrics["receiver_queue_wait_ms"].append(event_time_ms(receiver_queue_dequeue) - event_time_ms(receiver_queue_enqueue))
        if persist_flush and ws_write:
            stage_metrics["receiver_ws_write_ms"].append(event_time_ms(ws_write) - event_time_ms(persist_flush))
        if decode and ws_write:
            stage_metrics["server_critical_path_ms"].append(event_time_ms(ws_write) - event_time_ms(decode))
        if send_ts_ms is not None and ws_write:
            end_to_end_values.append(event_time_ms(ws_write) - float(send_ts_ms))
            stage_metrics["end_to_end_ms"].append(event_time_ms(ws_write) - float(send_ts_ms))
        if dispatch_start:
            conversation_key = first_field(dispatch_start.get("fields"), "conversation_key")
            if conversation_key:
                conversation_dispatch_times[str(conversation_key)].append(event_time_ms(dispatch_start))

    cq_gap_values: list[float] = []
    cq_gap_rows: list[dict] = []
    for conversation_key, timestamps in conversation_dispatch_times.items():
        timestamps.sort()
        for left, right in zip(timestamps, timestamps[1:]):
            gap = right - left
            cq_gap_values.append(gap)
            cq_gap_rows.append({"conversation_key": conversation_key, "cq_gap_ms": round(gap, 6)})

    partitions = int(
        first_nested(
            cfg,
            [
                ("kafka", "topic_partitions"),
                ("mainconfig", "topic_partitions"),
                ("kafkaConfig", "topicPartitions"),
            ],
            default=1,
        )
        or 1
    )
    pressure_window_ms = configured_pressure_window_ms(cfg, summary, send_times_ms)
    ready_timeline_rows, ready_timeline_summary = _ready_timeline(
        all_events,
        partitions,
        anchor_ms=base_time_ms,
        window_ms=pressure_window_ms,
        bin_ms=5.0,
    )
    ready_quantiles = [{"quantile": f"p{q}", "wait_ms": round(percentile(ready_wait_values, q / 100), 3) if ready_wait_values else None} for q in range(10, 100, 10)]
    ready_quantiles.extend(
        [{"quantile": f"p{q}", "wait_ms": round(percentile(ready_wait_values, q / 100), 3) if ready_wait_values else None} for q in range(91, 100)]
    )

    flush_gap_rows, flush_hist_rows, flush_gap_summary = _full_batch_flush_gap(all_events, events_by_id)
    unique_flush_rows = _unique_flush_rows(all_events)
    flush_detailed_rows = _flush_detailed_rows(all_events, events_by_id)
    batch_fill_quantiles_rows = _batch_fill_quantiles_rows(flush_detailed_rows)
    batch_fill_curve_rows = _batch_fill_curve_rows(all_events, events_by_id)
    flush_type_time_rows, flush_type_time_summary = _flush_type_time_rows(
        unique_flush_rows,
        anchor_ms=base_time_ms,
        window_ms=pressure_window_ms or 20000.0,
        bin_ms=50.0,
    )
    worker_ingress_window_ms = pressure_window_ms or 20000.0
    worker_ingress_rows, worker_ingress_summary_rows, worker_ingress_summary = _worker_ingress(
        all_events,
        anchor_ms=base_time_ms,
        window_ms=worker_ingress_window_ms,
        bin_ms=1.0,
    )
    worker_persist_rate_rows, worker_persist_rate_summary = _worker_persist_rate_rows(
        all_events,
        flush_detailed_rows,
        anchor_ms=base_time_ms,
        window_ms=worker_ingress_window_ms,
        bin_ms=50.0,
        smooth_window=5,
    )

    session_count = first_nested(
        cfg,
        [
            ("scenario", "session_count"),
            ("mainconfig", "session_count"),
        ],
        default=summary.get("pair_count"),
    )
    target_rate = first_nested(
        cfg,
        [
            ("scenario", "target_rate"),
            ("mainconfig", "target_rate"),
        ],
    )
    if not isinstance(target_rate, (int, float)):
        pair_count = summary.get("pair_count")
        send_interval_ms = summary.get("send_interval_ms")
        if isinstance(pair_count, (int, float)) and isinstance(send_interval_ms, (int, float)) and send_interval_ms > 0:
            target_rate = round(float(pair_count) * 1000.0 / float(send_interval_ms), 3)
    ideal_cq_speed_ms = None
    if isinstance(session_count, (int, float)) and session_count > 0 and isinstance(target_rate, (int, float)) and target_rate > 0:
        ideal_cq_speed_ms = round(float(session_count) * 1000.0 / float(target_rate), 3)

    flush_reason_counts: Counter[str] = Counter()
    flush_worker_counts: Counter[str] = Counter()
    flush_batch_sizes: list[float] = []
    for row in unique_flush_rows:
        flush_reason_counts[str(row["flush_reason"])] += 1
        flush_worker_counts[str(row["worker_index"])] += 1
        batch_size = row.get("flush_batch_size")
        if isinstance(batch_size, (int, float)):
            flush_batch_sizes.append(float(batch_size))

    stage_summary = {name: summarize(values) for name, values in stage_metrics.items()}
    read_write_window_rows: list[dict] = []
    if pressure_window_ms is not None and pressure_window_ms > 0:
        second_count = max(1, int(math.ceil(pressure_window_ms / 1000.0)))
        for second in range(second_count):
            read_write_window_rows.append(
                {
                    "second": second,
                    "send_count": int(send_second_counts.get(second, 0)),
                    "read_count": int(read_second_counts.get(second, 0)),
                    "write_count": int(write_second_counts.get(second, 0)),
                }
            )
    persist_summary = {
        "flush_count": sum(flush_reason_counts.values()),
        "flush_reason_counts": dict(flush_reason_counts),
        "flush_worker_counts": dict(flush_worker_counts),
        "avg_flush_batch_size": round(statistics.fmean(flush_batch_sizes), 3) if flush_batch_sizes else None,
        "avg_enqueue_queue_depth": round(statistics.fmean(enqueue_queue_depths), 3) if enqueue_queue_depths else None,
    }

    total_partition_msgs = sum(partition_counts.values())
    partition_rows = []
    for partition in sorted(partition_counts.keys()):
        count = partition_counts[partition]
        share = count / total_partition_msgs if total_partition_msgs else 0
        partition_rows.append({"partition": partition, "message_count": count, "share": round(share, 6)})

    normalized_summary = dict(summary)
    window_throughput = window_observed_throughput(summary, pressure_window_ms)
    if window_throughput is not None:
        normalized_summary["observed_throughput_msg_per_sec"] = round(window_throughput, 3)
        normalized_summary["window_observed_throughput_msg_per_sec"] = round(window_throughput, 3)

    return {
        "summary": normalized_summary,
        "stage_metrics": stage_summary,
        "persist": persist_summary,
        "partition_rows": partition_rows,
        "cq_gap_rows": cq_gap_rows,
        "cq_gap_summary": {
            **summarize(cq_gap_values),
            "ideal_cq_speed_ms": ideal_cq_speed_ms,
            "ratio_vs_ideal": round((statistics.fmean(cq_gap_values) / ideal_cq_speed_ms), 3) if cq_gap_values and ideal_cq_speed_ms else None,
            "active_conversations": len(conversation_dispatch_times),
        },
        "ready_timeline_rows": ready_timeline_rows,
        "ready_timeline_summary": ready_timeline_summary,
        "ready_wait_quantiles_rows": ready_quantiles,
        "ready_wait_summary": summarize(ready_wait_values),
        "flush_gap_rows": flush_gap_rows,
        "flush_gap_histogram_rows": flush_hist_rows,
        "flush_gap_summary": flush_gap_summary,
        "flush_detailed_rows": flush_detailed_rows,
        "batch_fill_quantiles_rows": batch_fill_quantiles_rows,
        "batch_fill_curve_rows": batch_fill_curve_rows,
        "flush_type_time_rows": flush_type_time_rows,
        "flush_type_time_summary": flush_type_time_summary,
        "unique_flush_rows": unique_flush_rows,
        "worker_ingress_rows": worker_ingress_rows,
        "worker_ingress_summary_rows": worker_ingress_summary_rows,
        "worker_ingress_summary": worker_ingress_summary,
        "worker_persist_rate_rows": worker_persist_rate_rows,
        "worker_persist_rate_summary": worker_persist_rate_summary,
        "per_second_send_counts": dict(sorted(send_second_counts.items())),
        "per_second_read_counts": dict(sorted(read_second_counts.items())),
        "per_second_write_counts": dict(sorted(write_second_counts.items())),
        "read_write_window_rows": read_write_window_rows,
        "time_anchor_ms": round(base_time_ms, 3) if base_time_ms is not None else None,
        "pressure_window_ms": round(pressure_window_ms, 3) if pressure_window_ms is not None else None,
        "trace_counts": {
            "registered_messages": len(trace.get("messages", [])),
            "event_count": len(trace.get("events", [])),
        },
    }


def export_analysis(bundle_dir: Path, analysis: dict) -> None:
    write_json(bundle_dir / "dashboard_summary.json", analysis)
    write_csv(bundle_dir / "dashboard_conversation_gap.csv", ["conversation_key", "cq_gap_ms"], analysis["cq_gap_rows"])
    write_csv(bundle_dir / "dashboard_ready_timeline.csv", ["time_ms", "avg_ready_depth_per_partition", "max_ready_depth_single_partition"], analysis["ready_timeline_rows"])
    write_csv(bundle_dir / "dashboard_ready_wait_quantiles.csv", ["quantile", "wait_ms"], analysis["ready_wait_quantiles_rows"])
    write_csv(bundle_dir / "dashboard_flush_distribution.csv", ["batch_size", "count", "reason"], _flatten_flush_distribution(analysis))
    write_csv(bundle_dir / "dashboard_send_read_write_20s.csv", ["second", "send_count", "read_count", "write_count"], analysis.get("read_write_window_rows", []))
    write_json(bundle_dir / "flush_gap_summary.json", analysis["flush_gap_summary"])
    write_csv(bundle_dir / "flush_gap_full_batch_events.csv", ["worker_index", "flush_seq", "flush_batch_size", "gap_ms"], analysis["flush_gap_rows"])
    write_csv(bundle_dir / "flush_gap_full_batch_histogram.csv", ["gap_bucket_ms", "count"], analysis["flush_gap_histogram_rows"])
    write_csv(bundle_dir / "flush_type_time_series.csv", ["flush_reason", "time_ms", "count"], analysis.get("flush_type_time_rows", []))
    write_csv(bundle_dir / "flush_detailed_rows.csv", ["worker_index", "flush_seq", "flush_reason", "flush_batch_size", "message_count", "enqueue_start_ms", "enqueue_end_ms", "worker_start_ms", "batch_collect_start_ms", "sql_exec_start_ms", "sql_exec_done_ms", "flush_done_at_ms", "formation_wait_ms", "exec_ms", "total_wait_plus_exec_ms", "exec_rate_msgs_per_ms", "effective_rate_msgs_per_ms"], analysis.get("flush_detailed_rows", []))
    write_csv(bundle_dir / "batch_fill_quantiles.csv", ["quantile", "fill_time_ms"], analysis.get("batch_fill_quantiles_rows", []))
    write_csv(bundle_dir / "batch_fill_curve.csv", ["batch_index", "avg_fill_time_ms", "p50_fill_time_ms", "p95_fill_time_ms"], analysis.get("batch_fill_curve_rows", []))
    write_csv(bundle_dir / "worker_ingress_series.csv", ["worker_index", "time_ms", "count"], analysis["worker_ingress_rows"])
    write_csv(bundle_dir / "worker_ingress_summary.csv", ["worker_index", "total_events", "events_in_window", "peak_count_per_bin", "avg_count_per_bin", "non_zero_bins"], analysis["worker_ingress_summary_rows"])
    write_json(bundle_dir / "worker_ingress_summary.json", analysis["worker_ingress_summary"])
    write_csv(bundle_dir / "worker_persist_rate_series.csv", ["worker_index", "time_ms", "ingress_rate_msgs_per_sec", "exec_rate_msgs_per_sec", "effective_rate_msgs_per_sec", "worker_gap_ms"], analysis.get("worker_persist_rate_rows", []))
    write_json(bundle_dir / "worker_persist_rate_summary.json", analysis.get("worker_persist_rate_summary", {}))


def _flatten_flush_distribution(analysis: dict) -> list[dict]:
    counter: dict[tuple[int, str], int] = defaultdict(int)
    for row in analysis.get("unique_flush_rows", []):
        batch_size = int(row.get("flush_batch_size", 0))
        reason = str(row.get("flush_reason", "unknown"))
        counter[(batch_size, reason)] += 1
    rows: list[dict] = []
    for (batch_size, reason), count in sorted(counter.items(), key=lambda item: (item[0][0], item[0][1])):
        rows.append({"batch_size": batch_size, "count": count, "reason": reason})
    return rows


def build_reports(bundle_dir: Path, analysis: dict, cfg: dict) -> None:
    _write_02_stage_visual(bundle_dir, analysis)
    _write_04(bundle_dir, analysis)
    _write_05(bundle_dir, analysis)
    _write_06(bundle_dir, analysis)
    _write_07(bundle_dir, analysis)
    _write_08(bundle_dir, analysis, cfg)
    _write_09(bundle_dir, analysis, cfg)
    _write_10(bundle_dir, analysis, cfg)
    _write_11(bundle_dir, analysis, cfg)
    _write_12(bundle_dir, analysis)
    _write_13(bundle_dir, analysis)
    _write_13b(bundle_dir, analysis)
    _write_14(bundle_dir, analysis)


def _write(path: Path, lines: list[str]) -> None:
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def _write_04(bundle_dir: Path, analysis: dict) -> None:
    persist = analysis["persist"]
    lines = [
        "# worker分配与flush",
        "",
        f"- flush 总次数：`{persist.get('flush_count')}`",
        f"- flush reason：`{json.dumps(persist.get('flush_reason_counts', {}), ensure_ascii=False)}`",
        f"- worker flush 分布：`{json.dumps(persist.get('flush_worker_counts', {}), ensure_ascii=False)}`",
        f"- 平均 flush batch：`{persist.get('avg_flush_batch_size')}`",
        f"- 平均 enqueue queue depth：`{persist.get('avg_enqueue_queue_depth')}`",
    ]
    _write(bundle_dir / "04_worker分配与flush.md", lines)


def _write_05(bundle_dir: Path, analysis: dict) -> None:
    rows = analysis["partition_rows"]
    total = sum(row["message_count"] for row in rows)
    hottest = max(rows, key=lambda row: row["message_count"]) if rows else None
    lines = [
        "# 分区占有情况",
        "",
        f"- active partitions：`{len(rows)}`",
        f"- total consumed messages：`{total}`",
        f"- hottest partition：`{hottest['partition'] if hottest else 'none'}`",
        f"- hottest share：`{round(hottest['share'] * 100, 2) if hottest else 0}%`",
        "",
        "| partition | message_count | share |",
        "| --- | --- | --- |",
    ]
    for row in rows:
        lines.append(f"| {row['partition']} | {row['message_count']} | {row['share']} |")
    _write(bundle_dir / "05_分区占有情况.md", lines)


def _write_06(bundle_dir: Path, analysis: dict) -> None:
    ready = analysis["ready_timeline_summary"]
    lines = [
        "# async_queue情况",
        "",
        "- 本轮未启用 fixed-shard partition_async，以下内容为 conversation bucket ready queue 视角。",
        f"- avg ready depth per partition：`{ready.get('avg_depth')}`",
        f"- p95 ready depth per partition：`{ready.get('p95_depth')}`",
        f"- max ready depth single partition：`{ready.get('max_depth')}`",
        f"- avg ready wait：`{analysis['ready_wait_summary'].get('avg_ms')}ms`",
    ]
    _write(bundle_dir / "06_async_queue情况.md", lines)


def _write_07(bundle_dir: Path, analysis: dict) -> None:
    cq = analysis["cq_gap_summary"]
    ready = analysis["ready_wait_summary"]
    lines = [
        "# conversation_bucket情况",
        "",
        f"- active conversations：`{cq.get('active_conversations')}`",
        f"- Ideal_CQ_speed：`{cq.get('ideal_cq_speed_ms')}ms`",
        f"- avg CQ_gap：`{cq.get('avg_ms')}ms`",
        f"- CQ gap ratio：`{cq.get('ratio_vs_ideal')}`",
        f"- p95 CQ_gap：`{cq.get('p95_ms')}ms`",
        f"- avg ready wait：`{ready.get('avg_ms')}ms`",
        f"- median ready wait：`{ready.get('median_ms')}ms`",
    ]
    _write(bundle_dir / "07_conversation_bucket情况.md", lines)


def _write_08(bundle_dir: Path, analysis: dict, cfg: dict) -> None:
    if not cfg.get("partition_async", {}).get("enabled", False):
        lines = [
            "# shard_gap情况",
            "",
            "- 本轮未启用 partition_async shard 模式。",
            f"- 作为替代口径，当前会话桶 CQ gap ratio：`{analysis['cq_gap_summary'].get('ratio_vs_ideal')}`",
        ]
    else:
        lines = ["# shard_gap情况", "", "- 当前仓库尚未恢复 fixed-shard 详细 gap 聚合。"]
    _write(bundle_dir / "08_shard_gap情况.md", lines)


def _write_09(bundle_dir: Path, analysis: dict, cfg: dict) -> None:
    lines = [
        "# 消费者情况",
        "",
        f"- consumer 实例数：`{cfg.get('consumers', {}).get('count')}`",
        f"- 注册 benchmark messages：`{analysis['trace_counts'].get('registered_messages')}`",
        f"- trace event 数：`{analysis['trace_counts'].get('event_count')}`",
        f"- active partitions：`{len(analysis['partition_rows'])}`",
    ]
    _write(bundle_dir / "09_消费者情况.md", lines)


def _write_10(bundle_dir: Path, analysis: dict, cfg: dict) -> None:
    summary = analysis["summary"]
    cq_ratio = analysis["cq_gap_summary"].get("ratio_vs_ideal")
    ready_avg = analysis["ready_wait_summary"].get("avg_ms")
    persist_avg = analysis["stage_metrics"].get("mysql_persist_ms", {}).get("avg_ms")
    p95_ms = first_field(summary.get("latency"), "p95_ms")
    expected_messages = summary.get("expected_messages")
    received_before_drain = summary.get("received_messages_before_drain")
    in_window_coverage = (
        round(float(received_before_drain) / float(expected_messages), 3)
        if isinstance(received_before_drain, (int, float)) and isinstance(expected_messages, (int, float)) and expected_messages > 0
        else None
    )
    lines = [
        "# 汇总情况分析",
        "",
        f"- CQ gap ratio：`{cq_ratio}`",
        f"- 窗口内写回率：`{in_window_coverage}`",
        f"- p95 end_to_end：`{p95_ms}ms`",
        f"- avg ready wait：`{ready_avg}ms`",
        f"- avg mysql_persist：`{persist_avg}ms`",
    ]
    if cq_ratio is not None or in_window_coverage is not None or p95_ms is not None:
        if (
            in_window_coverage is not None
            and in_window_coverage >= 0.99
            and (cq_ratio is None or cq_ratio <= 1.05)
            and (p95_ms is None or p95_ms <= 150)
        ):
            lines.append("- 当前档位在发送窗口内基本完成写回，调度与持久化保持稳定，主链路处于健康区。")
        elif (
            in_window_coverage is not None
            and in_window_coverage >= 0.95
            and (cq_ratio is None or cq_ratio <= 1.2)
            and (p95_ms is None or p95_ms <= 250)
        ):
            lines.append("- 当前档位已接近能力边界；窗口内存在轻微拖尾，但仍属可控积压区。")
        else:
            lines.append("- 当前档位已明显偏离目标进流；需优先结合窗口内写回率、p95 与队列等待判断主瓶颈。")
    _write(bundle_dir / "10_汇总情况分析.md", lines)


def _plot_text_only(ax, title: str, lines: list[str]) -> None:
    ax.axis("off")
    ax.set_facecolor(PANEL_ALT)
    ax.set_title(title, loc="left", fontsize=12, pad=12)
    ax.text(0.03, 0.97, "\n".join(lines), va="top", ha="left", fontsize=10, color=TEXT)


def _write_11(bundle_dir: Path, analysis: dict, cfg: dict) -> None:
    fig, axes = plt.subplots(2, 4, figsize=(22, 11))
    fig.patch.set_facecolor(FIG_BG)
    axes = axes.flatten()
    summary = analysis["summary"]
    time_anchor_ms = analysis.get("time_anchor_ms")
    pressure_window_ms = analysis.get("pressure_window_ms")

    expected = summary.get("expected_messages", 0) or 0
    received_before = summary.get("received_messages_before_drain", summary.get("received_messages", 0)) or 0
    recovered = summary.get("drain_recovered_messages")
    if recovered is None:
        recovered = (summary.get("received_messages", 0) or 0) - received_before
    throughput_title = plot_text("吞吐总览", "Throughput Overview")
    throughput_subtitle = plot_text("窗口内写回与拖尾补完", "In-window delivery and trailing recovery")
    setup_axis(axes[0], title=throughput_title, subtitle=throughput_subtitle)
    axes[0].bar([0], [received_before], width=0.6, color=GREEN, edgecolor=PANEL_BG, linewidth=1.2)
    axes[0].bar([0], [recovered], bottom=[received_before], width=0.6, color=RED, edgecolor=PANEL_BG, linewidth=1.2)
    axes[0].set_xticks([0], [plot_text("消息", "Messages")])
    axes[0].yaxis.set_major_formatter(human_count_axis())
    axes[0].set_ylim(0, max(expected, received_before + recovered) * 1.12 if expected else max(received_before + recovered, 1))
    axes[0].axhline(expected, color=GRID, linestyle="--", linewidth=1.0)
    axes[0].text(0, received_before / 2 if received_before else 0, nice_number(received_before, 0), ha="center", va="center", color="white", fontsize=10, fontweight="bold")
    if recovered:
        axes[0].text(0, received_before + recovered / 2, nice_number(recovered, 0), ha="center", va="center", color="white", fontsize=10, fontweight="bold")
    add_metric_card(
        axes[0],
        [
            f"expected {nice_number(expected, 0)}",
            f"window {nice_number(summary.get('duration_sec'))}s",
            f"observed(window) {nice_number(summary.get('observed_throughput_msg_per_sec'))} msg/s",
            f"p95 {nice_number(first_field(summary.get('latency'), 'p95_ms'))}ms",
            f"p99 {nice_number(first_field(summary.get('latency'), 'p99_ms'))}ms",
        ],
    )

    cq_gaps = [row["cq_gap_ms"] for row in analysis["cq_gap_rows"]]
    if cq_gaps:
        bins = min(30, max(8, int(len(cq_gaps) ** 0.5)))
        axes[1].hist(cq_gaps, bins=bins, color=BLUE, alpha=0.9, edgecolor=PANEL_BG, linewidth=0.8)
        ideal = analysis["cq_gap_summary"].get("ideal_cq_speed_ms")
        if ideal is not None:
            axes[1].axvline(ideal, color=ORANGE, linestyle="--", linewidth=1.4)
    setup_axis(
        axes[1],
        title=plot_text("CQ gap 分布", "CQ Gap Distribution"),
        subtitle=plot_text("会话调度间隔密度", "Conversation dispatch cadence"),
        xlabel="ms",
        ylabel=plot_text("频次", "Count"),
    )
    add_metric_card(
        axes[1],
        [
            f"ideal {nice_number(analysis['cq_gap_summary'].get('ideal_cq_speed_ms'))}ms",
            f"avg {nice_number(analysis['cq_gap_summary'].get('avg_ms'))}ms",
            f"ratio {nice_number(analysis['cq_gap_summary'].get('ratio_vs_ideal'))}",
            f"p95 {nice_number(analysis['cq_gap_summary'].get('p95_ms'))}ms",
        ],
    )

    timeline = analysis["ready_timeline_rows"]
    if timeline:
        xs = [row["time_ms"] for row in timeline]
        ys = [row["avg_ready_depth_per_partition"] for row in timeline]
        axes[2].fill_between(xs, ys, color=BLUE_LIGHT, alpha=0.35)
        axes[2].plot(xs, ys, linewidth=1.8, color=BLUE)
    ready_title = plot_text("ready 队列时序", "Ready Queue Timeline")
    if pressure_window_ms is not None:
        ready_subtitle = f"anchor={round(time_anchor_ms, 3) if time_anchor_ms is not None else 'n/a'}  window={round(pressure_window_ms, 3)}ms"
    else:
        ready_subtitle = None
    setup_axis(axes[2], title=ready_title, subtitle=ready_subtitle, xlabel="time_ms", ylabel=plot_text("平均深度", "Avg Depth"))
    add_metric_card(
        axes[2],
        [
            f"avg {nice_number(analysis['ready_timeline_summary'].get('avg_depth'))}",
            f"p95 {nice_number(analysis['ready_timeline_summary'].get('p95_depth'))}",
            f"max {nice_number(analysis['ready_timeline_summary'].get('max_depth'))}",
        ],
    )

    quantile_rows = analysis["ready_wait_quantiles_rows"]
    if quantile_rows:
        qx = [row["quantile"] for row in quantile_rows]
        qy = [row["wait_ms"] or 0 for row in quantile_rows]
        axes[3].plot(qx, qy, linewidth=1.8, color=PURPLE, marker="o", markersize=2.2)
        axes[3].tick_params(axis="x", rotation=60)
    setup_axis(axes[3], title=plot_text("ready 调度分位", "Ready Wait Quantiles"), subtitle=plot_text("尾部抬升最敏感", "Tail growth is the signal"), ylabel="ms")
    add_metric_card(
        axes[3],
        [
            f"avg {nice_number(analysis['ready_wait_summary'].get('avg_ms'))}ms",
            f"median {nice_number(analysis['ready_wait_summary'].get('median_ms'))}ms",
            f"p95 {nice_number(analysis['ready_wait_summary'].get('p95_ms'))}ms",
            f"p99 {nice_number(analysis['ready_wait_summary'].get('p99_ms'))}ms",
        ],
    )

    stage_names = ["conversation_dispatch_queue_wait_ms", "conversation_ready_queue_wait_ms", "mysql_persist_ms", "receiver_ws_write_ms"]
    stage_values = [analysis["stage_metrics"].get(name, {}).get("avg_ms") or 0 for name in stage_names]
    stage_colors = [BLUE, PURPLE, TEAL, GOLD]
    axes[4].bar(range(len(stage_names)), stage_values, color=stage_colors, width=0.62, edgecolor=PANEL_BG, linewidth=1.2)
    axes[4].set_xticks(range(len(stage_names)), ["dispatch", "ready", "persist", "ws"], rotation=20)
    setup_axis(axes[4], title=plot_text("主阶段均值", "Primary Stage Means"), subtitle=plot_text("看谁在撑高主路径", "Which stage is stretching the path"), ylabel="ms")
    for idx, value in enumerate(stage_values):
        axes[4].text(idx, value, nice_number(value), ha="center", va="bottom", fontsize=8, color=TEXT)

    flush_reason_counter: dict[str, Counter[int]] = defaultdict(Counter)
    for row in analysis.get("unique_flush_rows", []):
        batch_size = int(row.get("flush_batch_size", 0))
        reason = str(row.get("flush_reason", "unknown"))
        flush_reason_counter[reason][batch_size] += 1
    if flush_reason_counter:
        sizes = sorted({size for counter in flush_reason_counter.values() for size in counter.keys()})
        full_counts = [flush_reason_counter.get("batch_full", Counter()).get(size, 0) for size in sizes]
        timer_counts = [flush_reason_counter.get("timer", Counter()).get(size, 0) for size in sizes]
        single_counts = [flush_reason_counter.get("single", Counter()).get(size, 0) for size in sizes]
        axes[5].bar(sizes, full_counts, color=BLUE, width=0.9, label="full batch")
        axes[5].bar(sizes, timer_counts, bottom=full_counts, color=GREEN, width=0.9, label="timer")
        axes[5].bar(
            sizes,
            single_counts,
            bottom=[f + t for f, t in zip(full_counts, timer_counts)],
            color=GOLD,
            width=0.9,
            label="single",
        )
        axes[5].legend(fontsize=8, frameon=False)
    setup_axis(axes[5], title=plot_text("flush 分布", "Flush Distribution"), subtitle=plot_text("batch 是否吃满", "Whether batches saturate"), xlabel="batch size", ylabel=plot_text("次数", "Count"))

    all_stage_names = []
    all_stage_values = []
    for name, summary_row in analysis["stage_metrics"].items():
        avg = summary_row.get("avg_ms")
        if avg is None:
            continue
        all_stage_names.append(name.replace("_ms", ""))
        all_stage_values.append(avg)
    axes[6].bar(range(len(all_stage_names)), all_stage_values, color=TEAL, width=0.7, edgecolor=PANEL_BG, linewidth=1.0)
    axes[6].set_xticks(range(len(all_stage_names)), all_stage_names, rotation=50, ha="right")
    setup_axis(axes[6], title=plot_text("全阶段耗时", "Stage Latency"), subtitle=plot_text("均值视角", "Average-stage view"), ylabel="ms")

    sends = _per_second_counts_from_summary_and_trace(analysis, use="send")
    reads = _per_second_counts_from_summary_and_trace(analysis, use="read")
    writes = _per_second_counts_from_summary_and_trace(analysis, use="write")
    seconds = sorted(set(sends.keys()) | set(reads.keys()) | set(writes.keys()))
    send_vals = [sends.get(sec, 0) for sec in seconds]
    read_vals = [reads.get(sec, 0) for sec in seconds]
    write_vals = [writes.get(sec, 0) for sec in seconds]
    axes[7].plot(seconds, send_vals, label="send", linewidth=2.0, color=SLATE)
    axes[7].plot(seconds, read_vals, label="read", linewidth=2.0, color=BLUE)
    axes[7].plot(seconds, write_vals, label="write", linewidth=2.0, color=ORANGE)
    axes[7].fill_between(seconds, write_vals, send_vals, where=[w < s for w, s in zip(write_vals, send_vals)], color=RED, alpha=0.12)
    axes[7].legend(frameon=False, ncol=3, loc="upper left")
    rw_title = plot_text("每秒读写", "Per-Second Read/Write")
    if pressure_window_ms is not None:
        rw_subtitle = f"anchor={round(time_anchor_ms, 3) if time_anchor_ms is not None else 'n/a'}  window={round(pressure_window_ms, 3)}ms"
    else:
        rw_subtitle = None
    setup_axis(axes[7], title=rw_title, subtitle=rw_subtitle, xlabel=plot_text("秒", "Second"), ylabel=plot_text("消息数", "Messages"))
    axes[7].yaxis.set_major_formatter(human_count_axis())
    add_metric_card(
        axes[7],
        [
            f"in-window {nice_number(received_before / expected if expected else None)}",
            f"drain {nice_number(recovered, 0)}",
            f"success {nice_number(summary.get('delivery_success_rate'))}",
        ],
        loc=(0.98, 0.26),
    )

    fig.suptitle(plot_text("单聊压测可视化总览", "Single-Chat Pressure Test Dashboard"), x=0.02, ha="left", fontsize=18, fontweight="bold", color=TEXT)
    fig.tight_layout(rect=(0, 0, 1, 0.97))
    save_figure(fig, bundle_dir / "11_可视化总览.png")
    plt.close(fig)

    _write(
        bundle_dir / "11_可视化总览.md",
        [
            "# 可视化总览",
            "",
            f"- 图文件：`11_可视化总览.png`",
            f"- time_anchor_ms：`{analysis.get('time_anchor_ms')}`",
            f"- pressure_window_ms：`{analysis.get('pressure_window_ms')}`",
            f"- CQ gap ratio：`{analysis['cq_gap_summary'].get('ratio_vs_ideal')}`",
            f"- avg ready wait：`{analysis['ready_wait_summary'].get('avg_ms')}ms`",
            f"- avg mysql_persist：`{analysis['stage_metrics'].get('mysql_persist_ms', {}).get('avg_ms')}ms`",
        ],
    )


def _per_second_counts_from_summary_and_trace(analysis: dict, use: str) -> dict[int, int]:
    if use == "send":
        key = "per_second_send_counts"
    elif use == "read":
        key = "per_second_read_counts"
    else:
        key = "per_second_write_counts"
    raw = analysis.get(key, {})
    return {int(k): int(v) for k, v in raw.items()}


def _write_02_stage_visual(bundle_dir: Path, analysis: dict) -> None:
    ordered = [
        "conversation_bucket_enqueue_ms",
        "conversation_dispatch_queue_wait_ms",
        "conversation_bucket_queue_wait_ms",
        "conversation_ready_queue_wait_ms",
        "mysql_persist_ms",
        "receiver_ws_write_ms",
        "server_critical_path_ms",
        "end_to_end_ms",
    ]
    labels: list[str] = []
    p50s: list[float] = []
    p95s: list[float] = []
    p99s: list[float] = []
    avgs: list[float] = []
    maxs: list[float] = []
    for name in ordered:
        row = analysis.get("stage_metrics", {}).get(name, {})
        if not row or row.get("count", 0) == 0:
            continue
        labels.append(name.replace("_ms", ""))
        p50s.append(float(row.get("p50_ms") or 0.001))
        p95s.append(float(row.get("p95_ms") or 0.001))
        p99s.append(float(row.get("p99_ms") or 0.001))
        avgs.append(float(row.get("avg_ms") or 0.001))
        maxs.append(float(row.get("max_ms") or 0.001))
    if not labels:
        return
    fig, ax = plt.subplots(figsize=(14, 6.5))
    fig.patch.set_facecolor(FIG_BG)
    ys = list(range(len(labels)))
    for idx, y in enumerate(ys):
        ax.hlines(y, max(0.001, p50s[idx]), max(0.001, p99s[idx]), color=BLUE_LIGHT, linewidth=8, alpha=0.85)
        ax.hlines(y, max(0.001, p95s[idx]), max(0.001, maxs[idx]), color=ROSE, linewidth=2.2, alpha=0.7)
        ax.scatter(avgs[idx], y, color=BLUE, s=46, label="avg" if idx == 0 else "", zorder=3)
        ax.scatter(p50s[idx], y, color=ORANGE, s=34, label="p50" if idx == 0 else "", zorder=3)
        ax.scatter(p95s[idx], y, color=GREEN, s=34, label="p95" if idx == 0 else "", zorder=3)
        ax.scatter(p99s[idx], y, color=RED, s=34, label="p99" if idx == 0 else "", zorder=3)
        ax.scatter(maxs[idx], y, color=PURPLE, marker="x", s=42, label="max" if idx == 0 else "", zorder=3)
        value_text = (
            f"avg {nice_number(avgs[idx])}  "
            f"p50 {nice_number(p50s[idx])}  "
            f"p95 {nice_number(p95s[idx])}  "
            f"p99 {nice_number(p99s[idx])}  "
            f"max {nice_number(maxs[idx])}"
        )
        ax.text(
            max(0.001, maxs[idx]) * 1.12,
            y,
            value_text,
            va="center",
            ha="left",
            fontsize=8.5,
            color=TEXT,
        )
    ax.set_yticks(ys, labels)
    ax.set_xscale("log")
    setup_axis(ax, title=plot_text("全链路阶段分布", "Stage Latency Distribution"), subtitle=plot_text("粗条为 p50~p99，细线延伸到 max", "Thick bar = p50~p99, thin tail = to max"), xlabel="latency_ms (log scale)")
    ax.legend(loc="lower right", fontsize=8, frameon=False, ncol=5)
    ax.set_xlim(left=0.0008, right=max(10, max(maxs) * 4.5))
    fig.tight_layout()
    save_figure(fig, bundle_dir / "02_全链路分段.png")
    plt.close(fig)


def _write_12(bundle_dir: Path, analysis: dict) -> None:
    hist_rows = analysis["flush_gap_histogram_rows"]
    summary = analysis["flush_gap_summary"]
    full_batch_rows = analysis["flush_gap_rows"]
    flush_type_rows = analysis.get("flush_type_time_rows", [])
    flush_type_summary = analysis.get("flush_type_time_summary", {})

    fig = plt.figure(figsize=(12.2, 10.0))
    fig.patch.set_facecolor(FIG_BG)
    outer = fig.add_gridspec(3, 1, height_ratios=[1.35, 1.0, 1.15], hspace=0.42)
    ax_top = fig.add_subplot(outer[0])
    ax_bottom = fig.add_subplot(outer[1])
    third = outer[2].subgridspec(1, 3, wspace=0.18)
    ax_timer = fig.add_subplot(third[0])
    ax_batch = fig.add_subplot(third[1])
    ax_single = fig.add_subplot(third[2])

    if hist_rows:
        xs = [row["gap_bucket_ms"] for row in hist_rows]
        ys = [row["count"] for row in hist_rows]
        ax_top.bar(xs, ys, width=0.12, color=BLUE, edgecolor=PANEL_BG, linewidth=0.9)
        ax_top.plot(xs, ys, color=BLUE_LIGHT, linewidth=1.6)
        avg_gap = summary.get("avg_gap_ms")
        main_xlim = 15.0
        if isinstance(avg_gap, (int, float)) and avg_gap > 15:
            main_xlim = max(15.0, min(max(xs) if xs else 15.0, float(avg_gap) * 2.0))
        ax_top.set_xlim(0, main_xlim)
    setup_axis(
        ax_top,
        title=plot_text("full batch flush gap 分布", "Full-Batch Flush Gap Distribution"),
        subtitle=plot_text("主图默认聚焦 15ms 内，avg 超过 15ms 时自适应放宽", "Focus under 15ms by default; widen when avg exceeds 15ms"),
        xlabel="gap_ms",
        ylabel=plot_text("次数", "Count"),
    )
    add_metric_card(
        ax_top,
        [
            f"flush {table_value(summary.get('full_batch_flush_count'))}",
            f"avg {table_value(summary.get('avg_gap_ms'))}ms",
            f"p50 {table_value(summary.get('p50_gap_ms'))}ms",
            f"p95 {table_value(summary.get('p95_gap_ms'))}ms",
            f"p99 {table_value(summary.get('p99_gap_ms'))}ms",
        ],
    )

    if full_batch_rows:
        gap_values = [float(row["gap_ms"]) for row in full_batch_rows]
        quantiles = list(range(10, 100, 10)) + list(range(90, 101, 1))
        quantile_values = [percentile(gap_values, q / 100.0) for q in quantiles]
        xs_q = [q for q, value in zip(quantiles, quantile_values) if value is not None]
        ys_q = [float(value) for value in quantile_values if value is not None]
        ax_bottom.plot(xs_q, ys_q, color=PURPLE, linewidth=1.8)
        low_xs = [q for q in xs_q if q <= 90]
        low_ys = [y for q, y in zip(xs_q, ys_q) if q <= 90]
        high_xs = [q for q in xs_q if q >= 90]
        high_ys = [y for q, y in zip(xs_q, ys_q) if q >= 90]
        if low_xs:
            ax_bottom.scatter(low_xs, low_ys, color=BLUE, s=22, zorder=3)
        if high_xs:
            ax_bottom.scatter(high_xs, high_ys, color=RED, s=30, zorder=4)
        low_label_offsets = [(0, 12), (0, -14), (0, 18), (0, -20), (0, 24), (0, -26), (0, 30), (0, -32), (0, 36)]
        high_label_offsets = [(-18, 16), (18, -18), (-22, 24), (22, -26), (-26, 32), (26, -34), (-30, 40), (30, -42), (-34, 48), (34, -50), (0, 56)]
        low_idx = 0
        high_idx = 0
        for q, y in zip(xs_q, ys_q):
            is_high = q >= 90
            color = RED if is_high else SUBTEXT
            offsets = high_label_offsets if is_high else low_label_offsets
            offset_index = high_idx if is_high else low_idx
            dx, dy = offsets[offset_index % len(offsets)]
            if is_high:
                high_idx += 1
            else:
                low_idx += 1
            ax_bottom.annotate(
                nice_number(y),
                xy=(q, y),
                xytext=(dx, dy),
                textcoords="offset points",
                fontsize=7.5,
                color=color,
                ha="center",
                va="center",
                arrowprops={
                    "arrowstyle": "->",
                    "color": color,
                    "lw": 0.7,
                    "alpha": 0.7,
                    "shrinkA": 2,
                    "shrinkB": 2,
                },
                bbox={
                    "boxstyle": "round,pad=0.18",
                    "facecolor": PANEL_BG,
                    "edgecolor": GRID,
                    "alpha": 0.92,
                },
            )
        ax_bottom.set_xticks(list(range(10, 100, 10)) + list(range(91, 101, 1)))
        ax_bottom.set_xlim(10, 100)
        if ys_q:
            y_min = min(ys_q)
            y_max = max(ys_q)
            padding = max(0.25, (y_max - y_min) * 0.18)
            ax_bottom.set_ylim(max(0, y_min - padding), y_max + padding)
    setup_axis(
        ax_bottom,
        title=plot_text("flush gap 分位曲线", "Flush Gap Quantile Curve"),
        subtitle=plot_text("p10~p90 每 10 一个点，p90~p100 每 1 一个点；90 以上红色描点高亮", "p10~p90 step 10, p90~p100 step 1; 90+ highlighted in red"),
        xlabel=plot_text("分位点", "Percentile"),
        ylabel="gap_ms",
    )

    reason_to_ax = {
        "timer": (ax_timer, ORANGE, plot_text("timer flush 时间分布", "Timer Flush Distribution")),
        "batch_full": (ax_batch, GREEN, plot_text("full batch 时间分布", "Full-Batch Distribution")),
        "single": (ax_single, RED, plot_text("single flush 时间分布", "Single Flush Distribution")),
    }
    grouped = defaultdict(list)
    for row in flush_type_rows:
        grouped[str(row.get("flush_reason"))].append(row)
    for reason in ["timer", "batch_full", "single"]:
        ax, color, title = reason_to_ax[reason]
        rows = sorted(grouped.get(reason, []), key=lambda item: item["time_ms"])
        xs = [row["time_ms"] for row in rows]
        ys = [row["count"] for row in rows]
        max_y = max(ys) if ys else 0
        ax.fill_between(xs, ys, 0, step="mid", color=color, alpha=0.14)
        ax.step(xs, ys, where="mid", color=color, linewidth=1.05, alpha=0.98)
        ax.axhline(0, color=GRID, linewidth=0.9, linestyle="--", alpha=0.8, zorder=0)
        setup_axis(
            ax,
            title=title,
            subtitle=(
                f"50ms桶  anchor={flush_type_summary.get('time_anchor_ms')}  window={flush_type_summary.get('window_ms')}ms"
                if flush_type_summary
                else None
            ),
            xlabel="time_ms",
            ylabel=plot_text("次数", "Count"),
        )
        ax.set_xlim(0, flush_type_summary.get("window_ms") or 20000)
        lower_pad = 0.18 if max_y <= 1 else max_y * 0.10
        upper_pad = 0.35 if max_y <= 1 else max_y * 0.22
        ax.set_ylim(-lower_pad, max_y + upper_pad if max_y > 0 else 1.0)
        ax.yaxis.set_major_formatter(human_count_axis())
        add_metric_card(
            ax,
            [
                f"total {nice_number(sum(ys), 0)}",
                f"peak {nice_number(max_y, 0)}",
            ],
            loc=(0.98, 0.95),
        )

    save_figure(fig, bundle_dir / "12_flush_gap分析.png")
    plt.close(fig)
    _write(
        bundle_dir / "12_flush_gap分析.md",
        [
            "# flush_gap分析",
            "",
            f"- full batch flush 数：`{summary.get('full_batch_flush_count')}`",
            f"- avg gap：`{summary.get('avg_gap_ms')}ms`",
            f"- p50：`{summary.get('p50_gap_ms')}ms`",
            f"- p95：`{summary.get('p95_gap_ms')}ms`",
            f"- p99：`{summary.get('p99_gap_ms')}ms`",
            f"- flush 类型时间分布桶：`{flush_type_summary.get('bin_ms')}`ms",
        ],
    )


def _write_13(bundle_dir: Path, analysis: dict) -> None:
    output_dir = bundle_dir / "13_worker_ingress速率"
    output_dir.mkdir(parents=True, exist_ok=True)

    grouped_ingress: dict[int, list[dict]] = defaultdict(list)
    for row in analysis["worker_ingress_rows"]:
        grouped_ingress[int(row["worker_index"])].append(row)

    grouped_rate: dict[int, list[dict]] = defaultdict(list)
    for row in analysis.get("worker_persist_rate_rows", []):
        grouped_rate[int(row["worker_index"])].append(row)

    workers = sorted(set(grouped_ingress) | set(grouped_rate))
    summary = analysis.get("worker_persist_rate_summary", {})

    for worker_index in workers:
        rate_rows = sorted(grouped_rate.get(worker_index, []), key=lambda item: item["time_ms"])

        fig, (ax_top, ax_bottom) = plt.subplots(
            2,
            1,
            figsize=(13.2, 5.6),
            sharex=True,
            gridspec_kw={"height_ratios": [2.0, 1.0], "hspace": 0.18},
        )
        fig.patch.set_facecolor(FIG_BG)

        xs = [row["time_ms"] for row in rate_rows]
        ingress_series = [row.get("ingress_rate_msgs_per_sec") for row in rate_rows]
        exec_series = [row.get("exec_rate_msgs_per_sec") for row in rate_rows]
        effective_series = [row.get("effective_rate_msgs_per_sec") for row in rate_rows]
        gap_series = [row.get("worker_gap_ms") for row in rate_rows]

        def compact(values):
            ys = []
            for value in values:
                ys.append(float(value) if isinstance(value, (int, float)) else math.nan)
            return ys

        ax_top.plot(xs, compact(ingress_series), linewidth=1.35, color=BLUE, alpha=0.95, label="ingress rate")
        ax_top.plot(xs, compact(exec_series), linewidth=1.25, color=GREEN, alpha=0.95, label="flush exec rate")
        ax_top.plot(xs, compact(effective_series), linewidth=1.25, color=ORANGE, alpha=0.95, label="effective flush rate")
        ax_bottom.plot(xs, compact(gap_series), linewidth=1.2, color=RED, alpha=0.9, label="worker gap")

        title = f"worker {worker_index} ingress vs flush capacity"
        subtitle = f"anchor={summary.get('time_anchor_ms')}  window={summary.get('window_ms')}ms  bin={summary.get('bin_ms')}ms  smooth={summary.get('smooth_window')}"
        setup_axis(ax_top, title=title, subtitle=subtitle, xlabel=None, ylabel=plot_text("速率 (msg/s)", "Rate (msg/s)"))
        setup_axis(ax_bottom, title=plot_text("worker gap", "Worker Gap"), subtitle=plot_text("相邻两次 worker 开始处理之间的间隔", "Interval between consecutive worker starts"), xlabel="time_ms", ylabel="gap_ms")
        ax_top.yaxis.set_major_formatter(human_count_axis())
        ax_top.set_xlim(0, summary.get("window_ms") or 20000)

        ax_top.legend(loc="upper left", fontsize=8, frameon=False, ncol=3)
        ax_bottom.legend(loc="upper left", fontsize=8, frameon=False)

        ingress_peaks = [value for value in ingress_series if isinstance(value, (int, float))]
        exec_peaks = [value for value in exec_series if isinstance(value, (int, float))]
        effective_peaks = [value for value in effective_series if isinstance(value, (int, float))]
        gap_vals = [value for value in gap_series if isinstance(value, (int, float))]
        add_metric_card(
            ax_top,
            [
                f"ingress peak {nice_number(max(ingress_peaks) if ingress_peaks else None)}",
                f"exec peak {nice_number(max(exec_peaks) if exec_peaks else None)}",
                f"effective peak {nice_number(max(effective_peaks) if effective_peaks else None)}",
            ],
            loc=(0.985, 0.97),
        )
        add_metric_card(
            ax_bottom,
            [
                f"gap avg {nice_number(statistics.fmean(gap_vals) if gap_vals else None)}ms",
                f"gap max {nice_number(max(gap_vals) if gap_vals else None)}ms",
            ],
            loc=(0.985, 0.90),
        )

        save_figure(fig, output_dir / f"worker_{worker_index:02d}.png")
        plt.close(fig)

    _write(
        bundle_dir / "13_worker_ingress速率.md",
        [
            "# worker_ingress速率",
            "",
            f"- worker 数：`{analysis['worker_ingress_summary'].get('worker_count')}`",
            f"- window_ms：`{summary.get('window_ms')}`",
            f"- ingress 原始 bin_ms：`{analysis['worker_ingress_summary'].get('bin_ms')}`",
            f"- 速率图 bin_ms：`{summary.get('bin_ms')}`",
            f"- smooth_window：`{summary.get('smooth_window')}`",
            f"- time_anchor_ms：`{summary.get('time_anchor_ms')}`",
            f"- 图目录：`13_worker_ingress速率/`",
        ],
    )


def _write_13b(bundle_dir: Path, analysis: dict) -> None:
    quant_rows = analysis.get("batch_fill_quantiles_rows", [])
    curve_rows = analysis.get("batch_fill_curve_rows", [])
    if not quant_rows and not curve_rows:
        return

    fig, (ax_top, ax_bottom) = plt.subplots(2, 1, figsize=(11.6, 7.2), gridspec_kw={"height_ratios": [1.0, 1.15], "hspace": 0.24})
    fig.patch.set_facecolor(FIG_BG)

    if quant_rows:
        qs = [int(row["quantile"]) for row in quant_rows if row.get("fill_time_ms") is not None]
        ys = [float(row["fill_time_ms"]) for row in quant_rows if row.get("fill_time_ms") is not None]
        ax_top.plot(qs, ys, color=PURPLE, linewidth=1.4)
        low_qs = [q for q in qs if q <= 90]
        low_ys = [y for q, y in zip(qs, ys) if q <= 90]
        high_qs = [q for q in qs if q >= 90]
        high_ys = [y for q, y in zip(qs, ys) if q >= 90]
        if low_qs:
            ax_top.scatter(low_qs, low_ys, color=BLUE, s=18, zorder=3)
        if high_qs:
            ax_top.scatter(high_qs, high_ys, color=RED, s=22, zorder=4)
        setup_axis(
            ax_top,
            title=plot_text("攒满一批时间分位曲线", "Batch-Fill Time Quantiles"),
            subtitle=plot_text("p10~p90 每 10 一个点，p90~p99 每 1 一个点", "p10~p90 step 10, p90~p99 step 1"),
            xlabel=plot_text("分位点", "Percentile"),
            ylabel="fill_time_ms",
        )
        ax_top.set_xlim(10, 99)
        if ys:
            add_metric_card(ax_top, [f"avg {nice_number(statistics.fmean(ys))}ms", f"p50 {nice_number(percentile(ys, 0.50))}ms", f"p95 {nice_number(percentile(ys, 0.95))}ms", f"p99 {nice_number(percentile(ys, 0.99))}ms"])

    if curve_rows:
        xs = [int(row["batch_index"]) for row in curve_rows if row.get("avg_fill_time_ms") is not None]
        avg_ys = [float(row["avg_fill_time_ms"]) for row in curve_rows if row.get("avg_fill_time_ms") is not None]
        p50_ys = [float(row["p50_fill_time_ms"]) for row in curve_rows if row.get("p50_fill_time_ms") is not None]
        p95_ys = [float(row["p95_fill_time_ms"]) for row in curve_rows if row.get("p95_fill_time_ms") is not None]
        ax_bottom.plot(xs, avg_ys, color=BLUE, linewidth=1.35, alpha=0.95, label="avg")
        ax_bottom.plot(xs, p50_ys, color=GREEN, linewidth=1.2, alpha=0.92, label="p50")
        ax_bottom.plot(xs, p95_ys, color=ORANGE, linewidth=1.2, alpha=0.92, label="p95")
        setup_axis(
            ax_bottom,
            title=plot_text("批次形成过程曲线", "Batch-Fill Formation Curve"),
            subtitle=plot_text("横轴是从第 1 条到 batch_size，每个数量节点平均需要的形成时间", "Time needed to reach each fill count from 1 to batch size"),
            xlabel="batch_index",
            ylabel="fill_time_ms",
        )
        ax_bottom.set_xlim(1, max(xs) if xs else 100)
        ax_bottom.legend(loc="upper left", fontsize=8, frameon=False, ncol=3)

    save_figure(fig, bundle_dir / "15_batch_fill分析.png")
    plt.close(fig)
    _write(
        bundle_dir / "15_batch_fill分析.md",
        [
            "# batch_fill分析",
            "",
            "- 上图：攒满一批时间分位曲线（只看 batch_full）",
            "- 下图：从第 1 条到 batch_size，每个数量节点形成所需时间的离线曲线（基于真实 enqueue 时间恢复）",
        ],
    )


def _write_14(bundle_dir: Path, analysis: dict) -> None:
    rows = analysis.get("read_write_window_rows", [])
    lines = [
        "# 20s内每秒 websocket read/write",
        "",
        "- 口径说明：`read_count` 表示服务端 WebSocket 入站读到消息的数量，`write_count` 表示服务端 WebSocket 出站写回消息的数量。",
        "- 判定口径：`read_count >= 10000` 记为本秒达标，否则记为异常",
        "",
        "| second | read_count | write_count | read_gap_vs_10000 | read_ok |",
        "| --- | --- | --- | --- | --- |",
    ]
    for row in rows:
        read_count = int(row["read_count"])
        write_count = int(row["write_count"])
        gap = read_count - 10000
        ok = "Y" if read_count >= 10000 else "N"
        lines.append(f"| {row['second']} | {read_count} | {write_count} | {gap} | {ok} |")
    _write(bundle_dir / "14_20s读写明细.md", lines)
