<script lang="ts">
	import type { DeviceOutcome } from '$lib/device';

	const outlineButtonClass =
		'inline-flex h-11 w-full items-center justify-center rounded-md border border-border ' +
		'bg-background px-4 text-sm font-medium text-foreground transition-colors ' +
		'hover:bg-accent hover:text-accent-foreground';

	interface Props {
		outcome: DeviceOutcome;
		again: () => void;
	}

	let { outcome, again }: Props = $props();

	const COPY: Record<DeviceOutcome, { title: string; body: string; tone: string }> = {
		approved: {
			title: 'Approved',
			body: 'Your terminal is signing in now. You can close this page.',
			tone: 'text-fc-success'
		},
		denied: {
			title: 'Denied',
			body:
				'Nothing was signed in, and that code is now dead. If you did not start it, ' +
				'there is nothing else you need to do.',
			tone: 'text-muted-foreground'
		},
		unusable: {
			title: 'That code is not usable',
			body:
				'Codes last a few minutes and each one works once. Start again in your terminal ' +
				'and use the new code it shows you.',
			tone: 'text-fc-warning'
		},
		used: {
			title: 'You have already answered that code',
			body:
				'This device approved or denied it a moment ago, and each code works once. Start ' +
				'again in your terminal if you still need to sign in.',
			tone: 'text-fc-info'
		}
	};

	const copy = $derived(COPY[outcome]);
</script>

<div class="flex flex-col items-center gap-4 py-6 text-center" role="status">
	<span class={copy.tone}>
		{#if outcome === 'approved'}
			<svg width="40" height="40" viewBox="0 0 24 24" fill="none" aria-hidden="true">
				<circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="1.5" />
				<path
					d="m8.5 12.2 2.4 2.4 4.6-4.9"
					stroke="currentColor"
					stroke-width="1.5"
					stroke-linecap="round"
					stroke-linejoin="round"
				/>
			</svg>
		{:else if outcome === 'denied'}
			<svg width="40" height="40" viewBox="0 0 24 24" fill="none" aria-hidden="true">
				<circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="1.5" />
				<path
					d="m9.2 9.2 5.6 5.6M14.8 9.2l-5.6 5.6"
					stroke="currentColor"
					stroke-width="1.5"
					stroke-linecap="round"
				/>
			</svg>
		{:else if outcome === 'unusable'}
			<svg width="40" height="40" viewBox="0 0 24 24" fill="none" aria-hidden="true">
				<circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="1.5" />
				<path
					d="M12 7.5V12l3 1.8"
					stroke="currentColor"
					stroke-width="1.5"
					stroke-linecap="round"
					stroke-linejoin="round"
				/>
			</svg>
		{:else}
			<svg width="40" height="40" viewBox="0 0 24 24" fill="none" aria-hidden="true">
				<circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="1.5" />
				<path d="M12 11v5.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
				<circle cx="12" cy="8" r="1" fill="currentColor" />
			</svg>
		{/if}
	</span>

	<div class="space-y-1.5">
		<h1 class="font-heading text-2xl font-bold tracking-tight text-foreground">{copy.title}</h1>
		<p class="text-sm text-muted-foreground">{copy.body}</p>
	</div>
</div>

{#if outcome !== 'approved'}
	<button type="button" onclick={again} class={outlineButtonClass}>Enter another code</button>
{/if}
