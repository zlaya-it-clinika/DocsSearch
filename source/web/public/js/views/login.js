// Экран входа.
import { login_User } from '../api.js';
import { create_Element, create_Icon } from '../lib/dom.js';
import { AppTexts, AuthTexts, get_ErrorText } from '../texts.js';

// onSuccess({ user, csrfToken })
export function create_LoginView({ onSuccess, compact = false }) {
	const errorBox = create_Element('div', { className: 'form-error', attrs: { role: 'alert' } });
	errorBox.hidden = true;

	const loginInput = create_Element('input', {
		className: 'input',
		attrs: { id: 'login-name', name: 'username', autocomplete: 'username', required: true, autofocus: true },
	});
	const passwordInput = create_Element('input', {
		className: 'input',
		attrs: { id: 'login-password', name: 'password', type: 'password', autocomplete: 'current-password', required: true },
	});
	const submitButton = create_Element('button', {
		className: 'button button-primary button-block',
		attrs: { type: 'submit' },
		text: AuthTexts.submit,
	});

	const form = create_Element('form', { className: 'auth-form', attrs: { novalidate: true } }, [
		create_Element('div', { className: 'field' }, [
			create_Element('label', { className: 'field-label', text: AuthTexts.loginLabel, attrs: { for: 'login-name' } }),
			loginInput,
		]),
		create_Element('div', { className: 'field' }, [
			create_Element('label', { className: 'field-label', text: AuthTexts.passwordLabel, attrs: { for: 'login-password' } }),
			passwordInput,
		]),
		errorBox,
		submitButton,
	]);

	form.addEventListener('submit', async (event) => {
		event.preventDefault();
		const login = loginInput.value.trim();
		const password = passwordInput.value;
		if (login === '' || password === '') {
			(login === '' ? loginInput : passwordInput).focus();
			return;
		}
		errorBox.hidden = true;
		submitButton.disabled = true;
		submitButton.textContent = AuthTexts.submitting;
		try {
			const result = await login_User(login, password);
			onSuccess(result);
		} catch (error) {
			errorBox.textContent = get_ErrorText(error);
			errorBox.hidden = false;
			passwordInput.value = '';
			passwordInput.focus();
		} finally {
			submitButton.disabled = false;
			submitButton.textContent = AuthTexts.submit;
		}
	});

	const element = create_Element('main', { className: 'auth-page' }, [
		create_Element('div', { className: 'auth-card' }, [
			create_Element('div', { className: 'auth-header' }, [
				create_Element('span', { className: 'brand-mark' }, [create_Icon('logo')]),
				create_Element('h1', { className: 'auth-title', text: compact ? AppTexts.productName : AuthTexts.title }),
				create_Element('p', { className: 'auth-lead', text: AuthTexts.lead }),
			]),
			form,
			create_Element('p', { className: 'auth-footnote', text: AuthTexts.footnote }),
		]),
	]);

	return {
		element,
		focus: () => loginInput.focus(),
	};
}
