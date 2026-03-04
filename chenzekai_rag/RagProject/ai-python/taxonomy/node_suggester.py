"""Node name suggestion helpers."""

from __future__ import annotations

import re
from collections import Counter
from typing import List

from nlp.ollama_client import OllamaClient


_STOPWORDS = {"the", "and", "for", "with", "that", "this", "from", "into", "are"}


def suggest_node_name(samples: List[str], client: OllamaClient) -> str:
    """Suggest a semantic name for a cluster of samples."""
    if client.mock_mode:
        return _fallback_name(samples)

    prompt = (
        "Suggest a short taxonomy node name (3-6 words) for these samples:\n"
        + "\n".join(samples)
    )
    try:
        return client.generate(prompt).strip() or _fallback_name(samples)
    except Exception:
        return _fallback_name(samples)


def _fallback_name(samples: List[str]) -> str:
    words = re.findall(r"[A-Za-z0-9]+", " ".join(samples).lower())
    counts = Counter(word for word in words if word not in _STOPWORDS)
    top = [word for word, _ in counts.most_common(3)]
    return " / ".join(top) if top else "Unlabeled Cluster"
