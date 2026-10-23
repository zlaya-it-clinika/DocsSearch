# Запуск DocsSearch одной командой

Общий `docker-compose.yml` в этом каталоге поднимает весь проект:

| Сервис | Что это | Адрес |
|---|---|---|
| `web` | страница чата, админ-панель, виджет | `http://localhost:3000` |
| `api` | ядро на Go | внутри сети compose, для отладки `http://127.0.0.1:8080/docs` |
| `postgres` | PostgreSQL с pgvector: пользователи, сессии, документы, векторы | только внутри сети compose |
| `ollama`, `ollama-models` | локальная ИИ в контейнере, по желанию | профиль `ollama` |

Браузер сотрудника ходит только в `web`. Ядро и база наружу не открыты.

## Что нужно

- Docker Desktop, запущенный
- для ответов ИИ: Ollama на этой машине с моделями `nomic-embed-text` и `vikhr-nemo` (как поставить - в [LOCAL_AI.md](../LOCAL_AI.md)) или профиль `ollama` ниже

## Запуск

Все команды - из каталога `source`.

1. Настройки:

```bash
cp .env.example .env
```

В `.env` обязательно задать `DOCS_PASSWORD`. Остальное можно оставить как есть.

2. Сборка и запуск:

```bash
docker compose up -d --build
```

Первая сборка качает образы и зависимости, это занимает несколько минут. Дальше запуск идёт за секунды.

3. Проверка, что всё поднялось:

```bash
docker compose ps
```

У `postgres`, `api` и `web` должно быть `healthy`. Веб стартует только после того, как ядро ответило, а ядро - после базы.

4. Открыть `http://localhost:3000` и войти: `admin` / `admin` или `user` / `user`.

## Если Ollama на этой машине нет

Ollama запустится в контейнере, скачает `nomic-embed-text` и Vikhr-Nemo-12B (около 8 ГБ) и соберёт `vikhr-nemo` из [ollama/vikhr-nemo.Modelfile](ollama/vikhr-nemo.Modelfile):

1. В `.env` поменять адрес:

```
OLLAMA_URL=http://ollama:11434
```

2. Запуск с профилем:

```bash
docker compose --profile ollama up -d --build
```

3. Дождаться, пока скачаются модели. Сервис `ollama-models` завершится сам, ход загрузки виден в логе:

```bash
docker compose logs -f ollama-models
```

Модели лежат в томе `docssearch_ollama-models` и при следующих запусках не качаются заново. На процессоре ответы идут заметно медленнее, чем у Ollama на хосте с видеокартой.

## Первые шаги после запуска

1. Войти под `admin`, «Админ-панель» → «База знаний» → «Загрузить», выбрать файлы из [source/docs](docs). Это общие документы для всех сотрудников.
2. Войти под `user`, открыть «Документы» и загрузить личный документ, если ядро уже поддерживает личные документы.
3. Задать вопрос в чате своими словами. Сценарии проверки ответа, уточнения и честного отказа - в [LOCAL_AI.md](../LOCAL_AI.md).

## Повседневные команды

| Что сделать | Команда |
|---|---|
| Остановить, данные сохранятся | `docker compose down` |
| Запустить снова | `docker compose up -d` |
| Пересобрать после `git pull` | `docker compose up -d --build` |
| Логи всех сервисов | `docker compose logs -f` |
| Логи одного сервиса | `docker compose logs -f api` |
| Удалить вместе с базой и моделями | `docker compose --profile ollama down -v` |

Без `-v` база и скачанные модели остаются в томах Docker. С `-v` пропадают все загруженные документы, пользователи и чаты.

## Если что-то не работает

| Что видно | Что сделать |
|---|---|
| `required variable DOCS_PASSWORD is missing a value` | задать `DOCS_PASSWORD` в `source/.env` |
| `port is already allocated` на 3000 или 8080 | порт занят другой программой: поменять `WEB_PORT` или `API_PORT_LOCAL` в `.env` |
| сборка ядра падает на `apk add ... temporary error` | Docker не видит интернет по DNS. Docker Desktop → Settings → Docker Engine, добавить `"dns": ["8.8.8.8", "1.1.1.1"]`, Apply & restart, повторить сборку |
| в чате «Локальная модель ИИ недоступна» | Ollama не запущена, модели не скачаны или `OLLAMA_URL` не тот: для Ollama на хосте `http://host.docker.internal:11434`, для профиля `http://ollama:11434` |
| `web` не становится `healthy` | смотреть `docker compose logs api`: веб ждёт, пока ядро ответит |
| нужно проверить интерфейс без базы и модели | в `.env` выставить `API_MODE=stub` и перезапустить `docker compose up -d` |

## Отдельный запуск модулей

У ядра и веба остались свои compose: [core/docker-compose.yml](core/docker-compose.yml) и [web/docker-compose.yml](web/docker-compose.yml). Они нужны, чтобы разрабатывать модуль отдельно. Общий compose их не заменяет и не использует.
