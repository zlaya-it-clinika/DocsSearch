// Админ-панель: вкладки как в open-webui. Пользователи, база знаний, журнал, настройки входа.
import { create_Element, create_Icon } from '../lib/dom.js';
import { AdminTexts } from '../texts.js';
import { create_AuditTab } from './admin-audit.js';
import { create_KnowledgeTab } from './admin-knowledge.js';
import { create_SettingsTab } from './admin-settings.js';
import { create_UsersTab } from './admin-users.js';

const adminTabs = [
	{ id: 'users', icon: 'users', label: AdminTexts.tabUsers, create: create_UsersTab },
	{ id: 'knowledge', icon: 'book', label: AdminTexts.tabKnowledge, create: create_KnowledgeTab },
	{ id: 'audit', icon: 'list', label: AdminTexts.tabAudit, create: create_AuditTab },
	{ id: 'settings', icon: 'lock', label: AdminTexts.tabSettings, create: create_SettingsTab },
];

export function create_AdminView({ tab, user, config }) {
	const activeTab = adminTabs.find((item) => item.id === tab) ?? adminTabs[0];

	const tabsNav = create_Element('nav', { className: 'tabs', attrs: { 'aria-label': AdminTexts.title } },
		adminTabs.map((item) => create_Element('a', {
			className: item.id === activeTab.id ? 'tab is-active' : 'tab',
			attrs: {
				href: `/admin/${item.id}`,
				'data-link': true,
				'aria-current': item.id === activeTab.id ? 'page' : false,
			},
		}, [create_Icon(item.icon, 'icon-sm'), create_Element('span', { text: item.label })])));

	const content = activeTab.create({ user, config });

	const element = create_Element('div', { className: 'page' }, [
		create_Element('div', { className: 'page-inner' }, [
			create_Element('h1', { className: 'page-title', text: AdminTexts.title }),
			tabsNav,
			content.element,
		]),
	]);

	return {
		element,
		destroy: () => content.destroy?.(),
	};
}
