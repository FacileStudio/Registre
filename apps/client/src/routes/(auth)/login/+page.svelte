<script lang="ts">
	import { onMount } from 'svelte';
	import { Alert, Button, Card, Divider, Field, Icon, Input, Stack, icons } from '@facile/muse';
	import { t } from '$lib/i18n/i18n.svelte';
	import { getPasskey, passkeysSupported } from '$lib/webauthn';
	import { signupOpen as isSignupOpen } from '$lib/signup';

	type Step = 'password' | 'second-factor';
	type AuthStatus = 'checking' | 'redirecting' | 'handed-off' | 'ready' | 'submitting';

	let authRequest = $state('');
	let email = $state('');
	let password = $state('');
	let code = $state('');
	let challenge = $state('');
	let step = $state<Step>('password');
	let recovery = $state(false);
	let error = $state('');
	let status = $state<AuthStatus>('checking');
	let signupOpen = $state(false);
	let canPasskey = $state(false);

	/**
	 * REDIRECTED is what this page says when a sign-in was refused elsewhere and
	 * sent the browser here without a reason worth repeating.
	 *
	 * porte answers a failed browser login with a redirect to
	 * `/login?error=<reason>` rather than a JSON body, because the caller is a
	 * window and not a program. A login page that ignores that parameter throws
	 * away the only explanation the human is ever given and shows them a blank
	 * form instead, which reads as though nothing happened at all.
	 */

	/**
	 * PLAIN_REASON is the shape of a reason this page is willing to print back,
	 * and the guard is the point rather than tidiness.
	 *
	 * Anyone can put any text on the end of a link to this page, and this page
	 * belongs to the identity provider, on its own domain and its own
	 * certificate. Echoing whatever arrives would hand a stranger a message box
	 * on the most trusted login screen in the suite, which is a phishing lure
	 * with registre's name on it. porte's own reasons are short lowercase
	 * sentences, so anything that is not one gets REDIRECTED and nothing real is
	 * lost.
	 */
	const PLAIN_REASON = /^[a-z][a-z ,.'-]{2,79}$/i;

	/**
	 * HAND_OFF_MS is how long a redirect may leave this page alive before it
	 * counts as handed to a native app.
	 *
	 * A desktop or mobile app ends its sign-in on a custom-scheme URL. The
	 * browser hands that URL to the app and keeps this document open, so the
	 * spinner would turn forever. A web redirect unloads the page well before
	 * the timer fires, so only the native case ever sees the done state.
	 */
	const HAND_OFF_MS = 4000;

	/**
	 * redirect sends the browser on and, if the page is still here after
	 * HAND_OFF_MS, tells the human they are signed in and can close the tab.
	 */
	function redirect(target: string) {
		status = 'redirecting';
		window.location.assign(target);
		setTimeout(() => {
			if (status === 'redirecting') status = 'handed-off';
		}, HAND_OFF_MS);
	}

	/**
	 * refusal turns the `error` on the URL into a line this page will show,
	 * capitalised so it sits beside the messages above it.
	 */
	function refusal(reason: string): string {
		if (!reason) return '';
		if (!PLAIN_REASON.test(reason)) return t('login.redirected');
		return reason[0].toUpperCase() + reason.slice(1);
	}

	onMount(() => {
		const query = new URLSearchParams(window.location.search);
		const req = query.get('auth_request') ?? '';
		authRequest = req;
		error = refusal(query.get('error') ?? '');
		canPasskey = passkeysSupported();
		checkSession(req);
		isSignupOpen().then((open) => (signupOpen = open));
	});

	/**
	 * signInWithPasskey runs a discoverable assertion: no address to type, the
	 * authenticator offers the passkeys it holds for this site. A passkey is a
	 * second factor on its own, so the answer is always a redirect.
	 */
	async function signInWithPasskey() {
		if (status === 'submitting' || status === 'redirecting') return;
		status = 'submitting';
		error = '';
		try {
			const begin = await fetch('/api/login/passkey/begin', { method: 'POST', body: '{}' });
			const options = await begin.json();
			if (!begin.ok) throw new Error('begin');
			const credential = await getPasskey(options.publicKey);
			const response = await fetch('/api/login/passkey/finish', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ auth_request: authRequest, ceremony: options.ceremony, credential })
			});
			const body = await response.json().catch(() => null);
			if (!response.ok) {
				error = body?.error === 'no_access' ? t('login.noAccess') : t('login.passkeyRefused');
				status = 'ready';
				return;
			}
			redirect(body?.redirect || '/');
		} catch {
			error = t('login.passkeyCancelled');
			status = 'ready';
		}
	}

	async function checkSession(req: string) {
		try {
			const response = req
				? await fetch('/api/login/session', {
						method: 'POST',
						credentials: 'include',
						headers: { 'Content-Type': 'application/json', 'X-Facile-CSRF': '1' },
						body: JSON.stringify({ auth_request: req })
					})
				: await fetch('/api/login/session', { method: 'GET', credentials: 'include' });
			const body = await response.json().catch(() => null);
			if (response.ok && body?.redirect) {
				redirect(body.redirect);
				return;
			}
			if (body?.error === 'no_access') error = t('login.noAccess');
			status = 'ready';
		} catch {
			status = 'ready';
		}
	}

	async function submitPassword(event: SubmitEvent) {
		event.preventDefault();
		if (status === 'submitting' || status === 'redirecting') return;
		status = 'submitting';
		error = '';
		try {
			const response = await fetch('/api/login', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ auth_request: authRequest, email, password })
			});
			const body = await response.json().catch(() => null);
			if (!response.ok) {
				error = body?.error === 'no_access' ? t('login.noAccess') : t('login.refused');
				status = 'ready';
				return;
			}
			if (body?.challenge) {
				challenge = body.challenge;
				password = '';
				step = 'second-factor';
				status = 'ready';
				return;
			}
			redirect(body?.redirect || '/');
		} catch {
			error = t('common.failed');
			status = 'ready';
		}
	}

	async function submitCode(event: SubmitEvent) {
		event.preventDefault();
		if (!challenge || status === 'submitting' || status === 'redirecting') return;
		status = 'submitting';
		error = '';
		try {
			const response = await fetch('/api/login/totp', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ challenge, code: code.trim() })
			});
			const body = await response.json().catch(() => null);
			if (!response.ok) {
				error = body?.error === 'no_access' ? t('login.noAccess') : t('login.codeRefused');
				code = '';
				status = 'ready';
				return;
			}
			redirect(body?.redirect || '/');
		} catch {
			error = t('common.failed');
			status = 'ready';
		}
	}

	function useRecovery() {
		recovery = !recovery;
		code = '';
		error = '';
	}

	function startOver() {
		step = 'password';
		challenge = '';
		code = '';
		recovery = false;
		error = '';
		status = 'ready';
	}
</script>

<svelte:head>
	<title>{t('login.title')} — Facile</title>
</svelte:head>

<Card class="p-8">
	{#if status === 'handed-off'}
		<div class="stage py-8">
			<Stack gap="content" class="items-center text-center">
				<div class="flex size-12 items-center justify-center rounded-full bg-primary/10 text-primary">
					<Icon icon={icons.check} size={24} />
				</div>
				<div class="flex flex-col gap-1.5 pt-2">
					<h1 class="text-base font-medium text-foreground">{t('login.handedOff')}</h1>
					<p class="text-xs text-muted-foreground">{t('login.handedOffHint')}</p>
				</div>
			</Stack>
		</div>
	{:else if status === 'checking' || status === 'redirecting'}
		<div class="stage py-8">
			<Stack gap="content" class="items-center text-center">
				<div class="relative flex size-12 items-center justify-center">
					<div class="absolute inset-0 size-12 animate-ping rounded-full bg-primary/20 opacity-75"></div>
					<div class="size-8 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
				</div>
				<div class="flex flex-col gap-1.5 pt-2">
					<h1 class="text-base font-medium text-foreground">
						{status === 'redirecting' ? t('login.connecting') : t('login.signingIn')}
					</h1>
					<p class="text-xs text-muted-foreground">
						{status === 'redirecting' ? t('login.redirecting') : t('login.reusing')}
					</p>
				</div>
			</Stack>
		</div>
	{:else if step === 'password'}
		<div class="stage">
			<Stack>
				<div class="flex flex-col gap-1">
					<h1 class="text-fc-lg font-semibold text-fc-fg">{t('login.title')}</h1>
					<p class="text-fc-sm text-fc-fg-muted">
						{authRequest ? t('login.fromApp') : t('login.subtitle')}
					</p>
				</div>

				{#if canPasskey}
					<Button icon={icons.key} onclick={signInWithPasskey} disabled={status === 'submitting'}>
						{t('login.passkey')}
					</Button>
					<div class="flex items-center gap-3 text-fc-xs text-fc-fg-muted">
						<Divider class="flex-1" />{t('common.or')}<Divider class="flex-1" />
					</div>
				{/if}

				<form onsubmit={submitPassword} class="flex flex-col gap-4">
					<div aria-live="polite">
						{#if error}
							<Alert tone="danger" title={error} />
						{/if}
					</div>
					<Field label={t('common.email')}>
						<Input type="email" autocomplete="username" required bind:value={email} />
					</Field>
					<Field label={t('common.password')}>
						<Input type="password" autocomplete="current-password" required bind:value={password} />
					</Field>
					<Button type="submit" variant={canPasskey ? 'outline' : 'primary'} disabled={status === 'submitting'}>
						{status === 'submitting' ? t('login.submitting') : t('login.submit')}
					</Button>
					<a
						href="/recover"
						class="self-center rounded-fc-xs text-fc-sm text-fc-fg-muted underline
							underline-offset-2 transition-colors hover:text-fc-fg focus-visible:outline-2
							focus-visible:outline-offset-2 focus-visible:outline-fc-ring"
					>
						{t('login.forgot')}
					</a>
				</form>
				{#if signupOpen}
					<p class="text-center text-fc-sm text-fc-fg-muted">
						{t('login.noAccount')}
						<a
							href={authRequest ? `/signup?auth_request=${encodeURIComponent(authRequest)}` : '/signup'}
							class="font-medium text-fc-fg underline underline-offset-2"
						>
							{t('signup.title')}
						</a>
					</p>
				{/if}
			</Stack>
		</div>
	{:else}
		<div class="stage">
			<Stack>
				<div class="flex flex-col gap-1">
					<h1 class="text-fc-lg font-semibold text-fc-fg">{t('login.twoStep')}</h1>
					<p class="text-fc-sm text-fc-fg-muted">
						{recovery ? t('login.recoveryHint') : t('login.codeHint')}
					</p>
				</div>

				<form onsubmit={submitCode} class="flex flex-col gap-4">
					<div aria-live="polite">
						{#if error}
							<Alert tone="danger" title={error} />
						{/if}
					</div>
					{#if recovery}
						<Field label={t('login.recoveryCode')} helper={t('login.recoveryOnce')}>
							<Input type="text" autocomplete="off" required bind:value={code} />
						</Field>
					{:else}
						<Field label={t('login.code')}>
							<Input
								type="text"
								inputmode="numeric"
								maxlength={6}
								pattern={'[0-9]{6}'}
								required
								bind:value={code}
							/>
						</Field>
					{/if}
					<Button type="submit" disabled={status === 'submitting' || code.trim().length === 0}>
						{status === 'submitting' ? t('common.checking') : t('common.continue')}
					</Button>
					<Button variant="outline" onclick={useRecovery}>
						{recovery ? t('login.useApp') : t('login.useRecovery')}
					</Button>
					<Button variant="ghost" onclick={startOver}>{t('common.startAgain')}</Button>
				</form>
			</Stack>
		</div>
	{/if}
</Card>

<style>
	.stage {
		animation: stage-in 0.2s cubic-bezier(0.16, 1, 0.3, 1) both;
	}

	@keyframes stage-in {
		from {
			opacity: 0;
			transform: scale(0.98);
		}
		to {
			opacity: 1;
			transform: scale(1);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.stage {
			animation: none;
		}
	}
</style>
