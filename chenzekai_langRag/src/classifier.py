import json
import re
from typing import List, Dict, Any

from src.generator import get_llm


def _extract_json(text: str) -> Dict[str, Any]:
    match = re.search(r"\{.*\}", text, re.S)
    if not match:
        raise ValueError("No JSON object found")
    return json.loads(match.group(0))


def _sanitize_segment(value: str) -> str:
    cleaned = re.sub(r"[\\/\n\r\t]+", "_", value).strip()
    cleaned = re.sub(r"[<>:""|?*]", "_", cleaned)
    return cleaned or "未分类"


def _pad_levels(levels: List[str], depth: int) -> List[str]:
    levels = [lvl for lvl in levels if lvl]
    while len(levels) < depth:
        levels.append("未分类")
    return levels[:depth]


def classify_document(text: str, settings) -> Dict[str, Any]:
    llm = get_llm(settings)
    prompt = (
        "你是文档分类助手。请根据内容给出6层级分类路径，并生成3-6个中文标签。\n"
        "仅输出JSON，格式如下：\n"
        "{\"levels\":[\"一级\",\"二级\",\"三级\",\"四级\",\"五级\",\"六级\"],"
        "\"tags\":[\"标签1\",\"标签2\"],"
        "\"confidence\":0.0,"
        "\"title\":\"推荐文件名(不含扩展名)\"}\n\n"
        f"文档内容:\n{text}\n"
    )
    raw = llm.invoke(prompt)
    try:
        data = _extract_json(raw)
    except Exception:
        data = {}

    levels = [_sanitize_segment(x) for x in data.get("levels", []) if isinstance(x, str)]
    levels = _pad_levels(levels, settings.hierarchy_depth)
    tags = [str(x).strip() for x in data.get("tags", []) if str(x).strip()]
    confidence = float(data.get("confidence", 0.0) or 0.0)
    title = _sanitize_segment(str(data.get("title", "")))

    return {
        "levels": levels,
        "tags": tags,
        "confidence": max(0.0, min(1.0, confidence)),
        "title": title,
    }


def classify_query(query: str, settings) -> Dict[str, Any]:
    llm = get_llm(settings)
    prompt = (
        "你是问题意图识别助手。请将问题映射到6层级分类路径。\n"
        "仅输出JSON，格式如下：\n"
        "{\"levels\":[\"一级\",\"二级\",\"三级\",\"四级\",\"五级\",\"六级\"],"
        "\"confidence\":0.0}\n\n"
        f"问题: {query}\n"
    )
    raw = llm.invoke(prompt)
    try:
        data = _extract_json(raw)
    except Exception:
        data = {}

    levels = [_sanitize_segment(x) for x in data.get("levels", []) if isinstance(x, str)]
    levels = _pad_levels(levels, settings.hierarchy_depth)
    confidence = float(data.get("confidence", 0.0) or 0.0)

    return {
        "levels": levels,
        "confidence": max(0.0, min(1.0, confidence)),
    }
