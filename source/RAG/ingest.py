"""
Этап 2. Загрузка материалов кейса в базу знаний.

Запуск:
    python ingest.py            # разобрать документы и построить векторную базу
    python ingest.py --no-embed # только разобрать (без Ollama) — для проверки нарезки

Результат:
    data/chunks.json        — все фрагменты (можно открыть и проверить глазами)
    data/matrix.json        — матрица визирования в структурированном виде
    data/templates/*.json   — описание полей каждого модуля
    data/vectors.npy        — эмбеддинги фрагментов (в том же порядке, что chunks.json)
"""
import argparse
import json
import sys
import unicodedata

from config import (SOURCE_DIR, DATA_DIR, VECTORS_PATH, EMBED_MODEL, EMBED_OPTIONS,
                    OLLAMA_HOST, MAX_CHARS)
from extract import (read_pdf_clean, read_docx_lines, chunk_by_clauses,
                     parse_matrix, matrix_row_text, parse_module, parse_glossary)


def nfc(s: str) -> str:
    """Windows хранит «й» в именах файлов как «и» + знак краткости — приводим к обычному виду."""
    return unicodedata.normalize("NFC", s)


def find(pattern: str):
    files = sorted(SOURCE_DIR.glob(pattern))
    if not files:
        sys.exit(f"Не найден файл по шаблону {pattern} в {SOURCE_DIR}")
    return files


def build_chunks() -> list[dict]:
    chunks = []

    # 1. Инструкция по ДОУ (PDF с водяным знаком)
    pdf = find("Инструкция*.pdf")[0]
    ch = chunk_by_clauses(read_pdf_clean(pdf), "Инструкция по ДОУ", MAX_CHARS)
    print(f"  Инструкция: {len(ch)} фрагментов")
    glossary = parse_glossary(ch)
    (DATA_DIR / "glossary.json").write_text(json.dumps(glossary, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"  Сокращения из п. 1.4: {len(glossary)} -> data/glossary.json")
    chunks += ch

    # 2. Методические рекомендации по модульным документам
    for f in find("Методические*.docx"):
        ch = chunk_by_clauses(read_docx_lines(f), "Методические рекомендации по модульным документам",
                              MAX_CHARS, numbered_sections=True)
        print(f"  Методические рекомендации: {len(ch)} фрагментов")
        chunks += ch

    # 3. Матрица визирования: JSON для точного поиска кодом + строки для поиска по смыслу
    matrix = parse_matrix(find("Матрица*.docx")[0])
    (DATA_DIR / "matrix.json").write_text(json.dumps(matrix, ensure_ascii=False, indent=2), encoding="utf-8")
    for row in matrix["rows"]:
        chunks.append({"doc": "Матрица визирования", "chapter": row["group"],
                       "clause": row["id"], "part": None, "text": matrix_row_text(row)})
    chunks.append({"doc": "Матрица визирования", "chapter": "правила применения",
                   "clause": "общие положения", "part": None, "text": "\n".join(matrix["rules"])})
    print(f"  Матрица: {len(matrix['rows'])} строк -> data/matrix.json")

    # 4. Модули: описание полей + короткий фрагмент «какой модуль для чего»
    tdir = DATA_DIR / "templates"
    tdir.mkdir(parents=True, exist_ok=True)
    for f in find("Модуль*.docx"):
        mod = parse_module(f)
        stem = nfc(f.stem)
        mod["source"] = nfc(mod["source"])
        (tdir / f"{stem}.json").write_text(json.dumps(mod, ensure_ascii=False, indent=2), encoding="utf-8")
        fields = "; ".join(fl["name"] for fl in mod["fields"])
        chunks.append({"doc": "Модули документов", "chapter": mod["kind"], "clause": stem, "part": None,
                       "text": f"Модульный документ: {mod['kind'].lower()} «{mod['title_pattern']}». "
                               f"Файл: {mod['source']}. Заполняемые поля: {fields}."})
        print(f"  Модуль: {stem} — {len(mod['fields'])} полей")

    for i, c in enumerate(chunks):
        c["id"] = i
    return chunks


def embed_and_store(chunks: list[dict]):
    """Считает эмбеддинги и сохраняет их в data/vectors.npy.

    Для сотен фрагментов отдельная векторная БД не нужна: поиск — это одно
    матричное умножение. При росте базы этот файл заменяется на Qdrant/Chroma.
    """
    import numpy as np
    import ollama

    client = ollama.Client(host=OLLAMA_HOST)
    # В эмбеддинг добавляем «адрес» фрагмента: глава + пункт улучшают поиск
    texts = [f"{c['doc']}. {c['chapter']}\n{c['text']}" for c in chunks]
    vectors, batch = [], 16
    for i in range(0, len(texts), batch):
        vectors += client.embed(model=EMBED_MODEL, input=texts[i:i + batch],
                                options=EMBED_OPTIONS)["embeddings"]
        print(f"\r  Эмбеддинги: {min(i + batch, len(texts))}/{len(texts)}", end="")
    print()

    m = np.array(vectors, dtype=np.float32)
    m /= np.linalg.norm(m, axis=1, keepdims=True)   # нормируем: косинус = скалярное произведение
    np.save(VECTORS_PATH, m)
    print(f"  Векторы: {m.shape[0]} x {m.shape[1]} -> {VECTORS_PATH.name}")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--no-embed", action="store_true", help="только разобрать документы")
    args = ap.parse_args()

    DATA_DIR.mkdir(exist_ok=True)
    print(f"Читаю материалы из {SOURCE_DIR}")
    chunks = build_chunks()
    (DATA_DIR / "chunks.json").write_text(json.dumps(chunks, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"Всего фрагментов: {len(chunks)} -> data/chunks.json")

    if not args.no_embed:
        embed_and_store(chunks)
        print("Готово: база знаний построена.")


if __name__ == "__main__":
    main()
