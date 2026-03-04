"""FAISS-backed vector store with persistence."""

from __future__ import annotations

import json
import os
from pathlib import Path
from typing import Iterable, List, Optional

import faiss  # type: ignore
import numpy as np


class FaissStore:
    """Vector store backed by FAISS with persisted metadata."""

    def __init__(
        self,
        index_path: Optional[str] = None,
        metadata_path: Optional[str] = None,
    ) -> None:
        data_dir = Path(os.getenv("DATA_DIR", "data"))
        self.index_path = Path(index_path) if index_path else data_dir / "index" / "faiss.index"
        self.metadata_path = (
            Path(metadata_path) if metadata_path else data_dir / "index" / "metadata.json"
        )

        self._index: Optional[faiss.Index] = None
        self._metadata: List[dict] = []
        self._load()

    def add(self, vectors: Iterable[List[float]], metadata: Iterable[dict]) -> None:
        """Add vectors and metadata to the store."""
        vector_list = [list(vector) for vector in vectors]
        if not vector_list:
            return

        matrix = np.array(vector_list, dtype="float32")
        if matrix.ndim != 2:
            raise ValueError("vectors must be a 2D array")

        self._ensure_index(matrix.shape[1])
        faiss.normalize_L2(matrix)
        self._index.add(matrix)

        for meta in metadata:
            self._metadata.append(dict(meta))

        self._persist()

    def search(self, query_vector: List[float], k: int) -> List[dict]:
        """Search for the top-k nearest vectors and return metadata."""
        if self._index is None or not self._metadata:
            return []

        vector = np.array([query_vector], dtype="float32")
        faiss.normalize_L2(vector)
        k = min(k, len(self._metadata))
        scores, indices = self._index.search(vector, k)

        results = []
        for score, index in zip(scores[0], indices[0]):
            if index < 0:
                continue
            item = dict(self._metadata[index])
            item["score"] = float(score)
            results.append(item)
        return results

    def list_documents(self) -> List[dict]:
        """Return document summaries stored in memory."""
        documents: dict[str, dict] = {}
        for meta in self._metadata:
            doc_id = str(meta.get("doc_id", "unknown"))
            entry = documents.setdefault(
                doc_id,
                {
                    "doc_id": doc_id,
                    "chunks": 0,
                    "source_types": set(),
                    "source_paths": set(),
                    "title": None,
                    "labels": [],
                },
            )
            entry["chunks"] += 1
            _maybe_add(entry["source_types"], meta.get("source_type"))
            _maybe_add(entry["source_paths"], meta.get("source_path"))

            if entry["title"] is None and meta.get("title"):
                entry["title"] = meta.get("title")
            if not entry["labels"] and meta.get("labels"):
                entry["labels"] = _normalize_labels(meta.get("labels"))

        results: List[dict] = []
        for entry in documents.values():
            results.append(
                {
                    "doc_id": entry["doc_id"],
                    "chunks": entry["chunks"],
                    "source_types": sorted(entry["source_types"]),
                    "source_paths": sorted(entry["source_paths"]),
                    "title": entry["title"],
                    "labels": entry["labels"],
                }
            )

        return sorted(results, key=lambda item: item["doc_id"])

    def get_document(self, doc_id: str) -> Optional[dict]:
        """Return details for a single document."""
        items = []
        source_types = set()
        source_paths = set()
        title = None
        labels: list[str] = []

        for meta in self._metadata:
            if str(meta.get("doc_id")) != str(doc_id):
                continue
            text = str(meta.get("text", ""))
            items.append(
                {
                    "chunk_index": _coerce_index(meta.get("chunk_index")),
                    "text_preview": text[:200],
                    "metadata": {k: v for k, v in meta.items() if k != "text"},
                }
            )
            _maybe_add(source_types, meta.get("source_type"))
            _maybe_add(source_paths, meta.get("source_path"))
            if title is None and meta.get("title"):
                title = meta.get("title")
            if not labels and meta.get("labels"):
                labels = _normalize_labels(meta.get("labels"))

        if not items:
            return None

        return {
            "doc_id": str(doc_id),
            "chunks": len(items),
            "source_types": sorted(source_types),
            "source_paths": sorted(source_paths),
            "title": title,
            "labels": labels,
            "items": items,
        }

    def _ensure_index(self, dim: int) -> None:
        if self._index is None:
            self._index = faiss.IndexFlatIP(dim)
        elif self._index.d != dim:
            raise ValueError("Embedding dimension mismatch")

    def _load(self) -> None:
        if self.metadata_path.exists():
            self._metadata = json.loads(self.metadata_path.read_text(encoding="utf-8"))
        if self.index_path.exists():
            self._index = faiss.read_index(str(self.index_path))

    def _persist(self) -> None:
        self.index_path.parent.mkdir(parents=True, exist_ok=True)
        self.metadata_path.parent.mkdir(parents=True, exist_ok=True)

        if self._index is not None:
            faiss.write_index(self._index, str(self.index_path))
        self.metadata_path.write_text(
            json.dumps(self._metadata, ensure_ascii=False, indent=2),
            encoding="utf-8",
        )


def _maybe_add(container: set, value: object) -> None:
    if value is None:
        return
    container.add(str(value))


def _normalize_labels(value: object) -> list[str]:
    if isinstance(value, list):
        return [str(label) for label in value]
    if isinstance(value, str):
        return [item.strip() for item in value.split(",") if item.strip()]
    return []


def _coerce_index(value: object) -> object:
    if isinstance(value, int):
        return value
    if isinstance(value, str) and value.isdigit():
        return int(value)
    return value
