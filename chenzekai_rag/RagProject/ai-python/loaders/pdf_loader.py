"""PDF loader implementation."""

from __future__ import annotations

from pathlib import Path

from .base import BaseLoader, ParsedDocument


class PdfLoader(BaseLoader):
    """Load PDF documents into ParsedDocument objects."""

    def load(self, path: str) -> ParsedDocument:
        """Parse a PDF file and return a ParsedDocument."""
        try:
            from PyPDF2 import PdfReader  # type: ignore
        except ImportError as exc:  # pragma: no cover - optional dependency
            raise RuntimeError("PyPDF2 is required for PDF loading") from exc

        reader = PdfReader(path)
        text_chunks = []
        for page in reader.pages:
            page_text = page.extract_text() or ""
            if page_text.strip():
                text_chunks.append(page_text)
        text = "\n".join(text_chunks)
        source = Path(path)
        return ParsedDocument(
            doc_id=source.stem,
            text=text.strip(),
            metadata={"source_type": "pdf"},
            source_path=str(source),
        )
