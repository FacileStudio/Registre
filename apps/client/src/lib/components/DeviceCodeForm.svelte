<script lang="ts">
	import { Button } from '@facile/muse';
	import { t } from '$lib/i18n/i18n.svelte';
	import { CODE_LENGTH, inspect } from '$lib/usercode';

	const inputClass =
		'flex h-12 w-full rounded-fc-md bg-fc-field px-4 py-2 text-center ' +
		'font-mono text-xl uppercase tracking-[0.35em] text-fc-fg placeholder:text-fc-fg-muted ' +
		'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-fc-ring ' +
		'disabled:cursor-not-allowed disabled:opacity-50';
	const labelClass = 'text-sm font-medium leading-none text-fc-fg';
	const hintClass = 'text-xs leading-relaxed text-center';

	interface Props {
		code: string;
		busy: boolean;
		error: string;
		prefilled: boolean;
		submit: () => void;
	}

	let { code = $bindable(), busy, error, prefilled, submit }: Props = $props();

	/**
	 * SHAPE and STRAY are the two things this form has to be able to say, and
	 * they exist because the button is now stricter than it was.
	 *
	 * A code is only sent once it is whole, because a lookup of anything shorter
	 * cannot succeed. But a disabled button with no explanation is the same dead
	 * end by another route, so the hint under the field is never empty and it
	 * always says which of the two situations the human is in.
	 *
	 * SHAPE keeps saying that the dash is inserted rather than typed, which is
	 * the fact this hint was added to carry, and adds the length the button is
	 * now waiting for.
	 *
	 * STRAY is the one somebody actually hit. A code like `F28H-2J4K` is a
	 * device code from another tool; every digit in it is dropped on the way in,
	 * here and at the API alike, and the four letters that survive match
	 * nothing. Saying "letters only, no digits" names the rule, and naming the
	 * other tool is what stops the next five minutes being spent retyping.
	 */
	const SHAPE = $derived(t('device.shape', { length: CODE_LENGTH }));
	const STRAY = $derived(t('device.stray'));

	/**
	 * stray remembers whether the last thing typed or pasted carried characters
	 * the alphabet does not have.
	 *
	 * It has to be remembered rather than derived, because `code` is canonical
	 * by the time anything else reads it and a canonical code has no trace of
	 * what was dropped to make it. It clears itself on the next input event, so
	 * the message never outlives the mistake it describes.
	 */
	let stray = $state(false);

	const shape = $derived(inspect(code));
	const ready = $derived(shape.complete && !busy);
	const hint = $derived.by(() => {
		if (stray) return STRAY;
		if (shape.kept > 0 && !shape.complete) return `${shape.kept} of ${CODE_LENGTH} letters.`;
		return SHAPE;
	});

	/**
	 * reshape canonicalises the field on every input event, so what the human
	 * reads back is exactly what will be sent.
	 *
	 * One handler covers typing, pasting, dropping and autofill, because all
	 * four raise `input`. The field is written back explicitly rather than left
	 * to the binding: a keystroke the alphabet rejects leaves `code` unchanged,
	 * so nothing would tell the DOM to drop the character and the field would
	 * quietly disagree with the value being sent.
	 *
	 * Reformatting puts the caret at the end, which is the browser's own
	 * behaviour when a field's value changes. That is right for a code read off
	 * another screen left to right, and the guard keeps it from happening on
	 * the keystrokes that change nothing.
	 *
	 * This is also why the field carries no `maxlength`. Capping it at nine
	 * characters stopped it growing before this handler existed, but the cap
	 * applies to a paste as well, and a browser truncates the pasted text to fit
	 * before anything here can read it: a code copied out of a terminal with a
	 * space on either end arrived one letter short and looked like the human's
	 * mistake. The handler caps the length itself, on every event, so the
	 * attribute now only costs pastes.
	 */
	function reshape(event: Event & { currentTarget: HTMLInputElement }) {
		const field = event.currentTarget;
		const typed = inspect(field.value);
		stray = typed.stray;
		code = typed.code;
		if (field.value !== typed.code) field.value = typed.code;
	}

	function onsubmit(event: SubmitEvent) {
		event.preventDefault();
		if (!ready) return;
		submit();
	}
</script>

<div class="mb-6 text-center">
	<h1 class="font-heading text-2xl font-semibold tracking-tight text-fc-fg">{t('device.title')}</h1>
	<p class="mt-1.5 text-sm text-fc-fg-muted">
		{prefilled ? t('device.checkMatch') : t('device.typeCode')}
	</p>
</div>

<form {onsubmit} class="space-y-4">
	{#if error}
		<p role="alert" class="rounded-fc-md bg-fc-danger/10 px-3 py-2 text-sm text-fc-danger">
			{error}
		</p>
	{/if}

	<div class="space-y-2">
		<label for="user-code" class="{labelClass} sr-only">{t('device.code')}</label>
		<input
			id="user-code"
			name="user-code"
			type="text"
			value={code}
			oninput={reshape}
			required
			placeholder="ABCD-EFGH"
			autocomplete="one-time-code"
			autocapitalize="characters"
			autocorrect="off"
			spellcheck="false"
			aria-describedby="user-code-hint"
			disabled={busy}
			class={inputClass}
		/>
		<p
			id="user-code-hint"
			aria-live="polite"
			class="{hintClass} {stray ? 'text-fc-warning' : 'text-fc-fg-muted'}"
		>
			{hint}
		</p>
	</div>

	<Button type="submit" variant="primary" size="lg" disabled={!ready} class="w-full">
		{busy ? t('common.checking') : t('common.continue')}
	</Button>
</form>

<p class="mt-6 text-center text-xs text-fc-fg-muted">
	{t('device.codesLast')}
</p>
