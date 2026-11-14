<script lang="ts">
	import { Alert, Button, Stack } from '@facile/muse';
	import { page } from '$app/state';
	import { request } from '$lib/account';
	import { t } from '$lib/i18n/i18n.svelte';

	const token = $derived(page.url.searchParams.get('token') ?? '');

	let busy = $state(false);
	let done = $state(false);
	let failed = $state(false);

	/**
	 * unsubscribe spends the link's token on a POST. Opening the link alone
	 * deletes nothing, so a mail scanner that follows it unsubscribes nobody.
	 */
	async function unsubscribe() {
		if (busy) return;
		busy = true;
		failed = false;
		try {
			await request('/api/waitlist/unsubscribe', { token });
			done = true;
		} catch {
			failed = true;
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>{t('waitlist.title')} — Facile</title>
</svelte:head>

<Stack>
	<div class="flex flex-col items-center gap-1.5 text-center">
		<h1 class="font-heading text-2xl font-semibold tracking-tight text-fc-fg">
			{done ? t('waitlist.done') : t('waitlist.title')}
		</h1>
		<p class="text-fc-sm text-fc-fg-muted">{done ? t('waitlist.doneBody') : t('waitlist.body')}</p>
	</div>
	{#if failed || !token}
		<Alert tone="danger" title={t('waitlist.invalid')} />
	{/if}
	{#if !done && token}
		<Button size="lg" onclick={unsubscribe} disabled={busy}>
			{busy ? t('waitlist.working') : t('waitlist.confirm')}
		</Button>
	{/if}
</Stack>
