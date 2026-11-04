<!--
	The two-step sign-in status block: whether the factor is on, how many recovery
	codes are left, and the buttons that open the enrolment, regeneration and
	disable flows. The page owns those flows and passes them in as callbacks.
-->
<script lang="ts">
	import { t } from '$lib/i18n/i18n.svelte';
	import { Badge, Button, SettingsRow, SettingsSection, icons } from '@facile/muse';

	let {
		enrolled,
		remaining,
		onEnrol,
		onRegenerate,
		onDisable
	}: {
		enrolled: boolean;
		remaining: number;
		onEnrol: () => void;
		onRegenerate: () => void;
		onDisable: () => void;
	} = $props();
</script>

<SettingsSection
	title={t('account.twoStep')}
	description={t('totp.description')}
>
	<SettingsRow label={t('totp.status')} description={t('totp.statusHint')}>
		{#if enrolled}
			<Badge tone="success">{t('common.on')}</Badge>
		{:else}
			<Badge tone="neutral">{t('common.off')}</Badge>
		{/if}
	</SettingsRow>

	{#if enrolled}
		<SettingsRow
			label={t('totp.codes')}
			description={t('totp.codesHint')}
		>
			<div class="flex items-center gap-3">
				<span class="text-fc-sm tabular-nums text-fc-fg-muted">{t('totp.left', { count: remaining })}</span>
				<Button variant="outline" icon={icons.refresh} onclick={onRegenerate}>
					{t('totp.issue')}
				</Button>
			</div>
		</SettingsRow>
		<SettingsRow
			label={t('totp.offLabel')}
			description={t('totp.offHint')}
		>
			<Button variant="ghost-danger" icon={icons.revoke} onclick={onDisable}>{t('totp.turnOff')}</Button>
		</SettingsRow>
	{:else}
		<SettingsRow
			label={t('totp.onLabel')}
			description={t('totp.onHint')}
		>
			<Button icon={icons.shield} onclick={onEnrol}>{t('totp.turnOn')}</Button>
		</SettingsRow>
	{/if}
</SettingsSection>
