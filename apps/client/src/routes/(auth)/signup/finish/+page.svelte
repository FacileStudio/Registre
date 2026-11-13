<script lang="ts">
	import { invitationPath, landing, rememberNext } from '$lib/next';
	import { onMount } from 'svelte';
	import { Alert, Button, Field, Input, Select, Stack } from '@facile/muse';
	import SignupRequest from '$lib/components/SignupRequest.svelte';
	import TermsCheckbox from '$lib/components/TermsCheckbox.svelte';
	import { LOCALES, LOCALE_NAMES, isLocale, locale, setLocale, t, type Locale } from '$lib/i18n/i18n.svelte';
	import { MAX_PASSWORD, MIN_PASSWORD } from '$lib/policy';
	import { signup, SignupError } from '$lib/signup';

	type Status = 'checking' | 'invalid' | 'unchecked' | 'ready' | 'busy' | 'done';

	let token = $state('');
	let email = $state('');
	let name = $state('');
	let language = $state<string>(locale());
	let accepted = $state(false);
	let password = $state('');
	let status = $state<Status>('checking');
	let error = $state('');
	let invitation = $state('');

	const passwordLength = $derived([...password].length);
	const tooShort = $derived(passwordLength > 0 && passwordLength < MIN_PASSWORD);
	const ready = $derived(name.trim() !== '' && accepted && status === 'ready');

	/**
	 * Read the token, then take it out of the address bar: it is a credential
	 * for the next thirty minutes and should not sit in history. A link the
	 * server calls gone (410) gets the page that asks for a new one; any other
	 * failure says the check itself failed, so a person with a good link is
	 * not told to start over.
	 */
	onMount(async () => {
		token = new URLSearchParams(window.location.search).get('token') ?? '';
		const invited = new URLSearchParams(window.location.search).get('invitation');
		invitation = invitationPath(invited) ? (invited ?? '') : '';
		rememberNext(invitationPath(invited));
		history.replaceState(history.state, '', window.location.pathname);
		try {
			const pending = await signup.verify(token);
			email = pending.email;
			if (isLocale(pending.locale) && !new URLSearchParams(window.location.search).has('lang')) {
				pickLanguage(pending.locale);
			}
			status = 'ready';
		} catch (cause) {
			status = cause instanceof SignupError && cause.status === 410 ? 'invalid' : 'unchecked';
		}
	});

	/** pickLanguage changes the account's language and the page's together. */
	function pickLanguage(next: Locale) {
		language = next;
		setLocale(next);
	}

	/**
	 * create makes the account with a password, then follows the answer. A
	 * passkey is added later from the account page.
	 */
	async function create(event: SubmitEvent) {
		event.preventDefault();
		if (!ready || tooShort) return;
		status = 'busy';
		error = '';
		try {
			const answer = await signup.finish({ token, name: name.trim(), locale: language, accept_terms: accepted, password });
			status = 'done';
			window.location.assign(landing(answer.redirect || '/'));
		} catch (cause) {
			fail(cause);
		}
	}

	/** fail explains a refusal. */
	function fail(cause: unknown) {
		const code = cause instanceof SignupError ? cause.code : 'failed';
		const messages: Record<string, string> = {
			invalid_link: t('signup.linkGone'),
			password_breached: t('password.breached'),
			password_too_short: t('password.tooShort', { min: MIN_PASSWORD }),
			password_too_long: t('password.tooLong', { max: MAX_PASSWORD }),
			rate_limited: t('common.rateLimited')
		};
		error = messages[code] ?? t('common.failed');
		status = code === 'invalid_link' ? 'invalid' : 'ready';
	}
</script>

<svelte:head>
	<title>{t('signup.title')} — Facile</title>
</svelte:head>

<Stack>
		<div class="flex flex-col items-center gap-1.5 text-center">
			<h1 class="font-heading text-2xl font-semibold tracking-tight text-fc-fg">{t('signup.title')}</h1>
			{#if email}
				<p class="text-fc-sm text-fc-fg-muted">{t('signup.finishFor', { email })}</p>
			{/if}
		</div>

		{#if status === 'checking'}
			<p class="text-fc-sm text-fc-fg-muted">{t('common.loading')}</p>
		{:else if status === 'unchecked'}
			<Alert tone="danger" title={t('signup.checkFailed')} />
		{:else if status === 'invalid'}
			<Alert tone="warning" title={t('signup.linkGoneTitle')}>{t('signup.linkGone')}</Alert>
			<Button variant="secondary" href="/login">{t('login.title')}</Button>
			<SignupRequest {invitation} submitLabel={t('signup.resend')} />
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

			<form onsubmit={create} class="flex flex-col gap-4">
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
		{/if}
	</Stack>
