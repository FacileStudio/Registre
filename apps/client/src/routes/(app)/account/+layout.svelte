<script lang="ts">
	import { getContext, setContext } from 'svelte';
	import { page } from '$app/state';
	import {
		Alert,
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
		SESSION_CONTEXT,
		type AccountContext,
		type ShellSession,
		type AccountView
	} from '$lib/account';
	import { t } from '$lib/i18n/i18n.svelte';

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
				error = accountError(cause, t('account.loadFailed'));
			}
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		load();
	});

	const shell = getContext<ShellSession | undefined>(SESSION_CONTEXT);

	setContext<AccountContext>(ACCOUNT_CONTEXT, {
		get me() {
			return me;
		},
		refresh: async () => {
			await load();
			shell?.reload();
		}
	});

	const sections = $derived([
		{ id: 'profile', label: t('account.tab.profile'), icon: icons.userCircle, href: '/account' },
		{ id: 'security', label: t('account.tab.security'), icon: icons.shield, href: '/account/security' },
		{ id: 'privacy', label: t('account.tab.privacy'), icon: icons.download, href: '/account/privacy' },
		{ id: 'apps', label: t('account.tab.apps'), icon: icons.plug, href: '/account/apps' }
	]);

	const active = $derived(
		page.url.pathname.startsWith('/account/security')
			? 'security'
			: page.url.pathname.startsWith('/account/privacy')
				? 'privacy'
				: page.url.pathname.startsWith('/account/apps')
					? 'apps'
					: 'profile'
	);
</script>

<svelte:head>
	<title>{t('account.title')} — Facile</title>
</svelte:head>

<Page width="lg">
	<PageHeader
		title={t('account.title')}
		description={t('account.subtitle')}
	/>

	{#if loading}
		<Stack>
			<Skeleton class="h-9 w-64" />
			<Skeleton class="h-40 w-full" />
		</Stack>
	{:else if signedOut}
		<EmptyState
			icon={icons.userCircle}
			title={t('account.signedOut')}
			description={t('account.signedOutHint')}
		>
			<a href="/login" class="text-fc-sm font-medium text-fc-fg underline underline-offset-2">{t('login.title')}</a>
		</EmptyState>
	{:else if error}
		<Alert tone="danger" title={t('account.loadFailed')}>{error}</Alert>
	{:else}
		<Stack>
			<div>
				<Tabs items={sections} value={active} label={t('account.sections')} />
			</div>

			<PageTransition key={active} distance={8} duration={0.25} class="flex flex-col gap-10">
				{@render children()}
			</PageTransition>
		</Stack>
	{/if}
</Page>
