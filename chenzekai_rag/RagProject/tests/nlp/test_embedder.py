from nlp.embedder import embed_texts
from nlp.ollama_client import OllamaClient


def test_embedder_returns_vectors() -> None:
    client = OllamaClient(base_url="http://localhost:11434", model="mock", mock_mode=True)
    vectors = embed_texts(["hello"], client)
    assert len(vectors) == 1
    assert len(vectors[0]) > 0
