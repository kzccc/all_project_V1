"""Embedding generator helpers."""

from __future__ import annotations

from typing import List

from .ollama_client import OllamaClient


def embed_texts(texts: List[str], client: OllamaClient) -> List[List[float]]:
    """Convert texts into embedding vectors."""
    return client.embed_batch(texts)
