"""Clustering helpers for taxonomy discovery."""

from __future__ import annotations

from typing import List


def cluster_documents(vectors: List[List[float]], k: int) -> List[int]:
    """Assign cluster labels for document vectors."""
    if k <= 0:
        raise ValueError("k must be positive")
    return [index % k for index in range(len(vectors))]
