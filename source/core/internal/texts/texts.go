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
