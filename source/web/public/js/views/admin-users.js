// Вкладка «Пользователи», как в open-webui: список, поиск, добавление, смена имени, почты, пароля и роли.
import { create_User, list_Roles, list_Users, update_User } from '../api.js';
import { open_Modal } from '../components/modal.js';
import { show_Toast } from '../components/toast.js';
import { create_Button, create_Element, create_EmptyBlock, create_IconButton, create_LoadingBlock, create_SearchBox } from '../lib/dom.js';
import { format_DateTime, get_Initials } from '../lib/format.js';
import { Roles, get_NextRole, get_RoleLabel, get_UserRole, roleChoices } from '../roles.js';
import { AdminTexts, AppTexts, format_Text, get_ErrorText } from '../texts.js';

function create_RoleSelect(selectedRole, id) {
	return create_Element('select', { className: 'select', attrs: { id } }, roleChoices.map((role) => create_Element('option', {
		text: get_RoleLabel(role),
		attrs: { value: role, selected: role === selectedRole },
	})));
}

function get_RoleBadgeClass(role) {
	if (role === Roles.admin) {
		return 'badge badge-strong';
	}
	return role === Roles.pending ? 'badge badge-outline' : 'badge';
}

function create_Field(id, label, control, hint = '') {
	return create_Element('div', { className: 'field' }, [
		create_Element('label', { className: 'field-label', text: label, attrs: { for: id } }),
		control,
		hint ? create_Element('span', { className: 'field-hint', text: hint }) : null,
	]);
}

// Общая форма для создания и правки. При правке пароль меняется, только если поле заполнено.
// Свою роль администратор не меняет: ядро такой запрос отклонит
function selected_RoleIds(boxes) {
	return boxes.filter((input) => input.checked).map((input) => input.value);
}

function open_UserForm({ config, account, currentUserId, roles = [], onSaved }) {
	const isEdit = Boolean(account);
	const isSelf = isEdit && account.userId === currentUserId;
	const errorBox = create_Element('div', { className: 'form-error', attrs: { role: 'alert' } });
	errorBox.hidden = true;

	const loginInput = create_Element('input', { className: 'input', attrs: { id: 'user-login', type: 'email', autocomplete: 'off', required: true } });
	const passwordInput = create_Element('input', { className: 'input', attrs: { id: 'user-password', type: 'password', autocomplete: 'new-password', required: !isEdit } });
	const nameInput = create_Element('input', { className: 'input', attrs: { id: 'user-name', autocomplete: 'off', required: true } });
	const initialRole = isEdit ? get_UserRole(account, config) : Roles.user;
	const roleSelect = create_RoleSelect(initialRole, 'user-role');
	roleSelect.disabled = isSelf;
	const assigned = new Set(account?.roleIds ?? []);
	const roleBoxes = roles.map((role) => {
		const input = create_Element('input', { attrs: { type: 'checkbox', value: role.roleId, disabled: isSelf } });
		input.checked = assigned.size > 0 ? assigned.has(role.roleId) : role.roleId === 'role-user';
		return input;
	});
	const roleChecks = create_Element('div', { className: 'check-list' }, roles.map((role, index) => create_Element('label', { className: 'check-row' }, [
		roleBoxes[index],
		create_Element('span', { text: role.title }),
	])));
	if (isEdit) {
		nameInput.value = account.displayName;
		loginInput.value = account.login;
	}

	const form = create_Element('form', { className: 'form-grid', attrs: { novalidate: true, id: 'user-form' } }, [
		create_Field('user-name', AdminTexts.usersFieldName, nameInput),
		create_Field('user-login', AdminTexts.usersFieldLogin, loginInput),
		isEdit
			? create_Field('user-password', AdminTexts.usersFieldNewPassword, passwordInput, AdminTexts.usersFieldNewPasswordHint)
			: create_Field('user-password', AdminTexts.usersFieldPassword, passwordInput),
		roles.length > 0
			? create_Field('user-roles', AdminTexts.usersFieldRoles, roleChecks, isSelf ? AdminTexts.usersSelfRoleHint : AdminTexts.usersFieldRolesHint)
			: create_Field('user-role', AdminTexts.usersFieldRole, roleSelect, isSelf ? AdminTexts.usersSelfRoleHint : AdminTexts.usersFieldRoleHint),
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

	// Только изменённые поля: так правка имени не трогает роль и пароль
	const build_Changes = () => {
		const changes = {};
		const displayName = nameInput.value.trim();
		const login = loginInput.value.trim();
		if (displayName !== account.displayName) {
			changes.displayName = displayName;
		}
		if (login !== account.login) {
			changes.login = login;
		}
		if (passwordInput.value !== '') {
			changes.password = passwordInput.value;
		}
		if (!isSelf && roles.length > 0) {
			const nextIds = selected_RoleIds(roleBoxes);
			const same = nextIds.length === assigned.size && nextIds.every((roleId) => assigned.has(roleId));
			if (!same) {
				changes.roleIds = nextIds;
			}
		} else if (!isSelf && roleSelect.value !== initialRole) {
			changes.role = roleSelect.value;
		}
		return changes;
	};

	form.addEventListener('submit', async (event) => {
		event.preventDefault();
		const required = isEdit ? [nameInput, loginInput] : [nameInput, loginInput, passwordInput];
		const emptyField = required.find((input) => input.value.trim() === '');
		if (emptyField) {
			emptyField.focus();
			return;
		}
		if (roles.length > 0 && !isSelf && selected_RoleIds(roleBoxes).length === 0) {
			errorBox.textContent = AdminTexts.usersRolesRequired;
			errorBox.hidden = false;
			return;
		}
		errorBox.hidden = true;
		const changes = isEdit ? build_Changes() : null;
		if (isEdit && Object.keys(changes).length === 0) {
			modal.close();
			return;
		}
		submitButton.disabled = true;
		try {
			let saved = isEdit
				? await update_User(account.userId, changes)
				: await create_User({
					login: loginInput.value.trim(),
					password: passwordInput.value,
					displayName: nameInput.value.trim(),
					role: roles.length > 0 ? Roles.user : roleSelect.value,
				});
			if (!isEdit && roles.length > 0) {
				saved = await update_User(saved.userId, { roleIds: selected_RoleIds(roleBoxes) });
			}
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
	nameInput.focus();
}

export function create_UsersTab({ user, config }) {
	let accounts = [];
	let roles = [];
	let query = '';

	const countLabel = create_Element('span', { className: 'section-count' });
	const pendingLabel = create_Element('span', { className: 'badge badge-outline' });
	pendingLabel.hidden = true;
	const content = create_Element('div', {}, [create_LoadingBlock(AppTexts.loading)]);

	const reload = async () => {
		try {
			const payload = await list_Users();
			accounts = Array.isArray(payload?.users) ? payload.users : [];
			if (roles.length === 0) {
				try {
					roles = (await list_Roles())?.roles ?? [];
				} catch {
					roles = [];
				}
			}
			render_Table();
		} catch (error) {
			content.replaceChildren(create_EmptyBlock('alert', AdminTexts.usersLoadFailed, [
				create_Element('div', { className: 'callout-hint', text: get_ErrorText(error) }),
				create_Button(AppTexts.retry, { variant: 'outline', icon: 'refresh', onClick: reload }),
			]));
		}
	};

	const toggle_Role = async (account) => {
		const nextRole = get_NextRole(get_UserRole(account, config));
		try {
			const saved = await update_User(account.userId, { role: nextRole });
			Object.assign(account, saved);
			render_Table();
			show_Toast(AdminTexts.usersRoleChanged, 'success');
		} catch (error) {
			show_Toast(get_ErrorText(error), 'error');
		}
	};

	const render_RoleBadge = (account) => {
		if (roles.length > 0) {
			const titles = roles.filter((role) => (account.roleIds ?? []).includes(role.roleId)).map((role) => role.title);
			const label = titles.length > 0 ? titles.join(', ') : get_RoleLabel(get_UserRole(account, config));
			return create_Element('span', { className: 'badge', text: label });
		}
		const role = get_UserRole(account, config);
		const isSelf = account.userId === user.userId;
		// Себе роль не меняем, чтобы администратор случайно не потерял доступ к панели
		return create_Element('button', {
			className: get_RoleBadgeClass(role),
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
		const pendingCount = accounts.filter((account) => get_UserRole(account, config) === Roles.pending).length;
		pendingLabel.textContent = format_Text(AdminTexts.usersPendingCount, { count: pendingCount });
		pendingLabel.hidden = pendingCount === 0;
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
					create_Element('th', { text: AdminTexts.usersColumnCreated }),
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
					create_Element('td', { className: 'time-cell', text: format_DateTime(account.createdAt) || AppTexts.notSpecified }),
					create_Element('td', { className: 'table-cell-muted', text: account.userId }),
					create_Element('td', { className: 'table-cell-actions' }, [
						create_IconButton('edit', `${AdminTexts.usersEdit}: ${account.displayName}`, () => open_UserForm({
							config,
							account,
							roles,
							currentUserId: user.userId,
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
			create_Element('h2', { className: 'section-title' }, [AdminTexts.tabUsers, countLabel, pendingLabel]),
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
						roles,
						currentUserId: user.userId,
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

