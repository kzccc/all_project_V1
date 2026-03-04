"""Report generation helpers."""

from __future__ import annotations

from pathlib import Path
from typing import List

from .ollama_client import OllamaClient


def _fallback_report(query: str, evidence: List[str]) -> str:
    summary = query.strip() or "Summary"
    snippets = []
    for item in evidence:
        snippet = item.strip()
        if snippet:
            snippets.append(snippet[:120])
        if len(snippets) >= 3:
            break
    bullets = "\n".join(f"- {snippet}" for snippet in snippets) if snippets else "- No evidence provided."
    return f"Summary: {summary}\n\nKey points:\n{bullets}"


_PROMPT_PATH = Path(__file__).resolve().parents[1] / "prompts" / "report.txt"


def generate_report(query: str, evidence: List[str], client: OllamaClient) -> str:
    """Generate a report based on query and evidence passages."""
    template = _PROMPT_PATH.read_text(encoding="utf-8")
    prompt = template.format(query=query, evidence="\n".join(evidence))
    try:
        return client.generate(prompt).strip()
    except Exception:
        return _fallback_report(query, evidence)
