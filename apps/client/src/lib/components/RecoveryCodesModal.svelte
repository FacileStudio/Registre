<!--
	The shown-once recovery codes. The list lives only as long as this modal is
	open, because the server keeps hashes and cannot show the codes again, so the
	warning to save them now stays in front of the reader.
-->
<script lang="ts">
	import { Alert, Button, Modal, icons } from '@facile/muse';

	let {
		open = $bindable(),
		codes,
		copied,
		onCopy,
		onClose
	}: {
		open: boolean;
		codes: string[];
		copied: boolean;
		onCopy: () => void;
		onClose: () => void;
	} = $props();
</script>

<Modal bind:open title="Your recovery codes" dismissible={false} size="md" {onClose}>
	<div class="flex flex-col gap-4">
		<Alert tone="warning" title="Save these now, they will not be shown again">
			Registre stores hashes, so this is the only time it can show them. Each code works once.
			Keep them somewhere you can reach without your phone.
		</Alert>

		<ul class="grid grid-cols-2 gap-2 rounded-fc-md bg-fc-surface p-3">
			{#each codes as code (code)}
				<li class="font-fc-mono text-fc-sm tracking-wide text-fc-fg">{code}</li>
			{/each}
		</ul>

		<div class="flex flex-col-reverse gap-2 sm:flex-row sm:items-center sm:justify-between">
			<span class="text-fc-xs text-fc-fg-muted" aria-live="polite">
				{copied ? 'Copied to the clipboard.' : ''}
			</span>
			<div class="flex flex-col-reverse gap-2 sm:flex-row">
				<Button
					variant="outline"
					icon={copied ? icons.check : icons.copy}
					class="w-full sm:w-auto"
					onclick={onCopy}
				>
					{copied ? 'Copied' : 'Copy all codes'}
				</Button>
				<Button class="w-full sm:w-auto" onclick={() => (open = false)}>I have saved them</Button>
			</div>
		</div>
	</div>
</Modal>
