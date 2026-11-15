/**
 * Which language a page shows and which one a new account gets. Kept free of
 * runes so `bun apps/client/src/lib/i18n/detect.check.ts` can run it.
 */

/** Locale is one of the four languages Registre speaks. */
export type Locale = 'fr' | 'en' | 'es' | 'de';

/** LOCALES lists them in the order pickers show them. */
export const LOCALES: readonly Locale[] = ['fr', 'en', 'es', 'de'];

/** isLocale narrows a string to a supported locale. */
export function isLocale(value: unknown): value is Locale {
	return typeof value === 'string' && (LOCALES as readonly string[]).includes(value);
}

/** fromTag reduces a BCP 47 tag such as "fr-CA" to a supported locale. */
export function fromTag(tag: string): Locale | null {
	const language = tag.toLowerCase().split('-')[0];
	return isLocale(language) ? language : null;
}

/**
 * Signals are everything that can suggest a language: `?lang=` and
 * `?ui_locales=` on the URL, the choice the person made on this browser, and
 * the browser's own languages.
 */
export interface Signals {
	lang: string | null;
	uiLocales: string | null;
	chosen: string | null;
	languages: readonly string[];
}

/** firstOf answers the first supported locale among tags. */
function firstOf(tags: readonly string[]): Locale | null {
	for (const tag of tags) {
		const found = fromTag(tag);
		if (found) return found;
	}
	return null;
}

/**
 * ownLocale is the language that belongs to the person: what they chose,
 * else what their browser asks for, else English. A link's `?lang=` or
 * `ui_locales` never counts, so sign-up stores this one on the account.
 */
export function ownLocale(signals: Pick<Signals, 'chosen' | 'languages'>): Locale {
	return firstOf([signals.chosen ?? '']) ?? firstOf(signals.languages) ?? 'en';
}

/**
 * displayLocale is the language a page opens in: `?lang=` for this view,
 * then the person's choice, then the `ui_locales` an app sent with its
 * sign-in request, then the browser, then English.
 */
export function displayLocale(signals: Signals): Locale {
	return (
		firstOf([signals.lang ?? '']) ??
		firstOf([signals.chosen ?? '']) ??
		firstOf((signals.uiLocales ?? '').split(/\s+/)) ??
		ownLocale(signals)
	);
}
