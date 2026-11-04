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

	$effect(() => {
		token = new URLSearchParams(window.location.search).get('token') ?? '';
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
			const response = await fetch('/api/invites/redeem', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ token, password })
			});
			if (!response.ok) {
				const answer = await response.json().catch(() => null);
				error = answer?.error === 'password_breached' ? t('password.breached') : t('invite.used');
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
	<title>{t('invite.title')} — Facile</title>
</svelte:head>

<div class="mb-8">
	<h1 class="font-heading text-2xl font-bold tracking-tight text-foreground">
		{t('invite.title')}
	</h1>
	<p class="mt-1.5 text-sm text-muted-foreground">
		{t('invite.subtitle')}
	</p>
</div>

{#if done}
	<p role="status" class="rounded-md bg-muted px-3 py-3 text-sm text-muted-foreground">
		{t('invite.done')}
	</p>
{:else}
	<form onsubmit={submit} class="space-y-4">
		{#if error}
			<p role="alert" class="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
				{error}
			</p>
		{/if}

		<div class="space-y-1.5">
			<label for="token" class={labelClass}>{t('invite.token')}</label>
			<input
				id="token"
				name="token"
				type="text"
				bind:value={token}
				required
				autocomplete="off"
				autocapitalize="off"
				spellcheck="false"
				aria-describedby="token-hint"
				disabled={busy}
				class="{inputClass} font-mono"
			/>
			<p id="token-hint" class="text-xs text-muted-foreground">
				{t('invite.tokenHint')}
			</p>
		</div>

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
