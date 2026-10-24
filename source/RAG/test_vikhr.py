import json
import ollama

MODEL = "OxW/Vikhr-Nemo-12B-Instruct-R-21-09-24:q4_K_M"

# Системный промпт из карточки модели (сверьте текст с huggingface.co/Vikhrmodels/...)
GROUNDED_SYSTEM_PROMPT = (
    "Your task is to answer the user's questions using only the information from "
    "the provided documents. Give two answers to each question: one with a list of "
    "relevant document identifiers and the second with the answer to the question "
    "itself, using documents with these identifiers."
)

docs = [
    {"doc_id": 0, "title": "Инструкция по ДОУ, п. 5.1",
     "content": "Входящие документы регистрируются в ИИС ДО в день поступления."},
    {"doc_id": 1, "title": "Инструкция по ДОУ, п. 11.2",
     "content": "Дела формируются по номенклатуре дел подразделения."},
]

messages = [
    {"role": "system", "content": GROUNDED_SYSTEM_PROMPT},
    {"role": "documents", "content": json.dumps(docs, ensure_ascii=False)},
    {"role": "user", "content": "Когда нужно зарегистрировать входящее письмо?"},
]

# Шаг 1: модель выбирает релевантные документы
r1 = ollama.chat(model=MODEL, messages=messages, options={"temperature": 0.0})
print("ШАГ 1:", r1["message"]["content"])

# Шаг 2: модель отвечает по выбранным документам
messages.append({"role": "assistant", "content": r1["message"]["content"]})
r2 = ollama.chat(model=MODEL, messages=messages, options={"temperature": 0.3})
print("ШАГ 2:", r2["message"]["content"])