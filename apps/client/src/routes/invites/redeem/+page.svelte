<script lang="ts">
	import { Alert, Button, Card, Field, Input } from '@facile/muse';

	let token = $state('');
	let password = $state('');
	let confirm = $state('');
	let error = $state('');
	let done = $state(false);
	let busy = $state(false);

	$effect(() => {
		token = new URLSearchParams(window.location.search).get('token') ?? '';
	});

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (busy) return;
		if (password !== confirm) {
			error = 'The passwords do not match.';
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
				error = 'This invitation is not valid, or it has already been used.';
				return;
			}
			done = true;
		} catch {
			error = 'Something went wrong. Try again.';
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>Set your password — Registre</title>
</svelte:head>

<main class="grid min-h-dvh place-items-center bg-fc-page px-4">
	<Card class="w-full max-w-sm p-8">
		<h1 class="mb-1 text-fc-lg font-semibold text-fc-fg">Set your password</h1>
		<p class="mb-6 text-fc-sm text-fc-fg-muted">
			Your invitation from the Registre operator. Choose a password for your account.
		</p>

		{#if done}
			<Alert tone="success" title="Your password is set.">
					<p class="mt-1">You can now sign in from any Facile app.</p>
				</Alert>
		{:else}
			<form onsubmit={submit} class="flex flex-col gap-4">
				{#if error}
					<Alert tone="danger" title={error} />
				{/if}
				<Field label="Invitation token" for="token">
					<Input id="token" required bind:value={token} />
				</Field>
				<Field label="New password" for="password">
					<Input id="password" type="password" autocomplete="new-password" required bind:value={password} />
				</Field>
				<Field label="Confirm password" for="confirm">
					<Input id="confirm" type="password" autocomplete="new-password" required bind:value={confirm} />
				</Field>
				<Button type="submit" disabled={busy}>
					{busy ? 'Setting…' : 'Set password'}
				</Button>
			</form>
		{/if}
	</Card>
</main>
