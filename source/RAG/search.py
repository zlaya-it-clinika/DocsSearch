"""
Поиск по базе знаний (без LLM): что найдётся по вопросу.

    python search.py "Когда регистрируются входящие документы?"
    python search.py            # интерактивный режим

Функции search() / search_many() используют следующие этапы (ответы модели).
"""
import json
import sys

import numpy as np
import ollama

from config import DATA_DIR, VECTORS_PATH, EMBED_MODEL, EMBED_OPTIONS, OLLAMA_HOST, TOP_K

_client = ollama.Client(host=OLLAMA_HOST)
_chunks = json.loads((DATA_DIR / "chunks.json").read_text(encoding="utf-8"))
_vectors = np.load(VECTORS_PATH)
assert len(_chunks) == len(_vectors), "chunks.json и vectors.npy не совпадают — запустите python ingest.py"


def search_many(queries: list[str], k: int = TOP_K) -> list[list[dict]]:
    """Несколько запросов за один вызов модели эмбеддингов (быстрее, чем по одному)."""
    q = np.array(_client.embed(model=EMBED_MODEL, input=queries, options=EMBED_OPTIONS)["embeddings"],
                 dtype=np.float32)
    q /= np.linalg.norm(q, axis=1, keepdims=True)
    results = []
    for scores in q @ _vectors.T:                # косинусная близость со всеми фрагментами
        top = np.argsort(-scores)[:k]
        results.append([{**_chunks[i], "score": round(float(scores[i]), 3)} for i in top])
    return results


def search(query: str, k: int = TOP_K) -> list[dict]:
    return search_many([query], k)[0]


def show(query: str):
    print(f"\nВопрос: {query}")
    for h in search(query):
        first = h["text"].replace("\n", " ")[:100]
        print(f"  {h['score']:.2f} | {h['doc']}, п. {h['clause']} | {first}...")


if __name__ == "__main__":
    if len(sys.argv) > 1:
        show(" ".join(sys.argv[1:]))
    else:
        while (q := input("\nВопрос (Enter — выход): ").strip()):
            show(q)
