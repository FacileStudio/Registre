<!--
	Turning two-step sign-in off. It costs what turning it on cost, a password and
	one live code, and it says what the account is left with afterwards. The page
	makes the call.
-->
<script lang="ts">
	import { Alert, Button, Field, Input, Modal, icons } from '@facile/muse';

	let {
		open = $bindable(),
		password = $bindable(),
		code = $bindable(),
		busy,
		error,
		onSubmit,
		onCancel
	}: {
		open: boolean;
		password: string;
		code: string;
		busy: boolean;
		error: string;
		onSubmit: (event: SubmitEvent) => void;
		onCancel: () => void;
	} = $props();
</script>

<Modal bind:open title="Turn off two-step sign-in" showClose size="sm">
	<form class="flex flex-col gap-4" onsubmit={onSubmit}>
		<Alert tone="danger" title="Your password becomes the only guard">
			Anyone who learns your password can sign in to every app in the suite. Turning it off costs
			what turning it on cost: your password and one live code.
		</Alert>
		<div aria-live="polite">
			{#if error}
				<Alert tone="danger" title={error} />
			{/if}
		</div>
		<Field label="Your password">
			<Input
				type="password"
				autocomplete="current-password"
				required
				disabled={busy}
				bind:value={password}
			/>
		</Field>
		<Field label="Six-digit code" helper="A recovery code works here too.">
			<Input
				inputmode="numeric"
				autocomplete="one-time-code"
				required
				disabled={busy}
				class="font-fc-mono tracking-widest"
				bind:value={code}
			/>
		</Field>
		<div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
			<Button
				type="button"
				variant="ghost"
				class="w-full sm:w-auto"
				disabled={busy}
				onclick={onCancel}
			>
				Keep it on
			</Button>
			<Button
				type="submit"
				variant="danger"
				icon={icons.revoke}
				class="w-full sm:w-auto"
				disabled={busy || password.length === 0 || code.trim().length === 0}
			>
				{busy ? 'Turning off…' : 'Turn off'}
			</Button>
		</div>
	</form>
</Modal>
