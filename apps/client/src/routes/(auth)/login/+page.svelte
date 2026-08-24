<script lang="ts">
	import { Alert, Button, Card, Field, Input, Stack, icons } from '@facile/muse';

	type Step = 'password' | 'second-factor';

	let authRequest = $state('');
	let email = $state('');
	let password = $state('');
	let code = $state('');
	let challenge = $state('');
	let step = $state<Step>('password');
	let recovery = $state(false);
	let error = $state('');
	let busy = $state(false);

	const REFUSED = 'Wrong email or password.';
	const CODE_REFUSED = 'That code is not valid.';

	$effect(() => {
		authRequest = new URLSearchParams(window.location.search).get('auth_request') ?? '';
	});

	async function submitPassword(event: SubmitEvent) {
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
			const body = await response.json().catch(() => null);
			if (!response.ok) {
				error = REFUSED;
				return;
			}
			if (body?.challenge) {
				challenge = body.challenge;
				password = '';
				step = 'second-factor';
				return;
			}
			window.location.assign(body.redirect);
		} catch {
			error = 'Something went wrong. Try again.';
		} finally {
			busy = false;
		}
	}

	async function submitCode(event: SubmitEvent) {
		event.preventDefault();
		if (!challenge || busy) return;
		busy = true;
		error = '';
		try {
			const response = await fetch('/api/login/totp', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ challenge, code: code.trim() })
			});
			const body = await response.json().catch(() => null);
			if (!response.ok) {
				error = CODE_REFUSED;
				code = '';
				return;
			}
			window.location.assign(body.redirect);
		} catch {
			error = 'Something went wrong. Try again.';
		} finally {
			busy = false;
		}
	}

	function useRecovery() {
		recovery = !recovery;
		code = '';
		error = '';
	}

	function startOver() {
		step = 'password';
		challenge = '';
		code = '';
		recovery = false;
		error = '';
	}
</script>

<svelte:head>
	<title>Sign in — Registre</title>
</svelte:head>

<Card class="p-8">
	{#if step === 'password'}
		<Stack>
			<div class="flex flex-col gap-1">
				<h1 class="text-fc-lg font-semibold text-fc-fg">Sign in to Registre</h1>
				<p class="text-fc-sm text-fc-fg-muted">
					Sign-in starts from the app you are trying to reach.
				</p>
			</div>

			{#if !authRequest}
				<Alert tone="danger" title="This sign-in link is not valid.">
					Start again from the app that sent you here.
				</Alert>
			{:else}
				<form onsubmit={submitPassword} class="flex flex-col gap-4">
					<div aria-live="polite">
						{#if error}
							<Alert tone="danger" title={error} />
						{/if}
					</div>
					<Field label="Email">
						<Input type="email" autocomplete="username" required bind:value={email} />
					</Field>
					<Field label="Password">
						<Input type="password" autocomplete="current-password" required bind:value={password} />
					</Field>
					<Button type="submit" disabled={busy}>
						{busy ? 'Signing in…' : 'Sign in'}
					</Button>
					<a
						href="/recover"
						class="self-center rounded-fc-xs text-fc-sm text-fc-fg-muted underline
							underline-offset-2 transition-colors hover:text-fc-fg focus-visible:outline-2
							focus-visible:outline-offset-2 focus-visible:outline-fc-ring"
					>
						Forgotten your password?
					</a>
				</form>
			{/if}
		</Stack>
	{:else}
		<Stack>
			<div class="flex flex-col gap-1">
				<h1 class="text-fc-lg font-semibold text-fc-fg">Two-step sign-in</h1>
				<p class="text-fc-sm text-fc-fg-muted">
					{recovery
						? 'Enter one of the recovery codes you saved when you turned on two-step sign-in.'
						: 'Enter the six-digit code from your authenticator app.'}
				</p>
			</div>

			<form onsubmit={submitCode} class="flex flex-col gap-4">
				<div aria-live="polite">
					{#if error}
						<Alert tone="danger" title={error} />
					{/if}
				</div>

				{#if recovery}
					<Field label="Recovery code" helper="Each code works once.">
						<Input
							autocomplete="off"
							autocapitalize="off"
							spellcheck={false}
							required
							class="font-fc-mono"
							bind:value={code}
						/>
					</Field>
				{:else}
					<Field label="Six-digit code">
						<Input
							inputmode="numeric"
							autocomplete="one-time-code"
							maxlength={6}
							pattern={'[0-9]{6}'}
							required
							class="font-fc-mono tracking-widest"
							bind:value={code}
						/>
					</Field>
				{/if}

				<Button type="submit" disabled={busy || code.trim().length === 0}>
					{busy ? 'Checking…' : 'Continue'}
				</Button>

				<div class="flex flex-col gap-2">
					<Button type="button" variant="ghost" size="sm" onclick={useRecovery}>
						{recovery ? 'Use my authenticator app' : 'Use a recovery code instead'}
					</Button>
					<Button
						type="button"
						variant="ghost"
						size="sm"
						icon={icons.chevronLeft}
						onclick={startOver}
					>
						Start again
					</Button>
				</div>
			</form>
		</Stack>
	{/if}
</Card>
