<!--
	Asks for the password before issuing a fresh set of recovery codes, and says
	up front that the codes the reader holds now stop working. The page makes the
	call and shows the new set.
-->
<script lang="ts">
	import { t } from '$lib/i18n/i18n.svelte';
	import { Alert, Button, Field, Input, Modal, icons } from '@facile/muse';

	let {
		open = $bindable(),
		password = $bindable(),
		busy,
		error,
		onSubmit,
		onCancel
	}: {
		open: boolean;
		password: string;
		busy: boolean;
		error: string;
		onSubmit: (event: SubmitEvent) => void;
		onCancel: () => void;
	} = $props();
</script>

<Modal bind:open title={t('totp.issueTitle')} showClose size="sm">
	<form class="flex flex-col gap-4" onsubmit={onSubmit}>
		<Alert tone="warning" title={t('totp.oldStop')}>{t('totp.oldStopBody')}</Alert>
		<div aria-live="polite">
			{#if error}
				<Alert tone="danger" title={error} />
			{/if}
		</div>
		<Field label={t('password.yours')} helper={t('totp.issuePasswordHint')}>
			<Input
				type="password"
				autocomplete="current-password"
				required
				disabled={busy}
				bind:value={password}
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
				{t('common.cancel')}
			</Button>
			<Button
				type="submit"
				icon={icons.refresh}
				class="w-full sm:w-auto"
				disabled={busy || password.length === 0}
			>
				{busy ? t('common.saving') : t('totp.issue')}
			</Button>
		</div>
	</form>
</Modal>
