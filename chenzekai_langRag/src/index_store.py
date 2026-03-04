import json
from pathlib import Path
from typing import Dict, Any


LIBRARY_INDEX_NAME = "library_index.json"
STAGING_INDEX_NAME = "staging_index.json"


def _load_json(path: Path) -> Dict[str, Any]:
    if not path.exists():
        return {}
    return json.loads(path.read_text(encoding="utf-8"))


def _save_json(path: Path, data: Dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2), encoding="utf-8")


def library_index_path(settings) -> Path:
    return settings.processed_dir / LIBRARY_INDEX_NAME


def staging_index_path(settings) -> Path:
    return settings.staging_dir / STAGING_INDEX_NAME


def load_library_index(settings) -> Dict[str, Any]:
    return _load_json(library_index_path(settings))


def save_library_index(settings, data: Dict[str, Any]) -> None:
    _save_json(library_index_path(settings), data)


def load_staging_index(settings) -> Dict[str, Any]:
    return _load_json(staging_index_path(settings))


def save_staging_index(settings, data: Dict[str, Any]) -> None:
    _save_json(staging_index_path(settings), data)
