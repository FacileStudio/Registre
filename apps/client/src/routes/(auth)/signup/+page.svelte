<script lang="ts">
	import { onMount } from 'svelte';
	import { Alert, Stack } from '@facile/muse';
	import SignupRequest from '$lib/components/SignupRequest.svelte';
	import { t } from '$lib/i18n/i18n.svelte';
	import { invitationPath, rememberNext } from '$lib/next';

	let authRequest = $state('');
	let invitation = $state('');
	let closed = $state(false);

	onMount(async () => {
		const query = new URLSearchParams(window.location.search);
		authRequest = query.get('auth_request') ?? '';
		invitation = invitationPath(query.get('invitation')) ? (query.get('invitation') ?? '') : '';
		rememberNext(invitationPath(invitation));
		const config = await fetch('/api/config')
			.then((r) => r.json())
			.catch(() => null);
		if (config && config.signup !== true) closed = true;
	});
</script>

<svelte:head>
	<title>{t('signup.title')} — Facile</title>
</svelte:head>

<Stack>
	<div class="flex flex-col items-center gap-1.5 text-center">
		<h1 class="font-heading text-2xl font-semibold tracking-tight text-fc-fg">{t('signup.title')}</h1>
		<p class="text-fc-sm text-fc-fg-muted">{t('signup.subtitle')}</p>
	</div>

	{#if closed}
		<Alert tone="info" title={t('signup.closed')} />
	{:else}
		<SignupRequest {authRequest} {invitation} />
	{/if}

	<p class="text-center text-fc-sm text-fc-fg-muted">
		{t('signup.haveAccount')}
		<a href="/login" class="font-medium text-fc-fg underline underline-offset-2">{t('login.title')}</a>
	</p>
</Stack>
