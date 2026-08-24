/**
 * The self-service account API, and the one place its requests are built.
 *
 * Two rules are folded in here so no call site can forget either. The session
 * cookie is HttpOnly, so `credentials: 'include'` is what sends it at all; and
 * /api/account refuses any non-GET arriving without X-Facile-CSRF, before it
 * reads a password. A client that sends the cookie but not the header sees
 * every read succeed and every write answer 403, which reads as "the app
 * works, saving is broken" — the exact failure porte shipped to production
 * twice. The header's value carries no meaning; its presence is the signal,
 * because a cross-site form post cannot set one.
 */

/** AccountView is the signed-in person as /api/account reports them. */
export interface AccountView {
	email: string;
	name: string;
	totp_enrolled: boolean;
	recovery_codes_remaining: number;
}

/** TOTPEnrolment is the material needed to add the factor to an authenticator. */
export interface TOTPEnrolment {
	secret: string;
	uri: string;
	qr_png: string;
}

/** RecoveryCodes is a fresh set of single-use codes, shown exactly once. */
export interface RecoveryCodes {
	recovery_codes: string[];
}

/**
 * AccountError carries the status so a caller can tell "not signed in" from a
 * refused password without parsing prose.
 */
export class AccountError extends Error {
	readonly status: number;

	constructor(status: number, message: string) {
		super(message);
		this.name = 'AccountError';
		this.status = status;
	}
}

/** isUnauthorized reports whether a failure means there is no session. */
export function isUnauthorized(cause: unknown): boolean {
	return cause instanceof AccountError && cause.status === 401;
}

async function request<T>(path: string, body?: unknown): Promise<T> {
	const method = body === undefined ? 'GET' : 'POST';
	const headers = new Headers();
	if (method !== 'GET') {
		headers.set('X-Facile-CSRF', '1');
		headers.set('Content-Type', 'application/json');
	}
	let response: Response;
	try {
		response = await fetch(path, {
			method,
			headers,
			credentials: 'include',
			body: body === undefined ? undefined : JSON.stringify(body)
		});
	} catch {
		throw new AccountError(0, 'Registre did not answer. Check the connection and try again.');
	}
	const parsed = await response.json().catch(() => null);
	if (!response.ok) {
		const message =
			parsed && typeof parsed.error === 'string'
				? parsed.error
				: `Request failed (${response.status}).`;
		throw new AccountError(response.status, message);
	}
	return parsed as T;
}

/** account is the self-service API surface, one method per route. */
export const account = {
	me: () => request<AccountView>('/api/account'),
	beginTOTP: () => request<TOTPEnrolment>('/api/account/totp/begin', {}),
	confirmTOTP: (code: string) => request<RecoveryCodes>('/api/account/totp/confirm', { code }),
	disableTOTP: (password: string, code: string) =>
		request<unknown>('/api/account/totp/disable', { password, code }),
	regenerateRecoveryCodes: (password: string) =>
		request<RecoveryCodes>('/api/account/totp/recovery-codes', { password }),
	changePassword: (current_password: string, new_password: string) =>
		request<unknown>('/api/account/password', { current_password, new_password }),
	logout: () => request<unknown>('/api/account/logout', {})
};

/**
 * AccountContext is what the /account layout publishes to its tabs: the loaded
 * view and a reload, so a tab that changes the account never has to re-derive
 * the header the layout already owns.
 */
export interface AccountContext {
	readonly me: AccountView | null;
	refresh: () => Promise<void>;
}

/** ACCOUNT_CONTEXT keys AccountContext on the /account layout. */
export const ACCOUNT_CONTEXT = Symbol('registre.account');

/** accountError turns anything thrown by account into a line a human can read. */
export function accountError(cause: unknown, fallback: string): string {
	if (cause instanceof AccountError) return cause.message;
	if (cause instanceof Error && cause.message) return cause.message;
	return fallback;
}
