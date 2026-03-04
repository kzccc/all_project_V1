from pathlib import Path
from typing import List, Dict


def sanitize_segment(segment: str) -> str:
    if not segment:
        return "未分类"
    cleaned = segment.replace("/", "_").replace("\\", "_").strip()
    return cleaned or "未分类"


def pad_levels(levels: List[str], depth: int) -> List[str]:
    levels = [sanitize_segment(x) for x in levels if x]
    while len(levels) < depth:
        levels.append("未分类")
    return levels[:depth]


def path_to_levels(path: Path, depth: int) -> List[str]:
    parts = list(path.parts)
    if path.name in parts:
        parts = parts[:-1]
    return pad_levels(parts, depth)


def levels_to_path(levels: List[str]) -> Path:
    path = Path()
    for level in levels:
        path = path / sanitize_segment(level)
    return path


def build_metadata(levels: List[str]) -> Dict[str, str]:
    metadata = {f"level_{i + 1}": levels[i] for i in range(len(levels))}
    metadata["full_path"] = "/".join(levels)
    metadata["path_text"] = " / ".join(levels)
    return metadata
