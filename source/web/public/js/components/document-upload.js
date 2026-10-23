// Окно загрузки документа: новый общий или личный документ, либо новая редакция существующего.
import { upload_Document } from '../api.js';
import { Scopes } from '../documents.js';
import { create_Button, create_Element, create_Icon } from '../lib/dom.js';
import { build_AcceptAttribute, format_AllowedExtensions, validate_UploadFile } from '../lib/files.js';
import { format_FileSize } from '../lib/format.js';
import { Roles, get_AccessLevelForRole, get_DocumentAudience } from '../roles.js';
import { AppTexts, DocumentsTexts, FileTexts, RoleTexts, format_Text, get_ErrorText } from '../texts.js';
import { open_Modal } from './modal.js';
import { show_Toast } from './toast.js';

function create_Field(id, label, control, hint = '', wide = false) {
	return create_Element('div', { className: wide ? 'field field-wide' : 'field' }, [
		create_Element('label', { className: 'field-label', text: label, attrs: { for: id } }),
		control,
		hint ? create_Element('span', { className: 'field-hint', text: hint }) : null,
	]);
}

function pick_Title(scope, isNewEdition) {
	if (isNewEdition) {
		return DocumentsTexts.uploadEditionTitle;
	}
	return scope === Scopes.personal ? DocumentsTexts.uploadPersonalTitle : DocumentsTexts.uploadGlobalTitle;
}

// scope: global или personal. document задан - загружаем новую редакцию с тем же названием
export function open_DocumentUploadForm({ config, scope, document = null, onUploaded }) {
	let selectedFile = null;
	const isNewEdition = Boolean(document);
	const isGlobal = scope === Scopes.global;

	const errorBox = create_Element('div', { className: 'form-error', attrs: { role: 'alert' } });
	errorBox.hidden = true;

	const fileInput = create_Element('input', { attrs: { id: 'document-file', type: 'file', accept: build_AcceptAttribute(config), hidden: true } });
	const dropName = create_Element('span', { className: 'file-drop-name' });
	const dropZone = create_Element('div', {
		className: 'file-drop',
		attrs: { role: 'button', tabindex: 0, 'aria-label': DocumentsTexts.dropLead },
	}, [
		create_Icon('upload', 'icon-lg'),
		create_Element('span', { text: DocumentsTexts.dropLead }),
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
			create_Element('span', { className: 'field-label', text: DocumentsTexts.fieldFile }),
			dropZone,
			fileInput,
		]),
		create_Field('document-title', DocumentsTexts.fieldTitle, titleInput, isNewEdition ? '' : DocumentsTexts.fieldTitleHint, true),
		create_Field('document-edition', DocumentsTexts.fieldEdition, editionInput, DocumentsTexts.fieldEditionHint, !isGlobal),
		// Уровень доступа есть только у общего документа: личный видят владелец и администраторы
		isGlobal ? create_Field('document-audience', DocumentsTexts.fieldAccess, audienceSelect) : null,
		create_Element('p', {
			className: 'field-hint field-wide',
			text: isGlobal ? DocumentsTexts.globalUploadHint : DocumentsTexts.personalUploadHint,
		}),
		create_Element('div', { className: 'field-wide' }, [errorBox]),
	]);
	const submitButton = create_Button(DocumentsTexts.upload, { variant: 'primary', icon: 'upload', type: 'submit', attrs: { form: 'document-form' } });

	const modal = open_Modal({
		title: pick_Title(scope, isNewEdition),
		body: [form],
		footer: [create_Button(AppTexts.cancel, { variant: 'ghost', onClick: () => modal.close() }), submitButton],
	});

	form.addEventListener('submit', async (event) => {
		event.preventDefault();
		if (!selectedFile) {
			errorBox.textContent = DocumentsTexts.fileRequired;
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
				scope,
				accessLevel: isGlobal ? get_AccessLevelForRole(audienceSelect.value, config) : undefined,
			});
			modal.close();
			show_Toast(DocumentsTexts.uploaded, 'success');
			onUploaded();
		} catch (error) {
			errorBox.textContent = get_ErrorText(error);
			errorBox.hidden = false;
		} finally {
			submitButton.disabled = false;
		}
	});
}
