import { dev, version } from '$app/environment';
import { createDeferredJournal } from '@facile/journal';
import { handleErrorWith } from '@facile/journal/sveltekit';

/**
 * Client-side error reporting into Journal.
 *
 * The configuration comes over HTTP because the client is adapter-static
 * served by the Go binary: there is no environment for the bundle to read, and
 * a baked key would make rotating it a rebuild. GET /api/config is unauthenticated
 * and carries only the public, origin-locked kind of key.
 *
 * createDeferredJournal buffers what throws while that fetch is in flight,
 * which is exactly the window boot errors live in. A loader that returns null
 * or throws leaves the client permanently inert, so an instance with no key
 * configured goes quiet instead of turning an unreachable Journal into an
 * error the page has to handle.
 */
const journal = createDeferredJournal(async () => {
	const config = await fetch('/api/config').then((r) => r.json());
	if (!config.journal?.url || !config.journal?.key) return null;
	return {
		url: config.journal.url,
		key: config.journal.key,
		release: version,
		environment: dev ? 'development' : 'production'
	};
});

journal.install();

/**
 * install() covers window.onerror and unhandledrejection; this covers what
 * SvelteKit swallows before either sees it — a load function that threw, a
 * component that failed to render. Both are needed.
 */
export const handleError = handleErrorWith(journal);
