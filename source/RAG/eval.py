"""
Прогон тестовых вопросов через систему и подсчёт точности.

	python eval.py                 # все вопросы из tests.json
	python eval.py --cat матрица   # только одна категория
	python eval.py --id 30 36      # отдельные вопросы

Что проверяется:
  * источник - нужный пункт есть среди источников ответа (поле src);
  * факт     - в ответе есть хотя бы один ключевой фрагмент (поле kw);
  * отказ    - на вопрос вне базы система честно отказала (refuse: true).
Вопросы категории «уточнение» не оцениваются автоматически: смотрите ответ глазами.

Результат: таблица в консоли и data/eval_ДАТА.csv (открывается в Excel).
"""
import argparse
import csv
import json
import re
import time
from datetime import datetime
from pathlib import Path

from config import DATA_DIR
from rag import answer, REFUSAL

TESTS = Path(__file__).parent / "tests.json"


def run_one(t: dict) -> dict:
	history = []
	for h in t.get("history", []):
		r = answer(h, history)
		history += [{"role": "user", "content": h}, {"role": "assistant", "content": r["answer"]}]
	t0 = time.time()
	r = answer(t["q"], history)
	sec = round(time.time() - t0, 1)

	titles = " | ".join(s["title"] for s in r["sources"])
	text = r["answer"]
	refused = text.startswith(REFUSAL[:40]) or (not r["sources"] and r["reason"])

	if t.get("refuse"):
		src_ok = None
		fact_ok = None
		verdict = "OK" if refused else "ОШИБКА: ответил вместо отказа"
	elif t["cat"] == "уточнение":
		src_ok = fact_ok = None
		verdict = "смотреть вручную"
	else:
		# «п. 7.1» не должен совпадать с «п. 7.10»
		src_ok = any(re.search(re.escape(s) + r"(?![\d.])", titles) for s in t["src"]) if t["src"] else None
		fact_ok = any(k.lower() in text.lower() for k in t["kw"]) if t["kw"] else None
		if refused:
			verdict = "ОШИБКА: ложный отказ"
		elif src_ok and (fact_ok is not False):
			verdict = "OK"
		elif src_ok:
			verdict = "источник верный, факт не найден"
		elif fact_ok:
			verdict = "факт верный, источник другой"
		else:
			verdict = "ОШИБКА"
	return {"id": t["id"], "cat": t["cat"], "q": t["q"], "verdict": verdict,
			"src_ok": src_ok, "fact_ok": fact_ok, "sources": titles, "answer": text,
			"reason": r["reason"] or "", "sec": sec}


def main():
	ap = argparse.ArgumentParser()
	ap.add_argument("--cat")
	ap.add_argument("--id", nargs="*", type=int)
	args = ap.parse_args()

	tests = json.loads(TESTS.read_text(encoding="utf-8"))
	if args.cat:
		tests = [t for t in tests if t["cat"] == args.cat]
	if args.id:
		tests = [t for t in tests if t["id"] in args.id]

	results = []
	for t in tests:
		r = run_one(t)
		results.append(r)
		mark = "✓" if r["verdict"] == "OK" else ("?" if r["verdict"] == "смотреть вручную" else "✗")
		print(f"{mark} [{r['id']:>2}] {r['cat']:<14} {r['sec']:>5} с  {r['verdict']:<32} {r['q'][:60]}")

	# итоги
	print("\nИтоги по категориям:")
	cats = sorted({r["cat"] for r in results}, key=lambda c: [x["cat"] for x in results].index(c))
	total_ok = total = 0
	for c in cats:
		rs = [r for r in results if r["cat"] == c and r["verdict"] != "смотреть вручную"]
		if not rs:
			print(f"  {c:<14} - проверить вручную")
			continue
		ok = sum(r["verdict"] == "OK" for r in rs)
		total_ok += ok
		total += len(rs)
		print(f"  {c:<14} {ok}/{len(rs)}")
	if total:
		print(f"  {'ВСЕГО':<14} {total_ok}/{total} = {round(100 * total_ok / total)}%")
	times = [r["sec"] for r in results]
	print(f"  Среднее время ответа: {round(sum(times) / len(times), 1)} с")

	out = DATA_DIR / f"eval_{datetime.now():%Y%m%d_%H%M}.csv"
	with open(out, "w", encoding="utf-8-sig", newline="") as f:
		w = csv.DictWriter(f, fieldnames=list(results[0].keys()), delimiter=";")
		w.writeheader()
		w.writerows(results)
	print(f"\nПодробно (ответы и источники): {out}")


if __name__ == "__main__":
	main()
