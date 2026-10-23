// Таблица документов для страницы «Документы» и админ-панели.
// columns: title, owner, edition, access, uploaded. Действия показываются только при наличии обработчика и прав.
import { build_DocumentFileUrl, delete_Document } from '../api.js';
import { check_CanDelete, check_IsPersonal } from '../documents.js';
import { create_Element, create_Icon, create_IconButton } from '../lib/dom.js';
import { format_DateTime } from '../lib/format.js';
import { Roles, get_DocumentAudience, get_DocumentAudienceLabel } from '../roles.js';
import { AppTexts, DocumentsTexts, format_Text, get_ErrorText } from '../texts.js';
import { open_Confirm } from './confirm.js';
import { show_Toast } from './toast.js';

const columnTitles = {
	title: DocumentsTexts.columnTitle,
	owner: DocumentsTexts.columnOwner,
	edition: DocumentsTexts.columnEdition,
	access: DocumentsTexts.columnAccess,
	uploaded: DocumentsTexts.columnUploaded,
};

export function create_ScopeBadge(document, config) {
	if (check_IsPersonal(document)) {
		return create_Element('span', { className: 'badge badge-outline', text: DocumentsTexts.scopePersonal });
	}
	return create_Element('span', {
		className: get_DocumentAudience(document.accessLevel, config) === Roles.admin ? 'badge badge-strong' : 'badge',
		text: get_DocumentAudienceLabel(document.accessLevel, config),
	});
}

function render_Cell(column, document, config) {
	switch (column) {
		case 'title':
			return create_Element('td', { className: 'table-cell-wrap' }, [
				create_Element('div', { className: 'cell-title', text: document.title }),
				create_Element('div', { className: 'cell-subtitle', text: document.documentId }),
			]);
		case 'owner':
			return create_Element('td', {}, [
				create_Element('div', { className: 'cell-title', text: document.ownerName || AppTexts.notSpecified }),
				create_Element('div', { className: 'cell-subtitle', text: document.ownerId || '' }),
			]);
		case 'edition':
			return create_Element('td', { className: 'table-cell-muted', text: document.edition || AppTexts.notSpecified });
		case 'access':
			return create_Element('td', {}, [create_ScopeBadge(document, config)]);
		case 'uploaded':
			return create_Element('td', { className: 'time-cell', text: format_DateTime(document.uploadedAt) || AppTexts.notSpecified });
		default:
			return create_Element('td');
	}
}

async function confirm_AndDelete(document, onChanged) {
	const confirmed = await open_Confirm({
		title: DocumentsTexts.deleteTitle,
		text: format_Text(DocumentsTexts.deleteText, { title: document.title }),
		confirmLabel: DocumentsTexts.deleteConfirm,
		danger: true,
	});
	if (!confirmed) {
		return;
	}
	try {
		await delete_Document(document.documentId);
		show_Toast(DocumentsTexts.deleted, 'success');
		onChanged();
	} catch (error) {
		show_Toast(get_ErrorText(error), 'error');
	}
}

// onNewEdition(document) - загрузить новую редакцию, onChanged() - перечитать список после удаления
export function create_DocumentTable(documents, { config, columns, onNewEdition, onChanged }) {
	return create_Element('div', { className: 'table-wrap' }, [
		create_Element('table', { className: 'table' }, [
			create_Element('thead', {}, [create_Element('tr', {}, [
				...columns.map((column) => create_Element('th', { text: columnTitles[column] })),
				create_Element('th', { className: 'table-cell-actions' }),
			])]),
			create_Element('tbody', {}, documents.map((document) => create_Element('tr', {}, [
				...columns.map((column) => render_Cell(column, document, config)),
				create_Element('td', { className: 'table-cell-actions' }, [
					onNewEdition
						? create_IconButton('upload', `${DocumentsTexts.newEdition}: ${document.title}`, () => onNewEdition(document))
						: null,
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
					onChanged && check_CanDelete(document)
						? create_IconButton('trash', `${DocumentsTexts.delete}: ${document.title}`, () => confirm_AndDelete(document, onChanged))
						: null,
				]),
			]))),
		]),
	]);
}
