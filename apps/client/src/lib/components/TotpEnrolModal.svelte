<!--
	The enrolment modal: the QR code, the same secret as text for an app that
	cannot scan, and the confirmation code that proves the authenticator works.
	The page fetches the enrolment and confirms it.
-->
<script lang="ts">
	import { Alert, Button, Field, Input, Modal, SecretField, Spinner, icons } from '@facile/muse';
	import type { TOTPEnrolment } from '$lib/account';

	let {
		open = $bindable(),
		enrolment,
		code = $bindable(),
		busy,
		error,
		onSubmit,
		onCancel
	}: {
		open: boolean;
		enrolment: TOTPEnrolment | null;
		code: string;
		busy: boolean;
		error: string;
		onSubmit: (event: SubmitEvent) => void;
		onCancel: () => void;
	} = $props();
</script>

<Modal bind:open title="Turn on two-step sign-in" showClose size="md">
	{#if error && !enrolment}
		<Alert tone="danger" title="Could not start the enrolment">{error}</Alert>
	{:else if !enrolment}
		<div class="flex justify-center py-10" aria-live="polite">
			<Spinner label="Preparing your enrolment" />
		</div>
	{:else}
		<form class="flex flex-col gap-4" onsubmit={onSubmit}>
			<p class="text-fc-sm text-fc-fg-muted">
				Scan this with your authenticator app, then type the code it shows.
			</p>

			<div class="flex justify-center">
				<img
					src={`data:image/png;base64,${enrolment.qr_png}`}
					alt="QR code holding your two-step sign-in secret"
					class="size-44 rounded-fc-md bg-fc-bg p-2"
				/>
			</div>

			<SecretField
				value={enrolment.secret}
				label="Or type this secret in by hand"
				helper="Same secret as the QR code, for an app that cannot scan."
			/>

			<div aria-live="polite">
				{#if error}
					<Alert tone="danger" title={error} />
				{/if}
			</div>

			<Field label="Six-digit code">
				<Input
					inputmode="numeric"
					autocomplete="one-time-code"
					maxlength={6}
					pattern={'[0-9]{6}'}
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
					Cancel
				</Button>
				<Button
					type="submit"
					icon={icons.check}
					class="w-full sm:w-auto"
					disabled={busy || code.trim().length === 0}
				>
					{busy ? 'Checking…' : 'Confirm'}
				</Button>
			</div>
		</form>
	{/if}
</Modal>
