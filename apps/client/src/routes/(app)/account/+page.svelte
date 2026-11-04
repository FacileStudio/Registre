<script lang="ts">
	import { getContext } from 'svelte';
	import { Badge, Button, Input, Select, SettingsRow, SettingsSection, icons, toast } from '@facile/muse';
	import { account, accountError, ACCOUNT_CONTEXT, type AccountContext } from '$lib/account';
	import PictureSection from '$lib/components/PictureSection.svelte';
	import { LOCALES, LOCALE_NAMES, isLocale, setLocale, t } from '$lib/i18n/i18n.svelte';

	const ctx = getContext<AccountContext>(ACCOUNT_CONTEXT);

	let leaving = $state(false);
	let saving = $state(false);
	let name = $state('');
	let language = $state('en');

	$effect(() => {
		name = ctx.me?.name ?? '';
		language = ctx.me?.locale ?? 'en';
	});

	const changed = $derived(
		!!ctx.me && name.trim() !== '' && (name.trim() !== ctx.me.name || language !== ctx.me.locale)
	);

	/** save stores the name and language; the language also reaches every app as `locale`. */
	async function save(event: SubmitEvent) {
		event.preventDefault();
		if (!changed || saving) return;
		saving = true;
		try {
			await account.updateProfile(name.trim(), language);
			if (isLocale(language)) setLocale(language);
			toast.success(t('account.saved'));
			await ctx.refresh();
		} catch (cause) {
			toast.danger(accountError(cause, t('common.failed')));
		} finally {
			saving = false;
		}
	}

	async function signOut() {
		leaving = true;
		try {
			await account.logout();
			window.location.assign('/');
		} catch (cause) {
			toast.danger(accountError(cause, t('account.signOutFailed')));
			leaving = false;
		}
	}
</script>

{#if ctx.me}
	<PictureSection me={ctx.me} onchange={ctx.refresh} />
{/if}

<form onsubmit={save}>
	<SettingsSection title={t('account.details')} description={t('account.detailsHint')}>
		<SettingsRow label={t('common.email')}>
			<span class="font-fc-mono text-fc-sm text-fc-fg">{ctx.me?.email ?? '—'}</span>
		</SettingsRow>
		<SettingsRow label={t('signup.name')}>
			<Input aria-label={t('signup.name')} maxlength={100} bind:value={name} disabled={saving} />
		</SettingsRow>
		<SettingsRow label={t('common.language')} description={t('account.languageHint')}>
			<Select aria-label={t('common.language')} bind:value={language} disabled={saving}>
				{#each LOCALES as option (option)}
					<option value={option}>{LOCALE_NAMES[option]}</option>
				{/each}
			</Select>
		</SettingsRow>
		<SettingsRow label={t('account.twoStep')} description={t('account.twoStepHint')}>
			{#if ctx.me?.totp_enrolled}
				<Badge tone="success">{t('common.on')}</Badge>
			{:else}
				<Badge tone="neutral">{t('common.off')}</Badge>
			{/if}
		</SettingsRow>
		<SettingsRow label="">
			<Button type="submit" disabled={!changed || saving}>{saving ? t('common.saving') : t('common.save')}</Button>
		</SettingsRow>
	</SettingsSection>
</form>

<SettingsSection title={t('account.session')} description={t('account.sessionHint')}>
	<SettingsRow label={t('account.signOut')} description={t('account.signOutHint')}>
		<Button variant="outline" icon={icons.logout} disabled={leaving} onclick={signOut}>
			{leaving ? t('account.signingOut') : t('account.signOut')}
		</Button>
	</SettingsRow>
</SettingsSection>
