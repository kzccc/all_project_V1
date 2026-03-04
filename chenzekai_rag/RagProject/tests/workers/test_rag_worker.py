from nlp.ollama_client import OllamaClient
from vectorstore.faiss_store import FaissStore
from workers.ingest_worker import ingest_text
from workers.rag_worker import run_rag


def test_rag_worker_returns_report(tmp_path) -> None:
    store = FaissStore(
        index_path=str(tmp_path / "index.faiss"),
        metadata_path=str(tmp_path / "meta.json"),
    )
    client = OllamaClient(base_url="http://localhost:11434", model="mock", mock_mode=True)

    ingest_text("AI systems classify documents by topic.", "doc-1", store, client=client)
    result = run_rag("How does it classify?", store, client=client, k=1)

    assert result.evidence
    assert result.report.startswith("[MOCK RESPONSE]")
