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
import { parseDeepLink, withoutDeepLink } from './social-link';
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

for (const [name, catalog] of Object.entries({ en, fr, es, de })) {
	expect('login.discord' in catalog, `${name} has no login.discord`);
	expect('login.github' in catalog, `${name} has no login.github`);
	expect('connections.discord' in catalog, `${name} has no connections.discord`);
	expect('connections.github' in catalog, `${name} has no connections.github`);
}

expect(reasonKey('nonsense') === null, 'an unknown reason must map to nothing');
expect(
	startURL('discord', 'a b&c') === '/api/login/social/discord/start?auth_request=a%20b%26c',
	'startURL must escape the auth request'
);
expect(
	startURL('github', 'a b&c') === '/api/login/social/github/start?auth_request=a%20b%26c',
	'startURL must escape the auth request for github'
);

const deep = parseDeepLink(new URL('https://x.test/account/security?link=Discord&return_to=https%3A%2F%2Fplace.facile.studio%2Fa%3Fb%3D1').searchParams);
expect(deep?.provider === 'discord', 'the provider must be lowercased');
expect(deep?.returnTo === 'https://place.facile.studio/a?b=1', 'return_to must be decoded once');
const deepGithub = parseDeepLink(new URL('https://x.test/account/security?link=GitHub').searchParams);
expect(deepGithub?.provider === 'github', 'github provider must be lowercased');
expect(parseDeepLink(new URLSearchParams('link=discord'))?.returnTo === '', 'return_to is optional');
expect(parseDeepLink(new URLSearchParams('link=github'))?.returnTo === '', 'return_to is optional for github');
expect(parseDeepLink(new URLSearchParams('return_to=https%3A%2F%2Fa.test')) === null, 'no link, no deep link');
expect(parseDeepLink(new URLSearchParams('link=../x')) === null, 'a provider must be a plain name');
expect(parseDeepLink(new URLSearchParams(`link=discord&return_to=${'a'.repeat(3000)}`))?.returnTo === '', 'an oversized return_to is dropped');
expect(
	withoutDeepLink(new URL('https://x.test/account/security?link=discord&return_to=u&connection=linked#top')).href ===
		'https://x.test/account/security?connection=linked#top',
	'withoutDeepLink must keep other parameters and the hash'
);

if (failures > 0) process.exit(1);
console.log('social.check: ok');
