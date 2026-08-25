<script lang="ts">
	const inputClass =
		'flex h-11 w-full rounded-md border border-input bg-background px-3 py-2 text-center ' +
		'font-mono text-lg uppercase tracking-[0.3em] text-foreground focus-visible:outline-none ' +
		'focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed ' +
		'disabled:opacity-50';
	const labelClass = 'text-sm font-medium leading-none text-foreground';
	const primaryButtonClass =
		'inline-flex h-11 w-full items-center justify-center rounded-md bg-primary px-4 ' +
		'text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 ' +
		'disabled:pointer-events-none disabled:opacity-50';

	interface Props {
		code: string;
		busy: boolean;
		error: string;
		prefilled: boolean;
		submit: () => void;
	}

	let { code = $bindable(), busy, error, prefilled, submit }: Props = $props();

	const ready = $derived(code.trim().length > 0 && !busy);

	function onsubmit(event: SubmitEvent) {
		event.preventDefault();
		if (!ready) return;
		submit();
	}
</script>

<div class="mb-8">
	<h1 class="font-heading text-2xl font-bold tracking-tight text-foreground">Approve a sign-in</h1>
	<p class="mt-1.5 text-sm text-muted-foreground">
		{prefilled
			? 'Check this matches the code in your terminal, then continue.'
			: 'Type the code your terminal is showing you.'}
	</p>
</div>

<form {onsubmit} class="space-y-4">
	{#if error}
		<p role="alert" class="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
			{error}
		</p>
	{/if}

	<div class="space-y-1.5">
		<label for="user-code" class={labelClass}>Code</label>
		<input
			id="user-code"
			name="user-code"
			type="text"
			bind:value={code}
			required
			autocomplete="off"
			autocapitalize="characters"
			autocorrect="off"
			spellcheck="false"
			disabled={busy}
			class={inputClass}
		/>
	</div>

	<button type="submit" disabled={!ready} aria-busy={busy} class={primaryButtonClass}>
		{busy ? 'Checking…' : 'Continue'}
	</button>
</form>

<p class="mt-6 text-sm text-muted-foreground">
	Codes last a few minutes and each one works once. If yours has run out, start again in your
	terminal.
</p>
