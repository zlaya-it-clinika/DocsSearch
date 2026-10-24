"""
Извлечение текста и структуры из материалов кейса.

  * read_pdf_clean()  — текст PDF без серого водяного знака
  * chunk_by_clauses()— нарезка по главам и пунктам (1.1., 4.1.2. ...)
  * parse_matrix()    — матрица визирования -> список строк с условиями
  * parse_module()    — модульный документ -> описание полей для заполнения
"""
import re
from pathlib import Path

import pdfplumber
from docx import Document

CHAPTER_RE = re.compile(r"^(Глава\s+\d+\.|Приложение\s*№?\s*\d+)")
CLAUSE_RE = re.compile(r"^(\d+(?:\.\d+)+)\.\s")       # 1.1.  4.1.2.
NOTE_RE = re.compile(r"^(\d+)\.\s+([^.]{2,80})\.\s*(.*)$")  # "8. Дата ввода. Укажите ..."
FIELD_RE = re.compile(r"\[(\d+)\]")


# ---------------------------------------------------------------- PDF
def _is_watermark(obj) -> bool:
    """Водяной знак в инструкции набран серым цветом (0.45, 0.45, 0.45)."""
    if obj.get("object_type") != "char":
        return False
    col = obj.get("non_stroking_color")
    return (isinstance(col, (tuple, list)) and len(col) == 3
            and all(abs(c - 0.45) < 0.05 for c in col))


def read_pdf_clean(path: Path) -> list[str]:
    lines = []
    with pdfplumber.open(path) as pdf:
        for page in pdf.pages:
            text = page.filter(lambda o: not _is_watermark(o)).extract_text() or ""
            lines += text.splitlines()
    return [l.strip() for l in lines if l.strip() and not l.strip().isdigit()]  # без номеров страниц


# ---------------------------------------------------------------- DOCX
def read_docx_lines(path: Path) -> list[str]:
    doc = Document(path)
    lines = [p.text.strip() for p in doc.paragraphs]
    return [l for l in lines if l]


def docx_tables(path: Path) -> list[list[list[str]]]:
    doc = Document(path)
    return [[[c.text.strip() for c in row.cells] for row in t.rows] for t in doc.tables]


# ---------------------------------------------------------------- нарезка
SECTION_RE = re.compile(r"^\d+\.\s+[А-ЯЁ][^.]{3,80}$")   # «3. Подготовка файла проекта» (в методичках)


def chunk_by_clauses(lines: list[str], doc_title: str, max_chars: int,
                     numbered_sections: bool = False) -> list[dict]:
    """Один фрагмент = один пункт. К тексту добавляется название главы — это улучшает поиск."""
    chunks, buf = [], []
    chapter, clause = "", "вводная часть"
    pending_chapter = False

    def flush():
        if not buf:
            return
        body = "\n".join(buf)
        parts = [body[i:i + max_chars] for i in range(0, len(body), max_chars)]
        for n, part in enumerate(parts, 1):
            chunks.append({
                "doc": doc_title, "chapter": chapter, "clause": clause,
                "part": n if len(parts) > 1 else None,
                "text": part,
            })
        buf.clear()

    for line in lines:
        if CHAPTER_RE.match(line) or (numbered_sections and SECTION_RE.match(line)):
            flush()
            chapter, clause, pending_chapter = line, line, not numbered_sections
            continue
        m = CLAUSE_RE.match(line)
        if m:
            flush()
            clause, pending_chapter = m.group(1), False
            buf.append(line)
        elif pending_chapter and not buf:
            chapter += " " + line          # перенос длинного названия главы
            clause = chapter
        else:
            pending_chapter = False
            buf.append(line)
    flush()
    return chunks


# ---------------------------------------------------------------- сокращения
ABBR_RE = re.compile(r"([^;:]+?)\s*\(([А-ЯЁA-Z][^()]{0,25})\)\s*—")


def parse_glossary(chunks: list[dict]) -> dict[str, str]:
    """Сокращения из п. 1.4 Инструкции: {"ИИС ДО": "Интегрированная информационная система ..."}."""
    text = " ".join(c["text"] for c in chunks if c["clause"] == "1.4")
    text = re.sub(r"\s+", " ", text)
    glossary = {}
    for term, abbr in ABBR_RE.findall(text):
        term = term.strip().split("сокращения")[-1].strip(" :")
        abbr = re.sub(r"\s+", " ", abbr).strip()
        if term and len(abbr) <= 25:
            glossary[abbr] = term
    return glossary


# ---------------------------------------------------------------- матрица
def parse_matrix(path: Path) -> dict:
    rows = docx_tables(path)[0]
    header = rows[0]
    departments = ["Юридическое подразделение", "Департамент ДОУ", "Кадровое подразделение"]
    group, items = None, []
    for row in rows[1:]:
        if len(set(row)) == 1:               # строка-заголовок группы (объединённые ячейки)
            group = row[0].strip().lower()   # "внутренние документы" / "внешние документы"
            continue
        m = re.match(r"^(\d+)\.\s*(.*)$", row[0], re.S)
        items.append({
            "id": f"{'ВН' if 'внутр' in group else 'ИСХ'}-{m.group(1) if m else len(items) + 1}",
            "group": group,
            "doc_type": m.group(2) if m else row[0],
            "visas": {dep: row[i + 1] for i, dep in enumerate(departments)},
            "other": re.sub(r"\s*\n\s*", ", ", row[4].strip()) or None,  # прочие департаменты / условие
        })
    notes = [l for l in read_docx_lines(path) if re.match(r"^\d+\.|^Примечание", l)]
    return {"source": path.name, "header": header, "rules": notes, "rows": items}


def matrix_row_text(row: dict) -> str:
    """Строка матрицы в виде текста — для поиска по смыслу."""
    yes = [k for k, v in row["visas"].items() if v.strip().lower() == "да"]
    no = [k for k, v in row["visas"].items() if v.strip().lower() == "нет"]
    cond = [f"{k} ({v})" for k, v in row["visas"].items() if v.strip().lower() not in ("да", "нет", "")]
    parts = [f"Матрица обязательного визирования ({row['group']}), строка {row['id']}: {row['doc_type']}."]
    if yes:
        parts.append("Обязательно визируют: " + ", ".join(yes) + ".")
    if cond:
        parts.append("Визируют при условии: " + "; ".join(cond) + ".")
    if row["other"]:
        o = row["other"].strip().rstrip(".")
        if len(o.split()) <= 8:                      # просто перечень департаментов
            parts.append(f"Также обязательно визируют: {o}.")
        else:                                        # развёрнутое условие
            parts.append(f"Дополнительно: {o}.")
    if no:
        parts.append("Не требуется обязательная виза по Матрице: " + ", ".join(no) + ".")
    parts.append("Матрица применяется дополнительно к общему маршруту визирования по Инструкции.")
    return " ".join(parts)


# ---------------------------------------------------------------- модули
def parse_module(path: Path) -> dict:
    lines = read_docx_lines(path)
    tables = docx_tables(path)
    table_lines = [" | ".join(c for c in dict.fromkeys(r) if c) for t in tables for r in t]

    try:
        notes_start = next(i for i, l in enumerate(lines) if l.startswith("Примечания по заполнению"))
    except StopIteration:
        raise ValueError(f"{path.name}: не найден раздел «Примечания по заполнению»")
    rules_start = next((i for i, l in enumerate(lines) if l.startswith("Общие правила")), len(lines))

    body = lines[:notes_start]
    doc_kind = next((l for l in body if l in ("ПРИКАЗ", "РАСПОРЯЖЕНИЕ")), "")
    title = next((l for l in body if l.startswith(("О ", "Об "))), path.stem)

    # где в тексте стоит каждое поле [N] — даёт модели контекст
    context = {}
    for l in body + table_lines:
        for n in FIELD_RE.findall(l):
            context.setdefault(int(n), l)

    fields = []
    for l in lines[notes_start + 1:rules_start]:
        m = NOTE_RE.match(l)
        if not m:
            continue
        n, name, instr = int(m.group(1)), m.group(2), m.group(3)
        fields.append({
            "id": n,
            "name": name,
            "instruction": instr,
            "required": "необязательный" not in instr.lower(),
            "context": context.get(n, ""),
        })

    general_note = lines[notes_start + 1] if notes_start + 1 < rules_start else ""
    rules = lines[rules_start + 1:]
    return {
        "source": path.name,
        "kind": doc_kind,
        "title_pattern": title,
        "general_note": general_note,
        "fields": fields,
        "rules": rules,
        "body": body,
    }
