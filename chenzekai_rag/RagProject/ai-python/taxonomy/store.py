"""Persistence and helpers for document taxonomy."""

from __future__ import annotations

import json
import uuid
from dataclasses import dataclass, asdict
from pathlib import Path
from typing import Dict, Iterable, List, Optional, Tuple

from nlp.ollama_client import strip_think


@dataclass
class TaxonomyNode:
    """A single taxonomy node."""

    node_id: str
    parent_id: Optional[str]
    name: str
    auto_generated_flag: bool
    created_by: str
    confidence: float
    version: int


@dataclass
class TaxonomySuggestion:
    """Suggested path for a document."""

    doc_id: str
    recommended_node_id: str
    recommended_path: str
    candidates: List[dict]
    suggested_new: Optional[dict]


class TaxonomyStore:
    """Persisted taxonomy tree with document assignments."""

    def __init__(self, path: Path) -> None:
        self.path = path
        self.version = 1
        self.nodes: List[TaxonomyNode] = []
        self.assignments: Dict[str, str] = {}
        self._load()

    def list_nodes(self) -> List[TaxonomyNode]:
        return list(self.nodes)

    def get_tree(self) -> dict:
        return {
            "version": self.version,
            "nodes": [asdict(node) for node in self.nodes],
            "assignments": dict(self.assignments),
        }

    def create_node(
        self,
        name: str,
        *,
        parent_id: str = "root",
        auto_generated_flag: bool = False,
        created_by: str = "user",
        confidence: float = 0.6,
    ) -> TaxonomyNode:
        clean_name = strip_think(name).strip()
        if not clean_name:
            raise ValueError("Node name is required")
        existing = self._find_node_by_name(clean_name, parent_id)
        if existing:
            return existing
        node = TaxonomyNode(
            node_id=uuid.uuid4().hex,
            parent_id=parent_id,
            name=clean_name,
            auto_generated_flag=auto_generated_flag,
            created_by=created_by,
            confidence=confidence,
            version=self.version,
        )
        self.nodes.append(node)
        self._persist()
        return node

    def assign(self, doc_id: str, node_id: str) -> None:
        self.assignments[str(doc_id)] = str(node_id)
        self._persist()

    def batch_assign(self, assignments: Iterable[Tuple[str, str]]) -> None:
        for doc_id, node_id in assignments:
            self.assignments[str(doc_id)] = str(node_id)
        self._persist()

    def suggest(self, documents: Iterable[dict]) -> List[TaxonomySuggestion]:
        nodes = {node.node_id: node for node in self.nodes}
        suggestions: List[TaxonomySuggestion] = []
        for doc in documents:
            doc_id = str(doc.get("doc_id", ""))
            title = strip_think(doc.get("title") or "").strip()
            labels = [strip_think(label).strip() for label in doc.get("labels", []) if label]
            candidates = self._score_candidates(nodes, title, labels)
            if candidates:
                recommended = candidates[0]
                suggestions.append(
                    TaxonomySuggestion(
                        doc_id=doc_id,
                        recommended_node_id=recommended["node_id"],
                        recommended_path=recommended["path"],
                        candidates=candidates,
                        suggested_new=None,
                    )
                )
            else:
                suggested_name = labels[0] if labels else title or "未分类"
                suggested = {
                    "node_id": "new",
                    "path": suggested_name,
                    "confidence": 0.45,
                }
                root_path = self._path_for(nodes, "root")
                suggestions.append(
                    TaxonomySuggestion(
                        doc_id=doc_id,
                        recommended_node_id="root",
                        recommended_path=root_path,
                        candidates=[],
                        suggested_new=suggested,
                    )
                )
        return suggestions

    def propose_restructure(self, documents: Iterable[dict]) -> dict:
        base_nodes = [node for node in self.nodes if not node.auto_generated_flag]
        if not any(node.node_id == "root" for node in base_nodes):
            base_nodes.append(self._default_root())

        label_groups: Dict[str, List[str]] = {}
        for doc in documents:
            doc_id = str(doc.get("doc_id", ""))
            title = strip_think(doc.get("title") or "").strip()
            labels = [strip_think(label).strip() for label in doc.get("labels", []) if label]
            key = labels[0] if labels else (title or "未分类")
            label_groups.setdefault(key, []).append(doc_id)

        new_nodes: List[TaxonomyNode] = []
        assignments: Dict[str, str] = {}
        for name, doc_ids in label_groups.items():
            node = TaxonomyNode(
                node_id=uuid.uuid4().hex,
                parent_id="root",
                name=name,
                auto_generated_flag=True,
                created_by="system",
                confidence=0.5,
                version=self.version,
            )
            new_nodes.append(node)
            for doc_id in doc_ids:
                assignments[doc_id] = node.node_id

        return {
            "nodes": [asdict(node) for node in base_nodes + new_nodes],
            "assignments": assignments,
        }

    def apply_restructure(self, nodes: Iterable[dict], assignments: Dict[str, str]) -> None:
        node_models = []
        for node in nodes:
            node_models.append(
                TaxonomyNode(
                    node_id=node["node_id"],
                    parent_id=node.get("parent_id"),
                    name=node["name"],
                    auto_generated_flag=bool(node.get("auto_generated_flag", False)),
                    created_by=node.get("created_by", "system"),
                    confidence=float(node.get("confidence", 0.5)),
                    version=int(node.get("version", self.version)),
                )
            )
        if not any(node.node_id == "root" for node in node_models):
            node_models.append(self._default_root())
        self.nodes = node_models
        self.assignments = {str(key): str(value) for key, value in assignments.items()}
        self.version += 1
        self._persist()

    def _score_candidates(self, nodes: Dict[str, TaxonomyNode], title: str, labels: List[str]) -> List[dict]:
        tokens = _collect_tokens(title, labels)
        if not tokens:
            return []
        candidates: List[dict] = []
        for node in nodes.values():
            if node.node_id == "root":
                continue
            score = _score_match(node.name, tokens)
            if score <= 0:
                continue
            candidates.append(
                {
                    "node_id": node.node_id,
                    "path": self._path_for(nodes, node.node_id),
                    "confidence": score,
                }
            )
        candidates.sort(key=lambda item: item["confidence"], reverse=True)
        return candidates

    def _path_for(self, nodes: Dict[str, TaxonomyNode], node_id: str) -> str:
        path_nodes = []
        current = nodes.get(node_id)
        while current:
            path_nodes.append(current.name)
            if current.parent_id is None:
                break
            current = nodes.get(current.parent_id)
        if not path_nodes:
            return "根目录"
        return " / ".join(reversed(path_nodes))

    def _find_node_by_name(self, name: str, parent_id: str) -> Optional[TaxonomyNode]:
        for node in self.nodes:
            if node.parent_id == parent_id and node.name.lower() == name.lower():
                return node
        return None

    def _default_root(self) -> TaxonomyNode:
        return TaxonomyNode(
            node_id="root",
            parent_id=None,
            name="根目录",
            auto_generated_flag=False,
            created_by="system",
            confidence=1.0,
            version=self.version,
        )

    def _load(self) -> None:
        if self.path.exists():
            payload = json.loads(self.path.read_text(encoding="utf-8"))
            self.version = int(payload.get("version", 1))
            self.nodes = [TaxonomyNode(**node) for node in payload.get("nodes", [])]
            self.assignments = {str(k): str(v) for k, v in payload.get("assignments", {}).items()}
        if not self.nodes:
            self.nodes = [self._default_root()]
        self._persist()

    def _persist(self) -> None:
        self.path.parent.mkdir(parents=True, exist_ok=True)
        payload = {
            "version": self.version,
            "nodes": [asdict(node) for node in self.nodes],
            "assignments": self.assignments,
        }
        self.path.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")


def _collect_tokens(title: str, labels: List[str]) -> List[str]:
    tokens: List[str] = []
    if title:
        tokens.extend(_split_tokens(title))
    for label in labels:
        tokens.extend(_split_tokens(label))
    return [token.lower() for token in tokens if token.strip()]


def _split_tokens(text: str) -> List[str]:
    cleaned = text.replace("/", " ").replace("-", " ").replace("_", " ")
    tokens = [item.strip() for item in cleaned.split() if item.strip()]
    if tokens:
        return tokens
    return [text.strip()] if text.strip() else []


def _score_match(name: str, tokens: List[str]) -> float:
    name_lower = name.lower()
    best = 0.0
    for token in tokens:
        if token == name_lower:
            best = max(best, 0.9)
        elif token and (token in name_lower or name_lower in token):
            best = max(best, 0.6)
    return best
