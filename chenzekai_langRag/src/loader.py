from pathlib import Path
from typing import List

from langchain_community.document_loaders import PyPDFLoader, TextLoader

try:
    from langchain_community.document_loaders import Docx2txtLoader
except ImportError:  # pragma: no cover - optional dependency
    Docx2txtLoader = None


def load_documents(raw_dir: Path):
    documents = []
    for path in sorted(raw_dir.rglob("*")):
        if path.is_dir():
            continue
        ext = path.suffix.lower()
        loader = None
        if ext == ".pdf":
            loader = PyPDFLoader(str(path))
        elif ext in {".md", ".markdown", ".txt"}:
            loader = TextLoader(str(path), autodetect_encoding=True)
        elif ext == ".docx" and Docx2txtLoader is not None:
            loader = Docx2txtLoader(str(path))

        if loader is None:
            print(f"skip unsupported file: {path}")
            continue

        documents.extend(loader.load())
    return documents
