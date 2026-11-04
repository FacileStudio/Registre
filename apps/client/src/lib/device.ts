/**
 * The device verification API, and the one place its endpoints are named.
 *
 * RFC 8628 splits the grant in two. The CLI half — the device authorization
 * request and the token polling — never touches a browser and is not here.
 * This module is the other half: the screen a human opens to approve or deny
 * a code a terminal is showing them, on whatever device is in their hand.
 *
 * It is written against `apps/api/modules/device`, which mounts three routes
 * under `/api/device` behind `accounts.Guard`. Three of that package's
 * decisions reach into this file and none of them may be softened here.
 *
 * The session cookie is HttpOnly, so `credentials: 'include'` is what sends it
 * at all. Every route is gated on a signed-in human because the approval
 * copies the subject and the authentication time out of the session rather
 * than out of the request — an anonymous approval would mint a credential
 * nobody is accountable for.
 *
 * X-Facile-CSRF rides on the two POSTs and not on the GET, which is exactly
 * where `Guard.CSRF` requires it: a cookie is sent whether or not the page
 * that triggered the request was ours, and a custom header cannot be set
 * cross-origin without a preflight this API does not answer.
 *
 * And the API answers a bad user code with exactly one refusal: 404
 * `unknown_code` covers never issued, already answered, expired and already
 * spent, together and on purpose. The endpoints that would report them apart
 * are the ones a guesser would use to tell a real code from a made-up one. So
 * `unusable` is one outcome here rather than four, and its copy has to read
 * correctly for all four. Nothing in this module may widen that.
 *
 * The shape of the user code itself is not here. `$lib/usercode` owns it, and
 * every code handed to the calls below has already been through that module.
 */

import { locale, t } from '$lib/i18n/i18n.svelte';
import type { MessageKey } from '$lib/i18n/en';

/**
 * DeviceScope is one permission the waiting client asked for, already resolved
 * to something a human can weigh.
 */
export interface DeviceScope {
	readonly name: string;
	readonly label: string;
}

/**
 * DeviceRequestView is a pending flow as `GET /api/device` reports it: which
 * program is asking, what it would be allowed to do, and how long is left.
 *
 * It carries no code and no token, so it is safe to hold in component state.
 * It also carries nothing about the machine that started the flow, because a
 * hostname the CLI supplied would be a string an attacker chooses.
 */
export interface DeviceRequestView {
	client_id: string;
	client_name: string;
	scopes: string[];
	expires_at: string;
}

/**
 * DeviceOutcome is a state the flow does not leave.
 *
 * `approved` and `denied` are known because this page made the call that
 * caused them. `unusable` is the API's single refusal. `used` is decided here
 * and never by the server: it means this tab has already answered this code,
 * which is knowledge the tab owns and no response has to leak.
 */
export type DeviceOutcome = 'approved' | 'denied' | 'unusable' | 'used';

/**
 * DeviceError carries a terminal outcome when the request reached one, so a
 * caller can render the right ending instead of parsing prose.
 */
export class DeviceError extends Error {
	readonly status: number;
	readonly outcome: DeviceOutcome | null;

	constructor(status: number, message: string, outcome: DeviceOutcome | null) {
		super(message);
		this.name = 'DeviceError';
		this.status = status;
		this.outcome = outcome;
	}
}

/** isSignedOut reports whether a failure means there is no session to verify with. */
export function isSignedOut(cause: unknown): boolean {
	return cause instanceof DeviceError && cause.status === 401;
}

/** outcomeOf returns the terminal state a failure reached, or null when it reached none. */
export function outcomeOf(cause: unknown): DeviceOutcome | null {
	return cause instanceof DeviceError ? cause.outcome : null;
}

/**
 * RETRY and THROTTLED are the two failures worth trying again, so they are
 * messages rather than outcomes. RFC 8628 §5.2 requires the routes that accept
 * a user code to be rate limited, and `modules/device` gives one source
 * address the login endpoint's budget — so a human behind a shared address is
 * the one who will meet that limit without having earned it.
 */
const RETRY = 'device.retry';
const THROTTLED = 'device.throttled';

/**
 * SCOPE_LABELS says what a scope buys the app that asked for it. A consent
 * screen that prints `offline_access` at a human has not obtained consent, it
 * has obtained a click.
 */
const SCOPE_LABELS: Record<string, MessageKey> = {
	openid: 'device.scope.openid',
	profile: 'device.scope.profile',
	email: 'device.scope.email',
	roles: 'device.scope.roles',
	offline_access: 'device.scope.offline'
};

/**
 * describeScopes turns the scope strings into lines a human can weigh. An
 * unrecognised scope keeps its raw name rather than being dropped: a
 * permission the page cannot describe is still a permission being granted.
 */
export function describeScopes(scopes: string[]): DeviceScope[] {
	return scopes.map((name) => ({ name, label: SCOPE_LABELS[name] ? t(SCOPE_LABELS[name]) : name }));
}

/**
 * expiresAtLabel is the deadline as a clock time the human can read off, or an
 * empty string when the API sent something unparseable. A countdown would be a
 * timer to tear down and a number that keeps redrawing on a phone that is
 * about to be put away; the deadline itself does not move.
 */
export function expiresAtLabel(expiresAt: string): string {
	const at = new Date(expiresAt);
	if (Number.isNaN(at.getTime())) return '';
	return at.toLocaleTimeString(locale(), { hour: '2-digit', minute: '2-digit' });
}

/** STASH_KEY is where a typed code waits while its owner goes to sign in. */
const STASH_KEY = 'registre.device.user_code';

/**
 * stash keeps a typed code while its owner leaves to get a session, in the tab
 * rather than in the URL. sessionStorage dies with the tab and never reaches a
 * server log, a referrer or a browser history entry.
 */
export function stash(userCode: string): void {
	try {
		window.sessionStorage.setItem(STASH_KEY, userCode);
	} catch {
		return;
	}
}

/** unstash takes the stashed code and clears it, so it is read exactly once. */
export function unstash(): string {
	try {
		const held = window.sessionStorage.getItem(STASH_KEY) ?? '';
		window.sessionStorage.removeItem(STASH_KEY);
		return held;
	} catch {
		return '';
	}
}

/**
 * refusal turns a failed response into the DeviceError the page renders.
 *
 * Only 404 is terminal, because only 404 is the answer to a code the caller
 * gave. 403 is a missing CSRF header and 500 is registre's own fault, so both
 * are bugs a retry might survive rather than endings to show a human.
 */
function refusal(status: number): DeviceError {
	if (status === 404) return new DeviceError(status, '', 'unusable');
	return new DeviceError(status, t(status === 429 ? THROTTLED : RETRY), null);
}

/** send performs one verification call and refuses in one shape. */
async function send<T>(path: string, init: RequestInit): Promise<T> {
	let response: Response;
	try {
		response = await fetch(path, { ...init, credentials: 'include' });
	} catch {
		throw new DeviceError(0, t(RETRY), null);
	}
	const parsed = await response.json().catch(() => null);
	if (!response.ok) throw refusal(response.status);
	return parsed as T;
}

/** answer posts a decision about one user code. */
function answer(path: string, userCode: string): Promise<unknown> {
	return send<unknown>(path, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', 'X-Facile-CSRF': '1' },
		body: JSON.stringify({ user_code: userCode })
	});
}

/**
 * device is the verification surface, one method per route. Every path the
 * client knows about is on this object and nowhere else.
 */
export const device = {
	pending: (userCode: string) =>
		send<DeviceRequestView>(`/api/device?user_code=${encodeURIComponent(userCode)}`, {
			method: 'GET'
		}),
	approve: (userCode: string) => answer('/api/device/approve', userCode),
	deny: (userCode: string) => answer('/api/device/deny', userCode),

	/**
	 * signInPath is a destination, not a call — the browser navigates to it and
	 * registre answers with a redirect into its own /authorize.
	 *
	 * It carries no parameters and must not grow any. The route reads nothing
	 * from the request precisely so that it cannot be turned into an open
	 * redirector, and the user code stays in this tab's storage rather than
	 * riding through registre's access log twice on its way round.
	 */
	signInPath: '/api/device/authenticate'
};
