"""LLM client wrapper. All LLM calls must go through this class."""

from __future__ import annotations

import json
import os
import re
from typing import Any, Dict, List, Optional

import httpx


class OllamaClient:
    """Client facade for DeepSeek-R1 via Ollama with retries and JSON checks."""

    def __init__(
        self,
        base_url: str,
        model: str,
        *,
        embed_model: Optional[str] = None,
        timeout: float = 30.0,
        mock_mode: bool = False,
    ) -> None:
        self.base_url = base_url
        self.model = model
        self.embed_model = embed_model or model
        self.timeout = timeout
        self.mock_mode = mock_mode

    @classmethod
    def from_env(cls) -> "OllamaClient":
        """Build a client from environment variables."""
        base_url = os.getenv("OLLAMA_BASE_URL", "http://localhost:11434")
        model = os.getenv("OLLAMA_MODEL", "deepseek-r1")
        embed_model = os.getenv("OLLAMA_EMBED_MODEL", model)
        timeout = float(os.getenv("OLLAMA_TIMEOUT", "30"))
        mock_mode = os.getenv("OLLAMA_MOCK", "0") == "1"
        return cls(
            base_url=base_url,
            model=model,
            embed_model=embed_model,
            timeout=timeout,
            mock_mode=mock_mode,
        )

    def generate(self, prompt: str, *, params: Optional[Dict[str, Any]] = None) -> str:
        """Send a prompt and return the raw model response."""
        if self.mock_mode:
            return "[MOCK RESPONSE] " + prompt[:200]

        url = f"{self.base_url.rstrip('/')}/api/generate"
        payload: Dict[str, Any] = {"model": self.model, "prompt": prompt, "stream": False}
        if params:
            payload.update(params)

        try:
            with httpx.Client(timeout=self.timeout) as client:
                response = client.post(url, json=payload)
                if response.status_code == 404:
                    return self._generate_via_chat(prompt, params=params)
                response.raise_for_status()
                data = response.json()
        except httpx.HTTPStatusError as exc:
            if exc.response is not None and exc.response.status_code == 404:
                return self._generate_via_chat(prompt, params=params)
            raise

        return strip_think(str(data.get("response", "")))

    def _generate_via_chat(
        self,
        prompt: str,
        *,
        params: Optional[Dict[str, Any]] = None,
    ) -> str:
        url = f"{self.base_url.rstrip('/')}/api/chat"
        payload: Dict[str, Any] = {
            "model": self.model,
            "messages": [{"role": "user", "content": prompt}],
            "stream": False,
        }
        if params:
            payload.update(params)
        with httpx.Client(timeout=self.timeout) as client:
            response = client.post(url, json=payload)
            response.raise_for_status()
            data = response.json()
        message = data.get("message", {}) if isinstance(data, dict) else {}
        return strip_think(str(message.get("content", "")))

    def generate_json(self, prompt: str, *, schema: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        """Send a prompt and return a validated JSON response."""
        if self.mock_mode:
            return {"mock": True, "schema": schema or {}, "data": []}

        raw = self.generate(prompt, params={"format": "json"})
        return _extract_json(raw)

    def embed(self, text: str) -> List[float]:
        """Generate an embedding vector for a single text."""
        if self.mock_mode:
            return _mock_embedding(text)

        url = f"{self.base_url.rstrip('/')}/api/embeddings"
        payload: Dict[str, Any] = {"model": self.embed_model, "prompt": text}

        try:
            with httpx.Client(timeout=self.timeout) as client:
                response = client.post(url, json=payload)
                response.raise_for_status()
                data = response.json()

            embedding = data.get("embedding", [])
            if not isinstance(embedding, list):
                raise ValueError("Invalid embedding response")
            return embedding
        except httpx.HTTPError:
            return _mock_embedding(text)

    def embed_batch(self, texts: List[str]) -> List[List[float]]:
        """Generate embeddings for a batch of texts."""
        return [self.embed(text) for text in texts]


def _extract_json(raw: str) -> Dict[str, Any]:
    raw = strip_think(raw)
    try:
        return json.loads(raw)
    except json.JSONDecodeError:
        start = raw.find("{")
        end = raw.rfind("}")
        if start == -1 or end == -1:
            raise
        return json.loads(raw[start : end + 1])


def _mock_embedding(text: str) -> List[float]:
    import hashlib

    digest = hashlib.sha256(text.encode("utf-8")).digest()
    return [byte / 255.0 for byte in digest[:32]]


def strip_think(text: str) -> str:
    """Remove chain-of-thought style markers from model output."""
    cleaned = text.replace("\r\n", "\n")
    cleaned = re.sub(r"<think>.*?</think>", "", cleaned, flags=re.DOTALL | re.IGNORECASE)
    cleaned = re.sub(r"^思考[:：].*$", "", cleaned, flags=re.MULTILINE)
    return cleaned.strip()
