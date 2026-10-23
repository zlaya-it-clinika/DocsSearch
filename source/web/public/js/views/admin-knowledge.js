// Вкладка «База знаний»: общие документы компании и личные документы всех сотрудников.
import { list_Documents } from '../api.js';
import { open_DocumentUploadForm } from '../components/document-upload.js';
import { create_DocumentTable } from '../components/document-table.js';
import { Scopes, check_IsPersonal, filter_ByQuery, read_DocumentList, read_DocumentPermissions } from '../documents.js';
import { create_Button, create_Element, create_EmptyBlock, create_Icon, create_LoadingBlock, create_SearchBox } from '../lib/dom.js';
import { AdminTexts, AppTexts, DocumentsTexts, get_ErrorText } from '../texts.js';

function create_Notice(text, hint) {
	return create_Element('div', { className: 'callout' }, [
		create_Icon('info'),
		create_Element('div', { className: 'callout-content' }, [
			create_Element('div', { text }),
			hint ? create_Element('div', { className: 'callout-hint', text: hint }) : null,
		]),
	]);
}

export function create_KnowledgeTab({ user, config }) {
	let documents = [];
	let permissions = read_DocumentPermissions(null, user, config);
	let personalAvailable = false;
	let query = '';

	const globalCount = create_Element('span', { className: 'section-count' });
	const personalCount = create_Element('span', { className: 'section-count' });
	const globalContent = create_Element('div', {}, [create_LoadingBlock(AppTexts.loading)]);
	const personalContent = create_Element('div', {}, [create_LoadingBlock(AppTexts.loading)]);
	const uploadButton = create_Button(DocumentsTexts.uploadGlobal, {
		variant: 'primary',
		icon: 'upload',
		onClick: () => open_DocumentUploadForm({ config, scope: Scopes.global, onUploaded: reload }),
	});

	const render_Sections = () => {
		const searchFields = ['title', 'ownerName', 'ownerId'];
		const allGlobal = documents.filter((document) => !check_IsPersonal(document));
		const allPersonal = documents.filter(check_IsPersonal);
		const global = filter_ByQuery(allGlobal, query, searchFields);
		const personal = filter_ByQuery(allPersonal, query, searchFields);
		globalCount.textContent = String(allGlobal.length);
		personalCount.textContent = personalAvailable ? String(allPersonal.length) : '';
		uploadButton.hidden = !permissions.canUploadGlobal;

		globalContent.replaceChildren(global.length === 0
			? create_EmptyBlock('book', query ? DocumentsTexts.nothingFound : AdminTexts.knowledgeEmpty)
			: create_DocumentTable(global, {
				config,
				columns: ['title', 'edition', 'access', 'uploaded'],
				onNewEdition: permissions.canUploadGlobal
					? (document) => open_DocumentUploadForm({ config, scope: Scopes.global, document, onUploaded: reload })
					: null,
				onChanged: reload,
			}));

		if (!personalAvailable) {
			personalContent.replaceChildren(create_Notice(DocumentsTexts.personalNotSupported, DocumentsTexts.personalNotSupportedHint));
			return;
		}
		personalContent.replaceChildren(personal.length === 0
			? create_EmptyBlock('users', query ? DocumentsTexts.nothingFound : AdminTexts.knowledgePersonalEmpty)
			: create_DocumentTable(personal, {
				config,
				columns: ['title', 'owner', 'edition', 'uploaded'],
				onChanged: reload,
			}));
	};

	async function reload() {
		try {
			// scope=all отдаёт и личные документы всех сотрудников. Старое ядро параметр не знает и вернёт общий список
			const payload = await list_Documents(Scopes.all);
			documents = read_DocumentList(payload);
			permissions = read_DocumentPermissions(payload, user, config);
			personalAvailable = permissions.supportsScopes;
			render_Sections();
		} catch (error) {
			globalContent.replaceChildren(create_EmptyBlock('alert', AdminTexts.knowledgeLoadFailed, [
				create_Element('div', { className: 'callout-hint', text: get_ErrorText(error) }),
				create_Button(AppTexts.retry, { variant: 'outline', icon: 'refresh', onClick: reload }),
			]));
			personalContent.replaceChildren();
		}
	}

	const element = create_Element('div', { className: 'page-sections' }, [
		create_SearchBox(AdminTexts.knowledgeSearch, (value) => {
			query = value;
			render_Sections();
		}),
		create_Element('section', { className: 'page-section' }, [
			create_Element('div', { className: 'section-header' }, [
				create_Element('h2', { className: 'section-title' }, [DocumentsTexts.globalTitle, globalCount]),
				create_Element('div', { className: 'section-tools' }, [uploadButton]),
			]),
			create_Element('p', { className: 'section-lead', text: AdminTexts.knowledgeLead }),
			globalContent,
		]),
		create_Element('section', { className: 'page-section' }, [
			create_Element('div', { className: 'section-header' }, [
				create_Element('h2', { className: 'section-title' }, [AdminTexts.knowledgePersonalTitle, personalCount]),
			]),
			create_Element('p', { className: 'section-lead', text: AdminTexts.knowledgePersonalLead }),
			personalContent,
		]),
	]);

	reload();
	return { element };
}
