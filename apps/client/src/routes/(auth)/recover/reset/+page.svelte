<script lang="ts">
	import { Alert, Button, Field, Input, Stack } from '@facile/muse';
	import { t } from '$lib/i18n/i18n.svelte';
	import { MIN_PASSWORD } from '$lib/policy';

	let token = $state('');
	let password = $state('');
	let confirm = $state('');
	let error = $state('');
	let done = $state(false);
	let busy = $state(false);

	/**
	 * Read the token out of the query string, then take it back out of the
	 * address bar.
	 *
	 * The token is a credential. Left in the URL it goes into browser history,
	 * into the Referer of anything this page loads, and onto the screen of
	 * whoever is looking over the shoulder of someone resetting a password. It
	 * has to arrive this way because it arrives in an email, so the next best
	 * thing is for it not to stay. replaceState keeps it out of history rather
	 * than adding a second entry.
	 */
	$effect(() => {
		const query = new URLSearchParams(window.location.search);
		const fromURL = query.get('token');
		if (!fromURL) return;
		token = fromURL;
		history.replaceState(history.state, '', window.location.pathname);
	});

	const tooShort = $derived(password.length > 0 && [...password].length < MIN_PASSWORD);
	const mismatch = $derived(confirm.length > 0 && confirm !== password);

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (busy) return;
		if (password !== confirm) {
			error = t('password.mismatch');
			return;
		}
		busy = true;
		error = '';
		try {
			const response = await fetch('/api/recover/reset', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ token, password })
			});
			if (!response.ok) {
				const answer = await response.json().catch(() => null);
				error = answer?.error === 'password_breached' ? t('password.breached') : t('reset.used');
				return;
			}
			done = true;
		} catch {
			error = t('common.failed');
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>{t('reset.title')} — Facile</title>
</svelte:head>

<Stack>
	<div class="flex flex-col items-center gap-1.5 text-center">
		<h1 class="font-heading text-2xl font-semibold tracking-tight text-fc-fg">
			{t('reset.title')}
		</h1>
		<p class="text-fc-sm text-fc-fg-muted">
			{t('reset.subtitle')}
		</p>
	</div>

	{#if done}
		<Alert tone="success" title={t('reset.done')} />
		<Button href="/login" variant="primary" size="lg" class="w-full">
			{t('login.title')}
		</Button>
	{:else if !token}
		<Alert tone="danger" title={t('reset.invalid')} />
	{:else}
		<form onsubmit={submit} class="flex flex-col gap-4">
			<div aria-live="polite">
				{#if error}
					<Alert tone="danger" title={error} />
				{/if}
			</div>

			<Field
				label={t('password.new')}
				helper={t('password.rule', { min: MIN_PASSWORD })}
				error={tooShort ? t('password.tooShort', { min: MIN_PASSWORD }) : undefined}
			>
				<Input
					type="password"
					bind:value={password}
					placeholder="••••••••"
					required
					minlength={MIN_PASSWORD}
					autocomplete="new-password"
					disabled={busy}
				/>
			</Field>

			<Field
				label={t('password.confirm')}
				error={mismatch ? t('password.mismatch') : undefined}
			>
				<Input
					type="password"
					bind:value={confirm}
					placeholder="••••••••"
					required
					autocomplete="new-password"
					disabled={busy}
				/>
			</Field>

			<Button
				type="submit"
				variant="primary"
				size="lg"
				disabled={busy || tooShort || mismatch || !password || !confirm}
				class="w-full"
			>
				{busy ? t('common.saving') : t('reset.submit')}
			</Button>
		</form>
	{/if}

	<p class="text-center text-fc-sm text-fc-fg-muted">
		<a href="/recover" class="font-medium text-fc-fg underline underline-offset-2 hover:underline">
			{t('reset.again')}
		</a>
	</p>
</Stack>
