// Модальное окно на <dialog>: фокус внутри окна, Esc закрывает, клик по фону тоже.
import { create_Element, create_IconButton } from '../lib/dom.js';
import { AppTexts } from '../texts.js';

export function open_Modal({ title, body = [], footer = [], wide = false, onClose } = {}) {
	const titleId = `modal-title-${Math.random().toString(36).slice(2)}`;
	const bodyElement = create_Element('div', { className: 'modal-body' }, body);
	const footerElement = create_Element('div', { className: 'modal-footer' }, footer);
	footerElement.hidden = footer.length === 0;

	const dialog = create_Element('dialog', {
		className: wide ? 'modal modal-wide' : 'modal',
		attrs: { 'aria-labelledby': titleId },
	});
	const closeButton = create_IconButton('close', AppTexts.close, () => dialog.close());
	dialog.append(create_Element('div', { className: 'modal-inner' }, [
		create_Element('div', { className: 'modal-header' }, [
			create_Element('h2', { className: 'modal-title', text: title, attrs: { id: titleId } }),
			closeButton,
		]),
		bodyElement,
		footerElement,
	]));

	dialog.addEventListener('close', () => {
		dialog.remove();
		onClose?.();
	});
	// Клик точно по фону: у самого dialog нет отступов, содержимое занимает его целиком
	dialog.addEventListener('click', (event) => {
		if (event.target === dialog) {
			dialog.close();
		}
	});

	document.body.append(dialog);
	dialog.showModal();

	return {
		dialog,
		bodyElement,
		footerElement,
		close: () => dialog.close(),
		set_Footer(children) {
			footerElement.replaceChildren(...children);
			footerElement.hidden = children.length === 0;
		},
	};
}
