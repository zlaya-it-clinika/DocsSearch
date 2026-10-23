// Вкладка «База знаний»: список документов, загрузка нового документа и новой редакции.
import { build_DocumentFileUrl, list_Documents, upload_Document } from '../api.js';
import { open_Modal } from '../components/modal.js';
import { show_Toast } from '../components/toast.js';
import { create_Button, create_Element, create_EmptyBlock, create_Icon, create_IconButton, create_LoadingBlock, create_SearchBox } from '../lib/dom.js';
import { build_AcceptAttribute, format_AllowedExtensions, validate_UploadFile } from '../lib/files.js';
import { format_FileSize } from '../lib/format.js';
import { Roles, get_AccessLevelForRole, get_DocumentAudience, get_DocumentAudienceLabel } from '../roles.js';
import { AdminTexts, AppTexts, DocumentsTexts, FileTexts, RoleTexts, format_Text, get_ErrorText } from '../texts.js';

function create_Field(id, label, control, hint = '', wide = false) {
	return create_Element('div', { className: wide ? 'field field-wide' : 'field' }, [
		create_Element('label', { className: 'field-label', text: label, attrs: { for: id } }),
		control,
		hint ? create_Element('span', { className: 'field-hint', text: hint }) : null,
	]);
}

// document задан - загружаем новую редакцию с тем же названием
function open_UploadForm({ config, document, onUploaded }) {
	let selectedFile = null;
	const isNewEdition = Boolean(document);

	const errorBox = create_Element('div', { className: 'form-error', attrs: { role: 'alert' } });
	errorBox.hidden = true;

	const fileInput = create_Element('input', { attrs: { id: 'document-file', type: 'file', accept: build_AcceptAttribute(config), hidden: true } });
	const dropName = create_Element('span', { className: 'file-drop-name' });
	const dropZone = create_Element('div', {
		className: 'file-drop',
		attrs: { role: 'button', tabindex: 0, 'aria-label': AdminTexts.knowledgeDropLead },
	}, [
		create_Icon('upload', 'icon-lg'),
		create_Element('span', { text: AdminTexts.knowledgeDropLead }),
		create_Element('span', { className: 'field-hint', text: format_Text(FileTexts.allowedHint, { list: format_AllowedExtensions(config) }) }),
		dropName,
	]);

	const titleInput = create_Element('input', { className: 'input', attrs: { id: 'document-title', autocomplete: 'off' } });
	const editionInput = create_Element('input', { className: 'input', attrs: { id: 'document-edition', autocomplete: 'off' } });
	const audienceSelect = create_Element('select', { className: 'select', attrs: { id: 'document-audience' } }, [
		create_Element('option', { text: RoleTexts.documentForAll, attrs: { value: Roles.user } }),
		create_Element('option', { text: RoleTexts.documentForAdmins, attrs: { value: Roles.admin } }),
	]);
	if (isNewEdition) {
		titleInput.value = document.title;
		titleInput.readOnly = true;
		audienceSelect.value = get_DocumentAudience(document.accessLevel, config);
	}

	const select_File = (file) => {
		if (!file) {
			return;
		}
		const problem = validate_UploadFile(file, config);
		if (problem) {
			errorBox.textContent = problem;
			errorBox.hidden = false;
			return;
		}
		errorBox.hidden = true;
		selectedFile = file;
		dropName.textContent = `${file.name} · ${format_FileSize(file.size)}`;
		if (!isNewEdition && titleInput.value.trim() === '') {
			titleInput.value = file.name.replace(/\.[^.]+$/, '');
		}
	};

	dropZone.addEventListener('click', () => fileInput.click());
	dropZone.addEventListener('keydown', (event) => {
		if (event.key === 'Enter' || event.key === ' ') {
			event.preventDefault();
			fileInput.click();
		}
	});
	dropZone.addEventListener('dragover', (event) => {
		event.preventDefault();
		dropZone.classList.add('is-dragging');
	});
	dropZone.addEventListener('dragleave', () => dropZone.classList.remove('is-dragging'));
	dropZone.addEventListener('drop', (event) => {
		event.preventDefault();
		dropZone.classList.remove('is-dragging');
		select_File(event.dataTransfer.files[0]);
	});
	fileInput.addEventListener('change', () => select_File(fileInput.files[0]));

	const form = create_Element('form', { className: 'form-grid', attrs: { id: 'document-form', novalidate: true } }, [
		create_Element('div', { className: 'field field-wide' }, [
			create_Element('span', { className: 'field-label', text: AdminTexts.knowledgeFieldFile }),
			dropZone,
			fileInput,
		]),
		create_Field('document-title', AdminTexts.knowledgeFieldTitle, titleInput, isNewEdition ? '' : AdminTexts.knowledgeFieldTitleHint, true),
		create_Field('document-edition', AdminTexts.knowledgeFieldEdition, editionInput, AdminTexts.knowledgeFieldEditionHint),
		create_Field('document-audience', AdminTexts.knowledgeFieldAccess, audienceSelect),
		create_Element('div', { className: 'field-wide' }, [errorBox]),
	]);
	const submitButton = create_Button(AdminTexts.knowledgeUpload, { variant: 'primary', icon: 'upload', type: 'submit', attrs: { form: 'document-form' } });

	const modal = open_Modal({
		title: isNewEdition ? AdminTexts.knowledgeEditionTitle : AdminTexts.knowledgeUploadTitle,
		body: [form],
		footer: [create_Button(AppTexts.cancel, { variant: 'ghost', onClick: () => modal.close() }), submitButton],
	});

	form.addEventListener('submit', async (event) => {
		event.preventDefault();
		if (!selectedFile) {
			errorBox.textContent = AdminTexts.knowledgeFileRequired;
			errorBox.hidden = false;
			dropZone.focus();
			return;
		}
		errorBox.hidden = true;
		submitButton.disabled = true;
		try {
			await upload_Document(selectedFile, {
				title: titleInput.value,
				edition: editionInput.value,
				accessLevel: get_AccessLevelForRole(audienceSelect.value, config),
			});
			modal.close();
			show_Toast(AdminTexts.knowledgeUploaded, 'success');
			onUploaded();
		} catch (error) {
			errorBox.textContent = get_ErrorText(error);
			errorBox.hidden = false;
		} finally {
			submitButton.disabled = false;
		}
	});
}

export function create_KnowledgeTab({ config }) {
	let documents = [];
	let query = '';

	const countLabel = create_Element('span', { className: 'section-count' });
	const content = create_Element('div', {}, [create_LoadingBlock(AppTexts.loading)]);

	const reload = async () => {
		try {
			const payload = await list_Documents();
			documents = Array.isArray(payload?.documents) ? payload.documents : [];
			render_Table();
		} catch (error) {
			content.replaceChildren(create_EmptyBlock('alert', AdminTexts.knowledgeLoadFailed, [
				create_Element('div', { className: 'callout-hint', text: get_ErrorText(error) }),
				create_Button(AppTexts.retry, { variant: 'outline', icon: 'refresh', onClick: reload }),
			]));
		}
	};

	function render_Table() {
		countLabel.textContent = String(documents.length);
		const filtered = query ? documents.filter((document) => document.title.toLowerCase().includes(query)) : documents;
		if (filtered.length === 0) {
			content.replaceChildren(create_EmptyBlock('book', AdminTexts.knowledgeEmpty));
			return;
		}
		content.replaceChildren(create_Element('div', { className: 'table-wrap' }, [
			create_Element('table', { className: 'table' }, [
				create_Element('thead', {}, [create_Element('tr', {}, [
					create_Element('th', { text: AdminTexts.knowledgeColumnTitle }),
					create_Element('th', { text: AdminTexts.knowledgeColumnEdition }),
					create_Element('th', { text: AdminTexts.knowledgeColumnAccess }),
					create_Element('th', { className: 'table-cell-actions' }),
				])]),
				create_Element('tbody', {}, filtered.map((document) => create_Element('tr', {}, [
					create_Element('td', { className: 'table-cell-wrap' }, [
						create_Element('div', { className: 'cell-title', text: document.title }),
						create_Element('div', { className: 'cell-subtitle', text: document.documentId }),
					]),
					create_Element('td', { className: 'table-cell-muted', text: document.edition }),
					create_Element('td', {}, [create_Element('span', {
						className: get_DocumentAudience(document.accessLevel, config) === Roles.admin ? 'badge badge-strong' : 'badge',
						text: get_DocumentAudienceLabel(document.accessLevel, config),
					})]),
					create_Element('td', { className: 'table-cell-actions' }, [
						create_IconButton('upload', `${AdminTexts.knowledgeNewEdition}: ${document.title}`, () => open_UploadForm({ config, document, onUploaded: reload })),
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
					]),
				]))),
			]),
		]));
	}

	const element = create_Element('section', { className: 'page-section' }, [
		create_Element('div', { className: 'section-header' }, [
			create_Element('h2', { className: 'section-title' }, [AdminTexts.tabKnowledge, countLabel]),
			create_Element('div', { className: 'section-tools' }, [
				create_SearchBox(AdminTexts.knowledgeSearch, (value) => {
					query = value;
					render_Table();
				}),
				create_Button(AdminTexts.knowledgeUpload, {
					variant: 'primary',
					icon: 'upload',
					onClick: () => open_UploadForm({ config, document: null, onUploaded: reload }),
				}),
			]),
		]),
		create_Element('p', { className: 'section-lead', text: AdminTexts.knowledgeLead }),
		content,
	]);

	reload();
	return { element };
}
