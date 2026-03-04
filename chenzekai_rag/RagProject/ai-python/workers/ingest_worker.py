"""Document ingestion workflow."""

from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable, List, Optional

from loaders.base import ParsedDocument
from loaders.docx_loader import DocxLoader
from loaders.excel_loader import ExcelLoader
from loaders.pptx_loader import PptxLoader
from loaders.pdf_loader import PdfLoader
from loaders.txt_loader import TxtLoader
from nlp.embedder import embed_texts
from vectorstore.faiss_store import FaissStore


@dataclass
class IngestResult:
    """Output of the ingest workflow."""

    doc_id: str
    chunks: int


def ingest_file(
    path: str,
    store: FaissStore,
    *,
    client=None,
    chunk_size: Optional[int] = None,
    overlap: Optional[int] = None,
) -> IngestResult:
    """Ingest a file from disk into the vector store."""
    document = load_document(path)
    return ingest_document(
        document,
        store,
        client=client,
        chunk_size=chunk_size,
        overlap=overlap,
    )


def ingest_document(
    document: ParsedDocument,
    store: FaissStore,
    *,
    client=None,
    chunk_size: Optional[int] = None,
    overlap: Optional[int] = None,
    title: Optional[str] = None,
    labels: Optional[List[str]] = None,
    extra_metadata: Optional[dict] = None,
) -> IngestResult:
    """Ingest an already-parsed document into the vector store."""
    chunks = _chunk_text(
        document.text,
        chunk_size or int(os.getenv("CHUNK_SIZE", "1000")),
        overlap if overlap is not None else int(os.getenv("CHUNK_OVERLAP", "100")),
    )
    if client is None:
        raise ValueError("Ollama client is required for embeddings")
    vectors = embed_texts(chunks, client)
    metadata = [
        _build_metadata(
            document,
            chunk,
            index,
            title=title,
            labels=labels,
            extra_metadata=extra_metadata,
        )
        for index, chunk in enumerate(chunks)
    ]
    store.add(vectors, metadata)
    return IngestResult(doc_id=document.doc_id, chunks=len(chunks))


def ingest_text(
    text: str,
    doc_id: str,
    store: FaissStore,
    *,
    client=None,
    title: Optional[str] = None,
    labels: Optional[List[str]] = None,
    extra_metadata: Optional[dict] = None,
) -> IngestResult:
    """Ingest raw text content without reading from disk."""
    document = ParsedDocument(doc_id=doc_id, text=text, metadata={"source_type": "inline"})
    return ingest_document(
        document,
        store,
        client=client,
        title=title,
        labels=labels,
        extra_metadata=extra_metadata,
    )


def load_document(path: str) -> ParsedDocument:
    """Load a document from disk using the matching loader."""
    loader = _select_loader(path)
    return loader.load(path)


def _select_loader(path: str):
    suffix = Path(path).suffix.lower()
    if suffix == ".docx":
        return DocxLoader()
    if suffix == ".pptx":
        return PptxLoader()
    if suffix == ".pdf":
        return PdfLoader()
    if suffix in {".xlsx", ".xls"}:
        return ExcelLoader()
    return TxtLoader()


def _chunk_text(text: str, chunk_size: int, overlap: int) -> List[str]:
    if chunk_size <= 0:
        return [text]
    if overlap >= chunk_size:
        overlap = 0
    chunks: List[str] = []
    step = chunk_size - overlap
    for start in range(0, len(text), step):
        chunk = text[start : start + chunk_size]
        if chunk:
            chunks.append(chunk)
    return chunks


def _build_metadata(
    document: ParsedDocument,
    chunk: str,
    index: int,
    *,
    title: Optional[str] = None,
    labels: Optional[List[str]] = None,
    extra_metadata: Optional[dict] = None,
) -> dict:
    metadata = {"doc_id": document.doc_id, "chunk_index": str(index), "text": chunk}
    metadata.update(document.metadata)
    if document.source_path:
        metadata["source_path"] = document.source_path
    if title:
        metadata["title"] = title
    if labels:
        metadata["labels"] = labels
    if extra_metadata:
        metadata.update(extra_metadata)
    return metadata
