/**
 * The runnable check for `$lib/access`, a script because the client has no
 * test harness:
 *
 *   bun apps/client/src/lib/access.check.ts
 *
 * It exits non-zero on the first disagreement.
 */

import { appIcon, appLogo, matchesUser, relativeTime, toggleRole } from './access';
import type { UserView } from './admin';

let failed = 0;
function expect(label: string, got: unknown, want: unknown) {
	if (JSON.stringify(got) !== JSON.stringify(want)) {
		console.error(`${label}: got ${JSON.stringify(got)}, want ${JSON.stringify(want)}`);
		failed++;
	}
}

expect('toggle adds sorted', toggleRole(['member'], 'admin'), ['admin', 'member']);
expect('toggle removes', toggleRole(['admin', 'member'], 'admin'), ['member']);

const user = (extra: Partial<UserView>): UserView => ({
	id: 'u', email: 'yann@facile.studio', name: 'Yann', admin: false, email_verified: true,
	created_at: '', apps: [], totp: false, ...extra
});
expect('search by name', matchesUser(user({}), 'yan', 'all'), true);
expect('search misses', matchesUser(user({}), 'noah', 'all'), false);
expect('admins tab', matchesUser(user({}), '', 'admins'), false);
expect('disabled tab', matchesUser(user({ disabled_at: '2026-01-01T00:00:00Z' }), '', 'disabled'), true);
expect('active tab hides disabled', matchesUser(user({ disabled_at: '2026-01-01T00:00:00Z' }), '', 'active'), false);

const now = new Date('2026-11-03T12:00:00Z');
expect('never', relativeTime(undefined, now), 'Never');
expect('minutes', relativeTime('2026-11-03T11:55:00Z', now), '5 minutes ago');
expect('one day', relativeTime('2026-11-02T11:00:00Z', now), '1 day ago');

expect('appIcon agenda', appIcon('Agenda'), 'solar:calendar-add-bold-duotone');
expect('appIcon unknown', appIcon('Unknown'), undefined);
expect('appLogo agenda', appLogo('Agenda'), '/logos/agenda.svg');
expect('appLogo unknown', appLogo('Unknown'), undefined);

if (failed) process.exit(1);
console.log('access rules hold');
