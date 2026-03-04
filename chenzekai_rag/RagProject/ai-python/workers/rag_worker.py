"""RAG workflow."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Dict, List, Optional

from nlp.embedder import embed_texts
from nlp.report_generator import generate_report
from vectorstore.faiss_store import FaissStore
from vectorstore.retriever import Retriever


@dataclass
class RagResult:
    """Output of the RAG workflow."""

    report: str
    evidence: List[dict]


def run_rag(
    query: str,
    store: FaissStore,
    *,
    client,
    k: int = 5,
    filters: Optional[Dict[str, object]] = None,
) -> RagResult:
    """Retrieve relevant chunks and generate a report."""
    query_vector = embed_texts([query], client)[0]
    retriever = Retriever(store)
    evidence = retriever.retrieve(query_vector, k=k, filters=filters)
    report = generate_report(query, [item.get("text", "") for item in evidence], client)
    return RagResult(report=report, evidence=evidence)
