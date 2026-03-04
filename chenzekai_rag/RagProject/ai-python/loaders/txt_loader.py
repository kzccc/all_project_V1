"""Plain text loader implementation."""

from __future__ import annotations

from pathlib import Path

from .base import BaseLoader, ParsedDocument


class TxtLoader(BaseLoader):
    """Load text documents into ParsedDocument objects."""

    def load(self, path: str) -> ParsedDocument:
        """Parse a text file and return a ParsedDocument."""
        source = Path(path)
        text = source.read_text(encoding="utf-8")
        return ParsedDocument(
            doc_id=source.stem,
            text=text.strip(),
            metadata={"source_type": "txt"},
            source_path=str(source),
        )
