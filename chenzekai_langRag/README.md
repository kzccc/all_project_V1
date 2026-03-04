# 层次化 RAG 知识库（Ollama）

一个可落地的层次化 RAG：文档自动分类（6 层级）+ 结构化存储 + 结构感知检索 + LLM 生成。

## 目录结构

```
rag-project/
├── data/
│   ├── raw/
│   │   └── <层级目录>
│   ├── staging/            # 上传暂存区（确认后入库）
│   └── processed/
├── vector_store/
│   ├── chroma/
│   └── metadata.json
├── src/
│   ├── loader.py
│   ├── splitter.py
│   ├── embedder.py
│   ├── store.py
│   ├── retriever.py
│   ├── generator.py
│   └── main.py
├── config/
│   └── settings.py
├── requirements.txt
├── .env
└── README.md
```

## 使用

1) 安装依赖

```
pip install -r requirements.txt
```

2) 配置模型（`.env`）

```
OLLAMA_BASE_URL=http://localhost:11434
OLLAMA_MODEL=deepseek-r1:32b
OLLAMA_EMBED_MODEL=nomic-embed-text
RETRIEVER_TYPE=mmr
MMR_FETCH_K=20
MMR_LAMBDA_MULT=0.5
SCORE_THRESHOLD=
```

3) 放入文档

- 推荐使用 Web UI 上传并确认入库。
- 也可以手动放入 `data/raw` 下的层级目录。

4) 构建向量库并交互式提问

```
python -m src.main --ingest
```

或直接提问：

```
python -m src.main --query "你的问题"
```

## 说明

- 向量库默认使用 Chroma，存储在 `vector_store/chroma`。
- 如果需要强制重建向量库：`python -m src.main --rebuild`。
- Markdown 文件会先按标题切分，再做递归切分，提升主题一致性。
- 检索默认使用层次化检索（`RETRIEVER_TYPE=hierarchical`）。

## Web UI

启动 FastAPI 前端页面（端口 6666）：

```
uvicorn src.server:app --host 0.0.0.0 --port 6666
```

功能：

- 批量上传 → 自动分类与标签 → 确认后入库
- 层级文档库列表
- 结构化检索问答
