// Выбор категории знаний для чата: кнопка рядом с «+» под полем ввода.
// «Все знания» - поиск по всем категориям. Выбор хранится в сессии ядра, новый чат берёт последний выбор пользователя.
import { create_ProcessDot, get_KnownProcess, try_LoadProcesses } from '../processes.js';
import { create_Element, create_Icon } from '../lib/dom.js';
import { read_Preference, write_Preference } from '../preferences.js';
import { ProcessTexts } from '../texts.js';
import { open_Popover } from './popover.js';

function get_PreferenceName(user) {
	return `knowledge.${user.userId}`;
}

function read_LastProcess(user) {
	const stored = read_Preference(get_PreferenceName(user), '');
	return typeof stored === 'string' ? stored : '';
}

export function create_ProcessPicker({ user }) {
	let processes = [];
	let selected = read_LastProcess(user);
	let active = false;

	const dot = create_Element('span', { className: 'knowledge-chip-dot' });
	const label = create_Element('span', { className: 'knowledge-chip-label' });
	const button = create_Element('button', {
		className: 'knowledge-chip',
		attrs: { type: 'button', 'aria-haspopup': 'menu', 'aria-expanded': 'false', title: ProcessTexts.pickerLabel },
	}, [create_Icon('book', 'icon-sm'), dot, label, create_Icon('chevron-down', 'icon-sm')]);
	button.hidden = true;

	const render_Button = () => {
		const process = selected ? get_KnownProcess(selected) : null;
		label.textContent = process ? process.title : ProcessTexts.allKnowledge;
		dot.replaceChildren(process ? create_ProcessDot(process.color) : '');
		button.classList.toggle('is-filtered', Boolean(process));
		button.setAttribute('aria-label', `${ProcessTexts.pickerLabel}: ${label.textContent}`);
	};

	// Удалённая или неизвестная категория превращается во «Все знания»
	const apply_Selection = (processId, { remember = true } = {}) => {
		selected = processes.some((item) => item.processId === processId) ? processId : '';
		render_Button();
		if (remember) {
			write_Preference(get_PreferenceName(user), selected);
		}
	};

	const create_Option = (processId, title, hint, color, close) => {
		const isChecked = selected === processId;
		return create_Element('button', {
			className: 'menu-item topic-option',
			attrs: { type: 'button', role: 'menuitemradio', 'aria-checked': isChecked ? 'true' : 'false' },
			on: {
				click: () => {
					apply_Selection(processId);
					close();
				},
			},
		}, [
			isChecked ? create_Icon('check', 'icon-sm') : create_Element('span', { className: 'topic-option-spacer' }),
			color ? create_ProcessDot(color) : create_Icon('book', 'icon-sm'),
			create_Element('span', { className: 'topic-option-text' }, [
				create_Element('span', { className: 'topic-option-title', text: title }),
				hint ? create_Element('span', { className: 'topic-option-hint', text: hint }) : null,
			]),
		]);
	};

	button.addEventListener('click', () => {
		open_Popover(button, (close) => [
			create_Element('div', { className: 'menu-label', text: ProcessTexts.pickerTitle }),
			create_Option('', ProcessTexts.allKnowledge, ProcessTexts.allKnowledgeHint, null, close),
			create_Element('div', { className: 'menu-separator' }),
			...processes.map((process) => create_Option(process.processId, process.title, process.description, process.color, close)),
		], { placement: 'top-start' });
	});

	render_Button();
	try_LoadProcesses()
		.then((result) => {
			processes = result.processes;
			// Одна категория - выбирать нечего, кнопка не нужна
			active = result.supported && processes.length > 1;
			button.hidden = !active;
			apply_Selection(selected, { remember: false });
		})
		.catch(() => {
			button.hidden = true;
		});

	return {
		element: button,
		// undefined - ядро без процессов, поле processId не отправляем
		get_ProcessId: () => (active ? selected : undefined),
		// Выбор из истории сессии: не меняет выбор по умолчанию для новых чатов
		set_ProcessId: (processId) => {
			selected = typeof processId === 'string' ? processId : '';
			if (processes.length > 0) {
				apply_Selection(selected, { remember: false });
			}
		},
	};
}
