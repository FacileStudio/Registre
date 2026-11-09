<script lang="ts">
	import { Alert, Button, Field, Input, Stack } from '@facile/muse';
	import { t } from '$lib/i18n/i18n.svelte';

	let email = $state('');
	let sent = $state(false);
	let busy = $state(false);

	/*
	 * There is no failure branch and no "no account found" branch, by design.
	 * SPEC §7: same response whatever the address, or the form becomes an
	 * account-existence oracle. Even a network failure resolves to the same
	 * confirmation, because a retry costs nothing and a distinguishable error
	 * is the leak.
	 */
	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (busy) return;
		busy = true;
		try {
			await fetch('/api/recover', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ email: email.trim() })
			}).catch(() => undefined);
			sent = true;
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>{t('recover.title')} — Facile</title>
</svelte:head>

<Stack>
	<div class="flex flex-col items-center gap-1.5 text-center">
		<h1 class="font-heading text-2xl font-semibold tracking-tight text-fc-fg">
			{t('recover.title')}
		</h1>
		<p class="text-fc-sm text-fc-fg-muted">
			{t('recover.subtitle')}
		</p>
	</div>

	{#if sent}
		<Alert tone="info" title={t('recover.sent')} />
	{:else}
		<form onsubmit={submit} class="flex flex-col gap-4">
			<Field label={t('common.email')}>
				<Input
					type="email"
					bind:value={email}
					placeholder="you@example.com"
					required
					autocomplete="username"
					disabled={busy}
				/>
			</Field>

			<Button type="submit" variant="primary" size="lg" disabled={busy || !email.trim()} class="w-full">
				{busy ? t('common.sending') : t('recover.send')}
			</Button>
		</form>
	{/if}

	<p class="text-center text-fc-sm text-fc-fg-muted">
		<a href="/login" class="font-medium text-fc-fg underline underline-offset-2 hover:underline">
			{t('recover.back')}
		</a>
	</p>
</Stack>
