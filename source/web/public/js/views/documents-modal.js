// Окно со списком документов, доступных текущему пользователю (GET /documents).
import { build_DocumentFileUrl, list_Documents } from '../api.js';
import { open_Modal } from '../components/modal.js';
import { create_Element, create_EmptyBlock, create_Icon, create_LoadingBlock } from '../lib/dom.js';
import { get_DocumentAudienceLabel } from '../roles.js';
import { AppTexts, DocumentsTexts, get_ErrorText } from '../texts.js';

function render_DocumentRow(document, config) {
	return create_Element('li', { className: 'document-row' }, [
		create_Icon('file'),
		create_Element('div', { className: 'document-row-text' }, [
			create_Element('span', { className: 'cell-title', text: document.title }),
			create_Element('span', {
				className: 'cell-subtitle',
				text: `${DocumentsTexts.edition} ${document.edition} · ${get_DocumentAudienceLabel(document.accessLevel, config)}`,
			}),
		]),
		create_Element('a', {
			className: 'icon-button',
			attrs: {
				href: build_DocumentFileUrl(document.documentId),
				target: '_blank',
				rel: 'noopener',
				title: DocumentsTexts.openFile,
				'aria-label': `${DocumentsTexts.openFile}: ${document.title}`,
			},
		}, [create_Icon('download')]),
	]);
}

export function open_DocumentsModal(config) {
	const content = create_Element('div', {}, [create_LoadingBlock(AppTexts.loading)]);
	open_Modal({
		title: DocumentsTexts.modalTitle,
		body: [create_Element('p', { className: 'section-lead', text: DocumentsTexts.lead }), content],
	});

	list_Documents()
		.then((payload) => {
			const documents = Array.isArray(payload?.documents) ? payload.documents : [];
			if (documents.length === 0) {
				content.replaceChildren(create_EmptyBlock('book', DocumentsTexts.empty));
				return;
			}
			content.replaceChildren(create_Element('ul', { className: 'document-list' }, documents.map((document) => render_DocumentRow(document, config))));
		})
		.catch((error) => {
			content.replaceChildren(create_EmptyBlock('alert', DocumentsTexts.loadFailed, [
				create_Element('div', { className: 'callout-hint', text: get_ErrorText(error) }),
			]));
		});
}
