from vectorstore.faiss_store import FaissStore


def test_faiss_store_search_returns_best_match(tmp_path) -> None:
    store = FaissStore(
        index_path=str(tmp_path / "index.faiss"),
        metadata_path=str(tmp_path / "meta.json"),
    )
    vectors = [[1.0, 0.0], [0.0, 1.0]]
    metadata = [{"id": "alpha"}, {"id": "beta"}]
    store.add(vectors, metadata)

    results = store.search([1.0, 0.0], k=1)
    assert results[0]["id"] == "alpha"
