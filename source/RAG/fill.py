"""
Этап 4. Подготовка проекта документа на основе модуля - диалог в консоли.

	python fill.py              # выбрать модуль из списка и ответить на вопросы
	python fill.py --demo       # заполнить приказ о вводе ИС тестовыми данными (для проверки)

Готовый проект: data/out/Проект_....docx
"""
import sys

import ollama

from config import OLLAMA_HOST
from templates import FillSession, list_templates, template_title, matrix_route

DEMO_ANSWERS = [   # приказ о вводе в эксплуатацию ИС - для быстрой проверки
	"Автоматизированной системы учета обращений граждан",
	"протокола приемочных испытаний от 15.10.2026 № 12-ПИ",
	"Автоматизированная система учета обращений граждан",
	"АС УОГ",
	"разработанного",
	"технического задания от 01.03.2026 № ТЗ-45 «Автоматизированная система учета обращений граждан»",
	"01.11.2026",
	"Департаменту по работе с обращениями граждан",
	"регистрация, учет и контроль рассмотрения обращений граждан",
	"директор Департамента информационных технологий Иванов И. И.",
	"А. Б. Петров",
]


def ask(prompt: str) -> str:
	try:
		return input(prompt)
	except EOFError:
		return ""


def choose_template(templates: list[dict]) -> dict:
	print("Какой документ нужно подготовить?")
	for i, t in enumerate(templates, 1):
		print(f"  {i}. {template_title(t)}")
	while True:
		s = ask("Номер: ").strip()
		if s.isdigit() and 1 <= int(s) <= len(templates):
			return templates[int(s) - 1]
		print("Введите номер из списка.")


def run(template: dict, scripted: list[str] | None = None):
	llm = ollama.Client(host=OLLAMA_HOST)
	s = FillSession(template, llm=llm)
	print(f"\nГотовим: {template_title(template)}")
	print("Отвечайте на вопросы. Подсказка - правило из примечаний к модулю.\n")

	demo = scripted is not None
	scripted = list(scripted or [])
	shown = 0
	while (q := s.next_question()) is not None:
		for line in s.log[shown:]:
			if line.startswith("Поле"):
				print(f"  ✓ {line}")
		shown = len(s.log)
		opt = " (необязательное, Enter - пропустить)" if q["optional"] else ""
		print(f"[{s.progress()}] {q['name']}{opt}")
		print(f"   Подсказка: {q['hint']}")
		while True:
			value = scripted.pop(0) if scripted else ask("   > ")
			if demo and value:
				print(f"   > {value}")
			try:
				s.answer(value)
				break
			except ValueError as e:
				print(f"   {e}")

	print("\nСобираю документ…")
	res = s.build()
	print(f"\nГотово: {res['path']}")
	for line in res["log"]:
		if line.startswith("Абзац"):
			print(f"  • {line}")
	if res["problems"]:
		print("\nТребуют проверки:")
		for p in res["problems"]:
			print(f"  ! {p}")

	from check import print_report
	print_report(res["path"], res["check"])

	# Маршрут визирования по Матрице
	route = matrix_route(template)
	deps = list(route["always"])
	if route["condition"]:
		print(f"\nУточнение для маршрута: {route['condition']}")
		if ask("   Это ваш случай? (да/нет): ").strip().lower().startswith("д"):
			deps.append(route["condition"].split(" визирует")[0])
	print(f"\nМаршрут визирования (Матрица, строка {route['row']}):")
	print("  1. Ответственный исполнитель, участвовавшие в подготовке работники, руководитель подразделения")
	for i, d in enumerate(deps, 2):
		print(f"  {i}. {d}")
	print("  + лингвистическая экспертиза (Департамент ДОУ) - п. 4.2 Методических рекомендаций")
	print("\nВ поле «Примечание» регистрационной карточки (п. 2.3 Методических рекомендаций):")
	print(f"  «Проект подготовлен на основе модуля документа: {template['id'].replace('_', ' ')}, "
		  f"регистрационный № _____ от __.__.____, редакция (версия) _____»")


if __name__ == "__main__":
	templates = list_templates()
	if not templates:
		sys.exit("Нет шаблонов - сначала запустите python ingest.py")
	if "--demo" in sys.argv:
		t = next(t for t in templates if "эксплуатаци" in t["id"])
		run(t, DEMO_ANSWERS)
	else:
		run(choose_template(templates))
