/**
 * Shared spaces, as the signed-in person sees them.
 *
 * A space here is the suite's: Registre owns it and every Facile app mirrors
 * its members and roles, so this module is the only place in the suite where a
 * shared space is edited. Requests go through `$lib/account`, which carries the
 * session cookie and the CSRF header on every write.
 *
 * The rank rules below mirror what the API enforces. They decide what the page
 * offers, never what is allowed: the server refuses anything they get wrong.
 */

import { request, send } from './account';

/** Role is a member's rank in one space, highest first: owner, admin, member. */
export type Role = 'owner' | 'admin' | 'member';

/** ROLES lists every role from the highest rank to the lowest. */
export const ROLES: readonly Role[] = ['owner', 'admin', 'member'];

/** SpaceSummary is one row of the spaces I am in; `role` is my own. */
export interface SpaceSummary {
	id: string;
	name: string;
	role: Role;
	member_count: number;
}

/** SpaceMember is one person in a space. `picture` may be empty. */
export interface SpaceMember {
	user_id: string;
	email: string;
	name: string;
	picture: string;
	role: Role;
}

/** SpaceDetail is one space with its members; `role` is my own. */
export interface SpaceDetail {
	id: string;
	name: string;
	role: Role;
	members: SpaceMember[];
}

const base = (id: string) => `/api/spaces/${encodeURIComponent(id)}`;
const memberPath = (id: string, userID: string) => `${base(id)}/members/${encodeURIComponent(userID)}`;

/** spaces is the account spaces API, one method per route. */
export const spaces = {
	list: () => request<SpaceSummary[]>('/api/spaces'),
	create: (name: string) => request<SpaceSummary>('/api/spaces', { name }),
	get: (id: string) => request<SpaceDetail>(base(id)),
	rename: (id: string, name: string) => request<{ id: string; name: string }>(`${base(id)}/rename`, { name }),
	addMember: (id: string, email: string, role: Role) =>
		request<{ member: SpaceMember }>(`${base(id)}/members`, { email, role }),
	setRole: (id: string, userID: string, role: Role) =>
		request<{ member: SpaceMember }>(`${memberPath(id, userID)}/role`, { role }),
	removeMember: (id: string, userID: string) => send<{ ok: boolean }>(memberPath(id, userID), 'DELETE'),
	leave: (id: string) => request<{ ok: boolean }>(`${base(id)}/leave`, {}),
	remove: (id: string) => send<{ ok: boolean }>(base(id), 'DELETE')
};

/** rank turns a role into a number where a higher rank outranks a lower one. */
export function rank(role: Role): number {
	return ROLES.length - ROLES.indexOf(role);
}

/** canManage reports whether a role may rename a space and edit its members. */
export function canManage(role: Role): boolean {
	return rank(role) >= rank('admin');
}

/** assignable lists the roles someone holding `mine` may grant, highest first. */
export function assignable(mine: Role): Role[] {
	if (!canManage(mine)) return [];
	return ROLES.filter((role) => rank(role) <= rank(mine));
}

/**
 * canEdit reports whether I may change or remove `target`. I need admin or
 * above, `target` must not outrank me, and the last owner is never touched
 * because a space without an owner has nobody left to manage it.
 */
export function canEdit(mine: Role, target: SpaceMember, owners: number): boolean {
	if (!canManage(mine) || rank(target.role) > rank(mine)) return false;
	return !(target.role === 'owner' && owners <= 1);
}

/** ownerCount counts the owners among a space's members. */
export function ownerCount(members: readonly SpaceMember[]): number {
	return members.filter((member) => member.role === 'owner').length;
}

/** ROLE_TONE is the muse Badge tone each role is drawn with. */
export const ROLE_TONE: Record<Role, 'owner' | 'admin' | 'neutral'> = {
	owner: 'owner',
	admin: 'admin',
	member: 'neutral'
};
