<!--
	The account's passkeys: list, add, rename, remove. Adding and removing
	need a factor proven in the last ten minutes; when the server says so the
	reauthentication modal opens and the change resumes after it.
-->
<script lang="ts">
	import { Button, Input, SettingsRow, SettingsSection, icons, toast } from '@facile/muse';
	import { account, accountError, AccountError, type AccountView, type PasskeyView } from '$lib/account';
	import { locale, t } from '$lib/i18n/i18n.svelte';
	import { createPasskey, passkeysSupported } from '$lib/webauthn';
	import ReauthModal from './ReauthModal.svelte';

	let { me, onchange }: { me: AccountView; onchange: () => Promise<void> } = $props();

	let keys = $state<PasskeyView[]>([]);
	let busy = $state(false);
	let label = $state('');
	let reauthOpen = $state(false);
	let pending = $state<(() => Promise<void>) | null>(null);

	$effect(() => {
		load();
	});

	async function load() {
		keys = (await account.passkeys().catch(() => ({ passkeys: [] }))).passkeys;
	}

	/** guarded runs change, and on reauth_required asks for a factor and runs it again. */
	async function guarded(change: () => Promise<void>) {
		busy = true;
		try {
			await change();
			await load();
			await onchange();
		} catch (cause) {
			if (cause instanceof AccountError && cause.message === 'reauth_required') {
				pending = change;
				reauthOpen = true;
			} else if (cause instanceof AccountError && cause.message === 'last_factor') {
				toast.danger(t('passkeys.lastFactor'));
			} else {
				toast.danger(accountError(cause, t('passkeys.failed')));
			}
		} finally {
			busy = false;
		}
	}

	function add() {
		guarded(async () => {
			const options = await account.beginPasskey();
			const credential = await createPasskey(options.publicKey);
			await account.finishPasskey(options.ceremony, credential, label.trim());
			label = '';
			toast.success(t('passkeys.added'));
		});
	}

	function remove(key: PasskeyView) {
		guarded(async () => {
			await account.removePasskey(key.id);
			toast.success(t('passkeys.removed', { name: key.name }));
		});
	}

	function rename(key: PasskeyView) {
		const next = window.prompt(t('passkeys.renamePrompt'), key.name);
		if (next?.trim()) guarded(() => account.renamePasskey(key.id, next.trim()).then(() => undefined));
	}

	function resume() {
		const change = pending;
		pending = null;
		if (change) guarded(change);
	}

	const date = (value: string) => new Date(value).toLocaleDateString(locale());
</script>

<SettingsSection title={t('passkeys.title')} description={t('passkeys.description')}>
	{#each keys as key (key.id)}
		<SettingsRow
			label={key.name}
			description={key.last_used_at
				? t('passkeys.used', { date: date(key.last_used_at) })
				: t('passkeys.created', { date: date(key.created_at) })}
		>
			<div class="flex gap-2">
				<Button variant="ghost" icon={icons.edit} disabled={busy} onclick={() => rename(key)}>{t('passkeys.rename')}</Button>
				<Button variant="ghost-danger" icon={icons.remove} disabled={busy} onclick={() => remove(key)}>{t('passkeys.remove')}</Button>
			</div>
		</SettingsRow>
	{:else}
		<SettingsRow label={t('passkeys.none')} description={t('passkeys.noneHint')}>
			<span></span>
		</SettingsRow>
	{/each}
	{#if passkeysSupported()}
		<SettingsRow label={t('passkeys.add')} description={t('passkeys.addHint')}>
			<div class="flex flex-col gap-2 sm:flex-row">
				<Input aria-label={t('passkeys.name')} placeholder={t('passkeys.namePlaceholder')} maxlength={64} bind:value={label} disabled={busy} />
				<Button icon={icons.plus} disabled={busy} onclick={add}>{t('passkeys.add')}</Button>
			</div>
		</SettingsRow>
	{/if}
</SettingsSection>

<ReauthModal bind:open={reauthOpen} {me} onDone={resume} />
