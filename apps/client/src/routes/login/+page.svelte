<script lang="ts">
	import { Button, Card, Field, Input, Alert } from '@facile/muse';

	let authRequest = $state('');
	let email = $state('');
	let password = $state('');
	let error = $state('');
	let busy = $state(false);

	$effect(() => {
		authRequest = new URLSearchParams(window.location.search).get('auth_request') ?? '';
	});

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (!authRequest || busy) return;
		busy = true;
		error = '';
		try {
			const response = await fetch('/api/login', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ auth_request: authRequest, email, password })
			});
			const body = await response.json();
			if (!response.ok) {
				error = 'Wrong email or password.';
				return;
			}
			window.location.assign(body.redirect);
		} catch {
			error = 'Something went wrong. Try again.';
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>Sign in — Registre</title>
</svelte:head>

<main class="grid min-h-dvh place-items-center bg-fc-page px-4">
	<Card class="w-full max-w-sm p-8">
		<h1 class="mb-1 text-fc-lg font-semibold text-fc-fg">Sign in to Registre</h1>
		<p class="mb-6 text-fc-sm text-fc-fg-muted">
			The Facile Suite's identity provider. Sign-in starts from the app you are trying to reach.
		</p>

		{#if !authRequest}
			<Alert tone="danger" title="This sign-in link is not valid.">
				Start again from the app that sent you here.
			</Alert>
		{:else}
			<form onsubmit={submit} class="flex flex-col gap-4">
				{#if error}
					<Alert tone="danger" title={error} />
				{/if}
				<Field label="Email" for="email">
					<Input
						id="email"
						type="email"
						autocomplete="username"
						required
						autofocus
						bind:value={email}
					/>
				</Field>
				<Field label="Password" for="password">
					<Input
						id="password"
						type="password"
						autocomplete="current-password"
						required
						bind:value={password}
					/>
				</Field>
				<Button type="submit" disabled={busy}>
					{busy ? 'Signing in…' : 'Sign in'}
				</Button>
			</form>
		{/if}
	</Card>
</main>
