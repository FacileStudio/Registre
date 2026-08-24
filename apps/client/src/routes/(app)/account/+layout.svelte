<script lang="ts">
	import { setContext } from 'svelte';
	import { page } from '$app/state';
	import {
		Alert,
		Divider,
		EmptyState,
		Page,
		PageHeader,
		PageTransition,
		Skeleton,
		Stack,
		Tabs,
		icons
	} from '@facile/muse';
	import {
		account,
		accountError,
		isUnauthorized,
		ACCOUNT_CONTEXT,
		type AccountContext,
		type AccountView
	} from '$lib/account';

	let { children } = $props();

	let me = $state<AccountView | null>(null);
	let loading = $state(true);
	let signedOut = $state(false);
	let error = $state('');

	async function load() {
		loading = me === null;
		try {
			me = await account.me();
			signedOut = false;
			error = '';
		} catch (cause) {
			if (isUnauthorized(cause)) {
				signedOut = true;
				me = null;
			} else {
				error = accountError(cause, 'Could not load your account.');
			}
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		load();
	});

	setContext<AccountContext>(ACCOUNT_CONTEXT, {
		get me() {
			return me;
		},
		refresh: load
	});

	const sections = [
		{ id: 'profile', label: 'Profile', icon: icons.userCircle, href: '/account' },
		{ id: 'security', label: 'Security', icon: icons.shield, href: '/account/security' }
	];

	const active = $derived(page.url.pathname.startsWith('/account/security') ? 'security' : 'profile');
</script>

<svelte:head>
	<title>Your account — Registre</title>
</svelte:head>

<Page width="lg">
	<PageHeader
		title="Your account"
		description="Your sign-in details for every app in the Facile Suite."
	/>

	{#if loading}
		<Stack>
			<Skeleton class="h-9 w-64" />
			<Skeleton class="h-40 w-full" />
		</Stack>
	{:else if signedOut}
		<EmptyState
			icon={icons.userCircle}
			title="You are not signed in"
			description="Registre only knows you through the app that sent you here. Open a Facile app, sign in there, and come back."
		/>
	{:else if error}
		<Alert tone="danger" title="Could not load your account">{error}</Alert>
	{:else}
		<Stack>
			<div class="flex flex-col gap-4">
				<Tabs items={sections} value={active} label="Account sections" />
				<Divider />
			</div>

			<PageTransition key={active} distance={8} duration={0.25} class="flex flex-col gap-10">
				{@render children()}
			</PageTransition>
		</Stack>
	{/if}
</Page>
