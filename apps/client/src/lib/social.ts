/**
 * Social sign-in on the login page: which buttons to show, where a button
 * goes, and how a refusal the server sent back is explained.
 *
 * The sign-in itself is a full-page navigation to /api/login/social/…/start,
 * not a fetch: the provider's consent page is a document, and the callback
 * ends on a redirect the browser has to follow.
 */
import type { MessageKey } from '$lib/i18n/en';
import { DISCORD_ICON, GITHUB_ICON } from '$lib/icons';

export { reasonKey, startURL } from '$lib/social-reasons';

/** SocialProvider is how the login page draws one provider's button. */
export interface SocialProvider {
	name: string;
	label: MessageKey;
	title: MessageKey;
	icon: string;
}

/** PROVIDERS are the providers the page knows how to draw. */
export const PROVIDERS: Record<string, SocialProvider> = {
	discord: { name: 'discord', label: 'login.discord', title: 'connections.discord', icon: DISCORD_ICON },
	github: { name: 'github', label: 'login.github', title: 'connections.github', icon: GITHUB_ICON }
};

/**
 * loadProviders asks which buttons the login for authRequest shows. It answers
 * the providers this page can draw and nothing else, and an empty list on any
 * failure: a missing button must never block the password form.
 */
export async function loadProviders(authRequest: string): Promise<SocialProvider[]> {
	if (!authRequest) return [];
	try {
		const response = await fetch(`/api/login/providers?auth_request=${encodeURIComponent(authRequest)}`);
		const body = await response.json();
		const names: unknown = body?.providers;
		if (!Array.isArray(names)) return [];
		return names.filter((n): n is string => typeof n === 'string' && n in PROVIDERS).map((n) => PROVIDERS[n]);
	} catch {
		return [];
	}
}
