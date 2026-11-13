<script lang="ts">
	import { Alert, Button, Stack } from '@facile/muse';
	import { account } from '$lib/account';
	import { t } from '$lib/i18n/i18n.svelte';

	let busy = $state(false);
	let error = $state('');

	/**
	 * signOut ends the session with the CSRF-protected POST and goes home. The
	 * server shows this page instead of signing out when the link that led here
	 * came from outside the suite.
	 */
	async function signOut() {
		if (busy) return;
		busy = true;
		error = '';
		try {
			await account.logout();
			window.location.assign('/');
		} catch {
			error = t('account.signOutFailed');
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>{t('logout.title')} — Facile</title>
</svelte:head>

<Stack>
	<div class="flex flex-col items-center gap-1.5 text-center">
		<h1 class="font-heading text-2xl font-semibold tracking-tight text-fc-fg">{t('logout.title')}</h1>
		<p class="text-fc-sm text-fc-fg-muted">{t('logout.body')}</p>
	</div>
	{#if error}
		<Alert tone="danger" title={error} />
	{/if}
	<Button size="lg" onclick={signOut} disabled={busy}>{busy ? t('account.signingOut') : t('account.signOut')}</Button>
	<a href="/" class="self-center text-fc-sm text-fc-fg-muted underline underline-offset-2">{t('logout.stay')}</a>
</Stack>
