// Вкладка «База знаний»: общие документы компании и личные документы всех сотрудников.
import { create_Process, list_Documents, list_Processes } from '../api.js';
import { open_DocumentUploadForm } from '../components/document-upload.js';
import { create_DocumentTable, create_ProcessFilter } from '../components/document-table.js';
import { Scopes, check_IsPersonal, filter_ByQuery, read_DocumentList, read_DocumentPermissions } from '../documents.js';
import { show_Toast } from '../components/toast.js';
import { create_Button, create_Element, create_EmptyBlock, create_Icon, create_LoadingBlock, create_SearchBox } from '../lib/dom.js';
import { Permissions, has_Permission } from '../roles.js';
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
	let processes = [];
	let processTitles = {};
	let processFilter = '';
	let permissions = read_DocumentPermissions(null, user, config);
	let personalAvailable = false;
	let query = '';
	const canManageProcesses = has_Permission(user, config, Permissions.usersManage);

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
		const scoped = processFilter ? documents.filter((document) => document.processId === processFilter) : documents;
		const allGlobal = scoped.filter((document) => !check_IsPersonal(document));
		const allPersonal = scoped.filter(check_IsPersonal);
		const global = filter_ByQuery(allGlobal, query, searchFields);
		const personal = filter_ByQuery(allPersonal, query, searchFields);
		globalCount.textContent = String(allGlobal.length);
		personalCount.textContent = personalAvailable ? String(allPersonal.length) : '';
		uploadButton.hidden = !permissions.canUploadGlobal;

		globalContent.replaceChildren(global.length === 0
			? create_EmptyBlock('book', query ? DocumentsTexts.nothingFound : AdminTexts.knowledgeEmpty)
			: create_DocumentTable(global, {
				config,
				processTitles,
				columns: processes.length > 0 ? ['title', 'process', 'edition', 'access', 'uploaded'] : ['title', 'edition', 'access', 'uploaded'],
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
				processTitles,
				columns: processes.length > 0 ? ['title', 'process', 'owner', 'edition', 'uploaded'] : ['title', 'owner', 'edition', 'uploaded'],
				onChanged: reload,
			}));
	};

	const processList = create_Element('div', {}, [create_LoadingBlock(AppTexts.loading)]);
	const filterHost = create_Element('div', { className: 'section-tools' });
	const processTitleInput = create_Element('input', { className: 'input', attrs: { 'aria-label': AdminTexts.processesFieldTitle } });

	const render_Processes = () => {
		processList.replaceChildren(processes.length === 0
			? create_Element('div', { className: 'callout-hint', text: AdminTexts.processesEmpty })
			: create_Element('ul', { className: 'check-list' }, processes.map((process) => create_Element('li', { text: process.title }))));
	};

	async function reload_Processes() {
		try {
			processes = (await list_Processes())?.processes ?? [];
			processTitles = Object.fromEntries(processes.map((process) => [process.processId, process.title]));
			const filter = create_ProcessFilter(processes, (value) => {
				processFilter = value;
				render_Sections();
			});
			if (filter && filterHost.childElementCount === 0) {
				filterHost.replaceChildren(filter);
			}
			render_Processes();
		} catch (error) {
			processList.replaceChildren(create_Element('div', { className: 'callout-hint', text: get_ErrorText(error) }));
		}
	}

	async function reload() {
		try {
			await reload_Processes();
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
		create_Element('div', { className: 'section-header' }, [
			create_SearchBox(AdminTexts.knowledgeSearch, (value) => {
				query = value;
				render_Sections();
			}),
			filterHost,
		]),
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
		canManageProcesses ? create_Element('section', { className: 'page-section' }, [
			create_Element('h2', { className: 'section-title', text: AdminTexts.processesTitle }),
			create_Element('p', { className: 'section-lead', text: AdminTexts.processesLead }),
			processList,
			create_Element('form', { className: 'form-grid', on: { submit: async (event) => {
				event.preventDefault();
				const title = processTitleInput.value.trim();
				if (title === '') {
					processTitleInput.focus();
					return;
				}
				try {
					await create_Process({ title, selectPrompt: '', answerPrompt: '', clarifyPrompt: '' });
					processTitleInput.value = '';
					show_Toast(AdminTexts.processesCreated, 'success');
					await reload_Processes();
					render_Sections();
				} catch (error) {
					processList.append(create_Element('div', { className: 'form-error', text: get_ErrorText(error) }));
				}
			} } }, [
				create_Element('label', { className: 'field' }, [
					create_Element('span', { className: 'field-label', text: AdminTexts.processesFieldTitle }),
					processTitleInput,
				]),
				create_Button(AdminTexts.processesAdd, { variant: 'primary', icon: 'plus', type: 'submit' }),
			]),
		]) : null,
	]);

	reload();
	return { element };
}
