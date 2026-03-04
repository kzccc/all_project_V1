"""Draft staging for upload confirmation."""

from __future__ import annotations

import json
import uuid
from dataclasses import dataclass
from pathlib import Path
from typing import List, Optional

from loaders.base import ParsedDocument


@dataclass
class DraftDocument:
    """Document awaiting confirmation before ingestion."""

    draft_id: str
    doc_id: str
    text: str
    metadata: dict
    source_path: Optional[str]
    title: str
    labels: List[str]


class DraftStore:
    """Store drafts on disk so they survive process restarts."""

    def __init__(self, base_dir: Path) -> None:
        self.base_dir = base_dir
        self.base_dir.mkdir(parents=True, exist_ok=True)

    def create(
        self,
        document: ParsedDocument,
        *,
        title: str,
        labels: List[str],
    ) -> DraftDocument:
        """Create and persist a draft document."""
        draft_id = uuid.uuid4().hex
        draft = DraftDocument(
            draft_id=draft_id,
            doc_id=document.doc_id,
            text=document.text,
            metadata=document.metadata,
            source_path=document.source_path,
            title=title,
            labels=labels,
        )
        self.save(draft)
        return draft

    def save(self, draft: DraftDocument) -> None:
        """Persist a draft to disk."""
        text_path = self.base_dir / f"{draft.draft_id}.txt"
        json_path = self.base_dir / f"{draft.draft_id}.json"

        text_path.write_text(draft.text, encoding="utf-8")
        payload = {
            "draft_id": draft.draft_id,
            "doc_id": draft.doc_id,
            "metadata": draft.metadata,
            "source_path": draft.source_path,
            "title": draft.title,
            "labels": draft.labels,
            "text_path": str(text_path),
        }
        json_path.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")

    def load(self, draft_id: str) -> Optional[DraftDocument]:
        """Load a draft document by id."""
        json_path = self.base_dir / f"{draft_id}.json"
        if not json_path.exists():
            return None
        payload = json.loads(json_path.read_text(encoding="utf-8"))
        text_path = Path(payload.get("text_path", ""))
        text = text_path.read_text(encoding="utf-8") if text_path.exists() else ""
        return DraftDocument(
            draft_id=payload["draft_id"],
            doc_id=payload.get("doc_id", ""),
            text=text,
            metadata=payload.get("metadata", {}),
            source_path=payload.get("source_path"),
            title=payload.get("title", ""),
            labels=payload.get("labels", []),
        )

    def delete(self, draft_id: str) -> None:
        """Remove a draft and its stored text."""
        json_path = self.base_dir / f"{draft_id}.json"
        text_path = self.base_dir / f"{draft_id}.txt"
        if json_path.exists():
            json_path.unlink()
        if text_path.exists():
            text_path.unlink()
