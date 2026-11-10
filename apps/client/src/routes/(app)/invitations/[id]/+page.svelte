<!--
	One space invitation, the page the invitation mail links to. Opening it
	changes nothing: a mail scanner follows links, so only the Accept and
	Decline buttons, which POST with the session cookie and the CSRF header,
	answer the invitation.
-->
<script lang="ts">
	import { getContext } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Badge, Button, Card, EmptyState, Page, Skeleton, Stack, icons, toast } from '@facile/muse';
	import { AccountError, account, accountError, SESSION_CONTEXT, type ShellSession } from '$lib/account';
	import { locale, t } from '$lib/i18n/i18n.svelte';
	import type { MessageKey } from '$lib/i18n/en';
	import { invitationPath } from '$lib/next';
	import { signupOpen as isSignupOpen } from '$lib/signup';
	import { ROLE_TONE, spaces, type MyInvitation } from '$lib/spaces';

	type State = 'loading' | 'pending' | 'signed-out' | 'wrong-account' | 'expired' | 'gone' | 'failed';

	const session = getContext<ShellSession>(SESSION_CONTEXT);
	const id = $derived(page.params.id ?? '');
	const here = $derived(invitationPath(id));

	let view = $state<State>('loading');
	let invitation = $state<MyInvitation | null>(null);
	let busy = $state(false);
	let signupOpen = $state(false);

	const STATES: Record<number, State> = { 401: 'signed-out', 403: 'wrong-account', 410: 'expired', 404: 'gone' };

	$effect(() => {
		load(id);
	});

	/** load reads the invitation. A GET, so reading it never answers it. */
	async function load(invitationID: string) {
		view = 'loading';
		if (!invitationPath(invitationID)) {
			view = 'gone';
			return;
		}
		try {
			invitation = await spaces.invitation(invitationID);
			view = 'pending';
		} catch (cause) {
			view = (cause instanceof AccountError && STATES[cause.status]) || 'failed';
			if (view === 'signed-out') signupOpen = await isSignupOpen().catch(() => false);
		}
	}

	/** answer accepts or declines, then leaves for the space or the spaces page. */
	async function answer(accept: boolean) {
		if (!invitation || busy) return;
		busy = true;
		try {
			if (accept) {
				const joined = await spaces.accept(invitation.id);
				toast.success(t('spaces.joined', { space: invitation.space_name }));
				await goto(`/spaces/${encodeURIComponent(joined.id || invitation.space_id)}`);
			} else {
				await spaces.decline(invitation.id);
				toast.success(t('invitation.declined'));
				await goto('/spaces');
			}
		} catch (cause) {
			toast.danger(accountError(cause, t('common.failed')));
			await load(id);
		} finally {
			busy = false;
		}
	}

	/** switchAccount signs out and comes back here through the sign-in page. */
	async function switchAccount() {
		await account.logout().catch(() => null);
		window.location.assign(signInHref);
	}

	const signInHref = $derived(here ? `/login?next=${encodeURIComponent(here)}` : '/login');
	const signUpHref = $derived(`/signup?invitation=${encodeURIComponent(id)}`);
	const expires = $derived(invitation ? new Date(invitation.expires_at).toLocaleDateString(locale(), { dateStyle: 'long' }) : '');
</script>

<svelte:head>
	<title>{t('invitation.title')} — Facile</title>
</svelte:head>

<Page width="sm">
	{#if view === 'loading'}
		<Stack>
			<Skeleton class="h-40 w-full" />
		</Stack>
	{:else if view === 'pending' && invitation}
		<Card class="p-6">
			<Stack gap="content">
				<div class="flex items-center gap-3">
					<span class="flex size-12 shrink-0 items-center justify-center rounded-fc-md bg-fc-surface text-fc-fg">
						<span class="text-fc-lg font-semibold">{invitation.space_name.trim().charAt(0).toUpperCase() || '?'}</span>
					</span>
					<div class="min-w-0 flex-1">
						<p class="text-fc-xs text-fc-fg-muted">{t('invitation.title')}</p>
						<h1 class="text-fc-lg font-semibold text-fc-fg wrap-anywhere">{invitation.space_name}</h1>
					</div>
				</div>
				<p class="text-fc-sm text-fc-fg wrap-anywhere">
					{t('invitation.lead', { inviter: invitation.invited_by, space: invitation.space_name })}
				</p>
				<dl class="grid grid-cols-[auto_1fr] items-center gap-x-4 gap-y-2 text-fc-sm">
					<dt class="text-fc-fg-muted">{t('invitation.role')}</dt>
					<dd>
						<Badge tone={ROLE_TONE[invitation.role]} class="whitespace-nowrap">
							{t(`role.${invitation.role}` as MessageKey)}
						</Badge>
					</dd>
					<dt class="text-fc-fg-muted">{t('invitation.invitedBy')}</dt>
					<dd class="min-w-0 truncate text-fc-fg">{invitation.invited_by}</dd>
				</dl>
				<p class="text-fc-xs text-fc-fg-muted">
					{t('invitation.expires', { date: expires })}. {t('invitation.safety')}
				</p>
				<div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
					<Button variant="ghost" size="lg" disabled={busy} onclick={() => answer(false)}>{t('spaces.decline')}</Button>
					<Button size="lg" icon={icons.check} disabled={busy} onclick={() => answer(true)}>{t('spaces.accept')}</Button>
				</div>
			</Stack>
		</Card>
	{:else if view === 'signed-out'}
		<EmptyState icon={icons.mail} title={t('invitation.signInTitle')} description={t('invitation.signInHint')}>
			<div class="flex flex-col gap-2 sm:flex-row">
				<Button href={signInHref} size="lg">{t('invitation.signIn')}</Button>
				{#if signupOpen}
					<Button href={signUpHref} variant="ghost" size="lg">{t('invitation.signUp')}</Button>
				{/if}
			</div>
		</EmptyState>
	{:else if view === 'wrong-account'}
		<EmptyState
			icon={icons.userCircle}
			title={t('invitation.wrongTitle')}
			description={t('invitation.wrongHint', { email: session?.me?.email ?? '' })}
		>
			<Button size="lg" onclick={switchAccount}>{t('invitation.switch')}</Button>
		</EmptyState>
	{:else if view === 'expired'}
		<EmptyState icon={icons.clock} title={t('invitation.expiredTitle')} description={t('invitation.expiredHint')}>
			<Button href="/spaces" variant="ghost" size="lg">{t('invitation.mySpaces')}</Button>
		</EmptyState>
	{:else if view === 'gone'}
		<EmptyState icon={icons.info} title={t('invitation.goneTitle')} description={t('invitation.goneHint')}>
			<Button href="/spaces" variant="ghost" size="lg">{t('invitation.mySpaces')}</Button>
		</EmptyState>
	{:else}
		<EmptyState icon={icons.warning} title={t('common.failed')}>
			<Button size="lg" onclick={() => load(id)}>{t('common.startAgain')}</Button>
		</EmptyState>
	{/if}
</Page>
