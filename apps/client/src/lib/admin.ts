/**
 * The operator dashboard's HTTP client and view types.
 *
 * The operator credential is HTTP Basic on every /api/admin request, and it
 * lives in this module's memory for exactly as long as the tab does. It is
 * never written to localStorage or sessionStorage: a long-lived password in
 * web storage is readable by any script that reaches the page, and a reload
 * dropping the operator back to the sign-in form is the cheap, correct
 * failure.
 */

/** UserView is a person as /api/admin/users reports them. */
export interface UserView {
	id: string;
	email: string;
	name: string;
	admin: boolean;
	roles?: string[];
	email_verified: boolean;
	created_at: string;
	disabled_at?: string;
}

/** ClientView is an application or service account in the register. */
export interface ClientView {
	client_id: string;
	subject: string;
	name: string;
	kind: string;
	grant_types?: string[];
	scopes?: string[];
	audiences?: string[];
	redirect_uris?: string[];
	visible_roles?: string[];
	backchannel_logout_uri?: string;
	managed: boolean;
	created_at: string;
	disabled_at?: string;
}

/** InviteView is a single-use invitation and whether it has been redeemed. */
export interface InviteView {
	email: string;
	created_by: string;
	expires_at: string;
	created_at: string;
	redeemed: boolean;
}

/** KeyState is one signing key's position in the rotation lifecycle. */
export interface KeyState {
	id: string;
	algorithm: string;
	state: string;
	created_at: string;
	activated_at?: string;
	retired_at?: string;
	expires_at?: string;
}

export interface NewUser {
	email: string;
	name?: string;
	password: string;
	admin?: boolean;
}

export interface UpdateUser {
	name?: string;
	admin?: boolean;
	roles?: string[];
}

export interface NewClient {
	client_id: string;
	name: string;
	kind: string;
	redirect_uris?: string[];
	visible_roles?: string[];
	backchannel_logout_uri?: string;
}

export interface UpdateClient {
	name?: string;
	kind?: string;
	grant_types?: string[];
	scopes?: string[];
	audiences?: string[];
	redirect_uris?: string[];
	visible_roles?: string[];
	backchannel_logout_uri?: string;
}

/** CreatedClient carries the client plus its secret, shown exactly once. */
export interface CreatedClient {
	client: ClientView;
	secret: string;
}

/** CreatedInvite carries the invitation token, shown exactly once. */
export interface CreatedInvite {
	email: string;
	token: string;
}

const encoder = new TextEncoder();

/** authHeader builds the HTTP Basic value for an operator credential. */
export function authHeader(email: string, password: string): string {
	return 'Basic ' + btoa(String.fromCharCode(...encoder.encode(`${email}:${password}`)));
}

let credential = '';
let operator = '';

/** setOperator holds the operator credential in memory for this tab. */
export function setOperator(email: string, password: string): void {
	credential = authHeader(email, password);
	operator = email;
}

/** clearOperator forgets the operator credential. */
export function clearOperator(): void {
	credential = '';
	operator = '';
}

/** operatorEmail is the signed-in operator's address, empty when signed out. */
export function operatorEmail(): string {
	return operator;
}

/** hasOperator reports whether a credential is held. */
export function hasOperator(): boolean {
	return credential !== '';
}

/**
 * AdminError carries the status alongside the message so a caller can tell a
 * refused credential from a rejected body.
 */
export class AdminError extends Error {
	readonly status: number;

	constructor(status: number, message: string) {
		super(message);
		this.name = 'AdminError';
		this.status = status;
	}
}

async function call<T>(path: string, init: RequestInit = {}): Promise<T> {
	const method = init.method ?? (init.body === undefined ? 'GET' : 'POST');
	const headers = new Headers(init.headers);
	if (credential) headers.set('Authorization', credential);
	if (method !== 'GET') {
		headers.set('X-Facile-CSRF', '1');
		headers.set('Content-Type', 'application/json');
	}
	let response: Response;
	try {
		response = await fetch(path, { ...init, method, headers, credentials: 'include' });
	} catch {
		throw new AdminError(0, 'Registre did not answer. Check the connection and try again.');
	}
	const body = await response.json().catch(() => null);
	if (!response.ok) {
		const message =
			body && typeof body.error === 'string' ? body.error : `Request failed (${response.status}).`;
		throw new AdminError(response.status, message);
	}
	return body as T;
}

export const admin = {
	listUsers: () => call<UserView[]>('/api/admin/users'),
	createUser: (user: NewUser) =>
		call<UserView>('/api/admin/users', { method: 'POST', body: JSON.stringify(user) }),
	updateUser: (id: string, user: UpdateUser) =>
		call<UserView>(`/api/admin/users/${id}`, { method: 'PUT', body: JSON.stringify(user) }),
	setUserPassword: (id: string, password: string) =>
		call<{ ok: boolean; logout_targets: number }>(`/api/admin/users/${id}/password`, {
			method: 'POST',
			body: JSON.stringify({ password })
		}),
	disableUser: (id: string) =>
		call<{ disabled: string; logout_targets: number }>(`/api/admin/users/${id}/disable`, {
			method: 'POST'
		}),
	enableUser: (id: string) =>
		call<{ enabled: string }>(`/api/admin/users/${id}/enable`, { method: 'POST' }),
	deleteUser: (id: string) =>
		call<{ deleted: string }>(`/api/admin/users/${id}`, { method: 'DELETE' }),
	listInvites: () => call<InviteView[]>('/api/admin/invites'),
	createInvite: (email: string) =>
		call<CreatedInvite>('/api/admin/invites', { method: 'POST', body: JSON.stringify({ email }) }),
	listClients: () => call<ClientView[]>('/api/admin/clients'),
	getClient: (clientID: string) =>
		call<ClientView>(`/api/admin/clients/${encodeURIComponent(clientID)}`),
	createClient: (client: NewClient) =>
		call<CreatedClient>('/api/admin/clients', { method: 'POST', body: JSON.stringify(client) }),
	updateClient: (clientID: string, client: UpdateClient) =>
		call<ClientView>(`/api/admin/clients/${encodeURIComponent(clientID)}`, {
			method: 'PUT',
			body: JSON.stringify(client)
		}),
	rotateSecret: (clientID: string) =>
		call<{ client_id: string; secret: string }>(
			`/api/admin/clients/${encodeURIComponent(clientID)}/secret`,
			{ method: 'POST' }
		),
	disableClient: (clientID: string) =>
		call<{ disabled: string }>(`/api/admin/clients/${encodeURIComponent(clientID)}/disable`, {
			method: 'POST'
		}),
	enableClient: (clientID: string) =>
		call<{ enabled: string }>(`/api/admin/clients/${encodeURIComponent(clientID)}/enable`, {
			method: 'POST'
		}),
	deleteClient: (clientID: string) =>
		call<{ deleted: string }>(`/api/admin/clients/${encodeURIComponent(clientID)}`, {
			method: 'DELETE'
		}),
	listKeys: () => call<KeyState[]>('/api/admin/keys')
};

/**
 * formatDate renders an API timestamp as a plain calendar day. The API sends
 * UTC and the dashboard is an audit view, so the day is the useful precision.
 */
export function formatDate(value?: string): string {
	if (!value) return '—';
	const parsed = new Date(value);
	if (Number.isNaN(parsed.getTime())) return value;
	return parsed.toISOString().slice(0, 10);
}

/** errorMessage turns anything thrown by admin into a line a human can read. */
export function errorMessage(cause: unknown, fallback: string): string {
	if (cause instanceof AdminError) return cause.message;
	if (cause instanceof Error && cause.message) return cause.message;
	return fallback;
}
