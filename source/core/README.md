# Ядро DocsSearch

HTTP API на Go: учётные записи и права, база знаний, поиск по смыслу, ответы локальной модели, подготовка и проверка документов. Веб и виджет - тонкие клиенты этого API ([source/web](../web)).

Ответственный: PotJoke.

## Запуск

Весь проект целиком - из [source](..): `docker compose up -d --build`. Для разработки ядра отдельно, из `source/core`:

```bash
cp .env.example .env
go run ./cmd/api
```

Нужен Go 1.24 или новее. В `.env` для боевого режима:

```
API_MODE=live
DOCS_PASSWORD=любой-пароль
DATABASE_URL=postgres://docs:docs@localhost:5432/docssearch?sslmode=disable
OLLAMA_URL=http://127.0.0.1:11434
OLLAMA_CHAT_MODEL=vikhr-nemo
OLLAMA_EMBED_MODEL=nomic-embed-text
OLLAMA_EMBED_DIMENSIONS=768
```

PostgreSQL с pgvector для разработки: `docker compose up -d postgres` в этом каталоге. Таблицы создаются при запуске, миграции из [migrations](migrations) идемпотентны и применяются каждый раз. С `DEBUG=true` в логе появится `listen :8080 mode live`.

Страница ручных запросов: http://127.0.0.1:8080/docs, пароль `DOCS_PASSWORD`. На ней есть пример тела для каждого метода.

## Режимы

| `API_MODE` | Что делает | Зачем |
|---|---|---|
| `live` | PostgreSQL + pgvector, эмбеддинги и ответы через Ollama | боевой режим |
| `stub` | всё в памяти процесса, ответы фиксированные, Ollama нужна только вкладке «Модели ИИ» | клиенты разрабатываются без базы и модели |

В `stub` есть учётные записи `admin` / `admin` (главный администратор) и `user` / `user` (сотрудник), документы `doc-1` (открытая инструкция) и `doc-2` (закрытый регламент). С `STUB_SEED_USERS=false` их нет. Вид ответа в заглушке выбирается фразой: «нет доступа», «не найдено», «уточни».

В `live` стартовых учётных записей нет: первая регистрация на сайте создаёт главного администратора.

## Как отвечает на вопрос

Реплика в чате проходит шаги по порядку (логика - `internal/httpapi/live.go`, `serve_SessionMessage`):

```mermaid
flowchart TD
  ask[Реплика] --> mode{Режим «План» и право templates.use}
  mode -->|да| plan[Макет или модульный бланк: вопросы по полям, затем .docx]
  mode -->|нет| draft{Черновик во вложениях}
  draft -->|да| revision[draft_revision: правка черновика]
  draft -->|нет| overview{Вопрос называет документ целиком}
  overview -->|да| card[Справка о документе и ссылка на скачивание]
  overview -->|нет| search[Поиск: до 5 видимых фрагментов]
  search -->|только закрытые| denied[access_denied без модели]
  search -->|нет близости от 0.4| fallback[fallback без модели]
  search -->|два документа, разрыв меньше 0.05| clarify[clarify: уточняющий вопрос]
  search -->|есть фрагменты| select[Модель выбирает номера фрагментов]
  select -->|пусто| fallback
  select -->|номера| model[Модель отвечает по выбранным фрагментам]
  model -->|метка НЕТ_ОТВЕТА| fallback
  model --> answer[answer: пересказ, цитата, процент, ссылка на пункт]
```

Как ответ защищён от выдумок:

- Фрагменты выше прав сотрудника и чужие личные документы не попадают в промпт даже при отказе.
- Модель сначала выбирает номера фрагментов. Пустой выбор - `fallback`, второй вызов модели не делается.
- Числа в ответе сверяются с фрагментами (`internal/grounding`). Число не из фрагментов - вместо пересказа приводится ближайший пункт дословно.
- Ответ на `clarify` ищется вместе с исходным вопросом, второй раз ядро не переспрашивает.

В `answer` и `fallback` есть `contacts`, если загружен справочник подразделений (в названии «контакт» или «справочник» и «подраздел»).

Пороги заданы в `internal/livestore/reply.go`: близость от `0.4`, разрыв для уточнения `0.05`, до `5` фрагментов. Близость - единица минус косинусное расстояние.

## Режимы чата

Поле `mode` сообщения, режим хранится в сессии:

- `read` (по умолчанию) - только база знаний. На просьбу подготовить документ в ответе поле `hint` про «План».
- `plan` - подготовка документа. Вид узнаётся по словам реплики, затем модель выбирает его по смыслу, иначе ответ `template_choice` со списком. «отмена» останавливает документ.

Виды документов:

- **Макеты** (`internal/layout`, каталог и вопросы - `internal/texts/layouts.go`): служебная, докладная и пояснительная записки, запрос на закупку, деловое письмо, акт, протокол. Адресат и подписант предлагаются из вкладки «Организация».
- **Модульные бланки** (`internal/blank/modules`): приказы и распоряжения из материалов кейса, вопросы строятся по примечаниям бланка.

Ответ `template_field` несёт все поля без ответа в `template.questions` (`field`, `label`, `question`, `hint`, `optional`, `options`). Клиент может прислать ответы пачкой в `answers`: `[{ "field": "...", "value": "..." }]`. Пустой ответ обязательного поля - поле спросят снова. Готовый документ - `template_ready` с `draftText` и ссылкой `draftLink` на `.docx`.

Оформление - по Инструкции по ДОУ, о чём она молчит - по ГОСТ Р 7.0.97-2025. Правила лежат в профилях [internal/layout/formats](internal/layout/formats) (`instruction.json`, `gost.json`, `base.json`), у каждого правила указан пункт документа. Правило берётся из первого профиля, где оно есть, порядок меняет `LAYOUT_FORMAT_PRIORITY`. Ошибка в профиле останавливает запуск ядра.

## Права доступа

Права - это флаги, флаги собираются в роли. У пользователя одна роль.

| Роль | Флаги |
|---|---|
| Главный администратор | все |
| Администратор | все, кроме `roles.manage` |
| Сотрудник | `chat.use`, `documents.personal.upload`, `drafts.check`, `drafts.assist`, `templates.use` |

Остальные флаги: `documents.global.manage`, `documents.personal.view_all`, `users.manage`, `audit.view`, `ai.manage`, `organization.manage`, `sources.manage`. Полный список с описаниями отдаёт `GET /permissions`, каталог - `internal/texts/texts.go` (`Permission_Catalog`). Свои роли создаются в админ-панели, роли можно назначать по группам LDAP.

Правила для документов:

- Общий документ виден всем или только выбранным ролям. Без доступа: файл отвечает `403`, в поиск не попадает.
- Личный документ виден владельцу и обладателю `documents.personal.view_all`, но в поиск чата попадает только у владельца. Чужой личный документ отвечает `404`.
- Права на действия клиент не вычисляет: ядро отдаёт `canUploadGlobal`, `canUploadPersonal`, `canDelete`, `canRename`.

Учётные записи:

- Пароли хранятся как bcrypt, токен непрозрачный, в лог пароль и токен не пишутся.
- Ожидающий подтверждения получает `403 account_pending` везде, кроме профиля `GET /me` и контакта администратора.
- Себя удалить или понизить нельзя, главного администратора удаляет только обладатель `roles.manage`, последнего главного администратора не удалить.
- Вход через LDAP или Kerberos попадает только в учётную запись, созданную каталогом. Почта, занятая обычной учётной записью, даёт `409 directory_account_conflict`.

## База знаний

- Форматы `.txt`, `.pdf`, `.docx`, до 20 МБ. Текст режется по разделам и пунктам (`N.N`), без структуры - по 800 символов. Таблица в `.docx` режется по строкам с подписями столбцов.
- Повторная загрузка с тем же названием в той же области - новая редакция. Прежняя уходит в архив: `GET /documents/{id}/editions`. В поиске участвует только текущая.
- Стандартный набор (`STANDARD_DOCS_DIR`, в compose это `source/docs`) загружается при каждом запуске. Удалить его нельзя (`409 standard_document`), заменить новой редакцией можно. Векторы досчитываются в фоне, запуск не ждёт Ollama.
- Документы делятся по процессам (категориям), в чате можно выбрать несколько: поле `processIds`.
- Автозагрузка (`/sources`, флаг `sources.manage`): каталог на диске ядра, `demo` и лицензионный API Консультант Плюс. Источник создаётся выключенным.
- Внешний документ проверяется на оформление: `POST /drafts/check` (флаг `drafts.check`, в чате - скрепка, «Проверить оформление по регламенту»). У `.docx` ядро читает страницу, поля, стили и колонтитулы и сверяет их с профилями оформления `internal/layout/formats` (порядок - `LAYOUT_FORMAT_PRIORITY`): формат листа, поля, шрифт, размер, интервал и выравнивание основного текста, абзацный отступ, номер страницы, формат дат, инициалы, остатки шаблона (красный текст, номера полей, пометка модуля). Замечание `level` - `error` (нарушено правило) или `remark` (стоит проверить), в цитате - профиль и пункт, пункт Инструкции ведёт в её файл. После оформления идут рекомендации по тексту. У `.pdf` и `.txt` оформление не видно: `formatChecked: false`, проверяется только текст. После подтверждения сотрудника (кнопка «Исправить оформление») `POST /drafts/fix` исправляет `.docx` сам: лист, поля, номер страницы на первой странице и его выравнивание, шрифт, размер, интервал, выравнивание и отступ основного текста, формат дат и инициалов. Меняются только нужные элементы XML, порядок свойств - по схеме OOXML, слова документа не трогаются. Ответ: `draftLink` на исправленный файл в черновиках, `applied` - что исправлено, `recommendations` - что осталось вручную (остатки шаблона, номер страницы не в том колонтитуле). Основной текст - абзацы-предложения вне таблиц: строки по центру и справа, адресат в правой зоне, пункты списков и заголовки прописными исправление не выравнивает. Нет номера страницы - добавляется колонтитул с полем PAGE. Проверка ищет реквизиты: нет заголовка или подписи - замечание, а в ответе `requisites` - чего нет (`addressee`, `title`, `signature`) и должностные лица из вкладки «Организация». Перед исправлением сотрудник отвечает, кто подписывает, кому документ и о чем он: поля `signerId`, `addresseeId`, `title` в `POST /drafts/fix`. Подпись встает в конце (должность слева, инициалы и фамилия у правого поля), адресат - в правой зоне в дательном падеже (падеж ставит модель), заголовок - перед текстом.
- Смена модели эмбеддингов с другой длиной вектора: поменять `OLLAMA_EMBED_MODEL` и `OLLAMA_EMBED_DIMENSIONS`. При запуске ядро перестроит столбец векторов и пересчитает их в фоне (`internal/livestore/embedding_dims.go`).

## API

Все методы, кроме входа и `GET /auth/config`, ждут `Authorization: Bearer <токен>`. Ошибки - JSON `{ "error": "код" }`, коды в `internal/texts/texts.go`.

| Раздел | Методы |
|---|---|
| Вход | `POST /auth/login`, `/auth/signup`, `/auth/ldap`, `/auth/kerberos`, `/auth/logout`; `GET /auth/config` |
| Настройки входа | `GET`, `POST /auth/admin/config`, `/auth/admin/config/ldap`, `/auth/admin/config/ldap/server`; `GET /auth/admin/details` |
| Профиль и права | `GET /me`, `GET /permissions`, `GET`, `POST /roles`, `PATCH`, `DELETE /roles/{roleId}`, `GET /audit/permissions` |
| Чаты | `GET`, `POST /sessions`, `GET`, `PATCH`, `DELETE /sessions/{sessionId}`, `POST /sessions/{sessionId}/messages`, `POST /sessions/{sessionId}/attachments` |
| Документы | `GET`, `POST /documents`, `POST /documents/import`, `DELETE /documents/{id}`, `GET /documents/{id}/file`, `GET /documents/{id}/editions`, `GET /documents/{id}/editions/{editionId}/file` |
| Файлы документов | `GET /blanks/{blankId}/file`, `GET /drafts/{draftId}/file`, `POST /drafts/check` |
| Процессы | `GET`, `POST /processes`, `PATCH`, `DELETE /processes/{processId}` |
| Пользователи и журнал | `GET`, `POST /users`, `PATCH`, `DELETE /users/{userId}`, `GET /audit` |
| Модели ИИ | `GET`, `POST /ai/settings`, `POST /ai/verify`, `GET /ai/models`, `GET`, `POST /ai/models/pull`, `POST /ai/models/pull/cancel`, `/ai/models/delete`, `/ai/models/test`, `GET`, `POST /ai/reindex` |
| Организация | `GET /organization`, `POST /organization/officials`, `PATCH`, `DELETE /organization/officials/{officialId}`, `GET`, `POST`, `DELETE /organization/images/{kind}` |
| Источники | `GET`, `POST /sources`, `GET /sources/kinds`, `PATCH`, `DELETE /sources/{sourceId}`, `POST /sources/{sourceId}/verify`, `/sync`, `GET /sources/{sourceId}/runs` |

Пример вопроса:

```bash
curl -s -X POST http://127.0.0.1:8080/sessions/<sessionId>/messages \
  -H "Authorization: Bearer <токен>" -H "Content-Type: application/json" \
  -d '{"text": "Когда регистрируют входящий документ?", "mode": "read"}'
```

В ответе `kind` - одно из `answer`, `clarify`, `fallback`, `access_denied`, `template_choice`, `template_field`, `template_ready`, `draft_revision`, рядом `text`, `citations`, `relevancePercent` и, в зависимости от вида, `contacts`, `options`, `template`, `draft`, `hint`, `download`.

## Структура

| Пакет | Что внутри |
|---|---|
| `cmd/api` | точка входа сервера |
| `cmd/eval` | прогон контрольных вопросов через API |
| `internal/httpapi` | маршруты, боевой режим и заглушка, страница `/docs` |
| `internal/livestore` | PostgreSQL: учётные записи, роли, документы, поиск, источники |
| `internal/auth` | пароли, роли, LDAP, Kerberos |
| `internal/chunk` | извлечение текста из `.txt`, `.pdf`, `.docx` и нарезка |
| `internal/ollama` | клиент Ollama: эмбеддинги, ответы, управление моделями |
| `internal/grounding` | сверка чисел ответа с фрагментами |
| `internal/blank`, `internal/layout` | модульные бланки и макеты документов, сборка `.docx` |
| `internal/draftcheck`, `internal/draftassist` | проверка и правка черновика |
| `internal/formatcheck` | проверка оформления внешнего `.docx` по правилам Инструкции и ГОСТ |
| `internal/source` | коннекторы автозагрузки |
| `internal/texts` | все тексты и коды ошибок |

## Проверка

```bash
go test ./...
```

Тесты хранилища на PostgreSQL выполняются, если задан `DATABASE_URL`, иначе пропускаются. Качество ответов на контрольных вопросах (ядро запущено, `EVAL_API_URL` по умолчанию `http://127.0.0.1:8080`):

```bash
EVAL_LOGIN=сотрудник@почта EVAL_PASSWORD=пароль go run ./cmd/eval
```

Без `EVAL_LOGIN` прогон идёт от `user` / `user` заглушки.
