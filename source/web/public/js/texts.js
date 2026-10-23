// Все фиксированные подписи интерфейса. В коде компонентов текстов нет, только ссылки сюда.
// Правило CodeStyle: только дефис, никаких длинных тире.

export const AppTexts = Object.freeze({
	productName: 'DocsSearch',
	productTagline: 'Помощник по регламентам и документам',
	assistantName: 'DocsSearch',
	loading: 'Загрузка…',
	retry: 'Повторить',
	close: 'Закрыть',
	cancel: 'Отмена',
	save: 'Сохранить',
	copy: 'Копировать',
	copied: 'Скопировано',
	download: 'Скачать',
	refresh: 'Обновить',
	search: 'Поиск',
	notSpecified: '-',
	fatalTitle: 'Не удалось запустить интерфейс',
	fatalHint: 'Проверьте, что веб-сервер запущен, и обновите страницу.',
	reload: 'Обновить страницу',
});

export const AuthTexts = Object.freeze({
	title: 'Вход',
	lead: 'Войдите под учётной записью сотрудника',
	loginLabel: 'Логин',
	passwordLabel: 'Пароль',
	submit: 'Войти',
	submitting: 'Входим…',
	footnote: 'Данные обрабатываются внутри компании. Запросы не уходят во внешние сервисы.',
	sessionExpired: 'Сессия завершена. Войдите снова.',
	loggedOut: 'Вы вышли из системы',
});

export const RoleTexts = Object.freeze({
	admin: 'Администратор',
	user: 'Пользователь',
	documentForAll: 'Все сотрудники',
	documentForAdmins: 'Только администраторы',
});

export const SidebarTexts = Object.freeze({
	newChat: 'Новый чат',
	searchPlaceholder: 'Поиск по чатам',
	collapse: 'Скрыть панель',
	expand: 'Показать панель',
	empty: 'Чатов пока нет',
	nothingFound: 'Ничего не найдено',
	localHistoryNote: 'История чатов хранится в этом браузере, пока ядро не отдаёт список сессий.',
	loadFailed: 'Не удалось загрузить список чатов',
	groupToday: 'Сегодня',
	groupYesterday: 'Вчера',
	groupWeek: 'Предыдущие 7 дней',
	groupMonth: 'Предыдущие 30 дней',
	untitledChat: 'Новый чат',
	userMenu: 'Меню пользователя',
	themeLabel: 'Тема',
	themeSystem: 'Системная',
	themeLight: 'Светлая',
	themeDark: 'Тёмная',
	adminPanel: 'Админ-панель',
	documents: 'Документы',
	logout: 'Выйти',
});

export const ChatTexts = Object.freeze({
	greetingPrefix: 'Здравствуйте',
	greetingLead: 'Опишите задачу своими словами. Я найду нужные пункты инструкций, объясню их простым языком и помогу подготовить документ.',
	hintSources: 'Ответы со ссылками на пункты',
	hintTemplates: 'Черновики по бланкам',
	hintDrafts: 'Проверка готового черновика',
	placeholder: 'Задайте вопрос по регламентам',
	placeholderField: 'Ответ для поля «{field}»',
	placeholderWithFile: 'Добавьте комментарий или отправьте файл на проверку',
	send: 'Отправить',
	attach: 'Прикрепить черновик на проверку',
	removeAttachment: 'Убрать файл',
	disclaimer: 'Ответы строятся только по документам базы знаний. Важные решения сверяйте с оригиналом.',
	dropTitle: 'Отпустите файл',
	dropLead: 'Черновик будет проверен на соответствие инструкции',
	thinking: 'Ищу в документах…',
	sendFailed: 'Сообщение не отправлено',
	loadFailed: 'Не удалось открыть чат',
	notFoundTitle: 'Чат не найден',
	notFoundLead: 'Возможно, он был создан в другой сессии ядра.',
	startNew: 'Начать новый чат',
	copyMessage: 'Копировать текст',
	repeatQuestion: 'Задать вопрос ещё раз',
	fileTooLarge: 'Файл больше {size}',
	kindClarify: 'Уточнение',
	kindFallback: 'Нет в базе знаний',
	kindAccessDenied: 'Недостаточно прав',
	kindTemplateField: 'Заполнение бланка',
	kindTemplateReady: 'Черновик готов',
	kindDraftCheck: 'Проверка черновика',
	fallbackHint: 'Попробуйте сформулировать вопрос иначе или обратитесь в подразделение, которое сопровождает процесс.',
	accessDeniedHint: 'Если доступ нужен для работы, обратитесь к администратору системы.',
	clarifyHint: 'Ответьте в поле ниже, и я продолжу поиск.',
	relevance: 'Релевантность',
	sources: 'Источники',
	templateField: 'Поле',
	templateProgressHint: 'Ответьте в поле ниже. Можно писать своими словами.',
	draftCopy: 'Копировать',
	draftDownload: 'Скачать .txt',
	draftFileSuffix: 'черновик',
	draftCheckRequest: 'Проверить черновик',
	draftCheckTitle: 'Рекомендации по черновику',
	draftCheckEmpty: 'Замечаний по инструкции не найдено.',
	draftCheckLocalNote: 'Результат проверки виден только в этом окне и не сохраняется в истории чата.',
	unknownKind: 'Ответ',
});

export const CitationTexts = Object.freeze({
	modalTitle: 'Источник',
	document: 'Документ',
	edition: 'Редакция',
	section: 'Раздел',
	paragraph: 'Пункт',
	appendix: 'Приложение',
	relevance: 'Релевантность',
	quote: 'Цитата',
	openOriginal: 'Открыть оригинал',
	shortParagraph: 'п.',
	shortSection: 'разд.',
	shortAppendix: 'прил.',
});

export const DocumentsTexts = Object.freeze({
	modalTitle: 'Документы базы знаний',
	lead: 'Документы, доступные вам для поиска и ответов.',
	empty: 'Доступных документов нет',
	loadFailed: 'Не удалось загрузить документы',
	edition: 'ред.',
	openFile: 'Скачать оригинал',
});

export const AdminTexts = Object.freeze({
	title: 'Админ-панель',
	tabUsers: 'Пользователи',
	tabKnowledge: 'База знаний',
	tabAudit: 'Журнал',
	onlyAdmins: 'Раздел доступен только администраторам',
	backToChat: 'Вернуться в чат',

	usersSearch: 'Поиск по имени или логину',
	usersAdd: 'Добавить',
	usersEmpty: 'Пользователей нет',
	usersColumnRole: 'Роль',
	usersColumnName: 'Имя',
	usersColumnLogin: 'Логин',
	usersColumnId: 'ID',
	usersEdit: 'Изменить',
	usersRoleToggleHint: 'Нажмите, чтобы сменить роль',
	usersSelfRoleHint: 'Свою роль сменить нельзя',
	usersYou: 'вы',
	usersCreateTitle: 'Новый пользователь',
	usersEditTitle: 'Изменить пользователя',
	usersFieldLogin: 'Логин',
	usersFieldPassword: 'Пароль',
	usersFieldName: 'Отображаемое имя',
	usersFieldRole: 'Роль',
	usersCreated: 'Пользователь добавлен',
	usersUpdated: 'Изменения сохранены',
	usersRoleChanged: 'Роль изменена',
	usersLoadFailed: 'Не удалось загрузить пользователей',

	knowledgeLead: 'Загрузка с тем же названием заменяет актуальную редакцию документа.',
	knowledgeSearch: 'Поиск по названию',
	knowledgeUpload: 'Загрузить',
	knowledgeEmpty: 'В базе пока нет документов',
	knowledgeColumnTitle: 'Документ',
	knowledgeColumnEdition: 'Редакция',
	knowledgeColumnAccess: 'Доступ',
	knowledgeNewEdition: 'Новая редакция',
	knowledgeUploadTitle: 'Загрузка документа',
	knowledgeEditionTitle: 'Новая редакция документа',
	knowledgeFieldFile: 'Файл',
	knowledgeFieldTitle: 'Название',
	knowledgeFieldTitleHint: 'Если оставить пустым, возьмём имя файла',
	knowledgeFieldEdition: 'Редакция',
	knowledgeFieldEditionHint: 'Например, 2.0 или 2026-10',
	knowledgeFieldAccess: 'Кому доступен',
	knowledgeDropLead: 'Перетащите файл сюда или нажмите, чтобы выбрать',
	knowledgeFileRequired: 'Выберите файл',
	knowledgeUploaded: 'Документ принят',
	knowledgeLoadFailed: 'Не удалось загрузить список документов',

	auditLead: 'Кто задавал вопросы и какие документы были показаны в ответах.',
	auditSearch: 'Поиск по сотруднику или вопросу',
	auditEmpty: 'Записей пока нет',
	auditColumnTime: 'Время',
	auditColumnUser: 'Сотрудник',
	auditColumnQuestion: 'Вопрос',
	auditColumnDocuments: 'Показанные документы',
	auditNoDocuments: 'не показывались',
	auditLoadFailed: 'Не удалось загрузить журнал',
});

export const FormatTexts = Object.freeze({
	bytes: 'Б',
	kilobytes: 'КБ',
	megabytes: 'МБ',
	defaultFileName: 'документ',
});

export const WidgetTexts = Object.freeze({
	title: 'DocsSearch',
	newChat: 'Новый чат',
	openFull: 'Открыть в полном окне',
	close: 'Свернуть',
});

// Понятные сообщения по кодам ошибок ядра и веб-сервера
export const ErrorTexts = Object.freeze({
	invalid_credentials: 'Неверный логин или пароль',
	unauthorized: 'Сессия завершена. Войдите снова.',
	access_denied: 'Недостаточно прав для этого действия',
	not_found: 'Не найдено',
	login_taken: 'Такой логин уже занят',
	invalid_body: 'Сервер не принял данные. Проверьте заполнение полей.',
	live_not_ready: 'Боевой режим ядра ещё не включён',
	core_unavailable: 'Ядро недоступно. Проверьте, что оно запущено.',
	core_timeout: 'Ядро не ответило вовремя. Попробуйте ещё раз.',
	payload_too_large: 'Файл слишком большой',
	too_many_attempts: 'Слишком много попыток входа. Подождите несколько минут.',
	csrf_failed: 'Запрос отклонён из соображений безопасности. Обновите страницу.',
	origin_denied: 'Запрос отклонён из соображений безопасности. Обновите страницу.',
	method_not_allowed: 'Действие пока не поддерживается сервером',
	internal_error: 'Внутренняя ошибка веб-сервера',
	network_error: 'Нет связи с сервером',
	unknown: 'Что-то пошло не так. Попробуйте ещё раз.',
});

// Подстановка {name} в шаблон подписи
export function format_Text(template, values) {
	return template.replace(/\{(\w+)\}/g, (match, key) => (key in values ? String(values[key]) : match));
}

export function get_ErrorText(error) {
	const code = error?.code;
	if (code && ErrorTexts[code]) {
		return ErrorTexts[code];
	}
	return ErrorTexts.unknown;
}
