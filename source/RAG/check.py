"""
Проверка оформления документа (.docx) по ГОСТ Р 7.0.97-2025 и Инструкции по ДОУ.

	python check.py data\\out\\Проект_....docx      # проверить готовый проект
	python check.py мой_черновик.docx               # проверить любой черновик

Каждое замечание содержит ссылку на пункт ГОСТ или Инструкции.
Где ГОСТ и Инструкция расходятся, применяется правило из config.FORMAT_PRIORITY,
а расхождение показывается отдельной строкой.
"""
import re
import sys
from pathlib import Path

from docx import Document
from docx.enum.text import WD_ALIGN_PARAGRAPH

try:
	from config import FORMAT_PRIORITY
except ImportError:
	FORMAT_PRIORITY = "instruction"

BLANK_RE = re.compile(r"_{3,}|\[\d+\]|\{S\d+\}")
INITIALS_SPACED = re.compile(r"\b[А-ЯЁ]\.\s+[А-ЯЁ]\.")
INITIALS_TIGHT = re.compile(r"\b[А-ЯЁ]\.[А-ЯЁ]\.")
DATE_QUOTED = re.compile(r"«\d{1,2}»\s+[а-я]+\s+\d{4}")
DATE_WORD = re.compile(r"\b(\d{1,2})\s+(января|февраля|марта|апреля|мая|июня|июля|августа|сентября|октября|ноября|декабря)\s+\d{4}")
RED = {"C00000", "FF0000"}


def _font(run, para, doc):
	name = run.font.name or (para.style.font.name if para.style is not None else None) or doc.styles["Normal"].font.name
	size = run.font.size or (para.style.font.size if para.style is not None else None) or doc.styles["Normal"].font.size
	return name, (size.pt if size else None)


def body_paragraphs(doc):
	"""Абзацы основного текста: после заголовка «О …» до конца, без служебных зон."""
	started = False
	for p in doc.paragraphs:
		t = p.text.strip()
		if not t:
			continue
		if not started and re.match(r"^Об?\s", t):
			started = True
			continue
		if started and "НЕ УДАЛЯТЬ" not in t:
			yield p


def check_docx(path: Path) -> list[dict]:
	doc = Document(path)
	issues = []

	def add(level, text, ref):
		issues.append({"level": level, "text": text, "ref": ref})

	# --- поля страницы
	s = doc.sections[0]
	L, R, T, B = (round(x.mm) for x in (s.left_margin, s.right_margin, s.top_margin, s.bottom_margin))
	if L < 30 or R < 20 or T < 25 or B < 15:
		add("ошибка", f"Поля {L}/{R}/{T}/{B} мм (лев/прав/верх/низ); нужно не менее 30/20/25/15",
			"Инструкция, гл. 8 (размеры полей); ГОСТ Р 7.0.97-2025, п. 6.1 - минимум 20/10/20/20, "
			"левое ≥ 30 мм для документов длительного хранения")

	# --- нумерация страниц
	def num_par(part):
		for p in part.paragraphs:
			if any(x.tag.endswith("instrText") and "PAGE" in (x.text or "") for x in p._element.iter()):
				return p
		return None
	fp, hp = num_par(s.footer), num_par(s.header)
	if FORMAT_PRIORITY == "instruction":
		if not fp:
			add("ошибка", "Нет номера страницы в нижнем колонтитуле", "Инструкция, гл. 8")
		elif fp.alignment != WD_ALIGN_PARAGRAPH.RIGHT:
			add("ошибка", "Номер страницы не в правой части нижнего колонтитула", "Инструкция, гл. 8")
	if FORMAT_PRIORITY == "gost":
		if not hp:
			add("ошибка", "Нет номера страницы посередине верхнего поля", "ГОСТ Р 7.0.97-2025, п. 3.2")
		elif hp.alignment != WD_ALIGN_PARAGRAPH.CENTER:
			add("ошибка", "Номер страницы не посередине верхнего поля", "ГОСТ Р 7.0.97-2025, п. 3.2")
	if (fp or hp) and not s.different_first_page_header_footer:
		add("ошибка", "Номер страницы стоит и на первой странице; нумеруются вторая и последующие",
			"ГОСТ Р 7.0.97-2025, п. 3.2; Инструкция, гл. 8")
	add("расхождение", "Номер страницы: Инструкция - внизу справа; ГОСТ - посередине верхнего поля, 10 мм от края. "
		f"Применено: {'Инструкция' if FORMAT_PRIORITY == 'instruction' else 'ГОСТ'}",
		"Инструкция, гл. 8; ГОСТ Р 7.0.97-2025, п. 3.2")

	# --- основной текст
	bad_font, bad_size, bad_ls, not_justified, red, leftovers = set(), set(), set(), [], [], []
	for p in body_paragraphs(doc):
		t = p.text.strip()
		for r in p.runs:
			if not r.text.strip():
				continue
			name, size = _font(r, p, doc)
			if name and name != "Times New Roman":
				bad_font.add(name)
			if size and FORMAT_PRIORITY == "instruction" and size != 13:
				bad_size.add(size)
			if size and not 12 <= size <= 14:
				bad_size.add(size)
			if r.font.color is not None and r.font.color.type is not None and str(r.font.color.rgb) in RED:
				red.append(t[:60])
		ls = p.paragraph_format.line_spacing
		if isinstance(ls, float) and not 1.0 <= ls <= 1.5:
			bad_ls.add(round(ls, 2))
		if t not in ("ПРИКАЗЫВАЮ:", "ОБЯЗЫВАЮ:") and p.alignment not in (WD_ALIGN_PARAGRAPH.JUSTIFY, WD_ALIGN_PARAGRAPH.CENTER):
			not_justified.append(t[:60])
		if BLANK_RE.search(t) or re.search(r"\[[^\]]*/[^\]]*\]", t):
			leftovers.append(t[:80])
		if DATE_QUOTED.search(t):
			add("ошибка", f"Дата в кавычках: «{t[:60]}»", "ГОСТ Р 7.0.97-2025, п. 5.10; Инструкция, п. 8.7 - "
				"форматы 05.06.2024 или 05 июня 2024 г.")
		for m in DATE_WORD.finditer(t):
			if len(m.group(1)) == 1:
				add("замечание", f"День месяца одной цифрой: «{m.group(0)}», по ГОСТ - «0{m.group(0)}»",
					"ГОСТ Р 7.0.97-2025, п. 5.10 (пример: 05 июня 2024 г.)")
		if t in ("ПРИКАЗЫВАЮ:", "ОБЯЗЫВАЮ:") and p.paragraph_format.first_line_indent:
			add("ошибка", f"«{t}» с абзацным отступом", "Инструкция, пп. 9.5.4, 9.7.4")

	if bad_font:
		add("ошибка", f"Шрифт {', '.join(bad_font)} вместо Times New Roman", "Инструкция, гл. 8; ГОСТ, п. 3.3")
	if bad_size:
		add("ошибка", f"Размер шрифта {sorted(bad_size)} пт; Инструкция - 13 пт (ГОСТ допускает 12-14)",
			"Инструкция, гл. 8; ГОСТ Р 7.0.97-2025, п. 3.3")
	if bad_ls:
		add("ошибка", f"Межстрочный интервал {sorted(bad_ls)}; нужно 1,15-1,5", "Инструкция, гл. 8; ГОСТ, п. 3.4")
	if not_justified:
		add("ошибка", f"Текст не выровнен по ширине ({len(not_justified)} абз.), например: «{not_justified[0]}»",
			"ГОСТ Р 7.0.97-2025, п. 3.5")
	if red:
		add("ошибка", f"Остался красный текст шаблона: «{red[0]}»", "Методические рекомендации, п. 3.3")
	for t in leftovers:
		add("ошибка", f"Не заполнено или остался вариант/номер примечания: «{t}»",
			"Методические рекомендации, п. 3.3")

	# --- инициалы (подпись и текст)
	all_text = "\n".join(p.text for p in doc.paragraphs) + "\n" + "\n".join(
		c.text for tb in doc.tables for row in tb.rows for c in row.cells)
	if FORMAT_PRIORITY == "gost" and INITIALS_SPACED.search(all_text):
		add("ошибка", f"Пробел между инициалами: «{INITIALS_SPACED.search(all_text).group(0)}», по ГОСТ - «И.О.»",
			"ГОСТ Р 7.0.97-2025, п. 5.22")
	add("расхождение", "Инициалы: Инструкция (п. 8.19.2, пример «И. И. Иванов») и модули - с пробелом; "
		"ГОСТ (п. 5.22) - «И.О. Фамилия», без пробела между инициалами. "
		f"Применено: {'Инструкция' if FORMAT_PRIORITY == 'instruction' else 'ГОСТ'}",
		"Инструкция, п. 8.19.2; ГОСТ Р 7.0.97-2025, п. 5.22")

	# --- заголовок и служебные элементы
	texts = [p.text.strip() for p in doc.paragraphs if p.text.strip()]
	if not any(re.match(r"^Об?\s", t) for t in texts):
		add("ошибка", "Нет заголовка к тексту с предлогом «О»/«Об»", "ГОСТ Р 7.0.97-2025, п. 5.17")
	if any(t == "МОДУЛЬНЫЙ ДОКУМЕНТ" for t in texts):
		add("ошибка", "Не удалена пометка «МОДУЛЬНЫЙ ДОКУМЕНТ»", "Методические рекомендации, п. 3.3")
	if any("УТВЕРЖДАЮ" in c.text for tb in doc.tables for row in tb.rows for c in row.cells):
		add("ошибка", "Не удалён гриф утверждения модуля", "Методические рекомендации, п. 3.3")
	if not any("ВИЗУАЛИЗАЦИЯ ЭП" in t or "ЭЛЕКТРОННОЙ ПОДПИСЬЮ" in t.upper() for t in texts + [
			c.text for tb in doc.tables for row in tb.rows for c in row.cells]):
		add("замечание", "Нет зоны отметки об электронной подписи",
			"ГОСТ Р 7.0.97-2025, п. 5.23; Методические рекомендации, п. 3.3")
	return issues


def print_report(path: Path, issues: list[dict]):
	errors = [i for i in issues if i["level"] == "ошибка"]
	print(f"\nПроверка: {path.name}")
	print(f"Ошибок: {len(errors)}, замечаний: {sum(i['level'] == 'замечание' for i in issues)}")
	for level, mark in (("ошибка", "✗"), ("замечание", "!"), ("расхождение", "≠")):
		for i in issues:
			if i["level"] == level:
				print(f"  {mark} {i['text']}\n      основание: {i['ref']}")
	if not errors:
		print("  ✓ Нарушений оформления не найдено")


if __name__ == "__main__":
	if len(sys.argv) < 2:
		sys.exit("Укажите файл: python check.py документ.docx")
	p = Path(sys.argv[1])
	print_report(p, check_docx(p))
