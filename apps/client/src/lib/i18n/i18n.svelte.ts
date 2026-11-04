/**
 * Registre's message runtime: four languages, one flat catalog each, and the
 * current locale as one rune, so every `t(...)` in a template re-renders when
 * the locale changes.
 *
 * The locale is chosen in this order: `?lang=` on the URL (what muse's legal
 * footer and the apps link with), the account's stored locale once it is
 * known, the last choice made on this browser, the browser's languages, then
 * English. It is written to `<html lang>` so assistive technology reads the
 * page in the right language.
 */
import { de } from './de';
import { en, type MessageKey, type Messages } from './en';
import { es } from './es';
import { fr } from './fr';

/** Locale is one of the four languages Registre speaks. */
export type Locale = 'fr' | 'en' | 'es' | 'de';

/** LOCALES lists them in the order pickers show them. */
export const LOCALES: readonly Locale[] = ['fr', 'en', 'es', 'de'];

/** LOCALE_NAMES are each language's own name for itself. */
export const LOCALE_NAMES: Record<Locale, string> = {
	fr: 'Français',
	en: 'English',
	es: 'Español',
	de: 'Deutsch'
};

const catalogs: Record<Locale, Messages> = { fr, en, es, de };
const STORAGE_KEY = 'registre.locale';

/** isLocale narrows a string to a supported locale. */
export function isLocale(value: unknown): value is Locale {
	return typeof value === 'string' && (LOCALES as readonly string[]).includes(value);
}

/** fromTag reduces a BCP 47 tag such as "fr-CA" to a supported locale. */
export function fromTag(tag: string): Locale | null {
	const language = tag.toLowerCase().split('-')[0];
	return isLocale(language) ? language : null;
}

/** detect picks the starting locale from the URL, storage and the browser. */
function detect(): Locale {
	if (typeof window === 'undefined') return 'en';
	const asked = fromTag(new URLSearchParams(window.location.search).get('lang') ?? '');
	if (asked) return asked;
	const stored = readStored();
	if (stored) return stored;
	for (const tag of navigator.languages ?? [navigator.language]) {
		const found = fromTag(tag);
		if (found) return found;
	}
	return 'en';
}

let current = $state<Locale>(detect());

/** locale answers the current locale, reactively. */
export function locale(): Locale {
	return current;
}

/** pinnedByURL reports whether `?lang=` chose the locale, which wins over the account's. */
export function pinnedByURL(): boolean {
	return typeof window !== 'undefined' && new URLSearchParams(window.location.search).has('lang');
}

/** setLocale switches every page to next and remembers it on this browser. */
export function setLocale(next: Locale): void {
	current = next;
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
