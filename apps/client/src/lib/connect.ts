/**
 * The third-party connection API: the consent screen a person answers when an
 * MCP client (Claude, ChatGPT, Cursor...) asks to work in a Facile app for
 * them, and the connected apps list where they disconnect one.
 *
 * Written against `apps/api/modules/connect`. Every route needs the account
 * session cookie, so every call sends credentials, and the writes carry
 * X-Facile-CSRF. The API answers 404 for every flow this session may not
 * answer, so `gone` is one state here and not several.
 */

import { t } from '$lib/i18n/i18n.svelte';
import type { MessageKey } from '$lib/i18n/en';

/** PendingConnection is a third-party flow waiting on the person's answer. */
export interface PendingConnection {
	client_id: string;
	client_name: string;
	client_uri?: string;
	registration: string;
	redirect_host: string;
	loopback_only: boolean;
	resource: string;
	app_name: string;
	scopes: string[];
}

/** ConnectedApp is one live grant, as the account page lists it. */
export interface ConnectedApp {
	id: string;
	client_id: string;
	client_name: string;
	client_uri?: string;
	redirect_host: string;
	resource: string;
	app_name: string;
	scopes: string[];
	created_at: string;
	last_used_at?: string;
}

/** ConnectError carries the HTTP status so a page can tell gone from signed out. */
export class ConnectError extends Error {
	readonly status: number;

	constructor(status: number) {
		super(t('common.failed'));
		this.name = 'ConnectError';
		this.status = status;
	}
}

/** send performs one call and throws ConnectError on any failure. */
async function send<T>(path: string, init: RequestInit = {}): Promise<T> {
	let response: Response;
	try {
		response = await fetch(path, { ...init, credentials: 'include' });
	} catch {
		throw new ConnectError(0);
	}
	if (!response.ok) throw new ConnectError(response.status);
	return (response.status === 204 ? null : await response.json()) as T;
}

/** write is a mutating call with the CSRF header. */
function write<T>(path: string, method: string, body?: unknown): Promise<T> {
	return send<T>(path, {
		method,
		headers: { 'Content-Type': 'application/json', 'X-Facile-CSRF': '1' },
		body: body === undefined ? undefined : JSON.stringify(body)
	});
}

/**
 * describe turns scopes into lines a person can weigh. A scope ending in
 * .read or .write names the app; an unknown scope keeps its raw name, because
 * a permission the page cannot describe is still a permission.
 */
export function describe(scopes: string[], app: string): { name: string; label: string }[] {
	const fixed: Record<string, MessageKey> = {
		openid: 'device.scope.openid',
		profile: 'device.scope.profile',
		email: 'device.scope.email',
		offline_access: 'connect.scope.offline'
	};
	return scopes.map((name) => {
		if (fixed[name]) return { name, label: t(fixed[name]) };
		if (name.endsWith('.read')) return { name, label: t('connect.scope.read', { app }) };
		if (name.endsWith('.write')) return { name, label: t('connect.scope.write', { app }) };
		return { name, label: name };
	});
}

/** connect is the surface, one method per route. */
export const connect = {
	pending: (authRequest: string) =>
		send<PendingConnection>(`/api/connect?auth_request=${encodeURIComponent(authRequest)}`),
	answer: (authRequest: string, allow: boolean) =>
		write<{ redirect: string }>('/api/connect', 'POST', { auth_request: authRequest, allow }),
	apps: () => send<ConnectedApp[]>('/api/account/apps'),
	disconnect: (id: string) => write<null>(`/api/account/apps/${encodeURIComponent(id)}`, 'DELETE')
};
