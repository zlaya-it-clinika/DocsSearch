// Боковая панель: новый чат, поиск, история по датам, меню пользователя.
import { HistorySources } from '../chat-history.js';
import { create_Element, create_Icon, create_IconButton, create_SearchBox } from '../lib/dom.js';
import { get_DateGroupLabel, get_Initials } from '../lib/format.js';
import { Themes, get_Theme, set_Theme } from '../preferences.js';
import { check_IsAdmin, get_RoleLabel, get_UserRole } from '../roles.js';
import { AppTexts, SidebarTexts } from '../texts.js';
import { create_MenuItem, open_Popover } from './popover.js';

const themeOptions = [
	{ value: Themes.system, icon: 'monitor', label: SidebarTexts.themeSystem },
	{ value: Themes.light, icon: 'sun', label: SidebarTexts.themeLight },
	{ value: Themes.dark, icon: 'moon', label: SidebarTexts.themeDark },
];

function group_ByDate(items) {
	const groups = [];
	const now = new Date();
	for (const item of items) {
		const label = get_DateGroupLabel(item.updatedAt ?? item.createdAt, now);
		let group = groups.find((existing) => existing.label === label);
		if (!group) {
			group = { label, items: [] };
			groups.push(group);
		}
		group.items.push(item);
	}
	return groups;
}

function create_ThemeSwitch(closeMenu) {
	const current = get_Theme();
	return create_Element('div', { className: 'segmented', attrs: { role: 'group', 'aria-label': SidebarTexts.themeLabel } },
		themeOptions.map((option) => create_Element('button', {
			className: 'segmented-option',
			attrs: { type: 'button', 'aria-pressed': option.value === current ? 'true' : 'false', title: option.label },
			on: {
				click: () => {
					set_Theme(option.value);
					closeMenu();
				},
			},
		}, [create_Icon(option.icon, 'icon-sm'), create_Element('span', { text: option.label })])));
}

// onNewChat, onLogout, onToggle (свернуть), onOpenDocuments
export function create_Sidebar({ user, config, history, onNewChat, onLogout, onToggle, onOpenDocuments }) {
	let query = '';
	let activeSessionId = null;
	let currentUser = user;

	const list = create_Element('nav', { attrs: { 'aria-label': SidebarTexts.searchPlaceholder } });
	const note = create_Element('p', { className: 'sidebar-note', text: SidebarTexts.localHistoryNote });
	note.hidden = true;

	const render_List = () => {
		const { source, items } = history.get_Snapshot();
		note.hidden = source !== HistorySources.local || items.length === 0;
		const filtered = query ? items.filter((item) => item.title.toLowerCase().includes(query)) : items;
		if (filtered.length === 0) {
			list.replaceChildren(create_Element('div', {
				className: 'sidebar-note',
				text: query ? SidebarTexts.nothingFound : SidebarTexts.empty,
			}));
			return;
		}
		list.replaceChildren(...group_ByDate(filtered).map((group) => create_Element('div', { className: 'chat-group' }, [
			create_Element('div', { className: 'chat-group-title', text: group.label }),
			...group.items.map((item) => create_Element('a', {
				className: item.sessionId === activeSessionId ? 'chat-link is-active' : 'chat-link',
				text: item.title,
				attrs: {
					href: `/c/${encodeURIComponent(item.sessionId)}`,
					'data-link': true,
					title: item.title,
					'aria-current': item.sessionId === activeSessionId ? 'page' : false,
				},
			})),
		])));
	};

	const userName = create_Element('span', { className: 'user-button-name' });
	const userRole = create_Element('span', { className: 'user-button-role' });
	const avatar = create_Element('span', { className: 'avatar', attrs: { 'aria-hidden': 'true' } });
	const userButton = create_Element('button', {
		className: 'user-button',
		attrs: { type: 'button', 'aria-haspopup': 'menu', 'aria-expanded': 'false', 'aria-label': SidebarTexts.userMenu },
	}, [avatar, create_Element('span', { className: 'user-button-text' }, [userName, userRole])]);

	const render_User = () => {
		avatar.textContent = get_Initials(currentUser.displayName);
		userName.textContent = currentUser.displayName;
		userRole.textContent = get_RoleLabel(get_UserRole(currentUser, config));
	};

	userButton.addEventListener('click', () => {
		open_Popover(userButton, (close) => [
			create_Element('div', { className: 'menu-label', text: SidebarTexts.themeLabel }),
			create_ThemeSwitch(close),
			create_Element('div', { className: 'menu-separator' }),
			create_MenuItem(create_Icon('book'), SidebarTexts.documents, () => {
				close();
				onOpenDocuments();
			}),
			check_IsAdmin(currentUser, config)
				? create_MenuItem(create_Icon('shield'), SidebarTexts.adminPanel, close, { href: '/admin/users', dataLink: true })
				: null,
			create_Element('div', { className: 'menu-separator' }),
			create_MenuItem(create_Icon('logout'), SidebarTexts.logout, () => {
				close();
				onLogout();
			}),
		], { placement: 'top-start' });
	});

	const newChatLink = create_Element('a', {
		className: 'sidebar-nav-button',
		attrs: { href: '/', 'data-link': true },
		on: { click: () => onNewChat?.() },
	}, [create_Icon('new-chat'), create_Element('span', { text: SidebarTexts.newChat })]);

	const element = create_Element('aside', { className: 'sidebar', attrs: { 'aria-label': AppTexts.productName } }, [
		create_Element('div', { className: 'sidebar-header' }, [
			create_Element('a', { className: 'brand', attrs: { href: '/', 'data-link': true } }, [
				create_Element('span', { className: 'brand-mark' }, [create_Icon('logo')]),
				create_Element('span', { text: AppTexts.productName }),
			]),
			create_IconButton('sidebar', SidebarTexts.collapse, () => onToggle()),
		]),
		create_Element('div', { className: 'sidebar-actions' }, [
			newChatLink,
			create_SearchBox(SidebarTexts.searchPlaceholder, (value) => {
				query = value;
				render_List();
			}),
		]),
		create_Element('div', { className: 'sidebar-scroll' }, [list, note]),
		create_Element('div', { className: 'sidebar-footer' }, [userButton]),
	]);

	history.subscribe(render_List);
	render_User();
	render_List();

	return {
		element,
		set_ActiveChat(sessionId) {
			activeSessionId = sessionId;
			render_List();
		},
		update_User(nextUser) {
			currentUser = nextUser;
			render_User();
		},
		show_LoadError() {
			list.replaceChildren(create_Element('div', { className: 'sidebar-note', text: SidebarTexts.loadFailed }));
		},
	};
}
