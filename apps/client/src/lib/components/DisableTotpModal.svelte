<!--
	Turning two-step sign-in off. It costs what turning it on cost, a password and
	one live code, and it says what the account is left with afterwards. The page
	makes the call.
-->
<script lang="ts">
	import { t } from '$lib/i18n/i18n.svelte';
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

<Modal bind:open title={t('totp.offLabel')} showClose size="sm">
	<form class="flex flex-col gap-4" onsubmit={onSubmit}>
		<Alert tone="danger" title={t('totp.onlyGuard')}>{t('totp.onlyGuardBody')}</Alert>
		<div aria-live="polite">
			{#if error}
				<Alert tone="danger" title={error} />
			{/if}
		</div>
		<Field label={t('password.yours')}>
			<Input
				type="password"
				autocomplete="current-password"
				required
				disabled={busy}
				bind:value={password}
			/>
		</Field>
		<Field label={t('login.code')} helper={t('totp.recoveryWorks')}>
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
				{t('totp.keepOn')}
			</Button>
			<Button
				type="submit"
				variant="danger"
				icon={icons.revoke}
				class="w-full sm:w-auto"
				disabled={busy || password.length === 0 || code.trim().length === 0}
			>
				{busy ? t('common.saving') : t('totp.turnOff')}
			</Button>
		</div>
	</form>
</Modal>
