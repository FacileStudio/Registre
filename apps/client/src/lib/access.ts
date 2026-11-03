/**
 * Pure rules the admin dashboard applies to people and app access, kept out of
 * the pages so `access.check.ts` can run them without a browser.
 */

import type { AdminEvent, UserView } from './admin';

/** UserFilter is the status tab the users list shows. */
export type UserFilter = 'all' | 'active' | 'admins' | 'disabled';

/** toggleRole adds or removes role, keeping the list sorted and unique. */
export function toggleRole(roles: readonly string[], role: string): string[] {
	const next = new Set(roles);
	if (next.has(role)) next.delete(role);
	else next.add(role);
	return [...next].sort();
}

/** matchesUser reports whether a user belongs in the list for a query and a tab. */
export function matchesUser(user: UserView, query: string, filter: UserFilter): boolean {
	const disabled = Boolean(user.disabled_at);
	if (filter === 'active' && disabled) return false;
	if (filter === 'disabled' && !disabled) return false;
	if (filter === 'admins' && !user.admin) return false;
	const needle = query.trim().toLowerCase();
	return !needle || user.email.toLowerCase().includes(needle) || user.name.toLowerCase().includes(needle);
}

/** displayName is the name a person goes by, or their email when they set none. */
export function displayName(user: Pick<UserView, 'name' | 'email'>): string {
	return user.name.trim() || user.email;
}

const ACTIONS: Record<string, string> = {
	'user.create': 'created the account of',
	'user.update': 'edited',
	'user.password': 'reset the password of',
	'user.disable': 'disabled',
	'user.enable': 'enabled',
	'user.delete': 'deleted',
	'invite.create': 'invited',
	'client.create': 'registered',
	'client.update': 'reconfigured',
	'client.rotate_secret': 'rotated the secret of',
	'client.disable': 'disabled',
	'client.enable': 'enabled',
	'client.delete': 'deleted',
	'client.access_policy': 'changed who can use',
	'grant.set': 'gave app access to',
	'grant.revoke': 'removed app access from'
};

/** describeAction is the verb phrase the activity log shows for an event. */
export function describeAction(event: Pick<AdminEvent, 'action'>): string {
	return ACTIONS[event.action] ?? event.action;
}

/**
 * relativeTime renders how long ago a timestamp was, coarse on purpose: an
 * admin wants "3 days ago", not a second count.
 */
export function relativeTime(value: string | undefined, now: Date = new Date()): string {
	if (!value) return 'Never';
	const then = new Date(value);
	const seconds = Math.round((now.getTime() - then.getTime()) / 1000);
	if (Number.isNaN(seconds)) return value;
	const steps: Array<[number, string]> = [
		[60, 'second'],
		[60, 'minute'],
		[24, 'hour'],
		[30, 'day'],
		[12, 'month']
	];
	let amount = seconds;
	for (const [size, unit] of steps) {
		if (Math.abs(amount) < size) return amount <= 1 && unit === 'second' ? 'Just now' : plural(amount, unit);
		amount = Math.floor(amount / size);
	}
	return plural(amount, 'year');
}

function plural(amount: number, unit: string): string {
	return `${amount} ${unit}${amount === 1 ? '' : 's'} ago`;
}

const LOGOS = new Set([
	'agenda', 'antenne', 'ardoise', 'bulle', 'capsule', 'casier', 'courrier', 'echo', 'journal',
	'mycelium', 'nuage', 'opus', 'perception', 'plume', 'sablier', 'scribe', 'sonde', 'vision'
]);

/** appLogo is the brand glyph of a registered app, found by its name. */
export function appLogo(name: string): string | undefined {
	const slug = name.trim().toLowerCase();
	return LOGOS.has(slug) ? `/logos/${slug}.svg` : undefined;
}
