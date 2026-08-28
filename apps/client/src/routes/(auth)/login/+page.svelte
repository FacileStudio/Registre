<script lang="ts">
	import { onMount } from 'svelte';
	import { Alert, Button, Card, Field, Input, Stack } from '@facile/muse';

	type Step = 'password' | 'second-factor';
	type AuthStatus = 'checking' | 'redirecting' | 'ready' | 'submitting';

	let authRequest = $state('');
	let email = $state('');
	let password = $state('');
	let code = $state('');
	let challenge = $state('');
	let step = $state<Step>('password');
	let recovery = $state(false);
	let error = $state('');
	let status = $state<AuthStatus>('checking');

	const REFUSED = 'Wrong email or password.';
	const CODE_REFUSED = 'That code is not valid.';

	onMount(() => {
		const req = new URLSearchParams(window.location.search).get('auth_request') ?? '';
		authRequest = req;
		checkSession(req);
	});

	async function checkSession(req: string) {
		try {
			const url = req
				? `/api/login/session?auth_request=${encodeURIComponent(req)}`
				: '/api/login/session';
			const response = await fetch(url, {
				method: 'GET',
				credentials: 'include'
			});
			if (response.ok) {
				const body = await response.json().catch(() => null);
				if (body?.redirect) {
					status = 'redirecting';
					window.location.assign(body.redirect);
					return;
				}
			}
			status = 'ready';
		} catch {
			status = 'ready';
		}
	}

	async function submitPassword(event: SubmitEvent) {
		event.preventDefault();
		if (status === 'submitting' || status === 'redirecting') return;
		status = 'submitting';
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
				status = 'ready';
				return;
			}
			if (body?.challenge) {
				challenge = body.challenge;
				password = '';
				step = 'second-factor';
				status = 'ready';
				return;
			}
			status = 'redirecting';
			window.location.assign(body?.redirect || '/');
		} catch {
			error = 'Something went wrong. Try again.';
			status = 'ready';
		}
	}

	async function submitCode(event: SubmitEvent) {
		event.preventDefault();
		if (!challenge || status === 'submitting' || status === 'redirecting') return;
		status = 'submitting';
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
				status = 'ready';
				return;
			}
			status = 'redirecting';
			window.location.assign(body?.redirect || '/');
		} catch {
			error = 'Something went wrong. Try again.';
			status = 'ready';
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
		status = 'ready';
	}
</script>

<svelte:head>
	<title>Sign in — Registre</title>
</svelte:head>

<Card class="p-8">
	{#if status === 'checking' || status === 'redirecting'}
		<div class="stage py-8">
			<Stack gap="content" class="items-center text-center">
				<div class="relative flex size-12 items-center justify-center">
					<div class="absolute inset-0 size-12 animate-ping rounded-full bg-primary/20 opacity-75"></div>
					<div class="size-8 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
				</div>
				<div class="flex flex-col gap-1.5 pt-2">
					<h1 class="text-base font-medium text-foreground">
						{status === 'redirecting' ? 'Connecting to application…' : 'Signing you in…'}
					</h1>
					<p class="text-xs text-muted-foreground">
						{status === 'redirecting'
							? 'Redirecting you back automatically.'
							: 'Reusing your active Facile session.'}
					</p>
				</div>
			</Stack>
		</div>
	{:else if step === 'password'}
		<div class="stage">
			<Stack>
				<div class="flex flex-col gap-1">
					<h1 class="text-fc-lg font-semibold text-fc-fg">Sign in to Registre</h1>
					<p class="text-fc-sm text-fc-fg-muted">
						{authRequest
							? 'Sign-in starts from the app you are trying to reach.'
							: 'Sign in with your Facile account.'}
					</p>
				</div>

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
					<Button type="submit" disabled={status === 'submitting'}>
						{status === 'submitting' ? 'Signing in…' : 'Sign in'}
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
			</Stack>
		</div>
	{:else}
		<div class="stage">
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
							<Input type="text" autocomplete="off" required bind:value={code} />
						</Field>
					{:else}
						<Field label="Six-digit code">
							<Input
								type="text"
								inputmode="numeric"
								maxlength={6}
								pattern={'[0-9]{6}'}
								required
								bind:value={code}
							/>
						</Field>
					{/if}
					<Button type="submit" disabled={status === 'submitting' || code.trim().length === 0}>
						{status === 'submitting' ? 'Checking…' : 'Continue'}
					</Button>
					<Button variant="outline" onclick={useRecovery}>
						{recovery ? 'Use my authenticator app' : 'Use a recovery code instead'}
					</Button>
					<Button variant="ghost" onclick={startOver}>Start again</Button>
				</form>
			</Stack>
		</div>
	{/if}
</Card>

<style>
	.stage {
		animation: stage-in 0.2s cubic-bezier(0.16, 1, 0.3, 1) both;
	}

	@keyframes stage-in {
		from {
			opacity: 0;
			transform: scale(0.98);
		}
		to {
			opacity: 1;
			transform: scale(1);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.stage {
			animation: none;
		}
	}
</style>
