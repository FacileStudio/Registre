/**
 * The runnable check for `$lib/i18n/detect`: bun apps/client/src/lib/i18n/detect.check.ts
 */
import { displayLocale, ownLocale, type Signals } from './detect';

const none: Signals = { lang: null, uiLocales: null, chosen: null, languages: [] };
const cases: [string, Partial<Signals>, string, string][] = [
	['nothing known', {}, 'en', 'en'],
	['browser only', { languages: ['en-GB', 'en'] }, 'en', 'en'],
	['unsupported browser language first', { languages: ['it-IT', 'de-AT'] }, 'de', 'de'],
	['a link in French, a British browser', { lang: 'fr', languages: ['en-GB'] }, 'fr', 'en'],
	['a link in French, nothing else', { lang: 'fr' }, 'fr', 'en'],
	['a link beats the choice for display only', { lang: 'fr', chosen: 'de', languages: ['en-GB'] }, 'fr', 'de'],
	['the choice beats ui_locales and the browser', { uiLocales: 'fr', chosen: 'es', languages: ['en-GB'] }, 'es', 'es'],
	['ui_locales beats the browser for display only', { uiLocales: 'fr', languages: ['en-GB'] }, 'fr', 'en'],
	['ui_locales is a ranked list', { uiLocales: 'it de-CH fr', languages: ['en-GB'] }, 'de', 'en'],
	['an unknown link language is ignored', { lang: 'xx', uiLocales: 'zz', languages: ['es-ES'] }, 'es', 'es'],
	['a corrupt stored choice is ignored', { chosen: 'xx', languages: ['fr-FR'] }, 'fr', 'fr']
];
for (const [name, signals, shown, stored] of cases) {
	const all = { ...none, ...signals };
	const got = [displayLocale(all), ownLocale(all)];
	if (got[0] !== shown || got[1] !== stored) {
		throw new Error(`${name}: display ${got[0]}, account ${got[1]}; want ${shown}, ${stored}`);
	}
}
console.log('detect.check: ok');
