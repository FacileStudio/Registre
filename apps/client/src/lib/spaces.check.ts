/**
 * The runnable check for the rank rules in `$lib/spaces`, for the same reason
 * `usercode.check.ts` is a script: the client has no test harness.
 *
 *   bun apps/client/src/lib/spaces.check.ts
 *
 * It exits non-zero on the first disagreement.
 */

import { assignable, canEdit, type Role, type SpaceMember } from './spaces';

/** person builds a member holding `role`; only the role matters to the rules. */
function person(role: Role): SpaceMember {
	return { user_id: role, email: `${role}@example.com`, name: role, picture: '', role };
}

/** CASES are [my role, target role, owners in the space, expected canEdit]. */
const CASES: ReadonlyArray<readonly [Role, Role, number, boolean]> = [
	['owner', 'owner', 2, true],
	['owner', 'owner', 1, false],
	['owner', 'admin', 1, true],
	['admin', 'owner', 2, false],
	['admin', 'admin', 1, true],
	['admin', 'member', 1, true],
	['member', 'member', 1, false]
];

let failed = 0;
for (const [mine, target, owners, want] of CASES) {
	const got = canEdit(mine, person(target), owners);
	if (got !== want) {
		console.error(`canEdit(${mine}, ${target}, ${owners}) = ${got}, want ${want}`);
		failed++;
	}
}

const grants: ReadonlyArray<readonly [Role, string]> = [
	['owner', 'owner,admin,member'],
	['admin', 'admin,member'],
	['member', '']
];
for (const [mine, want] of grants) {
	const got = assignable(mine).join(',');
	if (got !== want) {
		console.error(`assignable(${mine}) = ${got}, want ${want}`);
		failed++;
	}
}

if (failed > 0) process.exit(1);
console.log(`spaces: ${CASES.length + grants.length} cases agree`);
