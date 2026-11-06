/**
 * The pure half of the Connections deep link: `/account/security?link=discord&return_to=<url>`.
 * Nothing here imports muse, so the check script can run it under bun.
 *
 * The page only reads the two parameters. The server decides whether
 * `return_to` may be used; the page never navigates to a URL the server
 * did not hand back.
 */

const PROVIDER = /^[a-z][a-z0-9_-]{0,31}$/;
const MAX_RETURN_TO = 2048;

/** DeepLink is a provider to link and, optionally, where to go back to afterwards. */
export interface DeepLink {
	provider: string;
	returnTo: string;
}

/** parseDeepLink reads `link` and `return_to`, or answers null when there is nothing to do. */
export function parseDeepLink(params: URLSearchParams): DeepLink | null {
	const provider = (params.get('link') ?? '').toLowerCase();
	if (!PROVIDER.test(provider)) return null;
	const returnTo = params.get('return_to') ?? '';
	return { provider, returnTo: returnTo.length <= MAX_RETURN_TO ? returnTo : '' };
}

/** withoutDeepLink is url with `link` and `return_to` removed and every other parameter kept. */
export function withoutDeepLink(url: URL): URL {
	const clean = new URL(url.href);
	clean.searchParams.delete('link');
	clean.searchParams.delete('return_to');
	return clean;
}
