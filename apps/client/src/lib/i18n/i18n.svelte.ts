/**
 * Registre's message runtime: four languages, one flat catalog each, and the
 * current locale as one rune, so every `t(...)` in a template re-renders when
 * the locale changes.
 *
 * The locale is chosen in this order: `?lang=` on the URL (what muse's legal
 * footer and the apps link with), the account's stored locale once it is
 * known, the last choice made on this browser, the `ui_locales` of an app's
 * sign-in request, the browser's languages, then English (see `./detect`).
 * `?lang=` and `ui_locales` are hints for the page in view: neither is
 * remembered, and neither becomes a new account's language. The locale is
 * written to `<html lang>` so assistive technology reads the page in the
 * right language.
 */
import { de } from './de';
import { LOCALES, displayLocale, isLocale, ownLocale, type Locale, type Signals } from './detect';
import { en, type MessageKey, type Messages } from './en';
import { es } from './es';
import { fr } from './fr';

/** LOCALE_NAMES are each language's own name for itself. */
export const LOCALE_NAMES: Record<Locale, string> = {
	fr: 'Français',
	en: 'English',
	es: 'Español',
	de: 'Deutsch'
};

export { LOCALES, isLocale, type Locale };

const catalogs: Record<Locale, Messages> = { fr, en, es, de };
const STORAGE_KEY = 'registre.locale';

/** signals reads what the URL, this browser's storage and the browser itself say. */
function signals(): Signals {
	if (typeof window === 'undefined') return { lang: null, uiLocales: null, chosen, languages: [] };
	const query = new URLSearchParams(window.location.search);
	return {
		lang: query.get('lang'),
		uiLocales: query.get('ui_locales'),
		chosen,
		languages: navigator.languages ?? [navigator.language]
	};
}

let chosen: Locale | null = readStored();
let current = $state<Locale>(displayLocale(signals()));
if (typeof document !== 'undefined') document.documentElement.lang = current;

/** locale answers the current locale, reactively. */
export function locale(): Locale {
	return current;
}

/**
 * accountLocale is the language a new account is created with: the person's
 * own choice or their browser's, never the `?lang=` a link carried.
 */
export function accountLocale(): Locale {
	return ownLocale(signals());
}

/** pinnedByURL reports whether `?lang=` chose the locale, which wins over the account's. */
export function pinnedByURL(): boolean {
	return typeof window !== 'undefined' && new URLSearchParams(window.location.search).has('lang');
}

/** setLocale switches every page to next and remembers it on this browser. */
export function setLocale(next: Locale): void {
	current = next;
	chosen = next;
	if (typeof document !== 'undefined') document.documentElement.lang = next;
	try {
		localStorage.setItem(STORAGE_KEY, next);
	} catch {
		return;
	}
}

/** readStored answers the locale chosen last on this browser, when storage allows it. */
function readStored(): Locale | null {
	try {
		const stored = localStorage.getItem(STORAGE_KEY);
		return isLocale(stored) ? stored : null;
	} catch {
		return null;
	}
}

/**
 * t answers the message for key in the current locale, with every `{name}`
 * replaced by vars.name. A key missing from a catalog cannot compile, so the
 * English fallback only ever covers a value left empty.
 */
export function t(key: MessageKey, vars?: Record<string, string | number>): string {
	let text = catalogs[current][key] || en[key];
	if (vars) {
		for (const [name, value] of Object.entries(vars)) text = text.replaceAll(`{${name}}`, String(value));
	}
	return text;
}
