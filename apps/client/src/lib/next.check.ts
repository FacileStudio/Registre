/**
 * The runnable check for `$lib/next`: bun apps/client/src/lib/next.check.ts
 */
import { invitationPath, safeNext } from './next';

const id = '0b9c1a2e-3f4d-4a5b-8c6d-7e8f9a0b1c2d';
const cases: [string | null, string | null][] = [
	[`/invitations/${id}`, `/invitations/${id}`],
	['//evil.example/invitations/x', null],
	['https://evil.example', null],
	[`/invitations/${id}/accept`, null],
	[`/invitations/${id}?x=1`, null],
	['/invitations/../admin', null],
	['/account', null],
	[null, null]
];
for (const [input, want] of cases) {
	const got = safeNext(input);
	if (got !== want) throw new Error(`safeNext(${input}) = ${got}, want ${want}`);
}
if (invitationPath('x"><script>') !== null) throw new Error('invitationPath kept a non-id');
console.log('next: ok');
