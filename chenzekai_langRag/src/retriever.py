from typing import List, Dict, Any

from src.classifier import classify_query


def _distance_to_similarity(distance: float) -> float:
    if distance is None:
        return 0.0
    return max(0.0, 1.0 - float(distance))


class HierarchicalRetriever:
    def __init__(self, vector_store, settings):
        self.vector_store = vector_store
        self.settings = settings

    def _build_filter(self, levels: List[str], depth: int) -> Dict[str, Any]:
        return {f"level_{i + 1}": levels[i] for i in range(depth)}

    def _search_with_filter(self, query: str, levels: List[str], depth: int):
        search_kwargs = {"k": self.settings.top_k}
        metadata_filter = self._build_filter(levels, depth)
        try:
            results = self.vector_store.similarity_search_with_score(query, **search_kwargs, filter=metadata_filter)
        except TypeError:
            # Older vector store API
            results = self.vector_store.similarity_search_with_score(query, k=self.settings.top_k)
        documents = []
        for doc, score in results:
            doc.metadata["score"] = _distance_to_similarity(score)
            doc.metadata["path_depth"] = depth
            documents.append(doc)
        return documents

    def get_relevant_documents(self, query: str):
        classification = classify_query(query, self.settings)
        levels = classification["levels"]
        confidence = classification["confidence"]

        if confidence < self.settings.classify_conf_threshold:
            return [doc for doc in self.vector_store.similarity_search(query, k=self.settings.top_k)]

        collected: List[Document] = []
        for depth in range(self.settings.hierarchy_depth, 0, -1):
            filtered_docs = self._search_with_filter(query, levels, depth)
            collected.extend(filtered_docs)
            if len(collected) >= self.settings.top_k:
                break

        return self._rank_documents(collected)

    def _rank_documents(self, documents: List) -> List:
        alpha = 1.0 - self.settings.path_weight
        beta = self.settings.path_weight

        def score(doc: Document) -> float:
            sim = float(doc.metadata.get("score", 0.0))
            depth = int(doc.metadata.get("path_depth", 0))
            path_score = depth / max(1, self.settings.hierarchy_depth)
            return alpha * sim + beta * path_score

        ranked = sorted(documents, key=score, reverse=True)
        return ranked[: self.settings.top_k]


def get_retriever(vector_store, settings):
    retriever_type = settings.retriever_type.strip().lower()
    if retriever_type == "hierarchical":
        return HierarchicalRetriever(vector_store, settings)

    search_kwargs = {"k": settings.top_k}

    if retriever_type == "mmr":
        search_kwargs["fetch_k"] = settings.mmr_fetch_k
        search_kwargs["lambda_mult"] = settings.mmr_lambda_mult
        return vector_store.as_retriever(search_type="mmr", search_kwargs=search_kwargs)

    if retriever_type in {"similarity_score_threshold", "score_threshold"}:
        score_threshold = settings.parsed_score_threshold()
        if score_threshold is not None:
            search_kwargs["score_threshold"] = score_threshold
            return vector_store.as_retriever(
                search_type="similarity_score_threshold",
                search_kwargs=search_kwargs,
            )

    return vector_store.as_retriever(search_kwargs=search_kwargs)
