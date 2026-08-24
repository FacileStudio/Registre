<script lang="ts">
	import { Alert, Button, Card, Field, Input, Stack } from '@facile/muse';
	import { MIN_PASSWORD } from '$lib/policy';


	let token = $state('');
	let password = $state('');
	let confirm = $state('');
	let error = $state('');
	let done = $state(false);
	let busy = $state(false);

	$effect(() => {
		token = new URLSearchParams(window.location.search).get('token') ?? '';
	});

	const tooShort = $derived(password.length > 0 && password.length < MIN_PASSWORD);
	const mismatch = $derived(confirm.length > 0 && confirm !== password);

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

<Card class="p-8">
	<Stack>
		<div class="flex flex-col gap-1">
			<h1 class="text-fc-lg font-semibold text-fc-fg">Set your password</h1>
			<p class="text-fc-sm text-fc-fg-muted">
				Your invitation from the Registre operator. Choose a password for your account.
			</p>
		</div>

		{#if done}
			<Alert tone="success" title="Your password is set.">
				You can now sign in from any Facile app.
			</Alert>
		{:else}
			<form onsubmit={submit} class="flex flex-col gap-4">
				<div aria-live="polite">
					{#if error}
						<Alert tone="danger" title={error} />
					{/if}
				</div>
				<Field label="Invitation token" helper="Filled in from the link you followed.">
					<Input required class="font-fc-mono" bind:value={token} />
				</Field>
				<Field
					label="New password"
					helper="At least {MIN_PASSWORD} characters."
					error={tooShort ? `At least ${MIN_PASSWORD} characters.` : undefined}
				>
					<Input
						type="password"
						autocomplete="new-password"
						minlength={MIN_PASSWORD}
						required
						bind:value={password}
					/>
				</Field>
				<Field
					label="Confirm password"
					error={mismatch ? 'The passwords do not match.' : undefined}
				>
					<Input type="password" autocomplete="new-password" required bind:value={confirm} />
				</Field>
				<Button type="submit" disabled={busy || tooShort || mismatch}>
					{busy ? 'Setting…' : 'Set password'}
				</Button>
			</form>
		{/if}
	</Stack>
</Card>
