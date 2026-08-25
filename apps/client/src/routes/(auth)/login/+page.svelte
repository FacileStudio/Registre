<script lang="ts">
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
	const outlineButtonClass =
		'inline-flex h-11 w-full items-center justify-center rounded-md border border-border ' +
		'bg-background px-4 text-sm font-medium text-foreground transition-colors ' +
		'hover:bg-accent hover:text-accent-foreground disabled:pointer-events-none ' +
		'disabled:opacity-50';
	const subtleButtonClass =
		'inline-flex h-11 w-full items-center justify-center rounded-md px-4 text-sm ' +
		'font-medium text-muted-foreground transition-colors hover:text-foreground ' +
		'disabled:pointer-events-none disabled:opacity-50';

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

{#if step === 'password'}
	<div class="mb-8">
		<h1 class="font-heading text-2xl font-bold tracking-tight text-foreground">
			Sign in to Registre
		</h1>
		<p class="mt-1.5 text-sm text-muted-foreground">
			Sign-in starts from the app you are trying to reach.
		</p>
	</div>

	{#if !authRequest}
		<p role="status" class="rounded-md bg-muted px-3 py-3 text-sm text-muted-foreground">
			There is nothing to sign in to from here. Open the Facile app you want to use and it
			will bring you back with everything this page needs.
		</p>

		<a href="/" class="mt-4 {outlineButtonClass}">Back to the home page</a>
	{:else}
		<form onsubmit={submitPassword} class="space-y-4">
			{#if error}
				<p role="alert" class="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
					{error}
				</p>
			{/if}

			<div class="space-y-1.5">
				<label for="email" class={labelClass}>Email</label>
				<input
					id="email"
					name="email"
					type="email"
					bind:value={email}
					placeholder="you@example.com"
					required
					autocomplete="username"
					disabled={busy}
					class={inputClass}
				/>
			</div>

			<div class="space-y-1.5">
				<label for="password" class={labelClass}>Password</label>
				<input
					id="password"
					name="password"
					type="password"
					bind:value={password}
					placeholder="••••••••"
					required
					autocomplete="current-password"
					disabled={busy}
					class={inputClass}
				/>
			</div>

			<button type="submit" disabled={busy} aria-busy={busy} class={primaryButtonClass}>
				{busy ? 'Signing in…' : 'Sign in'}
			</button>
		</form>

		<p class="mt-6 text-center text-sm text-muted-foreground">
			<a
				href="/recover"
				class="underline underline-offset-2 transition-colors hover:text-foreground"
			>
				Forgotten your password?
			</a>
		</p>
	{/if}
{:else}
	<div class="mb-8">
		<h1 class="font-heading text-2xl font-bold tracking-tight text-foreground">
			Two-step sign-in
		</h1>
		<p class="mt-1.5 text-sm text-muted-foreground">
			{recovery
				? 'Enter one of the recovery codes you saved when you turned on two-step sign-in.'
				: 'Enter the six-digit code from your authenticator app.'}
		</p>
	</div>

	<form onsubmit={submitCode} class="space-y-4">
		{#if error}
			<p role="alert" class="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
				{error}
			</p>
		{/if}

		{#if recovery}
			<div class="space-y-1.5">
				<label for="recovery-code" class={labelClass}>Recovery code</label>
				<input
					id="recovery-code"
					name="recovery-code"
					type="text"
					bind:value={code}
					required
					autocomplete="off"
					autocapitalize="off"
					spellcheck="false"
					aria-describedby="recovery-code-hint"
					disabled={busy}
					class="{inputClass} font-mono"
				/>
				<p id="recovery-code-hint" class="text-xs text-muted-foreground">
					Each code works once.
				</p>
			</div>
		{:else}
			<div class="space-y-1.5">
				<label for="totp-code" class={labelClass}>Six-digit code</label>
				<input
					id="totp-code"
					name="totp-code"
					type="text"
					bind:value={code}
					required
					inputmode="numeric"
					autocomplete="one-time-code"
					maxlength={6}
					pattern={'[0-9]{6}'}
					disabled={busy}
					class="{inputClass} font-mono tracking-widest"
				/>
			</div>
		{/if}

		<button
			type="submit"
			disabled={busy || code.trim().length === 0}
			aria-busy={busy}
			class={primaryButtonClass}
		>
			{busy ? 'Checking…' : 'Continue'}
		</button>

		<button type="button" onclick={useRecovery} class={outlineButtonClass}>
			{recovery ? 'Use my authenticator app' : 'Use a recovery code instead'}
		</button>

		<button type="button" onclick={startOver} class={subtleButtonClass}>Start again</button>
	</form>
{/if}
