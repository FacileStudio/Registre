<script lang="ts">
	import { Button, Icon, icons } from '@facile/muse';
	import { t } from '$lib/i18n/i18n.svelte';
	import type { MessageKey } from '$lib/i18n/en';
	import type { DeviceOutcome } from '$lib/device';

	interface Props {
		outcome: DeviceOutcome;
		clientName?: string;
		again: () => void;
	}

	let { outcome, clientName, again }: Props = $props();

	const TONES: Record<DeviceOutcome, string> = {
		approved: 'text-fc-success bg-fc-success/10',
		denied: 'text-fc-fg-muted bg-fc-field',
		unusable: 'text-fc-warning bg-fc-warning/10',
		used: 'text-fc-info bg-fc-info/10'
	};

	const copy = $derived({
		title: t(`device.outcome.${outcome}.title` as MessageKey),
		body:
			outcome === 'approved' && clientName
				? `You are signed in to ${clientName}. You can close this window.`
				: t(`device.outcome.${outcome}.body` as MessageKey),
		tone: TONES[outcome]
	});
</script>

<div class="flex flex-col items-center gap-5 py-4 text-center" role="status">
	<div class="flex size-16 items-center justify-center rounded-full {copy.tone}">
		{#if outcome === 'approved'}
			<Icon icon={icons.check} size={32} />
		{:else if outcome === 'denied'}
			<Icon icon={icons.revoke} size={32} />
		{:else if outcome === 'unusable'}
			<Icon icon={icons.clock} size={32} />
		{:else}
			<Icon icon={icons.info} size={32} />
		{/if}
	</div>

	<div class="space-y-1.5">
		<h1 class="font-heading text-2xl font-bold tracking-tight text-fc-fg">{copy.title}</h1>
		<p class="max-w-sm text-sm text-fc-fg-muted">{copy.body}</p>
	</div>
</div>

{#if outcome !== 'approved'}
	<div class="mt-4">
		<Button type="button" variant="secondary" size="lg" onclick={again} class="w-full">
			Enter another code
		</Button>
	</div>
{/if}
