<script lang="ts">
	import { onMount } from 'svelte';
	import { Alert, Button, Card, Field, Input, Select, Stack, icons } from '@facile/muse';
	import TermsCheckbox from '$lib/components/TermsCheckbox.svelte';
	import { LOCALES, LOCALE_NAMES, isLocale, locale, setLocale, t, type Locale } from '$lib/i18n/i18n.svelte';
	import { MAX_PASSWORD, MIN_PASSWORD } from '$lib/policy';
	import { signup, SignupError } from '$lib/signup';
	import { createPasskey, passkeysSupported } from '$lib/webauthn';

	type Status = 'checking' | 'invalid' | 'ready' | 'busy' | 'done';

	let token = $state('');
	let email = $state('');
	let name = $state('');
	let language = $state<string>(locale());
	let accepted = $state(false);
	let usePassword = $state(false);
	let password = $state('');
	let status = $state<Status>('checking');
	let error = $state('');
	let canPasskey = $state(false);

	const passwordLength = $derived([...password].length);
	const tooShort = $derived(passwordLength > 0 && passwordLength < MIN_PASSWORD);
	const ready = $derived(name.trim() !== '' && accepted && status === 'ready');

	/**
	 * Read the token, then take it out of the address bar: it is a credential
	 * for the next thirty minutes and should not sit in history.
	 */
	onMount(async () => {
		canPasskey = passkeysSupported();
		usePassword = !canPasskey;
		token = new URLSearchParams(window.location.search).get('token') ?? '';
		history.replaceState(history.state, '', window.location.pathname);
		try {
			const pending = await signup.verify(token);
			email = pending.email;
			if (isLocale(pending.locale) && !new URLSearchParams(window.location.search).has('lang')) {
				pickLanguage(pending.locale);
			}
			status = 'ready';
		} catch {
			status = 'invalid';
		}
	});

	/** pickLanguage changes the account's language and the page's together. */
	function pickLanguage(next: Locale) {
		language = next;
		setLocale(next);
	}

	/** withPasskey registers a passkey, then creates the account with it. */
	async function withPasskey() {
		if (!ready) return;
		status = 'busy';
		error = '';
		try {
			const options = await signup.passkey(token);
			const credential = await createPasskey(options.publicKey);
			await finish({ ceremony: options.ceremony, credential });
		} catch (cause) {
			fail(cause, 'passkey');
		}
	}

	/** withPassword creates the account with a password. */
	async function withPassword(event: SubmitEvent) {
		event.preventDefault();
		if (!ready || tooShort) return;
		status = 'busy';
		error = '';
		try {
			await finish({ password });
		} catch (cause) {
			fail(cause, 'failed');
		}
	}

	/** finish sends the form and follows the answer. */
	async function finish(factor: { password?: string; ceremony?: string; credential?: Record<string, unknown> }) {
		const answer = await signup.finish({ token, name: name.trim(), locale: language, accept_terms: accepted, ...factor });
		status = 'done';
		window.location.assign(answer.redirect || '/');
	}

	/** fail explains a refusal. */
	function fail(cause: unknown, fallback: string) {
		const code = cause instanceof SignupError ? cause.code : fallback;
		const messages: Record<string, string> = {
			invalid_link: t('signup.linkGone'),
			password_breached: t('password.breached'),
			password_too_short: t('password.tooShort', { min: MIN_PASSWORD }),
			password_too_long: t('password.tooLong', { max: MAX_PASSWORD }),
			passkey: t('signup.passkeyFailed'),
			rate_limited: t('common.rateLimited')
		};
		error = messages[code] ?? t('common.failed');
		status = code === 'invalid_link' ? 'invalid' : 'ready';
	}
</script>

<svelte:head>
	<title>{t('signup.title')} — Facile</title>
</svelte:head>

<Card class="p-8">
	<Stack>
		<div class="flex flex-col gap-1">
			<h1 class="text-fc-lg font-semibold text-fc-fg">{t('signup.title')}</h1>
			{#if email}
				<p class="text-fc-sm text-fc-fg-muted">{t('signup.finishFor', { email })}</p>
			{/if}
		</div>

		{#if status === 'checking'}
			<p class="text-fc-sm text-fc-fg-muted">{t('common.loading')}</p>
		{:else if status === 'invalid'}
			<Alert tone="danger" title={t('signup.linkGone')}>
				<a href="/signup" class="underline underline-offset-2">{t('signup.again')}</a>
			</Alert>
		{:else}
			<div aria-live="polite">
				{#if error}
					<Alert tone="danger" title={error} />
				{/if}
			</div>
			<Field label={t('signup.name')}>
				<Input autocomplete="name" required maxlength={100} bind:value={name} disabled={status !== 'ready'} />
			</Field>
			<Field label={t('common.language')}>
				<Select bind:value={language} onchange={() => pickLanguage(language as Locale)} disabled={status !== 'ready'}>
					{#each LOCALES as option (option)}
						<option value={option}>{LOCALE_NAMES[option]}</option>
					{/each}
				</Select>
			</Field>
			<TermsCheckbox bind:checked={accepted} disabled={status !== 'ready'} />

			{#if !usePassword}
				<Button icon={icons.key} onclick={withPasskey} disabled={!ready}>{t('signup.withPasskey')}</Button>
				<p class="text-fc-xs text-fc-fg-muted">{t('signup.passkeyNote')}</p>
				<Button variant="ghost" onclick={() => (usePassword = true)}>{t('signup.usePassword')}</Button>
			{:else}
				<form onsubmit={withPassword} class="flex flex-col gap-4">
					<Field label={t('common.password')} helper={t('password.rule', { min: MIN_PASSWORD })} error={tooShort ? t('password.tooShort', { min: MIN_PASSWORD }) : undefined}>
						<Input
							type="password"
							autocomplete="new-password"
							required
							minlength={MIN_PASSWORD}
							maxlength={MAX_PASSWORD}
							bind:value={password}
							disabled={status !== 'ready'}
						/>
					</Field>
					<Button type="submit" disabled={!ready || tooShort || passwordLength === 0}>{t('signup.create')}</Button>
				</form>
				{#if canPasskey}
					<Button variant="ghost" onclick={() => (usePassword = false)}>{t('signup.usePasskey')}</Button>
				{/if}
			{/if}
		{/if}
	</Stack>
</Card>
