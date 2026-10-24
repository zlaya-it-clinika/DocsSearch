"""
Этап 3. Ответ на вопрос: поиск + Vikhr-Nemo в режиме Grounded RAG.

	python rag.py                       # диалог в консоли
	python rag.py --debug               # то же + внутренние шаги
	python rag.py "Когда регистрируются входящие документы?"

Защита от выдумок:
  1. поиск: вопрос не по теме (низкая релевантность) -> отказ без обращения к модели;
  2. шаг 1 модели: выбирает номера документов с ответом (relevant_doc_ids);
	 пустой список -> ближайший пункт дословно (если поиск уверен) или честный отказ;
  3. шаг 2 модели: отвечает только по выбранным документам;
  4. к документам добавляется справочник сокращений из п. 1.4 - модель не «расшифровывает» сама.
Ссылки на источники берутся из relevant_doc_ids, а не из текста ответа.
"""
import json
import re
import sys
import time

import ollama

from config import (DATA_DIR, LLM_MODEL, OLLAMA_HOST, MIN_SCORE, MIN_SCORE_ALONE,
					FALLBACK_SCORE, TOP_K)
from search import search_many

_llm = ollama.Client(host=OLLAMA_HOST)
DEBUG = "--debug" in sys.argv

try:
	GLOSSARY = json.loads((DATA_DIR / "glossary.json").read_text(encoding="utf-8"))
except FileNotFoundError:
	GLOSSARY = {}

# Системный промпт режима Grounded RAG из карточки модели + наши правила.
# Авторы рекомендуют писать системный промпт на английском; отвечать модель будет по-русски.
SYSTEM_PROMPT = (
	"Your task is to answer the user's questions using only the information from "
	"the provided documents. Give two answers to each question: one with a list of "
	"relevant document identifiers and the second with the answer to the question "
	"itself, using documents with these identifiers.\n"
	"Additional rules. A document is relevant if it contains information needed for the answer, "
	"even if it uses other words than the question. Documents about a different subject "
	"(for example, outgoing instead of incoming documents) are not relevant. "
	"Answer in Russian, in plain simple language, briefly: 2-5 sentences, only what was asked. "
	"Do not write introductions, conclusions or summaries. "
	"Do not explain why a rule exists unless the documents say so. "
	"Keep abbreviations exactly as written; if needed, take their meaning only from the "
	"glossary document. Do not add facts that are not in the documents. "
	"If the user needs to perform actions, list them as numbered steps."
)

# Ollama по умолчанию даёт маленькое окно контекста - 6 фрагментов в него не помещаются
LLM_OPTIONS = {"num_ctx": 8192, "top_k": 40}

REFUSAL = ("В базе знаний нет информации по этому вопросу. Попробуйте переформулировать "
		   "его или обратитесь в Департамент ДОУ.")


def source_title(h: dict) -> str:
	if h["doc"] == "Матрица визирования":
		return f"Матрица визирования, строка {h['clause']}"
	if h["doc"] == "Модули документов":
		return f"Модульный документ «{h['clause'].replace('_', ' ')}»"
	section = ""
	if h.get("chapter") and h["chapter"] != h["clause"] and not h["chapter"].startswith("Глава"):
		section = f" (раздел «{h['chapter']}»)"
	return f"{h['doc']}, п. {h['clause']}{section}"


def glossary_doc(hits: list[dict], doc_id: int) -> dict | None:
	"""Справочник только тех сокращений, что встречаются в найденных фрагментах."""
	text = " ".join(h["text"] for h in hits)
	used = {a: t for a, t in GLOSSARY.items() if re.search(rf"(?<!\w){re.escape(a)}(?!\w)", text)}
	if not used:
		return None
	body = "; ".join(f"{a} - {t}" for a, t in used.items())
	return {"doc_id": doc_id, "title": "Инструкция по ДОУ, п. 1.4 (сокращения)", "content": body}


def parse_ids(text: str, max_id: int) -> list[int] | None:
	"""Достаёт relevant_doc_ids из ответа шага 1. None - если ответ не распознан.

	Понимает {"relevant_doc_ids": [0, 2]}, номера-строки ["0", "2"] и просто список [0, 2].
	"""
	ids = None
	m = re.search(r"\{.*?\}", text, re.S)
	if m:
		try:
			ids = json.loads(m.group(0)).get("relevant_doc_ids")
		except (json.JSONDecodeError, AttributeError):
			ids = None
	if ids is None:
		m = re.search(r"\[[\d\s,\"']*\]", text)
		if not m:
			return None
		ids = re.findall(r"\d+", m.group(0))
	if not isinstance(ids, list):
		return None
	out = []
	for i in ids:
		try:
			i = int(i)
		except (TypeError, ValueError):
			continue
		if 0 <= i < max_id and i not in out:       # отбрасываем несуществующие и повторы
			out.append(i)
	return out


def retrieve(question: str, history: list[dict]) -> tuple[list[dict], float]:
	"""Поиск с учётом диалога.

	Уточняющий вопрос («а если вечером?») ищем вместе с предыдущим вопросом,
	но порог проверяем и по самому вопросу: иначе вопрос не по теме («пирог»)
	«прилипает» к теме предыдущего и проходит фильтр.
	"""
	prev = [m["content"] for m in history if m["role"] == "user"][-1:]
	queries = [question] + ([" ".join(prev + [question])] if prev else [])
	results = search_many(queries, TOP_K)
	alone, hits = results[0], results[-1]
	best_alone = alone[0]["score"] if alone else 0.0
	if DEBUG:
		print(f"  [поиск] лучший по самому вопросу: {best_alone:.2f}; "
			  f"с контекстом: {hits[0]['score'] if hits else 0:.2f}")
	if best_alone < MIN_SCORE_ALONE:
		return [], best_alone
	return [h for h in hits if h["score"] >= MIN_SCORE], best_alone


def closest_clause(h: dict) -> str:
	return ("Точного ответа сформулировать не удалось. Ближайший по смыслу пункт - "
			f"{source_title(h)}:\n\n{h['text']}")


def answer(question: str, history: list[dict] | None = None) -> dict:
	history = history or []
	t0 = time.time()
	timing = {}

	def result(text, sources=(), reason=None):
		return {"answer": text, "sources": list(sources), "reason": reason,
				"seconds": round(time.time() - t0, 1), "timing": timing}

	hits, _ = retrieve(question, history)
	timing["поиск"] = round(time.time() - t0, 1)
	if not hits:
		return result(REFUSAL, reason="вопрос не относится к базе знаний")

	docs = [{"doc_id": i, "title": source_title(h), "content": h["text"]} for i, h in enumerate(hits)]
	gloss = glossary_doc(hits, len(docs))
	if gloss:
		docs.append(gloss)
	messages = (
		[{"role": "system", "content": SYSTEM_PROMPT},
		 {"role": "documents", "content": json.dumps(docs, ensure_ascii=False)}]
		+ history[-2:]          # только предыдущий обмен: длинная история замедляет и путает модель
		+ [{"role": "user", "content": question}]
	)

	# Шаг 1: какие документы релевантны (один повтор, если ответ не распознан)
	t1 = time.time()
	ids = None
	for attempt in range(2):
		r1 = _llm.chat(model=LLM_MODEL, messages=messages,
					   options={**LLM_OPTIONS, "temperature": 0.0, "num_predict": 64})
		step1 = r1["message"]["content"]
		ids = parse_ids(step1, len(docs))
		if DEBUG:
			print(f"  [шаг 1, попытка {attempt + 1}] модель ответила: {step1[:200]!r} -> {ids}")
		if ids is not None:
			break
	timing["шаг 1"] = round(time.time() - t1, 1)

	content_ids = [i for i in (ids or []) if i < len(hits)]     # без справочника сокращений
	if not content_ids:
		top = hits[0]
		if top["score"] >= FALLBACK_SCORE:   # поиск уверен - показываем пункт дословно, без генерации
			src = {"n": 1, "title": source_title(top), "relevance": int(top["score"] * 100),
				   "text": top["text"]}
			return result(closest_clause(top), [src], reason="модель не выбрала документ; показан пункт дословно")
		why = "модель не нашла ответа в документах" if ids == [] else "ответ модели на шаге 1 не распознан"
		return result(REFUSAL, reason=why)

	# Шаг 2: ответ по выбранным документам (+ справочник сокращений, если он есть)
	t2 = time.time()
	step2_ids = content_ids + ([gloss["doc_id"]] if gloss else [])
	messages.append({"role": "assistant", "content": json.dumps({"relevant_doc_ids": step2_ids})})
	r2 = _llm.chat(model=LLM_MODEL, messages=messages,
				   options={**LLM_OPTIONS, "temperature": 0.3, "num_predict": 400})
	timing["шаг 2"] = round(time.time() - t2, 1)

	sources = [{"n": n, "title": docs[i]["title"], "relevance": int(hits[i]["score"] * 100),
				"text": hits[i]["text"]} for n, i in enumerate(content_ids, 1)]
	return result(r2["message"]["content"].strip(), sources)


def print_result(r: dict):
	print("\n" + r["answer"])
	if r["sources"]:
		print("\nИсточники:")
		for s in r["sources"]:
			print(f"  [{s['n']}] {s['title']} - релевантность {s['relevance']}%")
	if r["reason"]:
		print(f"  (примечание: {r['reason']})")
	steps = ", ".join(f"{k} {v} с" for k, v in r.get("timing", {}).items())
	print(f"  ({r['seconds']} с: {steps})")


if __name__ == "__main__":
	args = [a for a in sys.argv[1:] if a != "--debug"]
	if args:
		print_result(answer(" ".join(args)))
		sys.exit()

	history = []
	print("Помощник по документообороту. Enter на пустой строке - выход.")
	while (q := input("\nВопрос: ").strip()):
		r = answer(q, history)
		print_result(r)
		history += [{"role": "user", "content": q}, {"role": "assistant", "content": r["answer"]}]
