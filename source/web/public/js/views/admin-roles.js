// Вкладка «Роли»: список, конструктор флагов и группа каталога.
import { create_Role, delete_Role, list_Permissions, list_Roles, update_Role } from '../api.js';
import { open_Confirm } from '../components/confirm.js';
import { open_Modal } from '../components/modal.js';
import { show_Toast } from '../components/toast.js';
import { create_Button, create_Element, create_EmptyBlock, create_IconButton, create_LoadingBlock } from '../lib/dom.js';
import { AppTexts, RoleAdminTexts, format_Text, get_ErrorText } from '../texts.js';

const permissionGroups = [
	{ title: RoleAdminTexts.groupChat, match: (permission) => permission.startsWith('chat.') },
	{ title: RoleAdminTexts.groupDocuments, match: (permission) => permission.startsWith('documents.') },
	{ title: RoleAdminTexts.groupDrafts, match: (permission) => permission.startsWith('drafts.') },
	{ title: RoleAdminTexts.groupTemplates, match: (permission) => permission.startsWith('templates.') },
	{ title: RoleAdminTexts.groupAccounts, match: (permission) => permission.startsWith('users.') || permission.startsWith('roles.') },
	{ title: RoleAdminTexts.groupAudit, match: (permission) => permission.startsWith('audit.') },
];

function group_Permissions(catalog) {
	const used = new Set();
	const groups = permissionGroups.map((group) => {
		const items = catalog.filter((item) => group.match(item.permission));
		for (const item of items) {
			used.add(item.permission);
		}
		return { title: group.title, items };
	}).filter((group) => group.items.length > 0);
	const rest = catalog.filter((item) => !used.has(item.permission));
	if (rest.length > 0) {
		groups.push({ title: RoleAdminTexts.groupOther, items: rest });
	}
	return groups;
}

function open_RoleForm({ role, catalog, onSaved }) {
	const isEdit = Boolean(role);
	const locked = role?.system === true;
	const errorBox = create_Element('div', { className: 'form-error', attrs: { role: 'alert' } });
	errorBox.hidden = true;
	const titleInput = create_Element('input', { className: 'input', attrs: { required: true } });
	const descriptionInput = create_Element('textarea', { className: 'input', attrs: { rows: 2 } });
	const groupInput = create_Element('input', { className: 'input' });
	titleInput.value = role?.title ?? '';
	descriptionInput.value = role?.description ?? '';
	groupInput.value = role?.ldapGroup ?? '';
	const selected = new Set(role?.permissions ?? []);
	const boxes = [];
	const permissionBlock = create_Element('div', { className: 'check-list' }, group_Permissions(catalog).map((group) => create_Element('div', {}, [
		create_Element('div', { className: 'field-label', text: group.title }),
		...group.items.map((item) => {
			const input = create_Element('input', { attrs: { type: 'checkbox', value: item.permission, disabled: locked } });
			input.checked = selected.has(item.permission);
			boxes.push(input);
			return create_Element('label', { className: 'check-row' }, [
				input,
				create_Element('span', {}, [
					create_Element('span', { text: item.title || item.permission }),
					item.description ? create_Element('span', { className: 'field-hint', text: item.description }) : null,
				]),
			]);
		}),
	])));
	if (locked) {
		titleInput.readOnly = true;
		descriptionInput.readOnly = true;
		groupInput.readOnly = true;
	}
	const form = create_Element('form', { className: 'form-grid', attrs: { id: 'role-form' } }, [
		create_Element('label', { className: 'field' }, [
			create_Element('span', { className: 'field-label', text: RoleAdminTexts.fieldTitle }),
			titleInput,
		]),
		create_Element('label', { className: 'field field-wide' }, [
			create_Element('span', { className: 'field-label', text: RoleAdminTexts.fieldDescription }),
			descriptionInput,
		]),
		create_Element('label', { className: 'field field-wide' }, [
			create_Element('span', { className: 'field-label', text: RoleAdminTexts.fieldLdapGroup }),
			groupInput,
			create_Element('span', { className: 'field-hint', text: RoleAdminTexts.fieldLdapGroupHint }),
		]),
		create_Element('div', { className: 'field field-wide' }, [
			create_Element('span', { className: 'field-label', text: RoleAdminTexts.fieldPermissions }),
			permissionBlock,
		]),
		locked ? create_Element('p', { className: 'field-hint field-wide', text: RoleAdminTexts.systemHint }) : null,
		create_Element('div', { className: 'field-wide' }, [errorBox]),
	]);
	const submitButton = create_Button(AppTexts.save, { variant: 'primary', type: 'submit', attrs: { form: 'role-form', disabled: locked } });
	const modal = open_Modal({
		title: isEdit ? RoleAdminTexts.editTitle : RoleAdminTexts.createTitle,
		body: [form],
		footer: [
			create_Button(AppTexts.cancel, { variant: 'ghost', onClick: () => modal.close() }),
			submitButton,
		],
	});
	form.addEventListener('submit', async (event) => {
		event.preventDefault();
		if (locked || titleInput.value.trim() === '') {
			titleInput.focus();
			return;
		}
		const fields = {
			title: titleInput.value.trim(),
			description: descriptionInput.value.trim(),
			ldapGroup: groupInput.value.trim(),
			permissions: boxes.filter((input) => input.checked).map((input) => input.value),
		};
		submitButton.disabled = true;
		try {
			const saved = isEdit ? await update_Role(role.roleId, fields) : await create_Role(fields);
			modal.close();
			show_Toast(RoleAdminTexts.saved, 'success');
			onSaved(saved);
		} catch (error) {
			errorBox.textContent = get_ErrorText(error);
			errorBox.hidden = false;
			submitButton.disabled = false;
		}
	});
}

export function create_RolesTab() {
	let roles = [];
	let catalog = [];
	const content = create_Element('div', {}, [create_LoadingBlock(AppTexts.loading)]);

	const reload = async () => {
		try {
			const [rolePayload, permissionPayload] = await Promise.all([list_Roles(), list_Permissions()]);
			roles = Array.isArray(rolePayload?.roles) ? rolePayload.roles : [];
			catalog = Array.isArray(permissionPayload?.permissions) ? permissionPayload.permissions : [];
			render_Table();
		} catch (error) {
			content.replaceChildren(create_EmptyBlock('alert', RoleAdminTexts.loadFailed, [
				create_Element('div', { className: 'callout-hint', text: get_ErrorText(error) }),
				create_Button(AppTexts.retry, { variant: 'outline', icon: 'refresh', onClick: reload }),
			]));
		}
	};

	const remove_Role = async (role) => {
		const confirmed = await open_Confirm({
			title: RoleAdminTexts.deleteTitle,
			text: format_Text(RoleAdminTexts.deleteText, { title: role.title }),
			confirmLabel: RoleAdminTexts.delete,
			danger: true,
		});
		if (!confirmed) {
			return;
		}
		try {
			await delete_Role(role.roleId);
			roles = roles.filter((item) => item.roleId !== role.roleId);
			render_Table();
			show_Toast(RoleAdminTexts.deleted, 'success');
		} catch (error) {
			show_Toast(get_ErrorText(error), 'error');
		}
	};

	function render_Table() {
		if (roles.length === 0) {
			content.replaceChildren(create_EmptyBlock('shield', RoleAdminTexts.empty));
			return;
		}
		content.replaceChildren(create_Element('div', { className: 'table-wrap' }, [
			create_Element('table', { className: 'table' }, [
				create_Element('thead', {}, [create_Element('tr', {}, [
					create_Element('th', { text: RoleAdminTexts.columnTitle }),
					create_Element('th', { text: RoleAdminTexts.columnUsers }),
					create_Element('th', { text: RoleAdminTexts.columnGroup }),
					create_Element('th', { className: 'table-cell-actions' }),
				])]),
				create_Element('tbody', {}, roles.map((role) => create_Element('tr', {}, [
					create_Element('td', {}, [
						create_Element('div', { className: 'cell-title', text: role.title }),
						role.description ? create_Element('div', { className: 'cell-subtitle', text: role.description }) : null,
					]),
					create_Element('td', { text: String(role.userCount ?? 0) }),
					create_Element('td', { className: 'table-cell-muted', text: role.ldapGroup || AppTexts.notSpecified }),
					create_Element('td', { className: 'table-cell-actions' }, [
						create_IconButton('edit', `${RoleAdminTexts.edit}: ${role.title}`, () => open_RoleForm({
							role,
							catalog,
							onSaved: (saved) => {
								Object.assign(role, saved);
								render_Table();
							},
						})),
						role.system ? null : create_IconButton('trash', `${RoleAdminTexts.delete}: ${role.title}`, () => remove_Role(role)),
					]),
				]))),
			]),
		]));
	}

	const element = create_Element('section', { className: 'page-section' }, [
		create_Element('div', { className: 'section-header' }, [
			create_Element('div', {}, [
				create_Element('h2', { className: 'section-title', text: RoleAdminTexts.lead }),
			]),
			create_Button(RoleAdminTexts.add, {
				variant: 'primary',
				icon: 'plus',
				onClick: () => open_RoleForm({
					role: null,
					catalog,
					onSaved: (saved) => {
						roles.push(saved);
						render_Table();
					},
				}),
			}),
		]),
		content,
	]);
	reload();
	return { element };
}
