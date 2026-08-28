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

	// The shape devices.Normalise stores and looks a code up in: twenty
	// consonants, no vowels and no digits, eight characters, grouped four and
	// four. Kept in step with internal/devices.Alphabet, CodeLength and
	// GroupSize by hand, because the client has no generated copy of them.
	const ALPHABET = 'BCDFGHJKLMNPQRSTVWXZ';
	const CODE_LENGTH = 8;
	const GROUP_SIZE = 4;

	// The dash is decoration. The server strips every character outside the
	// alphabet before it looks anything up, so "ccjvvmth" and "CCJV-VMTH" find
	// the same row, and nobody should have to reach a symbol keyboard to type
	// a separator the lookup discards. Typing the letters inserts it.
	function group(raw: string): string {
		let kept = '';
		for (const char of raw.toUpperCase()) {
			if (ALPHABET.includes(char) && kept.length < CODE_LENGTH) kept += char;
		}
		if (kept.length <= GROUP_SIZE) return kept;
		return `${kept.slice(0, GROUP_SIZE)}-${kept.slice(GROUP_SIZE)}`;
	}

	function oninput(event: Event) {
		const field = event.currentTarget as HTMLInputElement;
		const formatted = group(field.value);
		code = formatted;
		field.value = formatted;
	}

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
			value={code}
			{oninput}
			maxlength={CODE_LENGTH + 1}
			required
			autocomplete="one-time-code"
			autocapitalize="characters"
			autocorrect="off"
			spellcheck="false"
			aria-describedby="user-code-hint"
			disabled={busy}
			class={inputClass}
		/>
		<p id="user-code-hint" class="text-xs text-muted-foreground">
			Letters only, and the dash is added for you.
		</p>
	</div>

	<button type="submit" disabled={!ready} aria-busy={busy} class={primaryButtonClass}>
		{busy ? 'Checking…' : 'Continue'}
	</button>
</form>

<p class="mt-6 text-sm text-muted-foreground">
	Codes last a few minutes and each one works once. If yours has run out, start again in your
	terminal.
</p>
