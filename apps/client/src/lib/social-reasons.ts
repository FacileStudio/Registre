/**
 * The pure half of `$lib/social`: the reasons the server sends back on
 * `?error=` and the URL a sign-in button goes to. Nothing here imports muse,
 * so the check script can run it under bun.
 */
import type { MessageKey } from './i18n/en';

/**
 * REASONS maps the short sentences the server puts in `?error=` to catalog
 * keys. The login page prints only text matching its own plain-sentence guard,
 * so these are the sentences, verbatim, that modules/login sends back.
 */
export const REASONS: Record<string, MessageKey> = {
	'the sign-in link expired': 'login.reason.expired',
	'sign-in was cancelled': 'login.reason.canceled',
	'the sign-in failed, try again': 'login.reason.failed',
	'sign-up is closed': 'login.reason.closed',
	'the provider shared no verified email': 'login.reason.noEmail',
	'this account is linked to another user': 'login.reason.taken',
	'no access to this app': 'login.noAccess'
};

/** reasonKey is the catalog key for a reason the server sent, or null. */
export function reasonKey(reason: string): MessageKey | null {
	return REASONS[reason] ?? null;
}

/** startURL is where a button sends the browser. */
export function startURL(provider: string, authRequest: string): string {
	return `/api/login/social/${encodeURIComponent(provider)}/start?auth_request=${encodeURIComponent(authRequest)}`;
}
