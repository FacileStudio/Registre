<script lang="ts">
	import { Alert, Button, Card, Field, Input, Stack } from '@facile/muse';
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
			const response = await fetch('/api/recover/reset', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ token, password })
			});
			if (!response.ok) {
				error = 'This reset link is not valid, or it has already been used.';
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
	<title>Choose a new password — Registre</title>
</svelte:head>

<Card class="p-8">
	<Stack>
		<div class="flex flex-col gap-1">
			<h1 class="text-fc-lg font-semibold text-fc-fg">Choose a new password</h1>
			<p class="text-fc-sm text-fc-fg-muted">
				This link works once. Setting a password here signs out every session on the account.
			</p>
		</div>

		<div aria-live="polite">
			{#if done}
				<Alert tone="success" title="Your password is set.">
					Go back to the app you came from and sign in there.
				</Alert>
			{/if}
		</div>

		{#if !done}
			{#if !token}
				<Alert tone="danger" title="This reset link is not valid.">
					Start again from the reset form.
				</Alert>
			{:else}
				<form onsubmit={submit} class="flex flex-col gap-4">
					<div aria-live="polite">
						{#if error}
							<Alert tone="danger" title={error} />
						{/if}
					</div>
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
		{/if}

		<a
			href="/recover"
			class="self-center rounded-fc-xs text-fc-sm text-fc-fg-muted underline
				underline-offset-2 transition-colors hover:text-fc-fg focus-visible:outline-2
				focus-visible:outline-offset-2 focus-visible:outline-fc-ring"
		>
			Ask for a new link
		</a>
	</Stack>
</Card>
