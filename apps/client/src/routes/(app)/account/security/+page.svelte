<script lang="ts">
	import { getContext } from 'svelte';
	import { toast } from '@facile/muse';
	import { MIN_PASSWORD } from '$lib/policy';
	import DisableTotpModal from '$lib/components/DisableTotpModal.svelte';
	import PasswordSection from '$lib/components/PasswordSection.svelte';
	import RecoveryCodesModal from '$lib/components/RecoveryCodesModal.svelte';
	import RegenerateCodesModal from '$lib/components/RegenerateCodesModal.svelte';
	import TotpEnrolModal from '$lib/components/TotpEnrolModal.svelte';
	import TwoStepSection from '$lib/components/TwoStepSection.svelte';
	import PasskeysSection from '$lib/components/PasskeysSection.svelte';
	import { t } from '$lib/i18n/i18n.svelte';
	import {
		account,
		accountError,
		ACCOUNT_CONTEXT,
		type AccountContext,
		type TOTPEnrolment
	} from '$lib/account';


	const ctx = getContext<AccountContext>(ACCOUNT_CONTEXT);

	let current = $state('');
	let passwordCode = $state('');
	let next = $state('');
	let confirm = $state('');
	let savingPassword = $state(false);
	let passwordError = $state('');

	let enrolOpen = $state(false);
	let enrolment = $state<TOTPEnrolment | null>(null);
	let enrolCode = $state('');
	let enrolBusy = $state(false);
	let enrolError = $state('');

	let codes = $state<string[]>([]);
	let codesOpen = $state(false);
	let copied = $state(false);

	let regenOpen = $state(false);
	let regenPassword = $state('');
	let regenBusy = $state(false);
	let regenError = $state('');

	let offOpen = $state(false);
	let offPassword = $state('');
	let offCode = $state('');
	let offBusy = $state(false);
	let offError = $state('');

	const enrolled = $derived(ctx.me?.totp_enrolled ?? false);
	const remaining = $derived(ctx.me?.recovery_codes_remaining ?? 0);

	async function changePassword(event: SubmitEvent) {
		event.preventDefault();
		if (savingPassword) return;
		if (next !== confirm) {
			passwordError = t('password.mismatch');
			return;
		}
		savingPassword = true;
		passwordError = '';
		try {
			await account.changePassword(current, next, passwordCode.trim());
			passwordCode = '';
			current = '';
			next = '';
			confirm = '';
			toast.success(t('password.changed'));
		} catch (cause) {
			const message = accountError(cause, t('common.failed'));
			passwordError = message === 'password_breached' ? t('password.breached') : message;
		} finally {
			savingPassword = false;
		}
	}

	async function beginEnrolment() {
		enrolment = null;
		enrolCode = '';
		enrolError = '';
		enrolOpen = true;
		try {
			enrolment = await account.beginTOTP();
		} catch (cause) {
			enrolError = accountError(cause, t('totp.startFailed'));
		}
	}

	async function confirmEnrolment(event: SubmitEvent) {
		event.preventDefault();
		if (enrolBusy) return;
		enrolBusy = true;
		enrolError = '';
		try {
			const result = await account.confirmTOTP(enrolCode.trim());
			enrolOpen = false;
			enrolment = null;
			enrolCode = '';
			showCodes(result.recovery_codes);
			await ctx.refresh();
			toast.success(t('totp.isOn'));
		} catch (cause) {
			enrolError = accountError(cause, t('login.codeRefused'));
		} finally {
			enrolBusy = false;
		}
	}

	function openRegenerate() {
		regenPassword = '';
		regenError = '';
		regenOpen = true;
	}

	async function regenerate(event: SubmitEvent) {
		event.preventDefault();
		if (regenBusy) return;
		regenBusy = true;
		regenError = '';
		try {
			const result = await account.regenerateRecoveryCodes(regenPassword);
			regenOpen = false;
			regenPassword = '';
			showCodes(result.recovery_codes);
			await ctx.refresh();
		} catch (cause) {
			regenError = accountError(cause, t('common.failed'));
		} finally {
			regenBusy = false;
		}
	}

	function openTurnOff() {
		offPassword = '';
		offCode = '';
		offError = '';
		offOpen = true;
	}

	async function turnOff(event: SubmitEvent) {
		event.preventDefault();
		if (offBusy) return;
		offBusy = true;
		offError = '';
		try {
			await account.disableTOTP(offPassword, offCode.trim());
			offOpen = false;
			offPassword = '';
			offCode = '';
			await ctx.refresh();
			toast.warning(t('totp.isOff'));
		} catch (cause) {
			offError = accountError(cause, t('common.failed'));
		} finally {
			offBusy = false;
		}
	}

	/*
	 * Recovery codes exist for exactly as long as this modal is open. Nothing
	 * caches them, because the server keeps hashes and cannot show them again.
	 */
	function showCodes(issued: string[]) {
		codes = issued;
		copied = false;
		codesOpen = true;
	}

	function forgetCodes() {
		codes = [];
		copied = false;
	}

	async function copyCodes() {
		try {
			await navigator.clipboard.writeText(codes.join('\n'));
			copied = true;
		} catch {
			toast.danger(t('totp.noClipboard'));
		}
	}
</script>

{#if ctx.me}
	<PasskeysSection me={ctx.me} onchange={ctx.refresh} />
{/if}

{#if ctx.me?.has_password}
<PasswordSection
	minLength={MIN_PASSWORD}
	saving={savingPassword}
	error={passwordError}
	bind:current
	bind:code={passwordCode}
	needsCode={enrolled}
	bind:next
	bind:confirm
	onSubmit={changePassword}
/>
{/if}

<TwoStepSection
	{enrolled}
	{remaining}
	onEnrol={beginEnrolment}
	onRegenerate={openRegenerate}
	onDisable={openTurnOff}
/>

<TotpEnrolModal
	bind:open={enrolOpen}
	{enrolment}
	bind:code={enrolCode}
	busy={enrolBusy}
	error={enrolError}
	onSubmit={confirmEnrolment}
	onCancel={() => (enrolOpen = false)}
/>

<RecoveryCodesModal
	bind:open={codesOpen}
	{codes}
	{copied}
	onCopy={copyCodes}
	onClose={forgetCodes}
/>

<RegenerateCodesModal
	bind:open={regenOpen}
	bind:password={regenPassword}
	busy={regenBusy}
	error={regenError}
	onSubmit={regenerate}
	onCancel={() => (regenOpen = false)}
/>

<DisableTotpModal
	bind:open={offOpen}
	bind:password={offPassword}
	bind:code={offCode}
	busy={offBusy}
	error={offError}
	onSubmit={turnOff}
	onCancel={() => (offOpen = false)}
/>
