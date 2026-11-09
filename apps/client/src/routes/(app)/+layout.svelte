<script lang="ts">
	import { setContext } from 'svelte';
	import { page } from '$app/state';
	import { MobileNav, PageTransition, SideBar, Spinner, Topbar, icons } from '@facile/muse';
	import { account, SESSION_CONTEXT, type AccountView, type ShellSession } from '$lib/account';
	import { isLocale, pinnedByURL, setLocale, t } from '$lib/i18n/i18n.svelte';

	let { children } = $props();

	let me = $state<AccountView | null>(null);
	let loaded = $state(false);
	let collapsed = $state(false);
	let scroller: HTMLElement | null = $state(null);

	/*
	 * An operator on /admin has no account cookie at all, so a refusal is the
	 * normal case here, not an error worth surfacing: the rail falls back to a
	 * generic label and /account does the real gating. The home page reads the
	 * same answer from SESSION_CONTEXT rather than asking again.
	 */
	function loadMe() {
		account
			.me()
			.then((view) => {
				me = view;
				if (isLocale(view.locale) && !pinnedByURL()) setLocale(view.locale);
			})
			.catch(() => (me = null))
			.finally(() => (loaded = true));
	}

	$effect(loadMe);
	setContext<ShellSession>(SESSION_CONTEXT, {
		get me() {
			return me;
		},
		get loaded() {
			return loaded;
		},
		reload: loadMe
	});

	/*
	 * The home page is the one route a signed-out visitor reaches, and to them
	 * it is a public landing with its own header, not an app with a rail.
	 */
	const bare = $derived(page.url.pathname === '/' && !me);

	$effect(() => {
		if (page.url.pathname) scroller?.scrollTo({ top: 0 });
	});

	const links = $derived([
		{ label: t('nav.apps'), href: '/', icon: icons.home },
		...(me ? [{ label: t('nav.spaces'), href: '/spaces', icon: icons.space }] : []),
		...(me?.admin ? [{ label: t('nav.admin'), href: '/admin', icon: icons.shield }] : [])
	]);

	const navPages = $derived(
		links.map((link) => ({
			...link,
			active: link.href === '/' ? page.url.pathname === '/' : page.url.pathname.startsWith(link.href)
		}))
	);
	const onAccount = $derived(page.url.pathname.startsWith('/account'));
	const user = $derived(
		me ? { name: me.name || me.email, email: me.email, avatar: me.picture || undefined } : undefined
	);
	const userMenu = $derived({
		onSignOut: async () => {
			await account.logout().catch(() => null);
			window.location.assign('/');
		}
	});
	/*
	 * The shell transitions between sections, never within one. Keying on the
	 * full path remounted /admin's layout on every tab switch, which flashed
	 * its operator sign-in and re-ran its checks; sections run their own
	 * inner transition between tabs.
	 */
	const routeKey = $derived(page.url.pathname.split('/')[1] ?? '');
</script>

{#if bare}
	{#if loaded}
		{@render children()}
	{:else}
		<div class="flex min-h-dvh items-center justify-center bg-fc-page">
			<Spinner />
		</div>
	{/if}
{:else}
	<div class="flex h-dvh w-full overflow-hidden bg-fc-page">
		<div class="hidden h-full shrink-0 p-3 md:block">
			<SideBar
				icon={icons.shield}
				title="Facile"
				bind:collapsed
				pages={navPages}
				{user}
				userHref="/account"
				userActive={onAccount}
				{userMenu}
				class="h-full"
			/>
		</div>

		<!-- `min-w-0` so a wide table scrolls itself instead of pushing the shell
		     sideways, and `overscroll-contain` because <main> is the only scroller. -->
		<main bind:this={scroller} class="min-w-0 flex-1 overflow-auto overscroll-contain pb-28 md:pb-0">
			<Topbar class="md:hidden">
				<span class="text-fc-md font-semibold text-fc-fg">Facile</span>
			</Topbar>

			<PageTransition key={routeKey}>
				{@render children()}
			</PageTransition>
		</main>

		<MobileNav items={navPages} {user} profileHref="/account" profileActive={onAccount} {userMenu} />
	</div>
{/if}
