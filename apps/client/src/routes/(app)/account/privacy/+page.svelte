<script lang="ts">
	import { getContext } from 'svelte';
	import { Alert, Button, Field, Input, SettingsRow, SettingsSection, icons, toast } from '@facile/muse';
	import { account, accountError, AccountError, ACCOUNT_CONTEXT, type AccountContext } from '$lib/account';
	import LegalLinks from '$lib/components/LegalLinks.svelte';
	import ReauthModal from '$lib/components/ReauthModal.svelte';
	import { locale, t } from '$lib/i18n/i18n.svelte';

	const ctx = getContext<AccountContext>(ACCOUNT_CONTEXT);

	let confirmEmail = $state('');
	let busy = $state(false);
	let reauthOpen = $state(false);

	const scheduled = $derived(ctx.me?.deletion_scheduled_for ?? null);
	const matches = $derived(
		!!ctx.me && confirmEmail.trim().toLowerCase() === ctx.me.email.toLowerCase()
	);

	/** requestDeletion schedules the erasure, reauthenticating first when asked. */
	async function requestDeletion() {
		if (!matches || busy) return;
		busy = true;
		try {
			await account.requestDeletion(confirmEmail.trim());
			confirmEmail = '';
			toast.warning(t('privacy.scheduled'));
			await ctx.refresh();
		} catch (cause) {
			if (cause instanceof AccountError && cause.message === 'reauth_required') {
				reauthOpen = true;
			} else {
				toast.danger(accountError(cause, t('common.failed')));
			}
		} finally {
			busy = false;
		}
	}

	async function cancelDeletion() {
		busy = true;
		try {
			await account.cancelDeletion();
			toast.success(t('privacy.cancelled'));
			await ctx.refresh();
		} catch (cause) {
			toast.danger(accountError(cause, t('common.failed')));
		} finally {
			busy = false;
		}
	}
</script>

<SettingsSection title={t('privacy.export')} description={t('privacy.exportHint')}>
	<SettingsRow label={t('privacy.download')}>
		<Button variant="outline" icon={icons.download} href="/api/account/export">{t('privacy.download')}</Button>
	</SettingsRow>
</SettingsSection>

<SettingsSection title={t('privacy.legal')} description={t('privacy.legalHint')}>
	<SettingsRow label="">
		<LegalLinks />
	</SettingsRow>
</SettingsSection>

<SettingsSection title={t('privacy.delete')} description={t('privacy.deleteHint')}>
	{#if scheduled}
		<Alert tone="warning" title={t('privacy.pending', { date: new Date(scheduled).toLocaleDateString(locale()) })}>
			<Button variant="outline" disabled={busy} onclick={cancelDeletion}>{t('privacy.cancel')}</Button>
		</Alert>
	{:else}
		<Field label={t('privacy.confirmLabel', { email: ctx.me?.email ?? '' })}>
			<Input type="email" autocomplete="off" bind:value={confirmEmail} disabled={busy} />
		</Field>
		<Button variant="danger" icon={icons.remove} disabled={!matches || busy} onclick={requestDeletion}>
			{t('privacy.deleteButton')}
		</Button>
	{/if}
</SettingsSection>

{#if ctx.me}
	<ReauthModal bind:open={reauthOpen} me={ctx.me} onDone={requestDeletion} />
{/if}
