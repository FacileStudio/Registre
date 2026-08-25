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

<div class="mb-8">
	<h1 class="font-heading text-2xl font-bold tracking-tight text-foreground">
		Reset your password
	</h1>
	<p class="mt-1.5 text-sm text-muted-foreground">
		Give us the address you sign in with and we will send a reset link.
	</p>
</div>

{#if sent}
	<p role="status" class="rounded-md bg-muted px-3 py-3 text-sm text-muted-foreground">
		If that address has an account, a reset link is on its way. The link expires, so use it
		soon.
	</p>
{:else}
	<form onsubmit={submit} class="space-y-4">
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

		<button type="submit" disabled={busy} aria-busy={busy} class={primaryButtonClass}>
			{busy ? 'Sending…' : 'Send the reset link'}
		</button>
	</form>
{/if}

<p class="mt-6 text-center text-sm text-muted-foreground">
	<a href="/login" class="underline underline-offset-2 transition-colors hover:text-foreground">
		Back to sign-in
	</a>
</p>
