// Клиент API. Браузер ходит только в /api своего веб-сервера, тот добавляет токен ядра.
// Пути и формы ответов - по контракту из README репозитория.

const apiPrefix = '/api';
const csrfHeader = 'X-CSRF-Token';
const readOnlyMethods = new Set(['GET', 'HEAD']);

export class ApiError extends Error {
	constructor(status, code) {
		super(code);
		this.status = status;
		this.code = code;
	}
}

let csrfToken = '';
const unauthorizedListeners = new Set();

export function set_CsrfToken(token) {
	csrfToken = token ?? '';
}

// Подписка на 401: сессия истекла или ядро перезапустилось
export function on_Unauthorized(listener) {
	unauthorizedListeners.add(listener);
	return () => unauthorizedListeners.delete(listener);
}

async function read_Payload(response) {
	if (response.status === 204) {
		return null;
	}
	const contentType = response.headers.get('content-type') ?? '';
	if (!contentType.includes('application/json')) {
		return null;
	}
	try {
		return await response.json();
	} catch {
		return null;
	}
}

// silentUnauthorized: 401 здесь ожидаем (вход, проверка сессии) и не считаем выходом
async function request_Api(method, path, { json, formData, silentUnauthorized = false } = {}) {
	const headers = { Accept: 'application/json' };
	let body;
	if (json !== undefined) {
		headers['Content-Type'] = 'application/json';
		body = JSON.stringify(json);
	}
	if (formData) {
		body = formData;
	}
	if (!readOnlyMethods.has(method) && csrfToken) {
		headers[csrfHeader] = csrfToken;
	}

	let response;
	try {
		response = await fetch(apiPrefix + path, { method, headers, body, credentials: 'same-origin' });
	} catch {
		throw new ApiError(0, 'network_error');
	}
	const payload = await read_Payload(response);
	if (!response.ok) {
		const code = typeof payload?.error === 'string' ? payload.error : `http_${response.status}`;
		if (response.status === 401 && !silentUnauthorized) {
			for (const listener of unauthorizedListeners) {
				listener();
			}
		}
		throw new ApiError(response.status, code);
	}
	return payload;
}

function encode_Segment(value) {
	return encodeURIComponent(String(value));
}

// Метод не реализован в ядре: роут не найден или метод не разрешён
export function check_IsNotSupported(error) {
	return error instanceof ApiError && (error.status === 404 || error.status === 405) && error.code !== 'not_found';
}

// Настройки и вход

export function fetch_PublicConfig() {
	return request_Api('GET', '/config');
}

export function login_User(login, password) {
	return request_Api('POST', '/auth/login', { json: { login, password }, silentUnauthorized: true });
}

export function fetch_AuthSession() {
	return request_Api('GET', '/auth/session', { silentUnauthorized: true });
}

export function logout_User() {
	return request_Api('POST', '/auth/logout', { silentUnauthorized: true });
}

// Сессии чата

export function list_ChatSessions() {
	return request_Api('GET', '/sessions');
}

export function create_ChatSession() {
	return request_Api('POST', '/sessions');
}

// Переименование и удаление чата. Доступны, когда GET /sessions отдаёт canRename и canDelete
export function rename_ChatSession(sessionId, title) {
	return request_Api('PATCH', `/sessions/${encode_Segment(sessionId)}`, { json: { title } });
}

export function delete_ChatSession(sessionId) {
	return request_Api('DELETE', `/sessions/${encode_Segment(sessionId)}`);
}

export function fetch_ChatSession(sessionId) {
	return request_Api('GET', `/sessions/${encode_Segment(sessionId)}`);
}

export function send_ChatMessage(sessionId, text) {
	return request_Api('POST', `/sessions/${encode_Segment(sessionId)}/messages`, { json: { text } });
}

// Документы

// scope: global, personal или all (all - только администратор). Без параметра - всё, что видит пользователь
export function list_Documents(scope = '') {
	const query = scope ? `?scope=${encodeURIComponent(scope)}` : '';
	return request_Api('GET', `/documents${query}`);
}

export function delete_Document(documentId) {
	return request_Api('DELETE', `/documents/${encode_Segment(documentId)}`);
}

// fields: title, edition, scope, accessLevel - необязательные
export function upload_Document(file, fields = {}) {
	const formData = new FormData();
	formData.append('file', file);
	for (const [name, value] of Object.entries(fields)) {
		if (value !== undefined && value !== null && String(value).trim() !== '') {
			formData.append(name, String(value).trim());
		}
	}
	return request_Api('POST', '/documents', { formData });
}

export function build_DocumentFileUrl(documentId) {
	return `${apiPrefix}/documents/${encode_Segment(documentId)}/file`;
}

// originalLink из цитаты: /documents/<id>/file#p3.2. Пускаем только пути к документам
export function build_OriginalUrl(originalLink) {
	if (typeof originalLink !== 'string' || !originalLink.startsWith('/documents/')) {
		return null;
	}
	return apiPrefix + originalLink;
}

export function check_Draft(file) {
	const formData = new FormData();
	formData.append('file', file);
	return request_Api('POST', '/drafts/check', { formData });
}

// Пользователи и журнал (уровень администратора)

export function list_Users() {
	return request_Api('GET', '/users');
}

export function create_User(fields) {
	return request_Api('POST', '/users', { json: fields });
}

export function update_User(userId, fields) {
	return request_Api('PATCH', `/users/${encode_Segment(userId)}`, { json: fields });
}

export function fetch_AuditLog() {
	return request_Api('GET', '/audit');
}
