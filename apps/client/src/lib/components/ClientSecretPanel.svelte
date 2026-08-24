<!--
	A client secret shown once. Registre stores a hash, so the panel leads with
	the copy-now warning and the caller supplies the wording for its own case.
	The dismiss button is the caller's too, because closing means something
	different after a creation than after a rotation.
-->
<script lang="ts">
	import { Alert, Button, SecretField } from '@facile/muse';
	import type { Snippet } from 'svelte';

	let {
		secret,
		helper,
		actionLabel,
		onAction,
		children
	}: {
		secret: string;
		helper?: string;
		actionLabel: string;
		onAction: () => void;
		children: Snippet;
	} = $props();
</script>

<div class="flex flex-col gap-4">
	<Alert tone="warning" title="Copy it now">
		{@render children()}
	</Alert>

	<SecretField value={secret} visible autoHideMs={0} label="Client secret" {helper} />

	<div class="flex justify-end">
		<Button onclick={onAction}>{actionLabel}</Button>
	</div>
</div>
