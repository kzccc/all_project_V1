"""Retriever with optional metadata filtering."""

from __future__ import annotations

from typing import Dict, List, Optional

from .faiss_store import FaissStore


class Retriever:
    """Retrieve relevant documents using vector similarity."""

    def __init__(self, store: FaissStore) -> None:
        self.store = store

    def retrieve(
        self,
        query_vector: List[float],
        *,
        k: int = 5,
        filters: Optional[Dict[str, object]] = None,
    ) -> List[dict]:
        """Retrieve documents with optional metadata filters."""
        candidates = self.store.search(query_vector, k=max(k * 2, k))
        if not filters:
            return candidates[:k]

        results = []
        for candidate in candidates:
            if _matches_filters(candidate, filters):
                results.append(candidate)
            if len(results) >= k:
                break
        return results


def _matches_filters(candidate: dict, filters: Dict[str, object]) -> bool:
    for key, value in filters.items():
        candidate_value = candidate.get(key)
        if isinstance(value, (list, tuple, set)):
            if str(candidate_value) not in {str(item) for item in value}:
                return False
        else:
            if str(candidate_value) != str(value):
                return False
    return True
