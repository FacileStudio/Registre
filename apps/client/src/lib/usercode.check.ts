/**
 * The runnable check for `$lib/usercode`, and the reason it is a script.
 *
 * This client has no test harness: no vitest, no runner in package.json, no
 * test file anywhere under `src`. Adding one for two pure functions would be a
 * new dependency and a new gate to keep green, so this is the smallest thing
 * that fails if the logic breaks, and it needs nothing that is not already
 * installed.
 *
 *   bun apps/client/src/lib/usercode.check.ts
 *
 * It exits non-zero on the first disagreement. If a real harness ever arrives,
 * these cases move into it and this file goes.
 */

import { CODE_LENGTH, inspect, normalise } from './usercode';

/** CASES are the shapes a human actually produces, and what each must become. */
const CASES: ReadonlyArray<readonly [string, string]> = [
	['f28h-2j4k', 'FHJK'],
	[' bcdf ghjk ', 'BCDF-GHJK'],
	['bcdfghjk', 'BCDF-GHJK'],
	['BCDF-GHJK', 'BCDF-GHJK'],
	['bcdf.ghjk', 'BCDF-GHJK'],
	['BCDFGHJKLMNP', 'BCDF-GHJK'],
	['aeiou', ''],
	['', '']
];

/** fail reports one disagreement and stops, so the first failure is the one read. */
function fail(what: string): never {
	console.error(`usercode: ${what}`);
	process.exit(1);
}

for (const [raw, want] of CASES) {
	const got = normalise(raw);
	if (got !== want) fail(`normalise(${JSON.stringify(raw)}) = ${JSON.stringify(got)}, want ${want}`);
}

const pasted = inspect('f28h-2j4k');
if (!pasted.stray) fail('inspect did not notice the digits in f28h-2j4k');
if (pasted.complete) fail('inspect called a four-letter remainder complete');
if (pasted.kept !== 4) fail(`inspect kept ${pasted.kept} of f28h-2j4k, want 4`);

const spaced = inspect(' bcdf.ghjk ');
if (spaced.stray) fail('inspect treated a separator as a stray character');
if (!spaced.complete) fail('inspect did not call a whole code complete');
if (spaced.kept !== CODE_LENGTH) fail(`inspect kept ${spaced.kept} of a whole code`);

console.log(`usercode: ${CASES.length + 6} checks passed`);
