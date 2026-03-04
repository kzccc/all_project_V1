"""Document enrichment workflow."""

from __future__ import annotations

from dataclasses import dataclass
from typing import List

from nlp.label_extractor import extract_labels
from nlp.title_generator import generate_title


@dataclass
class EnrichResult:
    """Output of the enrich workflow."""

    title: str
    labels: List[str]


def enrich_document(text: str, *, client) -> EnrichResult:
    """Generate title and labels for a document."""
    title = generate_title(text, client)
    labels = extract_labels(text, client)
    return EnrichResult(title=title, labels=labels)
