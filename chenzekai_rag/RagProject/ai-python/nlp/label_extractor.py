"""Label extraction helpers."""

from __future__ import annotations

from pathlib import Path
from collections import Counter
from typing import List

from .ollama_client import OllamaClient, strip_think


_PROMPT_PATH = Path(__file__).resolve().parents[1] / "prompts" / "label.txt"
_STOPWORDS = {"the", "and", "for", "with", "that", "this", "from", "into", "are"}


def _fallback_labels(text: str) -> List[str]:
    words = []
    for token in text.replace("/", " ").replace("-", " ").replace("_", " ").split():
        token = token.strip().lower()
        if not token or token in _STOPWORDS:
            continue
        if token.isdigit():
            continue
        words.append(token)
    counts = Counter(words)
    top = [word for word, _ in counts.most_common(5)]
    return top or ["general"]


def extract_labels(text: str, client: OllamaClient) -> List[str]:
    """Extract structured labels from the input text."""
    template = _PROMPT_PATH.read_text(encoding="utf-8")
    prompt = template.format(text=text)
    try:
        result = client.generate_json(prompt)
    except Exception:
        return _fallback_labels(text)

    if isinstance(result, dict):
        labels = result.get("labels") or result.get("tags") or []
    else:
        labels = result or []

    cleaned = []
    for label in labels:
        label_text = strip_think(str(label)).strip()
        if label_text:
            cleaned.append(label_text)
    return cleaned or _fallback_labels(text)
