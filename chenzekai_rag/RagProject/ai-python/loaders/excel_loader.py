"""Excel loader implementation."""

from __future__ import annotations

from pathlib import Path

from .base import BaseLoader, ParsedDocument


class ExcelLoader(BaseLoader):
    """Load Excel documents into ParsedDocument objects."""

    def load(self, path: str) -> ParsedDocument:
        """Parse an Excel file and return a ParsedDocument."""
        try:
            import pandas as pd  # type: ignore
        except ImportError as exc:  # pragma: no cover - optional dependency
            raise RuntimeError("pandas is required for Excel loading") from exc

        sheets = pd.read_excel(path, sheet_name=None)
        parts = [frame.to_csv(index=False) for frame in sheets.values()]
        text = "\n".join(parts)
        source = Path(path)
        return ParsedDocument(
            doc_id=source.stem,
            text=text.strip(),
            metadata={"source_type": "excel"},
            source_path=str(source),
        )
