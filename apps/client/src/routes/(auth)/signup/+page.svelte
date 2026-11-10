<script lang="ts">
	import { onMount } from 'svelte';
	import { Alert, Button, Field, Input, Stack } from '@facile/muse';
	import { fetchAndSolve } from '$lib/altcha';
	import { t } from '$lib/i18n/i18n.svelte';
	import { signup, SignupError } from '$lib/signup';
	import { invitationPath, rememberNext } from '$lib/next';

	type Status = 'ready' | 'proving' | 'sending' | 'sent' | 'closed';

	let email = $state('');
	let authRequest = $state('');
	let invitation = $state('');
	let status = $state<Status>('ready');
	let error = $state('');

	onMount(async () => {
		const query = new URLSearchParams(window.location.search);
		authRequest = query.get('auth_request') ?? '';
		invitation = invitationPath(query.get('invitation')) ? (query.get('invitation') ?? '') : '';
		rememberNext(invitationPath(invitation));
		const config = await fetch('/api/config')
			.then((r) => r.json())
			.catch(() => null);
		if (config && config.signup !== true) status = 'closed';
	});

	/**
	 * submit solves the proof of work, then asks for the link. The answer is
	 * the same whether or not the address already has an account, so the page
	 * only ever says "check your inbox".
	 */
	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (status !== 'ready') return;
		error = '';
		status = 'proving';
		try {
			const proof = await fetchAndSolve();
			status = 'sending';
			await signup.start(email.trim(), proof, authRequest, invitation);
			status = 'sent';
		} catch (cause) {
			error = explain(cause);
			status = 'ready';
		}
	}

	/** explain turns a refusal into a sentence. */
	function explain(cause: unknown): string {
		if (cause instanceof SignupError && cause.code === 'email') return t('signup.badEmail');
		if (cause instanceof SignupError && cause.code === 'rate_limited') return t('common.rateLimited');
		if (cause instanceof SignupError && cause.status === 404) return t('signup.closed');
		return t('common.failed');
	}
</script>

<svelte:head>
	<title>{t('signup.title')} — Facile</title>
</svelte:head>

<Stack>
	<div class="flex flex-col items-center gap-1.5 text-center">
		<h1 class="font-heading text-2xl font-semibold tracking-tight text-fc-fg">{t('signup.title')}</h1>
		<p class="text-fc-sm text-fc-fg-muted">{t('signup.subtitle')}</p>
	</div>

	{#if status === 'closed'}
		<Alert tone="info" title={t('signup.closed')} />
	{:else if status === 'sent'}
		<Alert tone="success" title={t('signup.sentTitle')}>{t('signup.sentBody', { email: email.trim() })}</Alert>
	{:else}
		<form onsubmit={submit} class="flex flex-col gap-4">
			<div aria-live="polite">
				{#if error}
					<Alert tone="danger" title={error} />
				{/if}
			</div>
			<Field label={t('common.email')}>
				<Input type="email" autocomplete="email" required bind:value={email} disabled={status !== 'ready'} />
			</Field>
			<Button type="submit" disabled={status !== 'ready' || !email.trim()}>
				{status === 'proving' ? t('signup.proving') : status === 'sending' ? t('common.sending') : t('signup.continue')}
			</Button>
			<p class="text-fc-xs text-fc-fg-muted">{t('signup.powNote')}</p>
		</form>
	{/if}

	<p class="text-center text-fc-sm text-fc-fg-muted">
		{t('signup.haveAccount')}
		<a href="/login" class="font-medium text-fc-fg underline underline-offset-2">{t('login.title')}</a>
	</p>
</Stack>
