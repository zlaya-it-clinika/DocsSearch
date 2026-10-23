// Две роли: администратор и пользователь.
// Ядро отдаёт числовой уровень доступа, порог администратора приходит из .env веб-сервера.
// Если ядро начнёт отдавать поле role, оно имеет приоритет. Менять только здесь.
import { RoleTexts } from './texts.js';

export const Roles = Object.freeze({
	admin: 'admin',
	user: 'user',
});

export function get_UserRole(user, config) {
	if (user?.role === Roles.admin || user?.role === Roles.user) {
		return user.role;
	}
	return (user?.accessLevel ?? 0) >= config.adminAccessLevel ? Roles.admin : Roles.user;
}

export function check_IsAdmin(user, config) {
	return get_UserRole(user, config) === Roles.admin;
}

export function get_AccessLevelForRole(role, config) {
	return role === Roles.admin ? config.adminAccessLevel : config.userAccessLevel;
}

export function get_RoleLabel(role) {
	return role === Roles.admin ? RoleTexts.admin : RoleTexts.user;
}

// Документ закрыт, если его уровень не ниже уровня администратора
export function get_DocumentAudience(accessLevel, config) {
	return (accessLevel ?? 0) >= config.adminAccessLevel ? Roles.admin : Roles.user;
}

export function get_DocumentAudienceLabel(accessLevel, config) {
	return get_DocumentAudience(accessLevel, config) === Roles.admin ? RoleTexts.documentForAdmins : RoleTexts.documentForAll;
}
