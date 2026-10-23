// Список чатов для боковой панели.
// Основной источник - GET /sessions ядра. Пока ядро его не отдаёт, помним чаты,
// созданные в этом браузере: только id, заголовок и время, ответы всегда берутся из ядра.
import { check_IsNotSupported, list_ChatSessions } from './api.js';
import { read_Preference, write_Preference } from './preferences.js';
import { SidebarTexts } from './texts.js';

export const HistorySources = Object.freeze({
	core: 'core',
	local: 'local',
});

const maxLocalChats = 200;

function get_StorageName(user) {
	return `chats.${user.userId}`;
}

function normalize_Item(raw) {
	if (!raw || typeof raw.sessionId !== 'string') {
		return null;
	}
	return {
		sessionId: raw.sessionId,
		title: typeof raw.title === 'string' && raw.title.trim() !== '' ? raw.title : SidebarTexts.untitledChat,
		createdAt: raw.createdAt ?? null,
		updatedAt: raw.updatedAt ?? raw.createdAt ?? null,
	};
}

function sort_ByRecent(items) {
	return items.sort((first, second) => String(second.updatedAt ?? '').localeCompare(String(first.updatedAt ?? '')));
}

function read_LocalItems(user) {
	const stored = read_Preference(get_StorageName(user), []);
	return Array.isArray(stored) ? stored.map(normalize_Item).filter(Boolean) : [];
}

function write_LocalItems(user, items) {
	write_Preference(get_StorageName(user), items.slice(0, maxLocalChats));
}

export function create_ChatHistory(user) {
	let source = HistorySources.core;
	let items = [];
	const listeners = new Set();

	const notify = () => {
		for (const listener of listeners) {
			listener({ source, items });
		}
	};

	return {
		subscribe(listener) {
			listeners.add(listener);
			return () => listeners.delete(listener);
		},

		get_Snapshot() {
			return { source, items };
		},

		async load_Items() {
			try {
				const payload = await list_ChatSessions();
				const rawItems = Array.isArray(payload?.sessions) ? payload.sessions : [];
				source = HistorySources.core;
				items = sort_ByRecent(rawItems.map(normalize_Item).filter(Boolean));
			} catch (error) {
				if (!check_IsNotSupported(error)) {
					throw error;
				}
				source = HistorySources.local;
				items = sort_ByRecent(read_LocalItems(user));
			}
			notify();
			return { source, items };
		},

		// Чат создан или получил новое сообщение
		remember_Chat(sessionId, title) {
			const now = new Date().toISOString();
			const existing = items.find((item) => item.sessionId === sessionId);
			if (existing) {
				existing.updatedAt = now;
				if (title && existing.title === SidebarTexts.untitledChat) {
					existing.title = title;
				}
			} else {
				items.unshift({ sessionId, title: title || SidebarTexts.untitledChat, createdAt: now, updatedAt: now });
			}
			sort_ByRecent(items);
			if (source === HistorySources.local) {
				write_LocalItems(user, items);
			}
			notify();
		},

		// Ядро ответило 404 на чат из локального списка: убираем его
		forget_Chat(sessionId) {
			items = items.filter((item) => item.sessionId !== sessionId);
			if (source === HistorySources.local) {
				write_LocalItems(user, items);
			}
			notify();
		},
	};
}
