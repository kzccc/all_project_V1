from langchain_ollama import OllamaEmbeddings


def get_embedding_model(settings):
    return OllamaEmbeddings(
        model=settings.ollama_embed_model,
        base_url=settings.ollama_base_url,
    )
