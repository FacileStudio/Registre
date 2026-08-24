<script lang="ts">
	import { page } from '$app/state';
	import { MobileNav, PageTransition, SideBar, Topbar, icons } from '@facile/muse';
	import { account, type AccountView } from '$lib/account';

	let { children } = $props();

	let me = $state<AccountView | null>(null);
	let collapsed = $state(false);
	let scroller: HTMLElement | null = $state(null);

	/*
	 * The rail's user card is the only thing this needs, and an operator on
	 * /admin has no account cookie at all. A refusal is therefore the normal
	 * case here, not an error worth surfacing: the card falls back to a
	 * generic label and the /account route does the real gating.
	 */
	$effect(() => {
		account
			.me()
			.then((view) => (me = view))
			.catch(() => (me = null));
	});

	$effect(() => {
		if (page.url.pathname) scroller?.scrollTo({ top: 0 });
	});

	const links = [{ label: 'Administration', href: '/admin', icon: icons.shield }];

	const navPages = $derived(
		links.map((link) => ({ ...link, active: page.url.pathname.startsWith(link.href) }))
	);
	const onAccount = $derived(page.url.pathname.startsWith('/account'));
	const user = $derived.by(() => ({ name: me?.name || me?.email || 'Account' }));
	const routeKey = $derived(onAccount ? '/account' : page.url.pathname);
</script>

<div class="flex h-dvh w-full overflow-hidden bg-fc-page">
	<div class="hidden h-full shrink-0 p-3 md:block">
		<SideBar
			icon={icons.shield}
			title="Registre"
			bind:collapsed
			pages={navPages}
			{user}
			userHref="/account"
			userActive={onAccount}
			class="h-full"
		/>
	</div>

	<!-- `min-w-0` so a wide table scrolls itself instead of pushing the shell
	     sideways, and `overscroll-contain` because <main> is the only scroller. -->
	<main bind:this={scroller} class="min-w-0 flex-1 overflow-auto overscroll-contain pb-28 md:pb-0">
		<Topbar class="md:hidden">
			<span class="text-fc-md font-semibold text-fc-fg">Registre</span>
		</Topbar>

		<PageTransition key={routeKey}>
			{@render children()}
		</PageTransition>
	</main>

	<MobileNav items={navPages} {user} profileHref="/account" profileActive={onAccount} />
</div>
