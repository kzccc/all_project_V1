"""Title generation helpers."""

from __future__ import annotations

from pathlib import Path

from .ollama_client import OllamaClient


def _fallback_title(text: str) -> str:
    text = text.strip()
    if not text:
        return "Untitled Document"
    for line in text.splitlines():
        line = line.strip()
        if line:
            text = line
            break
    words = text.split()
    if len(words) > 12:
        return " ".join(words[:12])
    return text[:120]


_PROMPT_PATH = Path(__file__).resolve().parents[1] / "prompts" / "title.txt"


def generate_title(text: str, client: OllamaClient) -> str:
    """Generate a concise title for the input text."""
    template = _PROMPT_PATH.read_text(encoding="utf-8")
    prompt = template.format(text=text)
    try:
        title = client.generate(prompt).strip()
        if not title:
            snippet = text.strip()[:800]
            retry_prompt = (
                "Return a short, descriptive title (max 12 words). "
                "Answer with title only.\n\n"
                f"Document:\n{snippet}\n\nTitle:"
            )
            title = client.generate(retry_prompt).strip()
        return title or _fallback_title(text)
    except Exception:
        return _fallback_title(text)
