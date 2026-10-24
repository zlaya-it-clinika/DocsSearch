// Экран чата: история сессии, отправка вопросов, проверка черновика.
// Один и тот же экран работает на полной странице и в виджете (compact).
import { check_Draft, create_ChatSession, fetch_ChatSession, list_Processes, send_ChatMessage, upload_SessionAttachment } from '../api.js';
import { create_Composer } from '../components/composer.js';
import { MessageKinds, render_Message, render_PendingMessage } from '../components/message.js';
import { create_Button, create_Element, create_EmptyBlock, create_Icon, create_LoadingBlock } from '../lib/dom.js';
import { build_ChatTitle } from '../lib/format.js';
import { Permissions, has_Permission } from '../roles.js';
import { AppTexts, ChatTexts, get_ErrorText } from '../texts.js';

function render_Greeting(user) {
	const name = user?.displayName ? `, ${user.displayName}` : '';
	return create_Element('div', { className: 'chat-greeting' }, [
		create_Element('h1', { className: 'chat-greeting-title', text: `${ChatTexts.greetingPrefix}${name}` }),
		create_Element('p', { className: 'chat-greeting-lead', text: ChatTexts.greetingLead }),
		create_Element('div', { className: 'chat-hints' }, [
			create_Element('span', { className: 'chat-hint' }, [create_Icon('quote', 'icon-sm'), ChatTexts.hintSources]),
			create_Element('span', { className: 'chat-hint' }, [create_Icon('file', 'icon-sm'), ChatTexts.hintTemplates]),
			create_Element('span', { className: 'chat-hint' }, [create_Icon('file-check', 'icon-sm'), ChatTexts.hintDrafts]),
		]),
	]);
}

// onSessionCreated(sessionId) - ядро выдало id новой сессии, можно обновить адрес
export function create_ChatView({ sessionId = null, user, config, history, compact = false, onSessionCreated }) {
	let currentSessionId = sessionId;
	let messages = [];
	let pending = false;
	let loading = Boolean(sessionId);
	let destroyed = false;
	let dragDepth = 0;
	let processId = '';

	const allowCheck = has_Permission(user, config, Permissions.draftsCheck);
	const allowAssist = has_Permission(user, config, Permissions.draftsAssist);
	const canChat = has_Permission(user, config, Permissions.chatUse);

	const messagesList = create_Element('div', { className: 'chat-messages', attrs: { role: 'log', 'aria-live': 'polite' } });
	const greeting = render_Greeting(user);
	const column = create_Element('div', { className: 'chat-column' }, [greeting, messagesList]);
	const scroll = create_Element('div', { className: 'chat-scroll' }, [column]);

	const composer = create_Composer({
		onSubmit: (payload) => handle_Submit(payload),
		allowCheck,
		allowAssist,
		config,
	});
	const processSelect = create_Element('select', {
		className: 'select process-select',
		attrs: { 'aria-label': ChatTexts.processLabel },
	});
	processSelect.hidden = true;
	const quickActions = create_Element('div', { className: 'quick-actions' });
	quickActions.hidden = true;
	const footer = create_Element('div', { className: 'chat-footer' }, [
		processSelect,
		quickActions,
		canChat ? composer.element : create_Element('div', { className: 'callout-hint', text: ChatTexts.chatDenied }),
		create_Element('div', { className: 'chat-disclaimer', text: ChatTexts.disclaimer }),
	]);
	const dropOverlay = create_Element('div', { className: 'drop-overlay' }, [
		create_Icon('upload', 'icon-lg'),
		create_Element('div', { className: 'drop-overlay-title', text: ChatTexts.dropTitle }),
		create_Element('div', { text: ChatTexts.dropLead }),
	]);
	dropOverlay.hidden = true;

	const view = create_Element('div', { className: compact ? 'chat-view is-compact' : 'chat-view' }, [scroll, footer, dropOverlay]);

	const handlers = {
		onRepeat: (text) => send_Text(text),
		onDraftContinue: () => composer.focus(),
		onRetry: (message) => {
			if (message.retryFile && message.attachMode === 'assist') {
				run_DraftAssist(message.retryFile, message.text, message);
				return;
			}
			if (message.kind === MessageKinds.draftRequest && message.retryFile) {
				run_DraftCheck(message.retryFile, message);
				return;
			}
			send_Text(message.text, message);
		},
	};

	const quickPrompts = [
		[ChatTexts.quickExpand, ChatTexts.quickExpandText],
		[ChatTexts.quickShorten, ChatTexts.quickShortenText],
		[ChatTexts.quickFormat, ChatTexts.quickFormatText],
	];
	for (const [label, text] of quickPrompts) {
		quickActions.append(create_Button(label, {
			variant: 'outline',
			onClick: () => composer.set_Text(text),
		}));
	}

	processSelect.addEventListener('change', () => {
		processId = processSelect.value;
	});
	list_Processes().then((payload) => {
		const processes = Array.isArray(payload?.processes) ? payload.processes : [];
		if (destroyed || processes.length === 0) {
			return;
		}
		processSelect.append(
			create_Element('option', { text: ChatTexts.processAll, attrs: { value: '' } }),
			...processes.map((process) => create_Element('option', { text: process.title, attrs: { value: process.processId } })),
		);
		processSelect.hidden = false;
	}).catch(() => {});

	function scroll_ToBottom() {
		requestAnimationFrame(() => {
			scroll.scrollTop = scroll.scrollHeight;
		});
	}

	function find_FirstQuestion() {
		return messages.find((message) => message.role === 'user' && message.text)?.text ?? '';
	}

	// Если ассистент спросил поле бланка, подсказка в поле ввода называет это поле
	function update_FieldPrompt() {
		const lastMessage = messages[messages.length - 1];
		const isFieldQuestion = lastMessage?.role === 'assistant' && lastMessage.kind === MessageKinds.templateField;
		composer.set_FieldPrompt(isFieldQuestion ? (lastMessage.template?.fieldLabel ?? lastMessage.template?.field) : null);
	}

	function render_View() {
		if (destroyed) {
			return;
		}
		const isEmpty = messages.length === 0 && !pending && !loading;
		view.classList.toggle('is-empty', isEmpty);
		greeting.hidden = !isEmpty;
		if (loading) {
			messagesList.replaceChildren(create_LoadingBlock(ChatTexts.thinking));
			return;
		}
		messagesList.replaceChildren(
			...messages.map((message) => render_Message(message, handlers)),
			...(pending ? [render_PendingMessage()] : []),
		);
		update_FieldPrompt();
		quickActions.hidden = !messages.some((message) => message.kind === MessageKinds.draftRevision);
	}

	function render_LoadError(error) {
		loading = false;
		view.classList.remove('is-empty');
		greeting.hidden = true;
		if (error.status === 404) {
			history?.forget_Chat(currentSessionId);
			messagesList.replaceChildren(create_EmptyBlock('help', ChatTexts.notFoundTitle, [
				create_Element('div', { className: 'callout-hint', text: ChatTexts.notFoundLead }),
				create_Element('a', { className: 'button button-primary', attrs: { href: '/', 'data-link': true }, text: ChatTexts.startNew }),
			]));
			footer.hidden = true;
			return;
		}
		messagesList.replaceChildren(create_EmptyBlock('alert', ChatTexts.loadFailed, [
			create_Element('div', { className: 'callout-hint', text: get_ErrorText(error) }),
			create_Button(AppTexts.retry, { variant: 'outline', icon: 'refresh', onClick: () => load_Session() }),
		]));
	}

	async function load_Session() {
		loading = true;
		footer.hidden = false;
		render_View();
		try {
			const payload = await fetch_ChatSession(currentSessionId);
			if (destroyed) {
				return;
			}
			messages = Array.isArray(payload?.messages) ? payload.messages : [];
			loading = false;
			render_View();
			scroll_ToBottom();
		} catch (error) {
			if (!destroyed) {
				render_LoadError(error);
			}
		}
	}

	async function ensure_Session(firstText) {
		if (currentSessionId) {
			return currentSessionId;
		}
		const created = await create_ChatSession();
		currentSessionId = created.sessionId;
		history?.remember_Chat(currentSessionId, build_ChatTitle(firstText));
		onSessionCreated?.(currentSessionId);
		return currentSessionId;
	}

	async function send_Text(text, existingMessage = null) {
		const userMessage = existingMessage ?? { role: 'user', text };
		if (existingMessage) {
			existingMessage.failed = false;
		} else {
			messages.push(userMessage);
		}
		pending = true;
		composer.set_Busy(true);
		render_View();
		scroll_ToBottom();
		try {
			const activeSessionId = await ensure_Session(text);
			const reply = await send_ChatMessage(activeSessionId, text, { processId });
			messages.push({ ...reply, role: 'assistant' });
			history?.remember_Chat(activeSessionId, build_ChatTitle(find_FirstQuestion()));
		} catch (error) {
			userMessage.failed = true;
			userMessage.errorText = get_ErrorText(error);
		} finally {
			pending = false;
			composer.set_Busy(false);
			render_View();
			scroll_ToBottom();
		}
	}

	async function run_DraftCheck(file, existingMessage = null) {
		const requestMessage = existingMessage ?? {
			role: 'user',
			kind: MessageKinds.draftRequest,
			fileName: file.name,
			fileSize: file.size,
			retryFile: file,
		};
		if (existingMessage) {
			existingMessage.failed = false;
		} else {
			messages.push(requestMessage);
		}
		pending = true;
		composer.set_Busy(true);
		render_View();
		scroll_ToBottom();
		try {
			const payload = await check_Draft(file);
			messages.push({
				role: 'assistant',
				kind: MessageKinds.draftCheck,
				fileName: file.name,
				recommendations: Array.isArray(payload?.recommendations) ? payload.recommendations : [],
			});
		} catch (error) {
			requestMessage.failed = true;
			requestMessage.errorText = get_ErrorText(error);
		} finally {
			pending = false;
			composer.set_Busy(false);
			render_View();
			scroll_ToBottom();
		}
	}

	async function run_DraftAssist(file, text, existingMessage = null) {
		const question = text.trim() || ChatTexts.draftAssistDefault;
		const requestMessage = existingMessage ?? {
			role: 'user',
			text: question,
			attachMode: 'assist',
			retryFile: file,
			attachments: [{ fileName: file.name }],
		};
		if (existingMessage) {
			existingMessage.failed = false;
		} else {
			messages.push(requestMessage);
		}
		pending = true;
		composer.set_Busy(true);
		render_View();
		scroll_ToBottom();
		try {
			const activeSessionId = await ensure_Session(question);
			const uploaded = await upload_SessionAttachment(activeSessionId, file);
			requestMessage.attachments = [{ attachmentId: uploaded.attachmentId, fileName: uploaded.fileName }];
			const reply = await send_ChatMessage(activeSessionId, question, {
				processId,
				attachmentIds: [uploaded.attachmentId],
			});
			messages.push({ ...reply, role: 'assistant' });
			history?.remember_Chat(activeSessionId, build_ChatTitle(find_FirstQuestion()));
		} catch (error) {
			requestMessage.failed = true;
			requestMessage.errorText = get_ErrorText(error);
		} finally {
			pending = false;
			composer.set_Busy(false);
			render_View();
			scroll_ToBottom();
		}
	}

	// Проверка файла идёт отдельно. Помощь с черновиком отправляет файл и текст одним шагом.
	async function handle_Submit({ text, file, attachMode }) {
		if (file && attachMode === 'assist') {
			await run_DraftAssist(file, text);
			return;
		}
		if (file) {
			await run_DraftCheck(file);
		}
		if (text) {
			await send_Text(text);
		}
	}

	// Перетаскивание файла в окно чата
	const has_Files = (event) => Array.from(event.dataTransfer?.types ?? []).includes('Files');
	view.addEventListener('dragenter', (event) => {
		if (!has_Files(event)) {
			return;
		}
		event.preventDefault();
		dragDepth += 1;
		dropOverlay.hidden = false;
	});
	view.addEventListener('dragover', (event) => {
		if (has_Files(event)) {
			event.preventDefault();
		}
	});
	view.addEventListener('dragleave', () => {
		dragDepth = Math.max(0, dragDepth - 1);
		if (dragDepth === 0) {
			dropOverlay.hidden = true;
		}
	});
	view.addEventListener('drop', (event) => {
		if (!has_Files(event)) {
			return;
		}
		event.preventDefault();
		dragDepth = 0;
		dropOverlay.hidden = true;
		const [file] = event.dataTransfer.files;
		if (file) {
			composer.attach_File(file);
		}
	});

	render_View();
	if (currentSessionId) {
		load_Session();
	}

	return {
		element: view,
		get_SessionId: () => currentSessionId,
		focus: () => composer.focus(),
		destroy() {
			destroyed = true;
		},
	};
}
