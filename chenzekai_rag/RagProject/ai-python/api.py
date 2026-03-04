"""FastAPI HTTP layer."""

from __future__ import annotations

import os
import shutil
import uuid
from dataclasses import asdict
from pathlib import Path
from typing import Dict, Optional

from fastapi import FastAPI, File, HTTPException, UploadFile
from fastapi.responses import HTMLResponse
from pydantic import BaseModel, Field

from loaders.base import ParsedDocument
from nlp.ollama_client import OllamaClient, strip_think
from vectorstore.faiss_store import FaissStore
from workers.draft_worker import DraftStore
from workers.enrich_worker import EnrichResult, enrich_document
from workers.ingest_worker import ingest_document, ingest_file, ingest_text, load_document
from workers.rag_worker import RagResult, run_rag
from taxonomy.store import TaxonomyStore, TaxonomySuggestion

APP_DIR = Path(__file__).resolve().parent
UI_PATH = APP_DIR / "ui" / "index.html"


class IngestRequest(BaseModel):
    """Request body for ingesting documents."""

    path: Optional[str] = None
    content: Optional[str] = None
    doc_id: Optional[str] = None
    title: Optional[str] = None
    labels: Optional[list[str]] = None


class EnrichRequest(BaseModel):
    """Request body for enrichment."""

    text: str = Field(..., min_length=1)


class RagRequest(BaseModel):
    """Request body for RAG queries."""

    query: str = Field(..., min_length=1)
    top_k: int = 5
    filters: Optional[Dict[str, list[str] | str]] = None


class BatchIngestItem(BaseModel):
    """Single item for batch ingestion."""

    path: Optional[str] = None
    content: Optional[str] = None
    doc_id: Optional[str] = None
    title: Optional[str] = None
    labels: Optional[list[str]] = None


class BatchIngestRequest(BaseModel):
    """Batch ingestion request."""

    items: list[BatchIngestItem]
    auto_enrich: bool = False


class IngestResponse(BaseModel):
    """Response for ingest operations."""

    doc_id: str
    chunks: int


class BatchIngestResponse(BaseModel):
    """Response for batch ingestion."""

    results: list[IngestResponse]
    errors: list[str]


class EnrichResponse(BaseModel):
    """Response for enrich operations."""

    title: str
    labels: list[str]


class RagResponse(BaseModel):
    """Response for RAG operations."""

    report: str
    evidence: list[dict]


class DocumentSummary(BaseModel):
    """Summary of documents stored in memory."""

    doc_id: str
    chunks: int
    source_types: list[str]
    source_paths: list[str]
    title: Optional[str]
    labels: list[str]


class DocumentDetail(BaseModel):
    """Detailed document view with chunk previews."""

    doc_id: str
    chunks: int
    source_types: list[str]
    source_paths: list[str]
    title: Optional[str]
    labels: list[str]
    items: list[dict]


class DraftResponse(BaseModel):
    """Draft response returned after upload."""

    draft_id: str
    doc_id: str
    title: str
    labels: list[str]
    source_path: Optional[str]


class DraftConfirmRequest(BaseModel):
    """Payload to confirm a draft for ingestion."""

    doc_id: Optional[str] = None
    title: Optional[str] = None
    labels: Optional[list[str]] = None


class TaxonomyNodeModel(BaseModel):
    """Response model for taxonomy nodes."""

    node_id: str
    parent_id: Optional[str] = None
    name: str
    auto_generated_flag: bool = False
    created_by: str = "system"
    confidence: float = 0.5
    version: int = 1


class TaxonomyTreeResponse(BaseModel):
    """Response model for taxonomy tree."""

    version: int
    nodes: list[TaxonomyNodeModel]
    assignments: Dict[str, str]


class TaxonomySuggestItem(BaseModel):
    """Item to request taxonomy suggestions."""

    doc_id: str
    title: Optional[str] = None
    labels: list[str] = Field(default_factory=list)


class TaxonomySuggestRequest(BaseModel):
    """Request taxonomy suggestions for documents."""

    documents: list[TaxonomySuggestItem]


class TaxonomySuggestCandidate(BaseModel):
    """Candidate node suggestion."""

    node_id: str
    path: str
    confidence: float


class TaxonomySuggestResponse(BaseModel):
    """Suggestion response per document."""

    doc_id: str
    recommended_node_id: str
    recommended_path: str
    candidates: list[TaxonomySuggestCandidate]
    suggested_new: Optional[TaxonomySuggestCandidate]


class TaxonomyCreateNodeRequest(BaseModel):
    """Create a taxonomy node."""

    name: str = Field(..., min_length=1)
    parent_id: Optional[str] = "root"
    auto_generated_flag: bool = False
    created_by: str = "user"
    confidence: float = 0.6


class TaxonomyCreateNodeResponse(TaxonomyNodeModel):
    """Created node response."""


class TaxonomyAssignRequest(BaseModel):
    """Assign a document to a taxonomy node."""

    doc_id: str = Field(..., min_length=1)
    node_id: str = Field(..., min_length=1)


class TaxonomyBatchAssignRequest(BaseModel):
    """Batch assign documents to nodes."""

    assignments: list[TaxonomyAssignRequest]


class TaxonomyRestructureRequest(BaseModel):
    """Request a restructure proposal."""

    documents: list[TaxonomySuggestItem]


class TaxonomyRestructureResponse(BaseModel):
    """Restructure proposal response."""

    nodes: list[TaxonomyNodeModel]
    assignments: Dict[str, str]


class TaxonomyApplyRequest(BaseModel):
    """Apply a restructure proposal."""

    nodes: list[TaxonomyNodeModel]
    assignments: Dict[str, str]


def create_app() -> FastAPI:
    """Create and configure the FastAPI application."""
    app = FastAPI(title="AI Document Engine")

    data_dir = Path(os.getenv("DATA_DIR", "data"))
    upload_dir = Path(os.getenv("UPLOAD_DIR", data_dir / "uploads"))
    staging_dir = Path(os.getenv("STAGING_DIR", data_dir / "staging"))
    upload_dir.mkdir(parents=True, exist_ok=True)

    app.state.store = FaissStore()
    app.state.client = OllamaClient.from_env()
    app.state.drafts = DraftStore(staging_dir)
    app.state.upload_dir = upload_dir
    app.state.taxonomy = TaxonomyStore(data_dir / "taxonomy.json")

    @app.get("/", response_class=HTMLResponse)
    def ui() -> HTMLResponse:
        if not UI_PATH.exists():
            return HTMLResponse("UI not found", status_code=404)
        return HTMLResponse(UI_PATH.read_text(encoding="utf-8"))

    @app.get("/health")
    def health() -> Dict[str, str]:
        return {"status": "ok"}

    @app.get("/documents", response_model=list[DocumentSummary])
    def list_documents() -> list[DocumentSummary]:
        store: FaissStore = app.state.store
        cleaned = []
        for item in store.list_documents():
            item["title"] = strip_think(item.get("title", "") or "") or None
            item["labels"] = [strip_think(label) for label in item.get("labels", []) if strip_think(label)]
            cleaned.append(DocumentSummary(**item))
        return cleaned

    @app.get("/documents/{doc_id}", response_model=DocumentDetail)
    def get_document(doc_id: str) -> DocumentDetail:
        store: FaissStore = app.state.store
        data = store.get_document(doc_id)
        if data is None:
            raise HTTPException(status_code=404, detail="Document not found")
        data["title"] = strip_think(data.get("title", "") or "") or None
        data["labels"] = [strip_think(label) for label in data.get("labels", []) if strip_think(label)]
        return DocumentDetail(**data)

    @app.post("/drafts/upload", response_model=list[DraftResponse])
    async def upload_draft(files: list[UploadFile] = File(...)) -> list[DraftResponse]:
        store: DraftStore = app.state.drafts
        client: OllamaClient = app.state.client
        upload_root: Path = app.state.upload_dir
        results: list[DraftResponse] = []

        for file in files:
            if not file.filename:
                continue
            draft_suffix = f"{uuid.uuid4().hex}_{file.filename}"
            saved_path = upload_root / draft_suffix
            with saved_path.open("wb") as handle:
                shutil.copyfileobj(file.file, handle)

            loaded = load_document(str(saved_path))
            doc_id = Path(file.filename).stem or loaded.doc_id
            document = ParsedDocument(
                doc_id=doc_id,
                text=loaded.text,
                metadata={**loaded.metadata, "original_name": file.filename},
                source_path=str(saved_path),
            )
            enrich: EnrichResult = enrich_document(document.text, client=client)
            clean_title = strip_think(enrich.title).strip()
            if not clean_title:
                clean_title = doc_id or "Untitled Document"
            clean_labels = [strip_think(label) for label in enrich.labels if strip_think(label)]
            draft = store.create(document, title=clean_title, labels=clean_labels)
            results.append(
                DraftResponse(
                    draft_id=draft.draft_id,
                    doc_id=draft.doc_id,
                    title=strip_think(draft.title),
                    labels=[strip_think(label) for label in draft.labels if strip_think(label)],
                    source_path=draft.source_path,
                )
            )

        if not results:
            raise HTTPException(status_code=400, detail="No files uploaded")
        return results

    @app.post("/drafts/{draft_id}/confirm", response_model=IngestResponse)
    def confirm_draft(draft_id: str, payload: DraftConfirmRequest) -> IngestResponse:
        store: DraftStore = app.state.drafts
        vectorstore: FaissStore = app.state.store
        client: OllamaClient = app.state.client

        draft = store.load(draft_id)
        if draft is None:
            raise HTTPException(status_code=404, detail="Draft not found")

        doc_id = payload.doc_id or draft.doc_id
        title = payload.title or draft.title
        labels = payload.labels or draft.labels

        document = ParsedDocument(
            doc_id=doc_id,
            text=draft.text,
            metadata=draft.metadata,
            source_path=draft.source_path,
        )
        result = ingest_document(
            document,
            vectorstore,
            client=client,
            title=title,
            labels=labels,
        )
        store.delete(draft_id)
        return IngestResponse(**asdict(result))

    @app.post("/ingest", response_model=IngestResponse)
    def ingest(payload: IngestRequest) -> IngestResponse:
        store: FaissStore = app.state.store
        client: OllamaClient = app.state.client

        if payload.path:
            result = ingest_file(payload.path, store, client=client)
        elif payload.content:
            doc_id = payload.doc_id or "inline"
            result = ingest_text(
                payload.content,
                doc_id,
                store,
                client=client,
                title=payload.title,
                labels=payload.labels,
            )
        else:
            raise HTTPException(status_code=400, detail="Provide path or content")

        return IngestResponse(**asdict(result))

    @app.post("/ingest/batch", response_model=BatchIngestResponse)
    def ingest_batch(payload: BatchIngestRequest) -> BatchIngestResponse:
        store: FaissStore = app.state.store
        client: OllamaClient = app.state.client
        results: list[IngestResponse] = []
        errors: list[str] = []

        for item in payload.items:
            try:
                if item.path:
                    document = load_document(item.path)
                elif item.content:
                    document = ParsedDocument(
                        doc_id=item.doc_id or "inline",
                        text=item.content,
                        metadata={"source_type": "inline"},
                    )
                else:
                    raise ValueError("Provide path or content")

                title = item.title
                labels = item.labels
                if payload.auto_enrich and (not title or not labels):
                    enrich = enrich_document(document.text, client=client)
                    title = title or enrich.title
                    labels = labels or enrich.labels

                result = ingest_document(
                    document,
                    store,
                    client=client,
                    title=title,
                    labels=labels,
                )
                results.append(IngestResponse(**asdict(result)))
            except Exception as exc:  # pragma: no cover - batch errors surfaced to caller
                errors.append(str(exc))

        return BatchIngestResponse(results=results, errors=errors)

    @app.post("/enrich", response_model=EnrichResponse)
    def enrich(payload: EnrichRequest) -> EnrichResponse:
        client: OllamaClient = app.state.client
        result: EnrichResult = enrich_document(payload.text, client=client)
        data = asdict(result)
        data["title"] = strip_think(data.get("title", ""))
        data["labels"] = [strip_think(label) for label in data.get("labels", []) if strip_think(label)]
        return EnrichResponse(**data)

    @app.post("/rag", response_model=RagResponse)
    def rag(payload: RagRequest) -> RagResponse:
        store: FaissStore = app.state.store
        client: OllamaClient = app.state.client
        result: RagResult = run_rag(
            payload.query,
            store,
            client=client,
            k=payload.top_k,
            filters=payload.filters,
        )
        data = asdict(result)
        data["report"] = strip_think(data.get("report", ""))
        return RagResponse(**data)

    @app.get("/taxonomy", response_model=TaxonomyTreeResponse)
    def taxonomy_tree() -> TaxonomyTreeResponse:
        taxonomy: TaxonomyStore = app.state.taxonomy
        payload = taxonomy.get_tree()
        return TaxonomyTreeResponse(**payload)

    @app.post("/taxonomy/suggest", response_model=list[TaxonomySuggestResponse])
    def taxonomy_suggest(payload: TaxonomySuggestRequest) -> list[TaxonomySuggestResponse]:
        taxonomy: TaxonomyStore = app.state.taxonomy
        suggestions = taxonomy.suggest([doc.model_dump() for doc in payload.documents])
        return [TaxonomySuggestResponse(**asdict(item)) for item in suggestions]

    @app.post("/taxonomy/nodes", response_model=TaxonomyCreateNodeResponse)
    def taxonomy_create_node(payload: TaxonomyCreateNodeRequest) -> TaxonomyCreateNodeResponse:
        taxonomy: TaxonomyStore = app.state.taxonomy
        node = taxonomy.create_node(
            payload.name,
            parent_id=payload.parent_id or "root",
            auto_generated_flag=payload.auto_generated_flag,
            created_by=payload.created_by,
            confidence=payload.confidence,
        )
        return TaxonomyCreateNodeResponse(**asdict(node))

    @app.post("/taxonomy/assign")
    def taxonomy_assign(payload: TaxonomyAssignRequest) -> Dict[str, str]:
        taxonomy: TaxonomyStore = app.state.taxonomy
        taxonomy.assign(payload.doc_id, payload.node_id)
        return {"status": "ok"}

    @app.post("/taxonomy/assign/batch")
    def taxonomy_assign_batch(payload: TaxonomyBatchAssignRequest) -> Dict[str, str]:
        taxonomy: TaxonomyStore = app.state.taxonomy
        assignments = [(item.doc_id, item.node_id) for item in payload.assignments]
        taxonomy.batch_assign(assignments)
        return {"status": "ok"}

    @app.post("/taxonomy/restructure", response_model=TaxonomyRestructureResponse)
    def taxonomy_restructure(payload: TaxonomyRestructureRequest) -> TaxonomyRestructureResponse:
        taxonomy: TaxonomyStore = app.state.taxonomy
        proposal = taxonomy.propose_restructure([doc.model_dump() for doc in payload.documents])
        return TaxonomyRestructureResponse(**proposal)

    @app.post("/taxonomy/restructure/apply")
    def taxonomy_apply(payload: TaxonomyApplyRequest) -> Dict[str, str]:
        taxonomy: TaxonomyStore = app.state.taxonomy
        taxonomy.apply_restructure(
            [node.model_dump() for node in payload.nodes],
            payload.assignments,
        )
        return {"status": "ok"}

    return app
