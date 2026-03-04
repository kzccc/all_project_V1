from pathlib import Path

from langchain_chroma import Chroma


def _persist_if_supported(vector_store):
    if hasattr(vector_store, "persist"):
        vector_store.persist()


def build_vector_store(documents, embeddings, persist_directory: Path):
    vector_store = Chroma.from_documents(
        documents=documents,
        embedding=embeddings,
        persist_directory=str(persist_directory),
    )
    _persist_if_supported(vector_store)
    return vector_store


def load_vector_store(embeddings, persist_directory: Path):
    return Chroma(
        persist_directory=str(persist_directory),
        embedding_function=embeddings,
    )
