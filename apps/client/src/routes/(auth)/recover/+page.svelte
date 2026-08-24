<script lang="ts">
	import { Alert, Button, Card, Field, Input, Stack } from '@facile/muse';

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
	<title>Reset your password — Registre</title>
</svelte:head>

<Card class="p-8">
	<Stack>
		<div class="flex flex-col gap-1">
			<h1 class="text-fc-lg font-semibold text-fc-fg">Reset your password</h1>
			<p class="text-fc-sm text-fc-fg-muted">
				Give us the address you sign in with and we will send a reset link.
			</p>
		</div>

		<div aria-live="polite">
			{#if sent}
				<Alert tone="success" title="Check your inbox.">
					If that address has an account, a reset link is on its way. The link expires, so use it
					soon.
				</Alert>
			{/if}
		</div>

		{#if !sent}
			<form onsubmit={submit} class="flex flex-col gap-4">
				<Field label="Email">
					<Input type="email" autocomplete="username" required bind:value={email} />
				</Field>
				<Button type="submit" disabled={busy}>
					{busy ? 'Sending…' : 'Send the reset link'}
				</Button>
			</form>
		{/if}

		<a
			href="/login"
			class="self-center rounded-fc-xs text-fc-sm text-fc-fg-muted underline
				underline-offset-2 transition-colors hover:text-fc-fg focus-visible:outline-2
				focus-visible:outline-offset-2 focus-visible:outline-fc-ring"
		>
			Back to sign-in
		</a>
	</Stack>
</Card>
