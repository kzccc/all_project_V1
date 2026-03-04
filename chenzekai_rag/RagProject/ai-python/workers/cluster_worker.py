"""Clustering workflow."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Dict, List

from taxonomy.cluster import cluster_documents
from taxonomy.node_suggester import suggest_node_name


@dataclass
class ClusterResult:
    """Output of the cluster workflow."""

    labels: List[int]
    node_names: Dict[int, str]


def cluster_vectors(
    vectors: List[List[float]],
    samples: List[str],
    *,
    client,
    k: int = 5,
) -> ClusterResult:
    """Cluster vectors and propose taxonomy node names."""
    labels = cluster_documents(vectors, k)
    grouped: Dict[int, List[str]] = {}
    for label, sample in zip(labels, samples):
        grouped.setdefault(label, []).append(sample)
    node_names = {label: suggest_node_name(group, client) for label, group in grouped.items()}
    return ClusterResult(labels=labels, node_names=node_names)
