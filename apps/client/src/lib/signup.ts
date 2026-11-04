/**
 * The sign-up API, one function per route. None of these routes take the
 * account cookie: a person signing up has no account yet. /api/signup/finish
 * sets the cookie in its answer.
 */
import { locale } from '$lib/i18n/i18n.svelte';

/** SignupError carries the server's error code so pages can explain it. */
export class SignupError extends Error {
	readonly status: number;
	readonly code: string;

	constructor(status: number, code: string) {
		super(code);
		this.name = 'SignupError';
		this.status = status;
		this.code = code;
	}
}

/** post sends one JSON body and answers the parsed reply or throws SignupError. */
async function post<T>(path: string, body: unknown): Promise<T> {
	let response: Response;
	try {
		response = await fetch(path, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(body)
		});
	} catch {
		throw new SignupError(0, 'network');
	}
	const parsed = await response.json().catch(() => null);
	if (!response.ok) {
		const code = response.status === 429 ? 'rate_limited' : (parsed?.error ?? 'failed');
		throw new SignupError(response.status, code);
	}
	return parsed as T;
}

/** Pending is what a live link is for. */
export interface Pending {
	email: string;
	locale: string;
}

/** Options are passkey ceremony options plus their bearer. */
export interface Options {
	ceremony: string;
	publicKey: Record<string, unknown>;
}

/** FinishForm is the last step: profile, consent and one way to sign in. */
export interface FinishForm {
	token: string;
	name: string;
	locale: string;
	accept_terms: boolean;
	password?: string;
	ceremony?: string;
	credential?: Record<string, unknown>;
}

export const signup = {
	start: (email: string, altcha: string) =>
		post<{ status: string }>('/api/signup/start', { email, altcha, locale: locale() }),
	verify: (token: string) => post<Pending>('/api/signup/verify', { token }),
	passkey: (token: string) => post<Options>('/api/signup/passkey', { token }),
	finish: (form: FinishForm) => post<{ redirect: string }>('/api/signup/finish', form)
};

/** signupOpen reports whether public sign-up is open on this instance. */
export async function signupOpen(): Promise<boolean> {
	const config = await fetch('/api/config')
		.then((r) => r.json())
		.catch(() => null);
	return config?.signup === true;
}
