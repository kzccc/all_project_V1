from langchain_ollama import OllamaLLM


def get_llm(settings):
    return OllamaLLM(
        model=settings.ollama_model,
        base_url=settings.ollama_base_url,
    )


def build_prompt(question: str, context: str) -> str:
    return (
        "You are a helpful assistant. Answer using the provided context. "
        "If the context is insufficient, say you do not know.\n\n"
        f"Context:\n{context}\n\n"
        f"Question: {question}\n"
        "Answer:"
    )


def format_documents(documents) -> str:
    blocks = []
    for doc in documents:
        path = doc.metadata.get("full_path") or doc.metadata.get("source", "")
        header = f"[文档来源]: {path}" if path else "[文档来源]: 未知"
        blocks.append(f"{header}\n{doc.page_content}")
    return "\n\n".join(blocks)
