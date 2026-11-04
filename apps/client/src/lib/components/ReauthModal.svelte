<!--
	Proves a factor again before a sensitive change: a passkey when the account
	has one, else the password and, when two-step sign-in is on, a code. The
	server then treats the session as fresh for ten minutes.
-->
<script lang="ts">
	import { Alert, Button, Field, Input, Modal, icons } from '@facile/muse';
	import { account, accountError, type AccountView } from '$lib/account';
	import { t } from '$lib/i18n/i18n.svelte';
	import { getPasskey, passkeysSupported } from '$lib/webauthn';

	let {
		open = $bindable(),
		me,
		onDone
	}: { open: boolean; me: AccountView; onDone: () => void } = $props();

	let password = $state('');
	let code = $state('');
	let busy = $state(false);
	let error = $state('');

	const withPasskey = $derived(me.passkeys > 0 && passkeysSupported());

	/** done closes the modal and resumes the change that asked for it. */
	function done() {
		password = '';
		code = '';
		open = false;
		onDone();
	}

	async function passkey() {
		busy = true;
		error = '';
		try {
			const options = await account.beginReauthPasskey();
			const credential = await getPasskey(options.publicKey);
			await account.finishReauthPasskey(options.ceremony, credential);
			done();
		} catch (cause) {
			error = accountError(cause, t('reauth.failed'));
		} finally {
			busy = false;
		}
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		busy = true;
		error = '';
		try {
			await account.reauthPassword(password, code.trim());
			done();
		} catch {
			error = t('reauth.failed');
		} finally {
			busy = false;
		}
	}
</script>

<Modal bind:open title={t('reauth.title')} showClose size="sm">
	<div class="flex flex-col gap-4">
		<p class="text-fc-sm text-fc-fg-muted">{t('reauth.body')}</p>
		<div aria-live="polite">
			{#if error}
				<Alert tone="danger" title={error} />
			{/if}
		</div>
		{#if withPasskey}
			<Button icon={icons.key} disabled={busy} onclick={passkey}>{t('reauth.passkey')}</Button>
		{/if}
		{#if me.has_password}
			<form class="flex flex-col gap-4" onsubmit={submit}>
				<Field label={t('common.password')}>
					<Input type="password" autocomplete="current-password" required disabled={busy} bind:value={password} />
				</Field>
				{#if me.totp_enrolled}
					<Field label={t('login.code')}>
						<Input inputmode="numeric" autocomplete="one-time-code" required disabled={busy} bind:value={code} />
					</Field>
				{/if}
				<Button type="submit" variant={withPasskey ? 'outline' : 'primary'} disabled={busy || !password}>
					{t('common.continue')}
				</Button>
			</form>
		{/if}
	</div>
</Modal>
