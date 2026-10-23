// Поле ввода чата: растёт по мере набора, Enter отправляет, Shift+Enter переносит строку.
// Кнопка «+» прикрепляет черновик на проверку.
import { create_Element, create_Icon, create_IconButton } from '../lib/dom.js';
import { format_FileSize } from '../lib/format.js';
import { ChatTexts, format_Text } from '../texts.js';
import { show_Toast } from './toast.js';

const maxInputHeight = 220;
// Подсказка для окна выбора файла. Окончательно формат проверяет ядро
const draftFileAccept = '.txt,.md,.rtf,.doc,.docx,.odt,.pdf';

// onSubmit({ text, file }) должен вернуть Promise, поле блокируется до его завершения
export function create_Composer({ onSubmit, allowAttach = true, maxUploadBytes = 0 }) {
	let attachedFile = null;
	let busy = false;
	let basePlaceholder = ChatTexts.placeholder;

	const textarea = create_Element('textarea', {
		className: 'composer-input',
		attrs: { rows: 1, placeholder: basePlaceholder, 'aria-label': basePlaceholder },
	});
	const attachmentsRow = create_Element('div', { className: 'composer-attachments' });
	attachmentsRow.hidden = true;

	const fileInput = create_Element('input', {
		attrs: { type: 'file', accept: draftFileAccept, hidden: true, tabindex: -1 },
	});
	const attachButton = create_IconButton('plus', ChatTexts.attach, () => fileInput.click());
	attachButton.hidden = !allowAttach;

	const sendButton = create_Element('button', {
		className: 'composer-send',
		attrs: { type: 'submit', 'aria-label': ChatTexts.send, title: ChatTexts.send, disabled: true },
	}, [create_Icon('arrow-up')]);

	const form = create_Element('form', { className: 'composer' }, [
		attachmentsRow,
		textarea,
		create_Element('div', { className: 'composer-toolbar' }, [
			create_Element('div', {}, [attachButton, fileInput]),
			sendButton,
		]),
	]);

	const resize_Input = () => {
		textarea.style.height = 'auto';
		textarea.style.height = `${Math.min(textarea.scrollHeight, maxInputHeight)}px`;
		textarea.style.overflowY = textarea.scrollHeight > maxInputHeight ? 'auto' : 'hidden';
	};

	const update_SendState = () => {
		sendButton.disabled = busy || (textarea.value.trim() === '' && !attachedFile);
	};

	const update_Placeholder = () => {
		const placeholder = attachedFile ? ChatTexts.placeholderWithFile : basePlaceholder;
		textarea.placeholder = placeholder;
		textarea.setAttribute('aria-label', placeholder);
	};

	const render_Attachment = () => {
		attachmentsRow.replaceChildren();
		attachmentsRow.hidden = !attachedFile;
		if (attachedFile) {
			attachmentsRow.append(create_Element('div', { className: 'attachment-chip' }, [
				create_Icon('file-check'),
				create_Element('div', { className: 'attachment-chip-text' }, [
					create_Element('span', { className: 'attachment-chip-name', text: attachedFile.name }),
					create_Element('span', { className: 'attachment-chip-meta', text: `${ChatTexts.draftCheckRequest} · ${format_FileSize(attachedFile.size)}` }),
				]),
				create_IconButton('close', ChatTexts.removeAttachment, () => {
					attachedFile = null;
					render_Attachment();
					textarea.focus();
				}, 'icon-button icon-button-sm'),
			]));
		}
		update_Placeholder();
		update_SendState();
	};

	const submit_Form = async () => {
		if (busy) {
			return;
		}
		const text = textarea.value.trim();
		if (text === '' && !attachedFile) {
			return;
		}
		const file = attachedFile;
		textarea.value = '';
		attachedFile = null;
		render_Attachment();
		resize_Input();
		await onSubmit({ text, file });
		textarea.focus();
	};

	textarea.addEventListener('input', () => {
		resize_Input();
		update_SendState();
	});
	textarea.addEventListener('keydown', (event) => {
		if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) {
			event.preventDefault();
			submit_Form();
		}
	});
	form.addEventListener('submit', (event) => {
		event.preventDefault();
		submit_Form();
	});
	fileInput.addEventListener('change', () => {
		const [file] = fileInput.files ?? [];
		fileInput.value = '';
		if (file) {
			api.attach_File(file);
		}
	});

	const api = {
		element: form,

		focus() {
			textarea.focus();
		},

		set_Busy(value) {
			busy = value;
			attachButton.disabled = value;
			update_SendState();
		},

		// Подсказка меняется, когда ассистент спрашивает поле бланка
		set_FieldPrompt(fieldLabel) {
			basePlaceholder = fieldLabel ? format_Text(ChatTexts.placeholderField, { field: fieldLabel }) : ChatTexts.placeholder;
			update_Placeholder();
		},

		set_Text(text) {
			textarea.value = text;
			resize_Input();
			update_SendState();
			textarea.focus();
		},

		attach_File(file) {
			if (!allowAttach) {
				return;
			}
			if (maxUploadBytes > 0 && file.size > maxUploadBytes) {
				show_Toast(format_Text(ChatTexts.fileTooLarge, { size: format_FileSize(maxUploadBytes) }), 'error');
				return;
			}
			attachedFile = file;
			render_Attachment();
			textarea.focus();
		},
	};
	return api;
}
