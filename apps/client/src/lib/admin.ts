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

export interface ClientView {
	client_id: string;
	subject: string;
	name: string;
	kind: string;
	grant_types?: string[];
	scopes?: string[];
	audiences?: string[];
	created_at: string;
	disabled_at?: string;
}

export interface InviteView {
	email: string;
	created_by: string;
	expires_at: string;
	created_at: string;
	redeemed: boolean;
}

export interface KeyState {
	id: string;
	algorithm: string;
	state: string;
	created_at: string;
	activated_at?: string;
	retired_at?: string;
	expires_at?: string;
}

const encoder = new TextEncoder();

export function authHeader(email: string, password: string): string {
	return 'Basic ' + btoa(String.fromCharCode(...encoder.encode(`${email}:${password}`)));
}
