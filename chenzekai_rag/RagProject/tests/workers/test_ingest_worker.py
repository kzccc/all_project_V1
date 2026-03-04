from pathlib import Path

from nlp.ollama_client import OllamaClient
from vectorstore.faiss_store import FaissStore
from workers.ingest_worker import ingest_file


def test_ingest_worker_adds_chunks(tmp_path) -> None:
    sample_path = Path(__file__).resolve().parents[1] / "fixtures" / "sample.txt"
    store = FaissStore(
        index_path=str(tmp_path / "index.faiss"),
        metadata_path=str(tmp_path / "meta.json"),
    )
    client = OllamaClient(base_url="http://localhost:11434", model="mock", mock_mode=True)

    result = ingest_file(str(sample_path), store, client=client, chunk_size=40, overlap=0)

    assert result.doc_id == "sample"
    assert result.chunks >= 1
