"""Base types for document loaders."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Dict, Optional


@dataclass
class ParsedDocument:
    """Normalized document payload passed through the pipeline."""

    doc_id: str
    text: str
    metadata: Dict[str, object]
    source_path: Optional[str] = None


class BaseLoader:
    """Abstract loader interface for source documents."""

    def load(self, path: str) -> ParsedDocument:
        """Parse a file at path and return a ParsedDocument."""
        raise NotImplementedError
