<script lang="ts">
	import { t } from '$lib/i18n/i18n.svelte';
	import { MIN_PASSWORD } from '$lib/policy';

	const inputClass =
		'flex h-11 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ' +
		'text-foreground placeholder:text-muted-foreground focus-visible:outline-none ' +
		'focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed ' +
		'disabled:opacity-50';
	const labelClass = 'text-sm font-medium leading-none text-foreground';
	const primaryButtonClass =
		'inline-flex h-11 w-full items-center justify-center rounded-md bg-primary px-4 ' +
		'text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 ' +
		'disabled:pointer-events-none disabled:opacity-50';

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

<div class="mb-8">
	<h1 class="font-heading text-2xl font-bold tracking-tight text-foreground">
		{t('reset.title')}
	</h1>
	<p class="mt-1.5 text-sm text-muted-foreground">
		{t('reset.subtitle')}
	</p>
</div>

{#if done}
	<p role="status" class="rounded-md bg-muted px-3 py-3 text-sm text-muted-foreground">
		{t('reset.done')}
	</p>
{:else if !token}
	<p role="alert" class="rounded-md bg-destructive/10 px-3 py-3 text-sm text-destructive">
		{t('reset.invalid')}
	</p>
{:else}
	<form onsubmit={submit} class="space-y-4">
		{#if error}
			<p role="alert" class="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
				{error}
			</p>
		{/if}

		<div class="space-y-1.5">
			<label for="password" class={labelClass}>{t('password.new')}</label>
			<input
				id="password"
				name="password"
				type="password"
				bind:value={password}
				placeholder="••••••••"
				required
				minlength={MIN_PASSWORD}
				autocomplete="new-password"
				aria-invalid={tooShort}
				aria-describedby="password-hint"
				disabled={busy}
				class={inputClass}
			/>
			<p
				id="password-hint"
				class="text-xs {tooShort ? 'text-destructive' : 'text-muted-foreground'}"
			>
				{t('password.rule', { min: MIN_PASSWORD })}
			</p>
		</div>

		<div class="space-y-1.5">
			<label for="confirm-password" class={labelClass}>{t('password.confirm')}</label>
			<input
				id="confirm-password"
				name="confirm-password"
				type="password"
				bind:value={confirm}
				placeholder="••••••••"
				required
				autocomplete="new-password"
				aria-invalid={mismatch}
				aria-describedby={mismatch ? 'confirm-password-error' : undefined}
				disabled={busy}
				class={inputClass}
			/>
			{#if mismatch}
				<p id="confirm-password-error" class="text-sm text-destructive">
					{t('password.mismatch')}
				</p>
			{/if}
		</div>

		<button
			type="submit"
			disabled={busy || tooShort || mismatch}
			aria-busy={busy}
			class={primaryButtonClass}
		>
			{busy ? t('common.saving') : t('reset.submit')}
		</button>
	</form>
{/if}

<p class="mt-6 text-center text-sm text-muted-foreground">
	<a href="/recover" class="underline underline-offset-2 transition-colors hover:text-foreground">
		{t('reset.again')}
	</a>
</p>
