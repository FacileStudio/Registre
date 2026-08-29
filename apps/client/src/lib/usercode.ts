/**
 * The user code's shape, and the only place the client states it.
 *
 * It is a companion to `$lib/device`, which owns the three verification
 * endpoints. This module owns the string those endpoints are given, because the
 * page has to know what a code can contain before it sends one.
 *
 * The reason is that the API is deliberately forgiving in a way that reads as
 * silence. `devices.Normalise` drops every character outside the alphabet and
 * looks the remainder up, so a code from a different tool does not fail loudly:
 * `F28H-2J4K` becomes `FHJK`, matches nothing, and comes back as the same 404
 * an expired code gets. Canonicalising in the browser is what lets the form say
 * what happened instead of sending a request that was never going to work.
 *
 * It is a module rather than three constants inside the form because the form
 * is no longer the only caller. The page canonicalises a code arriving from a
 * link or from sessionStorage, neither of which the form has touched, and two
 * alphabets that could disagree is the failure this whole surface exists to
 * remove.
 *
 * Everything here is pure and synchronous. Nothing in this module may reach the
 * network or the DOM, so the form can call it on every keystroke.
 */

/**
 * ALPHABET, CODE_LENGTH and GROUP_SIZE are the shape itself, copied from the
 * constants of the same names in `apps/api/internal/devices`. That package's
 * doc carries the reasoning: twenty letters, no digits so 0/O and 1/I cannot be
 * misread off a terminal, and no vowels so no code can spell a word.
 *
 * A copy rather than something fetched at boot, because this is a constant of
 * the protocol and not configuration. zitadel is configured from those same Go
 * constants, so registre can only ever mint codes of this shape, and asking it
 * over the network what its own alphabet is would be a round trip to learn
 * something that cannot change between deploys. If the Go constants move, these
 * move in the same commit.
 */
export const ALPHABET = 'BCDFGHJKLMNPQRSTVWXZ';

/** CODE_LENGTH is how many alphabet characters a whole code has. */
export const CODE_LENGTH = 8;

/** GROUP_SIZE is how many characters sit between the dashes, for reading. */
export const GROUP_SIZE = 4;

/**
 * CodeShape is what one piece of raw input amounts to: the canonical code it
 * became, how much of a code that is, and whether anything was thrown away to
 * get there.
 *
 * `stray` is the field a form actually needs, and it is reported here because
 * it cannot be recovered afterwards. A field that has already been
 * canonicalised looks identical whether the human typed four letters or pasted
 * eight letters and four digits, and those two want opposite messages.
 */
export interface CodeShape {
	readonly code: string;
	readonly kept: number;
	readonly complete: boolean;
	readonly stray: boolean;
}

/**
 * isSeparator reports whether a character is spacing rather than a mistake.
 *
 * The three are the ones `devices.Normalise`'s own doc names as shapes a human
 * types: `bcdf ghjk`, `BCDF-GHJK` and `bcdf.ghjk`. They are how a code is
 * written down, printed and read aloud, so nobody should be told off for typing
 * one. Everything else outside the alphabet is a character the human did not
 * get from registre.
 */
function isSeparator(char: string): boolean {
	return char === '-' || char === '.' || /\s/.test(char);
}

/**
 * normalise turns anything a human typed or pasted into the one form registre
 * stores and looks codes up in, then regroups it for reading.
 *
 * It is `devices.Normalise` plus the grouping and the length cap: upper case,
 * keep only what the alphabet has, stop at CODE_LENGTH, and put a dash every
 * GROUP_SIZE. So `f28h-2j4k` becomes `FHJK` and ` bcdf ghjk ` becomes
 * `BCDF-GHJK`, which is what the server would have matched either way.
 *
 * Inserting the dash rather than expecting it is the point of the grouping.
 * The code is displayed as `CCJV-VMTH`, and a field that took that literally
 * made approving a sign-in from a phone mean switching to the symbol keyboard
 * for a character the lookup throws away.
 *
 * Dropping rather than rejecting is deliberate, and it is the Go's reason as
 * much as this one: no lookup key can then carry a wildcard, a quote or a
 * percent sign into whatever reads it next. The visible cost is that a code
 * from another tool shrinks instead of erroring, and `inspect` is how a caller
 * notices.
 */
export function normalise(raw: string): string {
	let kept = '';
	for (const char of raw.toUpperCase()) {
		if (kept.length === CODE_LENGTH) break;
		if (ALPHABET.includes(char)) kept += char;
	}
	const groups: string[] = [];
	for (let at = 0; at < kept.length; at += GROUP_SIZE) {
		groups.push(kept.slice(at, at + GROUP_SIZE));
	}
	return groups.join('-');
}

/**
 * inspect canonicalises raw input and reports what it cost, so a caller can
 * tell an unfinished code from one that was never a registre code at all.
 *
 * Call it on the raw value, never on a value already through `normalise`.
 * Canonicalising is lossy by design, and `stray` is the part of the loss a
 * human needs explained.
 */
export function inspect(raw: string): CodeShape {
	const code = normalise(raw);
	const kept = code.split('-').join('').length;
	const stray = [...raw.toUpperCase()].some(
		(char) => !ALPHABET.includes(char) && !isSeparator(char)
	);
	return { code, kept, complete: kept === CODE_LENGTH, stray };
}
