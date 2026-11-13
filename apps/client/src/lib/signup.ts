/**
 * The sign-up API, one function per route. None of these routes take the
 * account cookie: a person signing up has no account yet. /api/signup/finish
 * sets the cookie in its answer.
 */
import { locale } from '$lib/i18n/i18n.svelte';

/**
 * SignupError carries the server's error code so pages can explain it, and
 * for a 429 the seconds to wait before asking again (0 when unknown).
 */
export class SignupError extends Error {
	readonly status: number;
	readonly code: string;
	readonly retryAfter: number;

	constructor(status: number, code: string, retryAfter = 0) {
		super(code);
		this.name = 'SignupError';
		this.status = status;
		this.code = code;
		this.retryAfter = retryAfter;
	}
}

/** waitOf reads how long a 429 asks to wait, from the body or Retry-After. */
function waitOf(response: Response, parsed: { retry_after?: unknown } | null): number {
	const fromBody = Number(parsed?.retry_after);
	if (Number.isFinite(fromBody) && fromBody > 0) return Math.ceil(fromBody);
	const fromHeader = Number(response.headers.get('Retry-After'));
	return Number.isFinite(fromHeader) && fromHeader > 0 ? Math.ceil(fromHeader) : 0;
}

/** post sends one JSON body and answers the parsed reply or throws SignupError. */
async function post<T>(path: string, body: unknown): Promise<T> {
	let response: Response;
	try {
		response = await fetch(path, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json', 'X-Facile-CSRF': '1' },
			body: JSON.stringify(body)
		});
	} catch {
		throw new SignupError(0, 'network');
	}
	const parsed = await response.json().catch(() => null);
	if (!response.ok) {
		const code = response.status === 429 ? 'rate_limited' : (parsed?.error ?? 'failed');
		throw new SignupError(response.status, code, response.status === 429 ? waitOf(response, parsed) : 0);
	}
	return parsed as T;
}

/** Pending is what a live link is for. */
export interface Pending {
	email: string;
	locale: string;
}

/** FinishForm is the last step: profile, consent and the password. */
export interface FinishForm {
	token: string;
	name: string;
	locale: string;
	accept_terms: boolean;
	password: string;
}

export const signup = {
	start: (email: string, altcha: string, authRequest: string, invitation = '') =>
		post<{ status: string }>('/api/signup/start', {
			email,
			altcha,
			locale: locale(),
			auth_request: authRequest,
			invitation
		}),
	verify: (token: string) => post<Pending>('/api/signup/verify', { token }),
	finish: (form: FinishForm) => post<{ redirect: string }>('/api/signup/finish', form)
};

/** signupOpen reports whether public sign-up is open on this instance. */
export async function signupOpen(): Promise<boolean> {
	const config = await fetch('/api/config')
		.then((r) => r.json())
		.catch(() => null);
	return config?.signup === true;
}
