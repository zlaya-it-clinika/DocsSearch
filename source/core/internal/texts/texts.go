package texts

const (
	KindAnswer        = "answer"
	KindClarify       = "clarify"
	KindFallback      = "fallback"
	KindTemplateField = "template_field"
	KindTemplateReady = "template_ready"
	KindAccessDenied  = "access_denied"

	ErrorInvalidCredentials = "invalid_credentials"
	ErrorUnauthorized       = "unauthorized"
	ErrorAccessDenied       = "access_denied"
	ErrorNotFound           = "not_found"
	ErrorLoginTaken         = "login_taken"
	ErrorInvalidBody        = "invalid_body"
	ErrorLiveNotReady       = "live_not_ready"

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
	DocsHintMessage       = "Фразы заглушки: нет доступа, не найдено, уточни, шаблон. Иначе обычный ответ."
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

	AnswerText      = "Входящий документ регистрируют в день поступления и присваивают ему номер."
	AnswerQuote     = "Точная цитата из пункта инструкции."
	AnswerSection   = "3"
	AnswerParagraph = "3.2"

	ClarifyText      = "Уточните, о каком типе документа идет речь."
	FallbackText     = "В базе знаний нет сведений для ответа на этот вопрос."
	AccessDeniedText = "Недостаточно прав для просмотра документов по этому запросу."

	TemplateID         = "memo"
	TemplateTitle      = "Служебная записка"
	TemplateField      = "addressee"
	TemplateFieldLabel = "Адресат"
	TemplateFieldText  = "Укажите адресата."
	TemplateReadyText  = "Черновик собран."
	TemplateDraftText  = "Текст черновика."

	DraftRecommendation = "В проекте нет отметки о приложении."
	DraftQuote          = "Точная цитата требования."
	DraftSection        = "4"
	DraftParagraph      = "4.1"
)
