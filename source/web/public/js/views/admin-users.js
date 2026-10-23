// Вкладка «Пользователи»: список, поиск, добавление, смена имени и роли.
import { create_User, list_Users, update_User } from '../api.js';
import { open_Modal } from '../components/modal.js';
import { show_Toast } from '../components/toast.js';
import { create_Button, create_Element, create_EmptyBlock, create_IconButton, create_LoadingBlock, create_SearchBox } from '../lib/dom.js';
import { get_Initials } from '../lib/format.js';
import { Roles, get_AccessLevelForRole, get_RoleLabel, get_UserRole } from '../roles.js';
import { AdminTexts, AppTexts, get_ErrorText } from '../texts.js';

function create_RoleSelect(selectedRole, id) {
	return create_Element('select', { className: 'select', attrs: { id } }, [Roles.user, Roles.admin].map((role) => create_Element('option', {
		text: get_RoleLabel(role),
		attrs: { value: role, selected: role === selectedRole },
	})));
}

function create_Field(id, label, control, hint = '') {
	return create_Element('div', { className: 'field' }, [
		create_Element('label', { className: 'field-label', text: label, attrs: { for: id } }),
		control,
		hint ? create_Element('span', { className: 'field-hint', text: hint }) : null,
	]);
}

// Общая форма для создания и правки. При правке логин и пароль не меняются: этого нет в контракте
function open_UserForm({ config, account, onSaved }) {
	const isEdit = Boolean(account);
	const errorBox = create_Element('div', { className: 'form-error', attrs: { role: 'alert' } });
	errorBox.hidden = true;

	const loginInput = create_Element('input', { className: 'input', attrs: { id: 'user-login', autocomplete: 'off', required: true } });
	const passwordInput = create_Element('input', { className: 'input', attrs: { id: 'user-password', type: 'password', autocomplete: 'new-password', required: true } });
	const nameInput = create_Element('input', { className: 'input', attrs: { id: 'user-name', autocomplete: 'off', required: true } });
	const roleSelect = create_RoleSelect(isEdit ? get_UserRole(account, config) : Roles.user, 'user-role');
	if (isEdit) {
		nameInput.value = account.displayName;
	}

	const fields = isEdit
		? [create_Field('user-name', AdminTexts.usersFieldName, nameInput), create_Field('user-role', AdminTexts.usersFieldRole, roleSelect)]
		: [
			create_Field('user-login', AdminTexts.usersFieldLogin, loginInput),
			create_Field('user-password', AdminTexts.usersFieldPassword, passwordInput),
			create_Field('user-name', AdminTexts.usersFieldName, nameInput),
			create_Field('user-role', AdminTexts.usersFieldRole, roleSelect),
		];

	const form = create_Element('form', { className: 'form-grid', attrs: { novalidate: true, id: 'user-form' } }, [
		...fields,
		create_Element('div', { className: 'field-wide' }, [errorBox]),
	]);
	const submitButton = create_Button(AppTexts.save, { variant: 'primary', type: 'submit', attrs: { form: 'user-form' } });

	const modal = open_Modal({
		title: isEdit ? AdminTexts.usersEditTitle : AdminTexts.usersCreateTitle,
		body: [form],
		footer: [
			create_Button(AppTexts.cancel, { variant: 'ghost', onClick: () => modal.close() }),
			submitButton,
		],
	});

	form.addEventListener('submit', async (event) => {
		event.preventDefault();
		const displayName = nameInput.value.trim();
		const accessLevel = get_AccessLevelForRole(roleSelect.value, config);
		const required = isEdit ? [nameInput] : [loginInput, passwordInput, nameInput];
		const emptyField = required.find((input) => input.value.trim() === '');
		if (emptyField) {
			emptyField.focus();
			return;
		}
		errorBox.hidden = true;
		submitButton.disabled = true;
		try {
			const saved = isEdit
				? await update_User(account.userId, { displayName, accessLevel })
				: await create_User({ login: loginInput.value.trim(), password: passwordInput.value, displayName, accessLevel });
			modal.close();
			show_Toast(isEdit ? AdminTexts.usersUpdated : AdminTexts.usersCreated, 'success');
			onSaved(saved);
		} catch (error) {
			errorBox.textContent = get_ErrorText(error);
			errorBox.hidden = false;
		} finally {
			submitButton.disabled = false;
		}
	});
	(isEdit ? nameInput : loginInput).focus();
}

export function create_UsersTab({ user, config }) {
	let accounts = [];
	let query = '';

	const countLabel = create_Element('span', { className: 'section-count' });
	const content = create_Element('div', {}, [create_LoadingBlock(AppTexts.loading)]);

	const reload = async () => {
		try {
			const payload = await list_Users();
			accounts = Array.isArray(payload?.users) ? payload.users : [];
			render_Table();
		} catch (error) {
			content.replaceChildren(create_EmptyBlock('alert', AdminTexts.usersLoadFailed, [
				create_Element('div', { className: 'callout-hint', text: get_ErrorText(error) }),
				create_Button(AppTexts.retry, { variant: 'outline', icon: 'refresh', onClick: reload }),
			]));
		}
	};

	const toggle_Role = async (account) => {
		const nextRole = get_UserRole(account, config) === Roles.admin ? Roles.user : Roles.admin;
		try {
			const saved = await update_User(account.userId, { accessLevel: get_AccessLevelForRole(nextRole, config) });
			Object.assign(account, saved);
			render_Table();
			show_Toast(AdminTexts.usersRoleChanged, 'success');
		} catch (error) {
			show_Toast(get_ErrorText(error), 'error');
		}
	};

	const render_RoleBadge = (account) => {
		const role = get_UserRole(account, config);
		const isSelf = account.userId === user.userId;
		// Себе роль не меняем, чтобы администратор случайно не потерял доступ к панели
		return create_Element('button', {
			className: role === Roles.admin ? 'badge badge-strong' : 'badge',
			text: get_RoleLabel(role),
			attrs: {
				type: 'button',
				title: isSelf ? AdminTexts.usersSelfRoleHint : AdminTexts.usersRoleToggleHint,
				disabled: isSelf,
			},
			on: { click: () => toggle_Role(account) },
		});
	};

	function render_Table() {
		countLabel.textContent = String(accounts.length);
		const filtered = query
			? accounts.filter((account) => `${account.displayName} ${account.login}`.toLowerCase().includes(query))
			: accounts;
		if (filtered.length === 0) {
			content.replaceChildren(create_EmptyBlock('users', AdminTexts.usersEmpty));
			return;
		}
		content.replaceChildren(create_Element('div', { className: 'table-wrap' }, [
			create_Element('table', { className: 'table' }, [
				create_Element('thead', {}, [create_Element('tr', {}, [
					create_Element('th', { text: AdminTexts.usersColumnRole }),
					create_Element('th', { text: AdminTexts.usersColumnName }),
					create_Element('th', { text: AdminTexts.usersColumnLogin }),
					create_Element('th', { text: AdminTexts.usersColumnId }),
					create_Element('th', { className: 'table-cell-actions' }),
				])]),
				create_Element('tbody', {}, filtered.map((account) => create_Element('tr', {}, [
					create_Element('td', {}, [render_RoleBadge(account)]),
					create_Element('td', {}, [create_Element('div', { className: 'message-header' }, [
						create_Element('span', { className: 'avatar', text: get_Initials(account.displayName), attrs: { 'aria-hidden': 'true' } }),
						create_Element('span', { className: 'cell-title', text: account.displayName }),
						account.userId === user.userId ? create_Element('span', { className: 'badge badge-outline', text: AdminTexts.usersYou }) : null,
					])]),
					create_Element('td', { className: 'table-cell-muted', text: account.login }),
					create_Element('td', { className: 'table-cell-muted', text: account.userId }),
					create_Element('td', { className: 'table-cell-actions' }, [
						create_IconButton('edit', `${AdminTexts.usersEdit}: ${account.displayName}`, () => open_UserForm({
							config,
							account,
							onSaved: (saved) => {
								Object.assign(account, saved);
								render_Table();
							},
						})),
					]),
				]))),
			]),
		]));
	}

	const element = create_Element('section', { className: 'page-section' }, [
		create_Element('div', { className: 'section-header' }, [
			create_Element('h2', { className: 'section-title' }, [AdminTexts.tabUsers, countLabel]),
			create_Element('div', { className: 'section-tools' }, [
				create_SearchBox(AdminTexts.usersSearch, (value) => {
					query = value;
					render_Table();
				}),
				create_Button(AdminTexts.usersAdd, {
					variant: 'primary',
					icon: 'plus',
					onClick: () => open_UserForm({
						config,
						account: null,
						onSaved: (saved) => {
							accounts.push(saved);
							render_Table();
						},
					}),
				}),
			]),
		]),
		content,
	]);

	reload();
	return { element };
}

