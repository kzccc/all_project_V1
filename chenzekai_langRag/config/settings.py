from dataclasses import dataclass
from pathlib import Path
import os

BASE_DIR = Path(__file__).resolve().parents[1]


def _env(name: str, default: str) -> str:
    return os.getenv(name, default)


@dataclass(frozen=True)
class Settings:
    base_dir: Path = BASE_DIR
    data_dir: Path = BASE_DIR / "data"
    raw_dir: Path = BASE_DIR / "data" / "raw"
    staging_dir: Path = BASE_DIR / "data" / "staging"
    processed_dir: Path = BASE_DIR / "data" / "processed"
    vector_store_dir: Path = BASE_DIR / "vector_store"
    chroma_dir: Path = BASE_DIR / "vector_store" / "chroma"

    ollama_base_url: str = _env("OLLAMA_BASE_URL", "http://localhost:11434")
    ollama_model: str = _env("OLLAMA_MODEL", "deepseek-r1:32b")
    ollama_embed_model: str = _env("OLLAMA_EMBED_MODEL", "nomic-embed-text")

    chunk_size: int = int(_env("CHUNK_SIZE", "800"))
    chunk_overlap: int = int(_env("CHUNK_OVERLAP", "120"))
    top_k: int = int(_env("TOP_K", "4"))

    retriever_type: str = _env("RETRIEVER_TYPE", "mmr")
    mmr_fetch_k: int = int(_env("MMR_FETCH_K", "20"))
    mmr_lambda_mult: float = float(_env("MMR_LAMBDA_MULT", "0.5"))
    score_threshold: str = _env("SCORE_THRESHOLD", "")

    hierarchy_depth: int = int(_env("HIERARCHY_DEPTH", "6"))
    classify_conf_threshold: float = float(_env("CLASSIFY_CONF_THRESHOLD", "0.6"))
    path_weight: float = float(_env("PATH_WEIGHT", "0.35"))

    def parsed_score_threshold(self):
        if not self.score_threshold:
            return None
        return float(self.score_threshold)
