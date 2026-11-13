<script lang="ts">
	import { onDestroy } from 'svelte';
	import { Alert, Button, Field, Input } from '@facile/muse';
	import { fetchAndSolve } from '$lib/altcha';
	import { t } from '$lib/i18n/i18n.svelte';
	import { signup, SignupError } from '$lib/signup';

	/**
	 * SignupRequest asks for a sign-up link and then stays on the "check your
	 * inbox" state: where the mail went and to look in spam, a resend button
	 * behind a visible cooldown, and a way to change the address. The answer
	 * is the same whether or not the address already has an account, so the
	 * sent state says nothing about which one it was.
	 */
	interface Props {
		authRequest?: string;
		invitation?: string;
		submitLabel?: string;
	}

	let { authRequest = '', invitation = '', submitLabel = t('signup.continue') }: Props = $props();

	const RESEND_COOLDOWN = 60;

	type Status = 'ready' | 'proving' | 'sending' | 'sent';

	let email = $state('');
	let status = $state<Status>('ready');
	let resending = $state(false);
	let resent = $state(false);
	let error = $state('');
	let cooldown = $state(0);
	let timer: ReturnType<typeof setInterval> | undefined;

	onDestroy(() => clearInterval(timer));

	/** wait starts the countdown that gates the resend button. */
	function wait(seconds: number) {
		clearInterval(timer);
		cooldown = seconds;
		timer = setInterval(() => {
			cooldown = Math.max(0, cooldown - 1);
			if (cooldown === 0) clearInterval(timer);
		}, 1000);
	}

	/** request solves the proof of work, then asks for the link. */
	async function request(): Promise<boolean> {
		error = '';
		try {
			const proof = await fetchAndSolve();
			if (status !== 'sent') status = 'sending';
			await signup.start(email.trim(), proof, authRequest, invitation);
			wait(RESEND_COOLDOWN);
			return true;
		} catch (cause) {
			error = explain(cause);
			if (cause instanceof SignupError && cause.retryAfter > 0) wait(cause.retryAfter);
			return false;
		}
	}

	/** submit sends the first link. */
	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (status !== 'ready') return;
		status = 'proving';
		status = (await request()) ? 'sent' : 'ready';
	}

	/** resend sends another link once the cooldown is over. */
	async function resend() {
		if (resending || cooldown > 0) return;
		resending = true;
		resent = false;
		resent = await request();
		resending = false;
	}

	/** restart goes back to the address field. */
	function restart() {
		status = 'ready';
		error = '';
		resent = false;
	}

	/** waitLabel spells a wait in seconds under ninety, in minutes above. */
	function waitLabel(seconds: number): string {
		return seconds > 90 ? `${Math.ceil(seconds / 60)} min` : `${seconds} s`;
	}

	/** explain turns a refusal into a sentence that says what to do next. */
	function explain(cause: unknown): string {
		if (!(cause instanceof SignupError)) return t('common.failed');
		if (cause.code === 'email') return t('signup.badEmail');
		if (cause.code === 'mail_failed') return t('signup.mailFailed');
		if (cause.status === 404) return t('signup.closed');
		if (cause.code === 'rate_limited' && cause.retryAfter > 90) {
			return t('signup.tooManyLinks', { minutes: Math.ceil(cause.retryAfter / 60) });
		}
		if (cause.code === 'rate_limited') return t('common.rateLimited');
		return t('common.failed');
	}
</script>

<div aria-live="polite">
	{#if error}
		<Alert tone="danger" title={error} />
	{:else if resent}
		<Alert tone="success" title={t('signup.resent')} />
	{/if}
</div>

{#if status === 'sent'}
	<Alert tone="success" title={t('signup.sentTitle')}>{t('signup.sentBody', { email: email.trim() })}</Alert>
	<Button variant="secondary" onclick={resend} disabled={resending || cooldown > 0}>
		{resending ? t('common.sending') : cooldown > 0 ? t('signup.resendIn', { wait: waitLabel(cooldown) }) : t('signup.resend')}
	</Button>
	<Button variant="ghost" onclick={restart} disabled={resending}>{t('signup.otherAddress')}</Button>
{:else}
	<form onsubmit={submit} class="flex flex-col gap-4">
		<Field label={t('common.email')}>
			<Input type="email" autocomplete="email" required bind:value={email} disabled={status !== 'ready'} />
		</Field>
		<Button type="submit" disabled={status !== 'ready' || !email.trim() || cooldown > 0}>
			{status === 'proving'
				? t('signup.proving')
				: status === 'sending'
					? t('common.sending')
					: cooldown > 0
						? t('signup.resendIn', { wait: waitLabel(cooldown) })
						: submitLabel}
		</Button>
		<p class="text-fc-xs text-fc-fg-muted">{t('signup.powNote')}</p>
	</form>
{/if}
