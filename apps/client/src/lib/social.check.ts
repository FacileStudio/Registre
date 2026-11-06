/**
 * The runnable check for `$lib/social-reasons`: the server's reasons must each map to
 * a catalog key, and every key must exist in all four catalogs.
 *
 *   bun apps/client/src/lib/social.check.ts
 */

import { de } from './i18n/de';
import { en } from './i18n/en';
import { es } from './i18n/es';
import { fr } from './i18n/fr';
import { REASONS, reasonKey, startURL } from './social-reasons';

const PLAIN_REASON = /^[a-z][a-z ,.'-]{2,79}$/i;
let failures = 0;

function expect(ok: boolean, message: string) {
	if (!ok) {
		failures++;
		console.error(`FAIL: ${message}`);
	}
}

for (const [reason, key] of Object.entries(REASONS)) {
	expect(PLAIN_REASON.test(reason), `"${reason}" would be refused by the login page's own guard`);
	for (const [name, catalog] of Object.entries({ en, fr, es, de })) {
		expect(key in catalog, `${name} has no "${key}"`);
	}
}
expect(reasonKey('nonsense') === null, 'an unknown reason must map to nothing');
expect(
	startURL('discord', 'a b&c') === '/api/login/social/discord/start?auth_request=a%20b%26c',
	'startURL must escape the auth request'
);

if (failures > 0) process.exit(1);
console.log('social.check: ok');
