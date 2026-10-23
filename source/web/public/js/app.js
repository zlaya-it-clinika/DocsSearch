// Точка входа страницы: настройки, проверка сессии, маршруты.
import { fetch_AuthSession, fetch_PublicConfig, logout_User, on_Unauthorized, set_CsrfToken } from './api.js';
import { show_Toast } from './components/toast.js';
import { create_Button, create_Element, create_Icon } from './lib/dom.js';
import { apply_Theme, get_Theme } from './preferences.js';
import { check_IsAdmin } from './roles.js';
import { define_NotFound, define_Route, navigate_To, start_Router } from './router.js';
import { appState, update_State } from './state.js';
import { AdminTexts, AppTexts, AuthTexts, get_ErrorText } from './texts.js';
import { create_LoginView } from './views/login.js';
import { create_Shell } from './views/shell.js';

const root = document.getElementById('app');
let shell = null;

apply_Theme(get_Theme());
document.title = AppTexts.productName;

function render_Fatal(error) {
	root.replaceChildren(create_Element('main', { className: 'fatal-page' }, [
		create_Element('div', { className: 'fatal-card' }, [
			create_Icon('alert', 'icon-lg'),
			create_Element('h1', { className: 'auth-title', text: AppTexts.fatalTitle }),
			create_Element('p', { className: 'fatal-text', text: get_ErrorText(error) }),
			create_Element('p', { className: 'fatal-text', text: AppTexts.fatalHint }),
			create_Button(AppTexts.reload, { variant: 'primary', icon: 'refresh', onClick: () => window.location.reload() }),
		]),
	]));
}

function apply_Session(session) {
	set_CsrfToken(session.csrfToken);
	update_State({ user: session.user });
}

function reset_Session() {
	set_CsrfToken('');
	update_State({ user: null });
	shell = null;
}

async function handle_Logout() {
	try {
		await logout_User();
	} catch {
		// Сессию на клиенте сбрасываем в любом случае
	}
	reset_Session();
	navigate_To('/login', { replace: true });
	show_Toast(AuthTexts.loggedOut, 'info');
}

function ensure_Shell() {
	if (!shell) {
		shell = create_Shell({ root, user: appState.user, config: appState.config, onLogout: handle_Logout });
	}
	return shell;
}

// Маршруты требуют входа: без пользователя уводим на экран входа
function require_User(handler) {
	return (params) => {
		if (!appState.user) {
			navigate_To('/login', { replace: true });
			return;
		}
		handler(params);
	};
}

function show_Login() {
	if (appState.user) {
		navigate_To('/', { replace: true });
		return;
	}
	shell = null;
	const view = create_LoginView({
		onSuccess: (session) => {
			apply_Session(session);
			navigate_To('/', { replace: true });
		},
	});
	root.replaceChildren(view.element);
	view.focus();
}

function show_Admin(tab) {
	if (!check_IsAdmin(appState.user, appState.config)) {
		show_Toast(AdminTexts.onlyAdmins, 'error');
		navigate_To('/', { replace: true });
		return;
	}
	ensure_Shell().show_Admin(tab);
}

async function boot() {
	try {
		update_State({ config: await fetch_PublicConfig() });
	} catch (error) {
		render_Fatal(error);
		return;
	}

	try {
		apply_Session(await fetch_AuthSession());
	} catch (error) {
		if (error.status !== 401) {
			show_Toast(get_ErrorText(error), 'error');
		}
	}

	on_Unauthorized(() => {
		if (!appState.user) {
			return;
		}
		reset_Session();
		navigate_To('/login', { replace: true });
		show_Toast(AuthTexts.sessionExpired, 'info');
	});

	define_Route('/login', show_Login);
	define_Route('/', require_User(() => ensure_Shell().show_Chat(null)));
	define_Route('/documents', require_User(() => ensure_Shell().show_Documents()));
	define_Route('/c/:sessionId', require_User(({ sessionId }) => ensure_Shell().show_Chat(sessionId)));
	define_Route('/admin', require_User(() => navigate_To('/admin/users', { replace: true })));
	define_Route('/admin/:tab', require_User(({ tab }) => show_Admin(tab)));
	define_NotFound(() => navigate_To(appState.user ? '/' : '/login', { replace: true }));
	start_Router();
}

boot();
