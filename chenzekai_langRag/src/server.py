import json
import shutil
import threading
import uuid
from pathlib import Path
from typing import List, Dict, Any

from dotenv import load_dotenv
from fastapi import FastAPI, File, HTTPException, UploadFile
from fastapi.responses import HTMLResponse
from pydantic import BaseModel

from config.settings import Settings
from src.classifier import classify_document
from src.embedder import get_embedding_model
from src.generator import build_prompt, format_documents, get_llm
from src.hierarchy import build_metadata, levels_to_path, path_to_levels, sanitize_segment
from src.index_store import load_library_index, load_staging_index, save_library_index, save_staging_index
from src.loader import load_documents
from src.retriever import get_retriever
from src.splitter import split_documents
from src.store import build_vector_store, load_vector_store

load_dotenv()
settings = Settings()

app = FastAPI(title="层次化RAG知识库")
lock = threading.Lock()
vector_store = None
embedding_model = get_embedding_model(settings)
llm = get_llm(settings)

INDEX_HTML = """<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>层次化RAG知识库</title>
  <style>
    :root {
      --bg: #f2f2f7;
      --bg-2: #e6e8f0;
      --card: rgba(255, 255, 255, 0.82);
      --stroke: rgba(0, 0, 0, 0.08);
      --text: #0f0f14;
      --muted: #6b6f7a;
      --accent: #0a84ff;
      --accent-2: #5ac8fa;
      --shadow: 0 18px 45px rgba(15, 15, 20, 0.12);
      --radius: 22px;
    }

    * { box-sizing: border-box; }

    body {
      margin: 0;
      font-family: -apple-system, BlinkMacSystemFont, "SF Pro Text", "SF Pro Display", "Helvetica Neue", sans-serif;
      color: var(--text);
      background: radial-gradient(circle at top left, #ffffff 0%, var(--bg) 45%, var(--bg-2) 100%);
      min-height: 100vh;
    }

    .container {
      max-width: 1200px;
      margin: 0 auto;
      padding: 32px 20px 48px;
      display: grid;
      gap: 24px;
      grid-template-columns: minmax(260px, 380px) minmax(0, 1fr);
      align-items: start;
    }

    header {
      grid-column: 1 / -1;
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 6px 4px;
    }

    header h1 {
      font-size: 28px;
      margin: 0;
      font-weight: 600;
      letter-spacing: -0.4px;
    }

    header span {
      color: var(--muted);
      font-size: 14px;
    }

    .card {
      background: var(--card);
      border-radius: var(--radius);
      padding: 20px 22px;
      border: 1px solid var(--stroke);
      box-shadow: var(--shadow);
      backdrop-filter: blur(18px);
      animation: rise 0.5s ease both;
    }

    .card h2 {
      font-size: 18px;
      margin: 0 0 12px;
      font-weight: 600;
    }

    .list {
      display: grid;
      gap: 10px;
      max-height: 360px;
      overflow: auto;
      padding-right: 6px;
    }

    .item {
      padding: 12px 14px;
      border-radius: 16px;
      background: rgba(255, 255, 255, 0.75);
      border: 1px solid rgba(0, 0, 0, 0.06);
      display: grid;
      gap: 6px;
      font-size: 13px;
    }

    .item strong {
      font-size: 14px;
      font-weight: 600;
    }

    .muted { color: var(--muted); }

    .upload-box {
      display: grid;
      gap: 10px;
    }

    .upload-box input[type="file"] {
      border: 1px dashed rgba(0, 0, 0, 0.2);
      border-radius: 16px;
      padding: 16px;
      background: rgba(255, 255, 255, 0.8);
    }

    .button {
      appearance: none;
      border: none;
      border-radius: 14px;
      padding: 12px 18px;
      background: linear-gradient(120deg, var(--accent), var(--accent-2));
      color: white;
      font-size: 14px;
      font-weight: 600;
      cursor: pointer;
      box-shadow: 0 10px 20px rgba(10, 132, 255, 0.25);
      transition: transform 0.15s ease, box-shadow 0.15s ease;
    }

    .button.secondary {
      background: linear-gradient(120deg, #5b5f69, #9aa0ab);
      box-shadow: none;
    }

    .button:hover { transform: translateY(-1px); }

    .status {
      font-size: 12px;
      color: var(--muted);
      min-height: 16px;
    }

    .chat {
      display: grid;
      gap: 16px;
    }

    textarea {
      width: 100%;
      min-height: 110px;
      resize: vertical;
      border-radius: 16px;
      padding: 14px;
      border: 1px solid rgba(0, 0, 0, 0.15);
      font-size: 14px;
      font-family: inherit;
      background: rgba(255, 255, 255, 0.85);
    }

    .answer {
      border-radius: 18px;
      border: 1px solid rgba(0, 0, 0, 0.08);
      padding: 16px;
      background: rgba(255, 255, 255, 0.85);
      min-height: 140px;
      white-space: pre-wrap;
      line-height: 1.5;
    }

    .row {
      display: flex;
      gap: 12px;
      align-items: center;
      flex-wrap: wrap;
    }

    .tag {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      padding: 2px 8px;
      border-radius: 999px;
      background: rgba(10, 132, 255, 0.12);
      color: #0a5dc2;
      font-size: 12px;
      margin-right: 6px;
    }

    .input {
      border-radius: 12px;
      border: 1px solid rgba(0, 0, 0, 0.12);
      padding: 8px 10px;
      font-size: 13px;
      background: rgba(255, 255, 255, 0.9);
      min-width: 180px;
    }

    .checkbox {
      display: flex;
      align-items: center;
      gap: 6px;
      font-size: 13px;
    }

    @keyframes rise {
      from { opacity: 0; transform: translateY(8px); }
      to { opacity: 1; transform: translateY(0); }
    }

    @media (max-width: 980px) {
      .container { grid-template-columns: 1fr; }
    }
  </style>
</head>
<body>
  <div class="container">
    <header>
      <h1>层次化RAG知识库</h1>
      <span>智能分类 · 结构化检索 · 本地模型</span>
    </header>

    <section class="card">
      <h2>上传文档</h2>
      <div class="upload-box">
        <input id="fileInput" type="file" multiple accept=".pdf,.docx,.md,.markdown,.txt" />
        <button class="button" id="uploadBtn">上传并生成标签</button>
        <div class="status" id="uploadStatus"></div>
      </div>
    </section>

    <section class="card">
      <h2>待确认入库</h2>
      <div class="list" id="stagingList"></div>
      <div class="row" style="margin-top:12px;">
        <button class="button" id="commitBtn">确认选中入库</button>
        <button class="button secondary" id="refreshBtn">刷新列表</button>
        <span class="status" id="stagingStatus"></span>
      </div>
    </section>

    <section class="card">
      <h2>已入库文档</h2>
      <div class="list" id="libraryList"></div>
      <div class="status" id="libraryStatus"></div>
    </section>

    <section class="card" style="grid-column: 1 / -1;">
      <h2>提问与回答</h2>
      <div class="chat">
        <textarea id="question" placeholder="请输入你的问题...\n例如：Transformer的注意力机制是什么？"></textarea>
        <div class="row">
          <button class="button" id="askBtn">发送提问</button>
          <span class="status" id="askStatus"></span>
        </div>
        <div class="answer" id="answerBox">回答会显示在这里。</div>
      </div>
    </section>
  </div>

  <script>
    function escapeHtml(value) {
      return String(value || '')
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/\"/g, '&quot;')
        .replace(/'/g, '&#39;');
    }

    function initApp() {
      const stagingList = document.getElementById('stagingList');
      const libraryList = document.getElementById('libraryList');
      const uploadStatus = document.getElementById('uploadStatus');
      const stagingStatus = document.getElementById('stagingStatus');
      const libraryStatus = document.getElementById('libraryStatus');
      const askStatus = document.getElementById('askStatus');
      const answerBox = document.getElementById('answerBox');

      uploadStatus.textContent = '前端已就绪。';

      window.addEventListener('error', (event) => {
        uploadStatus.textContent = `前端错误：${event.message}`;
      });
      window.addEventListener('unhandledrejection', (event) => {
        uploadStatus.textContent = `请求失败：${event.reason}`;
      });

      function renderTags(tags) {
        if (!tags || !tags.length) return '<span class="muted">无标签</span>';
        return tags.map(tag => `<span class="tag">${escapeHtml(tag)}</span>`).join('');
      }

      async function fetchStaging() {
        stagingStatus.textContent = '正在加载...';
        const res = await fetch('/api/staging');
        const data = await res.json();
        stagingList.innerHTML = '';
        if (!data.items || !data.items.length) {
          stagingList.innerHTML = '<div class="item">暂无待确认文档。</div>';
        } else {
          data.items.forEach(item => {
            const levels = (item.levels || []).join(' / ');
            const el = document.createElement('div');
            el.className = 'item';
            el.dataset.id = item.id;
            el.innerHTML = `
              <div class="row">
                <label class="checkbox">
                  <input type="checkbox" class="select" checked />
                  <strong>${escapeHtml(item.original_name)}</strong>
                </label>
              </div>
              <div class="muted">分类路径：${escapeHtml(levels)}</div>
              <div>${renderTags(item.tags)}</div>
              <div class="row">
                <label class="checkbox">
                  <input type="checkbox" class="rename-toggle" />
                  启用重命名
                </label>
                <input class="input rename-input" type="text" placeholder="输入新文件名" value="${escapeHtml(item.suggested_name)}" />
              </div>
            `;
            stagingList.appendChild(el);
          });
        }
        stagingStatus.textContent = `待确认：${(data.items || []).length} 个`;
      }

      async function fetchLibrary() {
        libraryStatus.textContent = '正在加载...';
        const res = await fetch('/api/library');
        const data = await res.json();
        libraryList.innerHTML = '';
        if (!data.documents || !data.documents.length) {
          libraryList.innerHTML = '<div class="item">暂无入库文档。</div>';
        } else {
          data.documents.forEach(doc => {
            const el = document.createElement('div');
            el.className = 'item';
            el.innerHTML = `<strong>${escapeHtml(doc.name)}</strong><div class="muted">${doc.size_kb} KB</div>`;
            libraryList.appendChild(el);
          });
        }
        libraryStatus.textContent = `已入库：${(data.documents || []).length} 个`;
      }

      document.getElementById('uploadBtn').addEventListener('click', async () => {
        const input = document.getElementById('fileInput');
        if (!input.files.length) {
          uploadStatus.textContent = '请先选择文件。';
          return;
        }
        uploadStatus.textContent = '上传中，正在生成标签...';
        const form = new FormData();
        for (const file of input.files) {
          form.append('files', file);
        }
        try {
          const res = await fetch('/api/upload', { method: 'POST', body: form });
          const data = await res.json();
          if (!res.ok) {
            uploadStatus.textContent = data.detail || '上传失败。';
            return;
          }
          uploadStatus.textContent = data.message || '上传完成。';
          input.value = '';
          await fetchStaging();
        } catch (err) {
          uploadStatus.textContent = '上传失败，请检查网络。';
        }
      });

      document.getElementById('refreshBtn').addEventListener('click', async () => {
        await fetchStaging();
        await fetchLibrary();
      });

      document.getElementById('commitBtn').addEventListener('click', async () => {
        const items = Array.from(document.querySelectorAll('#stagingList .item'));
        const payload = [];
        items.forEach(item => {
          const id = item.dataset.id;
          const selected = item.querySelector('.select').checked;
          if (!selected) return;
          const renameEnabled = item.querySelector('.rename-toggle').checked;
          const renameValue = item.querySelector('.rename-input').value.trim();
          payload.push({ id, rename_enabled: renameEnabled, rename_to: renameValue });
        });
        if (!payload.length) {
          stagingStatus.textContent = '请选择至少一个文档。';
          return;
        }
        stagingStatus.textContent = '正在入库并重建向量库...';
        const res = await fetch('/api/staging/commit', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ items: payload })
        });
        const data = await res.json();
        if (!res.ok) {
          stagingStatus.textContent = data.detail || '入库失败。';
          return;
        }
        stagingStatus.textContent = data.message || '入库完成。';
        await fetchStaging();
        await fetchLibrary();
      });

      document.getElementById('askBtn').addEventListener('click', async () => {
        const question = document.getElementById('question').value.trim();
        if (!question) {
          askStatus.textContent = '请输入问题。';
          return;
        }
        askStatus.textContent = '思考中...';
        answerBox.textContent = '';
        const res = await fetch('/api/query', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ question })
        });
        const data = await res.json();
        answerBox.textContent = data.answer || '暂无回答。';
        if (data.sources && data.sources.length) {
          const sources = '\\n\\n来源:\\n' + data.sources.map(s => `- ${escapeHtml(s)}`).join('\\n');
          answerBox.textContent += sources;
        }
        askStatus.textContent = '';
      });

      fetchStaging();
      fetchLibrary();
    }

    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', initApp);
    } else {
      initApp();
    }
  </script>
</body>
</html>
"""


class QueryRequest(BaseModel):
    question: str


class CommitItem(BaseModel):
    id: str
    rename_enabled: bool = False
    rename_to: str = ""


class CommitRequest(BaseModel):
    items: List[CommitItem]


def ensure_dirs() -> None:
    settings.raw_dir.mkdir(parents=True, exist_ok=True)
    settings.staging_dir.mkdir(parents=True, exist_ok=True)
    settings.processed_dir.mkdir(parents=True, exist_ok=True)
    settings.vector_store_dir.mkdir(parents=True, exist_ok=True)
    settings.chroma_dir.mkdir(parents=True, exist_ok=True)


def list_documents() -> List[Dict[str, Any]]:
    documents = []
    for path in sorted(settings.raw_dir.rglob("*")):
        if not path.is_file():
            continue
        rel_path = path.relative_to(settings.raw_dir)
        stat = path.stat()
        documents.append({"name": str(rel_path), "size_kb": max(1, int(stat.st_size / 1024))})
    return documents


def extract_text(path: Path, max_chars: int = 3000) -> str:
    ext = path.suffix.lower()
    text = ""
    if ext in {".md", ".markdown", ".txt"}:
        text = path.read_text(encoding="utf-8", errors="ignore")
    elif ext == ".pdf":
        from langchain_community.document_loaders import PyPDFLoader

        loader = PyPDFLoader(str(path))
        docs = loader.load()
        text = "\n".join(doc.page_content for doc in docs)
    elif ext == ".docx":
        try:
            from langchain_community.document_loaders import Docx2txtLoader

            loader = Docx2txtLoader(str(path))
            docs = loader.load()
            text = "\n".join(doc.page_content for doc in docs)
        except Exception:
            text = ""
    if len(text) > max_chars:
        return text[:max_chars]
    return text


def rebuild_vector_store() -> None:
    global vector_store
    documents = load_documents(settings.raw_dir)
    if not documents:
        vector_store = None
        return

    if settings.chroma_dir.exists():
        shutil.rmtree(settings.chroma_dir)
        settings.chroma_dir.mkdir(parents=True, exist_ok=True)

    library_index = load_library_index(settings)
    for doc in documents:
        source = doc.metadata.get("source", "")
        if source:
            try:
                rel = Path(source).resolve().relative_to(settings.raw_dir.resolve())
            except Exception:
                rel = Path(source).name
            levels = path_to_levels(Path(rel), settings.hierarchy_depth)
            metadata = build_metadata(levels)
            index_key = str(Path(rel))
            if index_key in library_index:
                metadata["tags"] = library_index[index_key].get("tags", [])
            doc.metadata.update(metadata)

    chunks = split_documents(documents, settings.chunk_size, settings.chunk_overlap)
    vector_store = build_vector_store(chunks, embedding_model, settings.chroma_dir)


def load_or_create_store():
    global vector_store
    if vector_store is not None:
        return vector_store

    has_store = settings.chroma_dir.exists() and any(settings.chroma_dir.iterdir())
    if has_store:
        vector_store = load_vector_store(embedding_model, settings.chroma_dir)
        return vector_store

    rebuild_vector_store()
    return vector_store


def clean_source(source: str) -> str:
    if not source:
        return ""
    try:
        return str(Path(source).resolve().relative_to(settings.raw_dir.resolve()))
    except Exception:
        return source


def unique_path(directory: Path, filename: str) -> Path:
    safe_name = Path(filename).name
    candidate = directory / safe_name
    if not candidate.exists():
        return candidate

    stem = candidate.stem
    suffix = candidate.suffix
    counter = 1
    while True:
        candidate = directory / f"{stem}_{counter}{suffix}"
        if not candidate.exists():
            return candidate
        counter += 1


def build_filename(name: str, extension: str) -> str:
    name = sanitize_segment(name)
    name = name.replace(" ", "_")
    name = name.strip("._") or "document"
    if not extension.startswith("."):
        extension = "." + extension
    return f"{name}{extension}"


@app.on_event("startup")
def startup_event() -> None:
    ensure_dirs()


@app.get("/")
def index():
    return HTMLResponse(INDEX_HTML)


@app.get("/api/library")
def api_library():
    return {"documents": list_documents()}


@app.get("/api/staging")
def api_staging():
    data = load_staging_index(settings)
    return {"items": data.get("items", [])}


@app.post("/api/upload")
async def api_upload(files: List[UploadFile] = File(...)):
    allowed = {".pdf", ".md", ".markdown", ".txt", ".docx"}
    staging = load_staging_index(settings)
    items = staging.get("items", [])
    added = []

    for upload in files:
        ext = Path(upload.filename).suffix.lower()
        if ext not in allowed:
            continue
        file_id = uuid.uuid4().hex
        stored_name = f"{file_id}_{Path(upload.filename).name}"
        destination = settings.staging_dir / stored_name
        with destination.open("wb") as buffer:
            shutil.copyfileobj(upload.file, buffer)

        text = extract_text(destination)
        classification = classify_document(text, settings)
        item = {
            "id": file_id,
            "original_name": upload.filename,
            "stored_name": stored_name,
            "extension": ext,
            "levels": classification["levels"],
            "tags": classification["tags"],
            "confidence": classification["confidence"],
            "suggested_name": classification["title"],
        }
        items.append(item)
        added.append(item)

    staging["items"] = items
    save_staging_index(settings, staging)

    if not added:
        raise HTTPException(status_code=400, detail="没有可识别的文件。")

    return {"message": f"已上传 {len(added)} 个文件，标签已生成。", "items": added}


@app.post("/api/staging/commit")
def api_commit(request: CommitRequest):
    staging = load_staging_index(settings)
    items = staging.get("items", [])
    items_by_id = {item["id"]: item for item in items}

    if not request.items:
        raise HTTPException(status_code=400, detail="未选择任何文档。")

    library_index = load_library_index(settings)
    moved = []

    for req_item in request.items:
        if req_item.id not in items_by_id:
            continue
        item = items_by_id[req_item.id]
        levels = item["levels"]
        extension = item["extension"]
        original_base = Path(item["original_name"]).stem

        if req_item.rename_enabled:
            name = req_item.rename_to.strip() or item.get("suggested_name") or original_base
        else:
            name = original_base

        filename = build_filename(name, extension)
        dest_dir = settings.raw_dir / levels_to_path(levels)
        dest_dir.mkdir(parents=True, exist_ok=True)
        destination = unique_path(dest_dir, filename)

        source = settings.staging_dir / item["stored_name"]
        if not source.exists():
            continue
        shutil.move(str(source), str(destination))
        rel_path = str(destination.relative_to(settings.raw_dir))
        library_index[rel_path] = {
            "levels": levels,
            "tags": item.get("tags", []),
        }
        moved.append(rel_path)

    staging["items"] = [item for item in items if item["id"] not in {i.id for i in request.items}]
    save_staging_index(settings, staging)
    save_library_index(settings, library_index)

    with lock:
        rebuild_vector_store()

    return {"message": f"已入库 {len(moved)} 个文档，并重建向量库。", "moved": moved}


@app.post("/api/query")
def api_query(request: QueryRequest):
    question = request.question.strip()
    if not question:
        raise HTTPException(status_code=400, detail="问题不能为空。")

    with lock:
        store = load_or_create_store()
        if store is None:
            return {"answer": "当前没有可用文档。", "sources": []}

    retriever = get_retriever(store, settings)
    documents = []
    if hasattr(retriever, "get_relevant_documents"):
        documents = retriever.get_relevant_documents(question)
    else:
        documents = retriever.invoke(question)
    context = format_documents(documents)
    prompt = build_prompt(question, context)
    answer = llm.invoke(prompt)

    sources = [clean_source(doc.metadata.get("source", "")) for doc in documents]
    sources = sorted({s for s in sources if s})

    return {"answer": answer.strip(), "sources": sources}
