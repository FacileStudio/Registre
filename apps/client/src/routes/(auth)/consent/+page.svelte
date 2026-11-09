<script lang="ts">
	import { onMount } from 'svelte';
	import { Alert, Button, Stack } from '@facile/muse';
	import TermsCheckbox from '$lib/components/TermsCheckbox.svelte';
	import { request } from '$lib/account';
	import { t } from '$lib/i18n/i18n.svelte';

	let authRequest = $state('');
	let next = $state('/');
	let accepted = $state(false);
	let busy = $state(false);
	let error = $state('');

	onMount(() => {
		const query = new URLSearchParams(window.location.search);
		authRequest = query.get('auth_request') ?? '';
		next = localPath(query.get('next') ?? '');
	});

	/** localPath keeps a same-origin path and refuses anything that could leave the site. */
	function localPath(candidate: string): string {
		return /^\/(?![\/\\])/.test(candidate) ? candidate : '/';
	}

	/**
	 * accept records the acceptance, then resumes the sign-in that was waiting
	 * on it: the server completes the pending app login only now.
	 */
	async function accept() {
		if (!accepted || busy) return;
		busy = true;
		error = '';
		try {
			await request('/api/account/consent', { accept_terms: true });
			if (!authRequest) {
				window.location.assign(next);
				return;
			}
			const response = await fetch('/api/login/session', {
				method: 'POST',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json', 'X-Facile-CSRF': '1' },
				body: JSON.stringify({ auth_request: authRequest })
			});
			const body = await response.json().catch(() => null);
			window.location.assign(response.ok && body?.redirect ? body.redirect : '/');
		} catch {
			error = t('common.failed');
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>{t('consent.title')} — Facile</title>
</svelte:head>

<Stack>
	<div class="flex flex-col items-center gap-1.5 text-center">
		<h1 class="font-heading text-2xl font-semibold tracking-tight text-fc-fg">{t('consent.title')}</h1>
		<p class="text-fc-sm text-fc-fg-muted">{t('consent.body')}</p>
	</div>
	{#if error}
		<Alert tone="danger" title={error} />
	{/if}
	<TermsCheckbox bind:checked={accepted} disabled={busy} />
	<Button onclick={accept} disabled={!accepted || busy}>{busy ? t('common.saving') : t('common.continue')}</Button>
	<a href="/account" class="self-center text-fc-sm text-fc-fg-muted underline underline-offset-2">
		{t('consent.decline')}
	</a>
</Stack>
