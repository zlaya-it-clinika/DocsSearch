// Маршруты /api: вход и выход обрабатывает сам веб-сервер, остальное уходит в ядро.
import { build_SessionCookie, parse_Cookies } from './cookies.js';
import { call_CoreJson, forward_ToCore } from './core-client.js';
import { RequestError, read_ClientAddress, read_JsonBody, send_Empty, send_Json } from './http-utils.js';
import { apply_SecurityHeaders, check_RequestOrigin, match_CsrfToken } from './security.js';
import { ErrorCodes } from './texts.js';

// Разделы ядра, которые можно вызывать из браузера. Вход ядра (/auth/login) и страница /docs
// сюда не входят: токен ядра не должен попасть в браузер.
const proxiedCoreSections = new Set(['me', 'sessions', 'documents', 'users', 'audit', 'drafts', 'templates']);

const csrfHeader = 'x-csrf-token';
const readOnlyMethods = new Set(['GET', 'HEAD']);

// Пользователь без служебных полей ядра
function pick_PublicUser(payload) {
	if (!payload || typeof payload !== 'object') {
		return null;
	}
	const { token, ...user } = payload;
	return user;
}

function read_SessionId(request, config) {
	return parse_Cookies(request.headers.cookie)[config.cookieName] ?? '';
}

function clear_SessionCookie(response, config) {
	response.setHeader('Set-Cookie', build_SessionCookie(config, '', 0));
}

// Меняющие запросы: свой origin и CSRF-токен из сессии
function require_SafeWrite(request, session, config) {
	if (!check_RequestOrigin(request, config)) {
		throw new RequestError(403, ErrorCodes.originDenied);
	}
	if (!match_CsrfToken(session.csrfToken, request.headers[csrfHeader])) {
		throw new RequestError(403, ErrorCodes.csrfFailed);
	}
}

function build_PublicConfig(config) {
	return {
		adminAccessLevel: config.adminAccessLevel,
		userAccessLevel: config.userAccessLevel,
		maxUploadBytes: config.maxUploadBytes,
		allowedUploadExtensions: config.allowedUploadExtensions,
		debug: config.debug,
	};
}

async function handle_Login(context) {
	const { config, request, response, sessionStore, loginLimiter } = context;
	if (request.method !== 'POST') {
		throw new RequestError(405, ErrorCodes.methodNotAllowed);
	}
	if (!check_RequestOrigin(request, config)) {
		throw new RequestError(403, ErrorCodes.originDenied);
	}
	const clientAddress = read_ClientAddress(request);
	if (loginLimiter.check_IsBlocked(clientAddress)) {
		throw new RequestError(429, ErrorCodes.tooManyAttempts);
	}
	const body = await read_JsonBody(request, config.maxJsonBytes);
	if (typeof body?.login !== 'string' || typeof body?.password !== 'string' || body.login.trim() === '') {
		throw new RequestError(400, ErrorCodes.invalidBody);
	}

	const coreReply = await call_CoreJson(config, 'POST', '/auth/login', {
		body: { login: body.login.trim(), password: body.password },
	});
	if (coreReply.status === 401) {
		loginLimiter.register_Failure(clientAddress);
	}
	if (coreReply.status !== 200 || typeof coreReply.payload?.token !== 'string') {
		send_Json(response, coreReply.status === 200 ? 502 : coreReply.status, coreReply.payload ?? { error: ErrorCodes.coreUnavailable });
		return;
	}

	loginLimiter.reset_Address(clientAddress);
	const previousSessionId = read_SessionId(request, config);
	if (previousSessionId) {
		sessionStore.destroy_Session(previousSessionId);
	}
	const session = sessionStore.create_Session(coreReply.payload.token, pick_PublicUser(coreReply.payload));
	response.setHeader('Set-Cookie', build_SessionCookie(config, session.id, config.sessionTtlMs / 1000));
	send_Json(response, 200, { user: session.user, csrfToken: session.csrfToken });
}

async function handle_SessionInfo(context) {
	const { config, request, response, sessionStore, session } = context;
	if (request.method !== 'GET') {
		throw new RequestError(405, ErrorCodes.methodNotAllowed);
	}
	if (!session) {
		clear_SessionCookie(response, config);
		throw new RequestError(401, ErrorCodes.unauthorized);
	}
	// Профиль берём у ядра каждый раз: администратор мог сменить уровень
	const coreReply = await call_CoreJson(config, 'GET', '/me', { token: session.coreToken });
	if (coreReply.status === 401) {
		sessionStore.destroy_Session(session.id);
		clear_SessionCookie(response, config);
		throw new RequestError(401, ErrorCodes.unauthorized);
	}
	if (coreReply.status !== 200) {
		send_Json(response, coreReply.status, coreReply.payload ?? { error: ErrorCodes.coreUnavailable });
		return;
	}
	const user = pick_PublicUser(coreReply.payload);
	sessionStore.update_SessionUser(session.id, user);
	send_Json(response, 200, { user, csrfToken: session.csrfToken });
}

async function handle_Logout(context) {
	const { config, request, response, sessionStore, session } = context;
	if (request.method !== 'POST') {
		throw new RequestError(405, ErrorCodes.methodNotAllowed);
	}
	if (session) {
		require_SafeWrite(request, session, config);
		// В контракте ядра выхода пока нет. Когда появится - токен отзовётся и там
		try {
			await call_CoreJson(config, 'POST', '/auth/logout', { token: session.coreToken });
		} catch {
			// Ядро недоступно - сессию веба всё равно закрываем
		}
		sessionStore.destroy_Session(session.id);
	}
	clear_SessionCookie(response, config);
	send_Empty(response, 204);
}

async function handle_Proxy(context, apiPath, search) {
	const { config, request, response, sessionStore, session } = context;
	const section = apiPath.split('/')[1] ?? '';
	if (!proxiedCoreSections.has(section)) {
		throw new RequestError(404, ErrorCodes.notFound);
	}
	if (!session) {
		throw new RequestError(401, ErrorCodes.unauthorized);
	}
	if (!readOnlyMethods.has(request.method)) {
		require_SafeWrite(request, session, config);
	}
	const declaredLength = Number.parseInt(request.headers['content-length'] ?? '0', 10);
	if (declaredLength > config.maxUploadBytes) {
		throw new RequestError(413, ErrorCodes.payloadTooLarge);
	}
	await forward_ToCore(config, request, response, {
		corePath: apiPath + search,
		token: session.coreToken,
		onUnauthorized: () => {
			sessionStore.destroy_Session(session.id);
			clear_SessionCookie(response, config);
		},
	});
}

export async function handle_Api(context, url) {
	const { config, request, response, sessionStore } = context;
	apply_SecurityHeaders(response, config, 'api');
	const apiPath = url.pathname.slice('/api'.length) || '/';
	const session = sessionStore.find_Session(read_SessionId(request, config));
	const routeContext = { ...context, session };

	switch (apiPath) {
		case '/config':
			send_Json(response, 200, build_PublicConfig(config));
			return;
		case '/auth/login':
			await handle_Login(routeContext);
			return;
		case '/auth/session':
			await handle_SessionInfo(routeContext);
			return;
		case '/auth/logout':
			await handle_Logout(routeContext);
			return;
		default:
			if (apiPath.startsWith('/auth/')) {
				throw new RequestError(404, ErrorCodes.notFound);
			}
			await handle_Proxy(routeContext, apiPath, url.search);
	}
}

