/**
 * The runnable check for the redirect allowlist in `$lib/billing`.
 *
 *   bun apps/client/src/lib/billing.check.ts
 *
 * It exits non-zero on the first disagreement.
 */

import { isStripeURL } from './billing';

/** CASES are [URL, expected isStripeURL]. */
const CASES: ReadonlyArray<readonly [string, boolean]> = [
	['https://checkout.stripe.com/c/pay/cs_test', true],
	['https://billing.stripe.com/p/session/x', true],
	['http://checkout.stripe.com/c/pay', false],
	['https://checkout.stripe.com.evil.example/x', false],
	['https://user@checkout.stripe.com/x', false],
	['javascript:alert(1)', false],
	['/spaces', false],
	['', false]
];

let failed = 0;
for (const [url, want] of CASES) {
	const got = isStripeURL(url);
	if (got !== want) {
		console.error(`isStripeURL(${url}) = ${got}, want ${want}`);
		failed++;
	}
}

if (failed > 0) process.exit(1);
console.log(`billing: ${CASES.length} cases agree`);
