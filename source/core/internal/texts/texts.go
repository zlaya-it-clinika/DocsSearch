package texts

const (
	KindAnswer        = "answer"
	KindClarify       = "clarify"
	KindFallback      = "fallback"
	KindTemplateField = "template_field"
	KindTemplateReady = "template_ready"
	KindAccessDenied  = "access_denied"

	RoleAdmin = "admin"
	RoleUser  = "user"

	ScopeGlobal   = "global"
	ScopePersonal = "personal"
	ScopeAll      = "all"

	ErrorInvalidCredentials = "invalid_credentials"
	ErrorUnauthorized       = "unauthorized"
	ErrorAccessDenied       = "access_denied"
	ErrorNotFound           = "not_found"
	ErrorLoginTaken         = "login_taken"
	ErrorInvalidBody        = "invalid_body"
	ErrorLiveNotReady       = "live_not_ready"
	ErrorEmbedSize          = "embed_size_mismatch"
	ErrorEmbedUnavailable   = "embed_unavailable"
	ErrorStore              = "store_unavailable"

	LiveDatabaseMissing = "Для API_MODE=live нужен DATABASE_URL."
	LiveDatabaseConnect = "Не удалось подключиться к PostgreSQL."
	LiveEmbedDimensions = "OLLAMA_EMBED_DIMENSIONS должен быть положительным числом."

	DocsWrongPassword  = "Неверный пароль."
	DocsPasswordNotSet = "В .env не задан DOCS_PASSWORD."
	DocsBadForm        = "Не удалось прочитать форму."

	DocsLoginTitle    = "Вход в документацию API"
	DocsHeading       = "Документация API"
	DocsLoginLead     = "Страница с запросами к локальному ядру. Пароль берется из DOCS_PASSWORD."
	DocsPasswordLabel = "Пароль"
	DocsLoginButton   = "Войти"
	DocsMissingHint   = "Добавьте пароль в .env рядом с сервером и перезапустите его."

	DocsConsoleTitle     = "DocsSearch API"
	DocsConsoleLead      = "Запросы уходят на этот же сервер. В заглушке: user / user и admin / admin."
	DocsLogoutButton     = "Выйти"
	DocsTokenLabel       = "Токен API, заголовок Authorization"
	DocsTokenPlaceholder = "Появится после POST /auth/login"
	DocsSendButton       = "Отправить"
	DocsFilePrefix       = "Файл, "
	DocsFileSuffix       = " байт"

	DocsHintLogin         = "Вход сотрудника. Токен из ответа подставится в поле выше."
	DocsHintMe            = "Текущий пользователь без токена в теле ответа."
	DocsHintCreateSession = "Новая сессия. Ее id подставится в запросы истории и сообщений."
	DocsHintHistory       = "История сессии."
	DocsHintMessage       = "Фразы заглушки: нет доступа, не найдено, уточни. Запрос с названием модуля начинает бланк."
	DocsHintListDocuments = "Только документы не выше уровня пользователя."
	DocsHintUpload        = "Загрузка в базу. Файл в заглушке не индексируется, но появляется в списке."
	DocsHintFile          = "Файл оригинала. doc-2 закрыт для user."
	DocsHintUsers         = "Список учетных записей. Нужен уровень 2."
	DocsHintCreateUser    = "Создание учетной записи. Нужен уровень 2."
	DocsHintPatchUser     = "Смена имени и уровня. Нужен уровень 2."
	DocsHintAudit         = "Журнал вопросов. Нужен уровень 2."
	DocsHintDraft         = "Проверка черновика. В заглушке одна фиксированная рекомендация."

	DocsExampleLoginBody = `{
  "login": "user",
  "password": "user"
}`
	DocsExampleMessageBody = `{
  "text": "Как зарегистрировать входящий документ?"
}`
	DocsExampleCreateUserBody = `{
  "login": "clerk",
  "password": "clerk",
  "displayName": "Делопроизводитель",
  "accessLevel": 1
}`
	DocsExamplePatchUserBody = `{
  "displayName": "Сотрудник архива",
  "accessLevel": 1
}`

	PhraseAccessDenied = "нет доступа"
	PhraseNotFound     = "не найдено"
	PhraseClarify      = "уточни"
	PhraseTemplate     = "шаблон"
	PhrasePrepare      = "как подготовить"

	BlankSkipNo   = "нет"
	BlankSkipPass = "пропустить"
	BlankSkipDash = "-"

	StatusAccepted = "accepted"

	AnswerRelevancePercent = 86

	SeedAdminLogin       = "admin"
	SeedAdminPassword    = "admin"
	SeedAdminID          = "user-admin"
	SeedAdminDisplayName = "Администратор"
	SeedAdminAccessLevel = 2

	SeedEmployeeLogin       = "user"
	SeedEmployeePassword    = "user"
	SeedEmployeeID          = "user-1"
	SeedEmployeeDisplayName = "Сотрудник"
	SeedEmployeeAccessLevel = 1

	SeedInstructionID        = "doc-1"
	SeedInstructionTitle     = "Инструкция по документообороту"
	SeedInstructionFileName  = "instruction.txt"
	SeedInstructionBody      = "Инструкция по документообороту. Пункт 3.2. Входящий документ регистрируют в день поступления."
	SeedRestrictedID         = "doc-2"
	SeedRestrictedTitle      = "Закрытый регламент"
	SeedRestrictedFileName   = "restricted.txt"
	SeedRestrictedBody       = "Закрытый регламент. Доступ ограничен."
	SeedDocumentAccessOpen   = 1
	SeedDocumentAccessClosed = 2

	EditionDefault       = "1.0"
	DefaultDocumentTitle = "Документ"
	PlainTextContentType = "text/plain; charset=utf-8"
	DocxContentType      = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

	AnswerText      = "Входящий документ регистрируют в день поступления и присваивают ему номер."
	AnswerQuote     = "Точная цитата из пункта инструкции."
	AnswerSection   = "3"
	AnswerParagraph = "3.2"

	ClarifyText      = "Уточните, о каком типе документа идет речь."
	FallbackText     = "В базе знаний нет сведений для ответа на этот вопрос."
	AccessDeniedText = "Недостаточно прав для просмотра документов по этому запросу."

	TemplateReadyText = "Черновик собран."

	PrepareAnswerText           = "Чтобы подготовить документ:\n1. Проверьте состав реквизитов (пункт 8.1).\n2. Оформите реквизиты, которые положены этому виду документа (пункт 8.2)."
	PrepareParagraphComposition = "8.1"
	PrepareParagraphList        = "8.2"

	LiveSelectPrompt   = "По списку фрагментов верни только номера тех, которые нужны для ответа на вопрос. Номера через запятую. Если ни один фрагмент не подходит, верни нет. Не пиши сам ответ и не добавляй фактов."
	LiveAnswerPrompt   = "Отвечай только по переданным фрагментам инструкций. Не добавляй факты, которых в них нет. Сложные формулировки и термины объясни простыми словами. Не копируй текст пункта дословно. Коротко, по-русски."
	LiveClarifyPrompt  = "Вопрос подходит к разным документам. Задай один короткий уточняющий вопрос, о каком документе речь. Не отвечай по существу и не добавляй фактов."
	LiveQuestionLabel  = "Вопрос"
	LiveFragmentsLabel = "Фрагменты"
	LiveDocumentLabel  = "Документ"
	LiveEditionLabel   = "Редакция"
	LiveSectionLabel   = "Раздел"
	LiveParagraphLabel = "Пункт"
	LiveAppendixLabel  = "Приложение"

	DraftRecommendation   = "В проекте нет отметки о приложении."
	DraftSignatureMissing = "В проекте нет подписи."

	EvalCoreUnavailable = "Живое ядро недоступно."
	EvalLoginFailed     = "Не удалось войти в живое ядро."
	EvalRequestFailed   = "Запрос к живому ядру не выполнен."
	EvalPointsLine      = "верных пунктов: %d из %d\n"
	EvalFallbackLine    = "fallback: %d из %d\n"
)
