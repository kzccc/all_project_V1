import argparse
import json
from pathlib import Path
import shutil
import sys

from dotenv import load_dotenv

from config.settings import Settings
from src.embedder import get_embedding_model
from src.generator import build_prompt, format_documents, get_llm
from src.hierarchy import build_metadata, path_to_levels
from src.index_store import load_library_index
from src.loader import load_documents
from src.retriever import get_retriever
from src.splitter import split_documents
from src.store import build_vector_store, load_vector_store


def ensure_dirs(settings: Settings) -> None:
    settings.raw_dir.mkdir(parents=True, exist_ok=True)
    settings.staging_dir.mkdir(parents=True, exist_ok=True)
    settings.processed_dir.mkdir(parents=True, exist_ok=True)
    settings.chroma_dir.mkdir(parents=True, exist_ok=True)


def write_metadata(settings: Settings, sources, doc_count: int, chunk_count: int) -> None:
    metadata = {
        "documents": doc_count,
        "chunks": chunk_count,
        "sources": sources,
        "chunk_size": settings.chunk_size,
        "chunk_overlap": settings.chunk_overlap,
        "embedding_model": settings.ollama_embed_model,
    }
    path = settings.vector_store_dir / "metadata.json"
    path.write_text(json.dumps(metadata, indent=2), encoding="utf-8")


def ingest(settings: Settings):
    documents = load_documents(settings.raw_dir)
    if not documents:
        print("No documents found in data/raw.")
        return None

    library_index = load_library_index(settings)

    for doc in documents:
        source = doc.metadata.get("source", "")
        if source:
            try:
                rel = Path(source).resolve().relative_to(settings.raw_dir.resolve())
            except Exception:
                rel = Path(source).name
            levels = path_to_levels(Path(rel), settings.hierarchy_depth)
            metadata = build_metadata(levels)
            index_key = str(Path(rel))
            if index_key in library_index:
                tags = library_index[index_key].get("tags", [])
                metadata["tags"] = tags
            doc.metadata.update(metadata)

    chunks = split_documents(documents, settings.chunk_size, settings.chunk_overlap)
    embeddings = get_embedding_model(settings)
    vector_store = build_vector_store(chunks, embeddings, settings.chroma_dir)

    sources = sorted({doc.metadata.get("source", "") for doc in documents if doc.metadata.get("source")})
    write_metadata(settings, sources, len(documents), len(chunks))
    return vector_store


def load_or_create(settings: Settings, rebuild: bool):
    embeddings = get_embedding_model(settings)
    has_store = settings.chroma_dir.exists() and any(settings.chroma_dir.iterdir())
    if has_store and not rebuild:
        return load_vector_store(embeddings, settings.chroma_dir)

    if rebuild and settings.chroma_dir.exists():
        shutil.rmtree(settings.chroma_dir)
        settings.chroma_dir.mkdir(parents=True, exist_ok=True)

    return ingest(settings)


def retrieve_documents(retriever, question: str):
    if hasattr(retriever, "invoke"):
        return retriever.invoke(question)
    if hasattr(retriever, "get_relevant_documents"):
        return retriever.get_relevant_documents(question)
    if hasattr(retriever, "_get_relevant_documents"):
        return retriever._get_relevant_documents(question)
    raise AttributeError("Retriever does not support document retrieval methods.")


def answer_question(question: str, retriever, llm):
    documents = retrieve_documents(retriever, question)
    context = format_documents(documents)
    prompt = build_prompt(question, context)
    answer = llm.invoke(prompt)

    print("\n" + answer.strip())
    sources = [doc.metadata.get("source") for doc in documents if doc.metadata.get("source")]
    if sources:
        print("\nSources:")
        for source in sorted(set(sources)):
            print(f"- {source}")


def interactive_loop(retriever, llm):
    while True:
        question = input("\nQ> ").strip()
        if question.lower() in {"exit", "quit"}:
            break
        if not question:
            continue
        answer_question(question, retriever, llm)


def main():
    load_dotenv()
    settings = Settings()
    ensure_dirs(settings)

    parser = argparse.ArgumentParser(description="Basic LangChain RAG with Ollama")
    parser.add_argument("--ingest", action="store_true", help="Ingest documents and build vector store")
    parser.add_argument("--rebuild", action="store_true", help="Rebuild vector store from scratch")
    parser.add_argument("--query", type=str, help="Ask one question and exit")
    args = parser.parse_args()

    vector_store = load_or_create(settings, rebuild=args.rebuild or args.ingest)
    if vector_store is None:
        return

    retriever = get_retriever(vector_store, settings)
    llm = get_llm(settings)

    if args.query:
        answer_question(args.query, retriever, llm)
        return

    if not sys.stdin.isatty():
        print("No interactive stdin detected; exiting after rebuild.")
        return

    interactive_loop(retriever, llm)


if __name__ == "__main__":
    main()
