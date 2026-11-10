/**
 * Where to land after signing in or signing up, when that is a space
 * invitation's page rather than the home page.
 *
 * Only an invitation path is accepted. Anything else on `?next=` is dropped,
 * so the parameter cannot send a freshly signed-in browser to a page of a
 * stranger's choosing. It is kept in sessionStorage because a social sign-in
 * leaves this origin and comes back to `/`, and the tab is the same tab.
 */

const KEY = 'registre.next';
const INVITATION_ID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

/** invitationPath is the page of one invitation, or null for anything that is not an id. */
export function invitationPath(id: string | null | undefined): string | null {
	return id && INVITATION_ID.test(id) ? `/invitations/${id}` : null;
}

/** safeNext keeps `path` only when it is an invitation page. */
export function safeNext(path: string | null | undefined): string | null {
	const match = /^\/invitations\/([^/?#]+)$/.exec(path ?? '');
	return match ? invitationPath(match[1]) : null;
}

/** rememberNext keeps a safe landing page for this tab. */
export function rememberNext(path: string | null | undefined): void {
	const safe = safeNext(path);
	if (!safe) return;
	try {
		sessionStorage.setItem(KEY, safe);
	} catch {
		return;
	}
}

/** takeNext answers the remembered landing page once, then forgets it. */
export function takeNext(): string | null {
	try {
		const kept = safeNext(sessionStorage.getItem(KEY));
		sessionStorage.removeItem(KEY);
		return kept;
	} catch {
		return null;
	}
}

/** landing replaces the home page with the remembered landing page. Any other target, such as an app callback or the consent page, is kept. */
export function landing(target: string): string {
	if (target !== '/') return target;
	return takeNext() ?? target;
}
