// Отрисовка реплик чата. Вид ответа ассистента выбирается по полю kind из контракта ядра.
import { build_LinkedFileUrl } from '../api.js';
import { copy_ToClipboard, create_Element, create_Icon, create_IconButton, create_Logo, download_TextFile } from '../lib/dom.js';
import { build_SafeFileName, format_FileSize } from '../lib/format.js';
import { render_Markdown } from '../lib/markdown.js';
import { reveal_Message } from '../lib/reveal.js';
import { create_ProcessBadge, get_KnownProcess } from '../processes.js';
import { AppTexts, ChatTexts, ProcessTexts } from '../texts.js';
import { create_CitationChip, render_Relevance, render_Sources } from './citations.js';
import { show_Toast } from './toast.js';

export const MessageKinds = Object.freeze({
	answer: 'answer',
	clarify: 'clarify',
	fallback: 'fallback',
	templateField: 'template_field',
	templateReady: 'template_ready',
	// Режим «План» не узнал вид документа и предлагает список
	templateChoice: 'template_choice',
	accessDenied: 'access_denied',
	// Локальные виды: проверка черновика не входит в историю сессии ядра
	draftRequest: 'draft_request',
	draftCheck: 'draft_check',
	draftRevision: 'draft_revision',
});

const kindLabels = {
	[MessageKinds.clarify]: ChatTexts.kindClarify,
	[MessageKinds.fallback]: ChatTexts.kindFallback,
	[MessageKinds.accessDenied]: ChatTexts.kindAccessDenied,
	[MessageKinds.templateField]: ChatTexts.kindTemplateField,
	[MessageKinds.templateReady]: ChatTexts.kindTemplateReady,
	[MessageKinds.templateChoice]: ChatTexts.kindTemplateChoice,
	[MessageKinds.draftCheck]: ChatTexts.kindDraftCheck,
	[MessageKinds.draftRevision]: ChatTexts.kindDraftRevision,
};

async function copy_WithFeedback(text) {
	try {
		await copy_ToClipboard(text);
		show_Toast(AppTexts.copied, 'success');
	} catch {
		show_Toast(AppTexts.copied, 'info');
	}
}

function create_CopyButton(text) {
	return create_IconButton('copy', ChatTexts.copyMessage, () => copy_WithFeedback(text), 'icon-button icon-button-sm');
}

function create_Callout(iconName, text, hint, variant = '') {
	return create_Element('div', { className: ['callout', variant].filter(Boolean).join(' ') }, [
		create_Icon(iconName),
		create_Element('div', { className: 'callout-content' }, [
			render_Markdown(text),
			hint ? create_Element('div', { className: 'callout-hint', text: hint }) : null,
		]),
	]);
}

function create_DocCard(iconName, title, subtitle, bodyChildren, actions = []) {
	const visibleActions = actions.filter(Boolean);
	return create_Element('div', { className: 'doc-card' }, [
		create_Element('div', { className: 'doc-card-header' }, [
			create_Icon(iconName),
			create_Element('div', { className: 'doc-card-title' }, [
				create_Element('div', { text: title }),
				subtitle ? create_Element('div', { className: 'doc-card-subtitle', text: subtitle }) : null,
			]),
		]),
		create_Element('div', { className: 'doc-card-body' }, bodyChildren),
		visibleActions.length > 0 ? create_Element('div', { className: 'doc-card-actions' }, visibleActions) : null,
	]);
}

function create_DownloadLink(link, label) {
	const href = build_LinkedFileUrl(link);
	if (!href) {
		return null;
	}
	return create_Element('a', {
		className: 'button button-outline',
		attrs: { href, download: '' },
	}, [create_Icon('download', 'icon-sm'), create_Element('span', { text: label })]);
}

function render_Contacts(contacts) {
	if (!Array.isArray(contacts) || contacts.length === 0) {
		return null;
	}
	return create_Element('div', { className: 'doc-card contact-card' }, [
		create_Element('div', { className: 'doc-card-header' }, [
			create_Icon('users'),
			create_Element('div', { className: 'doc-card-title' }, [
				create_Element('div', { text: ChatTexts.contactsTitle }),
			]),
		]),
		create_Element('div', { className: 'doc-card-body' }, contacts.map((contact) => create_Element('div', { className: 'contact-row' }, [
			create_Element('strong', { text: contact.department || AppTexts.notSpecified }),
			contact.channel ? create_Element('div', { className: 'callout-hint', text: contact.channel }) : null,
		]))),
	]);
}

// Справка о документе: документ целиком для скачивания
function render_Download(download) {
	const url = build_LinkedFileUrl(download?.link, '/documents/');
	if (!url) {
		return null;
	}
	return create_Element('div', { className: 'message-download' }, [
		create_Element('a', {
			className: 'button button-ghost message-download-button',
			attrs: { href: url, target: '_blank', rel: 'noopener', title: download.document },
		}, [create_Icon('download', 'icon-sm'), create_Element('span', { text: `${ChatTexts.downloadDocument}: ${download.document}` })]),
	]);
}

function render_AnswerBody(message) {
	const citations = Array.isArray(message.citations) ? message.citations : [];
	const children = [render_Markdown(message.text)];
	const download = render_Download(message.download);
	if (download) {
		children.push(download);
	}
	if (Number(message.relevancePercent) > 0) {
		children.push(render_Relevance(message.relevancePercent));
	}
	if (citations.length > 0) {
		children.push(render_Sources(citations, message.relevancePercent));
	}
	const contacts = render_Contacts(message.contacts);
	if (contacts) {
		children.push(contacts);
	}
	return children;
}

function render_TemplateFieldBody(message) {
	const template = message.template ?? {};
	return [
		render_Markdown(message.text),
		create_DocCard('file', template.title ?? ChatTexts.kindTemplateField, '', [
			create_Element('div', { className: 'field-step' }, [
				create_Element('span', { className: 'field-step-label', text: `${ChatTexts.templateField}:` }),
				create_Element('strong', { text: template.fieldLabel ?? template.field ?? AppTexts.notSpecified }),
			]),
			create_Element('div', { className: 'callout-hint', text: ChatTexts.templateProgressHint }),
		], [
			create_DownloadLink(template.fileLink, ChatTexts.downloadBlank),
		]),
	];
}

function render_TemplateReadyBody(message) {
	const template = message.template ?? {};
	const draftText = template.draftText ?? '';
	const fileName = `${build_SafeFileName(template.title)} - ${ChatTexts.draftFileSuffix}.txt`;
	return [
		render_Markdown(message.text),
		create_DocCard('file-check', template.title ?? ChatTexts.kindTemplateReady, ChatTexts.kindTemplateReady, [
			create_Element('pre', { className: 'draft-text', text: draftText }),
		], [
			create_Element('button', {
				className: 'button button-outline',
				attrs: { type: 'button' },
				on: { click: () => copy_WithFeedback(draftText) },
			}, [create_Icon('copy', 'icon-sm'), create_Element('span', { text: ChatTexts.draftCopy })]),
			create_Element('button', {
				className: 'button button-outline',
				attrs: { type: 'button' },
				on: { click: () => download_TextFile(fileName, draftText) },
			}, [create_Icon('download', 'icon-sm'), create_Element('span', { text: ChatTexts.draftDownload })]),
			create_DownloadLink(template.fileLink, ChatTexts.downloadBlank),
			create_DownloadLink(template.draftLink, ChatTexts.downloadDocx),
		]),
	];
}

function render_DraftCheckBody(message) {
	const recommendations = Array.isArray(message.recommendations) ? message.recommendations : [];
	const list = recommendations.length === 0
		? create_Element('div', { className: 'callout-hint', text: ChatTexts.draftCheckEmpty })
		: create_Element('ol', { className: 'recommendations' }, recommendations.map((recommendation, index) => create_Element('li', { className: 'recommendation' }, [
			create_Element('div', { text: recommendation.text }),
			recommendation.citation
				? create_Element('div', { className: 'chip-list' }, [create_CitationChip(recommendation.citation, index, 0)])
				: null,
		])));
	return [
		create_DocCard('file-check', ChatTexts.draftCheckTitle, message.fileName, [list]),
		create_Element('div', { className: 'local-note', text: ChatTexts.draftCheckLocalNote }),
	];
}

function render_DraftRevisionBody(message, handlers) {
	const draft = message.draft ?? {};
	const changes = Array.isArray(draft.changes) ? draft.changes : [];
	const changeList = changes.length === 0
		? null
		: create_Element('ol', { className: 'recommendations' }, changes.map((change, index) => create_Element('li', { className: 'recommendation' }, [
			create_Element('div', { text: change.text }),
			change.citation
				? create_Element('div', { className: 'chip-list' }, [create_CitationChip(change.citation, index, 0)])
				: null,
		])));
	return [
		render_Markdown(message.text),
		create_DocCard('file-check', draft.title || ChatTexts.kindDraftRevision, ChatTexts.draftChanges, [
			changeList,
			create_Element('pre', { className: 'draft-text', text: draft.draftText ?? '' }),
		], [
			create_Element('button', {
				className: 'button button-outline',
				attrs: { type: 'button' },
				on: { click: () => copy_WithFeedback(draft.draftText ?? '') },
			}, [create_Icon('copy', 'icon-sm'), create_Element('span', { text: ChatTexts.draftCopy })]),
			create_DownloadLink(draft.draftLink, ChatTexts.downloadDocx),
			create_Element('button', {
				className: 'button button-outline',
				attrs: { type: 'button' },
				on: { click: () => handlers.onDraftContinue?.() },
			}, [create_Icon('edit', 'icon-sm'), create_Element('span', { text: ChatTexts.draftContinue })]),
		]),
	];
}

function render_AssistantBody(message, handlers) {
	switch (message.kind) {
		case MessageKinds.answer:
			return render_AnswerBody(message);
		case MessageKinds.clarify:
			return [create_Callout('help', message.text, ChatTexts.clarifyHint)];
		case MessageKinds.fallback:
			return [create_Callout('info', message.text, ChatTexts.fallbackHint), render_Contacts(message.contacts)];
		case MessageKinds.accessDenied:
			return [create_Callout('lock', message.text, ChatTexts.accessDeniedHint, 'callout-warning')];
		case MessageKinds.templateField:
			return render_TemplateFieldBody(message);
		case MessageKinds.templateReady:
			return render_TemplateReadyBody(message);
		case MessageKinds.templateChoice:
			return [create_Callout('file', message.text, ChatTexts.templateChoiceHint)];
		case MessageKinds.draftCheck:
			return render_DraftCheckBody(message);
		case MessageKinds.draftRevision:
			return render_DraftRevisionBody(message, handlers);
		default:
			// Новый вид ответа из ядра: показываем текст и источники, если они есть
			return render_AnswerBody(message);
	}
}

function build_AssistantCopyText(message) {
	if (message.kind === MessageKinds.templateReady && message.template?.draftText) {
		return message.template.draftText;
	}
	if (message.kind === MessageKinds.draftRevision && message.draft?.draftText) {
		return message.draft.draftText;
	}
	if (message.kind === MessageKinds.draftCheck) {
		return (message.recommendations ?? []).map((item, index) => `${index + 1}. ${item.text}`).join('\n');
	}
	return message.text ?? '';
}

function create_AssistantHeader(kind) {
	const label = kindLabels[kind];
	return create_Element('div', { className: 'message-header' }, [
		create_Logo('message-logo'),
		create_Element('span', { className: 'message-author', text: AppTexts.productName }),
		label ? create_Element('span', { className: 'kind-label', text: label }) : null,
	]);
}

// Где искал ответ: категории, выбранные в чате. При поиске по всем знаниям ядро поле не отдаёт
function render_SearchScope(message) {
	const process = message.process;
	if (!process?.processId) {
		return null;
	}
	const ids = Array.isArray(process.processIds) && process.processIds.length > 0 ? process.processIds : [process.processId];
	const titles = String(process.title ?? '').split(', ');
	return create_Element('div', { className: 'message-scope' }, [
		create_Element('span', { text: `${ProcessTexts.searchedIn}:` }),
		...ids.map((processId, index) => {
			const known = get_KnownProcess(processId);
			return create_ProcessBadge({ title: known?.title ?? titles[index] ?? processId, color: known?.color, description: known?.description });
		}),
	]);
}

// Вопросы ассистента с ответами пользователя одним блоком ленты, как отвеченные вопросы в Claude Code.
// rows: [{ label, value }]. Пустой value - вопрос пропущен. unanswered - на вопросы не ответили (отмена, смена режима)
export function render_QuestionAnswers({ title, rows, unanswered = false }) {
	return create_Element('div', { className: unanswered ? 'qa-block is-unanswered' : 'qa-block' }, [
		create_Element('div', { className: 'qa-block-header' }, [
			create_Icon(unanswered ? 'help' : 'check', 'icon-sm'),
			create_Element('span', { className: 'qa-block-title', text: title }),
			create_Element('span', { className: 'qa-block-status', text: unanswered ? ChatTexts.questionUnanswered : ChatTexts.questionAnswered }),
		]),
		rows.length > 0
			? create_Element('dl', { className: 'qa-block-rows' }, rows.flatMap((row) => [
				create_Element('dt', { className: 'qa-block-question', text: row.label }),
				create_Element('dd', {
					className: row.value ? 'qa-block-answer' : 'qa-block-answer is-empty',
					text: row.value || (unanswered ? ChatTexts.questionNoAnswer : ChatTexts.questionSkipped),
				}),
			]))
			: null,
	]);
}

// Подсказка ядра под ответом: например, что документы готовятся в режиме «План»
function render_Hint(hint) {
	if (typeof hint !== 'string' || hint.trim() === '') {
		return null;
	}
	return create_Element('div', { className: 'message-hint' }, [create_Icon('info', 'icon-sm'), create_Element('span', { text: hint })]);
}

// message.reveal - ответ только что пришёл: он появляется плавно, один раз. Перерисовка показывает его целиком
function render_AssistantMessage(message, handlers) {
	const header = create_AssistantHeader(message.kind);
	const body = create_Element('div', { className: 'message-body' }, [...render_AssistantBody(message, handlers), render_SearchScope(message), render_Hint(message.hint)]);
	const element = create_Element('div', { className: 'message message-assistant', dataset: { kind: message.kind ?? '' } }, [
		header,
		body,
		create_Element('div', { className: 'message-actions' }, [create_CopyButton(build_AssistantCopyText(message))]),
	]);
	if (message.reveal) {
		delete message.reveal;
		element.classList.add('is-new');
		reveal_Message(body, { onProgress: handlers.onRevealProgress });
	}
	return element;
}

function render_UserMessage(message, handlers) {
	const children = [];
	if (message.kind === MessageKinds.draftRequest) {
		children.push(create_Element('div', { className: 'message-attachment' }, [
			create_Icon('file-check'),
			create_Element('span', { text: `${ChatTexts.draftCheckRequest}: ${message.fileName}` }),
			Number.isFinite(message.fileSize)
				? create_Element('span', { className: 'attachment-chip-meta', text: format_FileSize(message.fileSize) })
				: null,
		]));
	}
	for (const attachment of Array.isArray(message.attachments) ? message.attachments : []) {
		children.push(create_Element('div', { className: 'message-attachment' }, [
			create_Icon('file'),
			create_Element('span', { text: `${ChatTexts.draftAssistRequest}: ${attachment.fileName}` }),
		]));
	}
	if (message.text) {
		children.push(create_Element('div', { className: 'message-bubble', text: message.text }));
	}
	if (message.failed) {
		children.push(create_Element('div', { className: 'message-error', attrs: { role: 'alert' } }, [
			create_Icon('alert', 'icon-sm'),
			create_Element('span', { text: message.errorText ?? ChatTexts.sendFailed }),
			handlers.onRetry
				? create_Element('button', {
					className: 'button button-outline',
					attrs: { type: 'button' },
					on: { click: () => handlers.onRetry(message) },
				}, [create_Icon('refresh', 'icon-sm'), create_Element('span', { text: AppTexts.retry })])
				: null,
		]));
	}
	const actions = [];
	if (message.text) {
		actions.push(create_CopyButton(message.text));
		if (handlers.onRepeat && !message.failed) {
			actions.push(create_IconButton('refresh', ChatTexts.repeatQuestion, () => handlers.onRepeat(message.text), 'icon-button icon-button-sm'));
		}
	}
	if (actions.length > 0) {
		children.push(create_Element('div', { className: 'message-actions' }, actions));
	}
	return create_Element('div', { className: 'message message-user' }, children);
}

// handlers: onRepeat(text) - задать вопрос заново, onRetry(message) - повторить неотправленное,
// onRevealProgress() - новый ответ проявляется, экран держит низ ленты
export function render_Message(message, handlers = {}) {
	if (message.role === 'user') {
		return render_UserMessage(message, handlers);
	}
	return render_AssistantMessage(message, handlers);
}

export function render_PendingMessage() {
	const header = create_AssistantHeader('');
	return create_Element('div', { className: 'message message-assistant', attrs: { 'aria-live': 'polite' } }, [
		header,
		create_Element('div', { className: 'message-body' }, [
			create_Element('div', { className: 'typing', attrs: { role: 'status', 'aria-label': ChatTexts.thinking } }, [
				create_Element('span'),
				create_Element('span'),
				create_Element('span'),
			]),
		]),
	]);
}
