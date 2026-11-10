<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Button, Icon, Spinner, icons } from '@facile/muse';
	import { t } from '$lib/i18n/i18n.svelte';
	import { connect, describe, ConnectError, type PendingConnection } from '$lib/connect';

	type Stage = 'loading' | 'ask' | 'gone' | 'signed-out';

	let stage = $state<Stage>('loading');
	let pending = $state<PendingConnection | null>(null);
	let busy = $state(false);
	let error = $state('');

	const authRequest = $derived(page.url.searchParams.get('auth_request') ?? '');
	const scopes = $derived(pending ? describe(pending.scopes, pending.app_name) : []);
	const client = $derived(pending?.client_name || pending?.client_id || '');

	onMount(async () => {
		try {
			pending = await connect.pending(authRequest);
			stage = 'ask';
		} catch (cause) {
			stage = cause instanceof ConnectError && cause.status === 401 ? 'signed-out' : 'gone';
		}
	});

	/** answer sends the decision, then follows the API to the callback or the client. */
	async function answer(allow: boolean) {
		if (busy) return;
		busy = true;
		error = '';
		try {
			const next = await connect.answer(authRequest, allow);
			window.location.assign(next.redirect);
		} catch (cause) {
			if (cause instanceof ConnectError && cause.status === 404) stage = 'gone';
			else error = t('common.failed');
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>{t('account.tab.apps')} — Facile</title>
</svelte:head>

{#if stage === 'loading'}
	<div class="flex justify-center py-10"><Spinner /></div>
{:else if stage === 'ask' && pending}
	<div class="flex flex-col gap-6 text-center">
		{#if error}
			<p role="alert" class="rounded-fc-md bg-fc-danger/10 px-3 py-2 text-sm text-fc-danger">{error}</p>
		{/if}

		<div class="flex flex-col items-center gap-2">
			<div class="flex size-14 items-center justify-center rounded-fc-2xl bg-fc-field text-fc-fg">
				<Icon icon={icons.plug} size={28} />
			</div>
			<h1 class="font-heading text-2xl font-semibold tracking-tight break-words text-fc-fg">
				{t('connect.title', { client })}
			</h1>
			<p class="text-sm text-fc-fg-muted">{t('connect.body', { client, app: pending.app_name })}</p>
		</div>

		<div class="flex flex-col gap-3 rounded-fc-xl bg-fc-field p-4 text-left">
			<div class="flex items-center justify-between gap-3 text-xs text-fc-fg-muted">
				<span class="shrink-0">{t('connect.returnsTo')}</span>
				<span class="min-w-0 truncate font-mono text-sm text-fc-fg">{pending.redirect_host}</span>
			</div>
			<div class="flex items-center justify-between gap-3 text-xs text-fc-fg-muted">
				<span class="shrink-0">{t('apps.access', { app: pending.app_name })}</span>
				<span class="min-w-0 truncate font-mono text-xs text-fc-fg">{pending.resource}</span>
			</div>
		</div>

		{#if pending.loopback_only}
			<p class="flex items-start gap-2 rounded-fc-md bg-fc-warning/10 px-3 py-2 text-left text-sm text-fc-fg">
				<Icon icon={icons.warning} size={16} class="mt-0.5 shrink-0 text-fc-warning" />
				<span>{t('connect.loopback')}</span>
			</p>
		{/if}
		<p class="text-left text-xs text-fc-fg-muted">{t('connect.unverified')}</p>

		<div class="flex flex-col gap-2 text-left">
			<p class="text-xs font-semibold uppercase tracking-wider text-fc-fg-muted">{t('connect.asking')}</p>
			<ul class="flex flex-col gap-2.5">
				{#each scopes as scope (scope.name)}
					<li class="flex items-start gap-2.5 text-sm text-fc-fg">
						<span class="mt-1 size-2 shrink-0 rounded-full bg-fc-accent"></span>
						<span class="min-w-0">{scope.label}</span>
					</li>
				{/each}
			</ul>
		</div>

		<div class="flex flex-col gap-2.5 pt-2">
			<Button type="button" variant="primary" size="lg" disabled={busy} onclick={() => answer(true)} class="w-full">
				{t('connect.allow')}
			</Button>
			<Button type="button" variant="secondary" size="lg" disabled={busy} onclick={() => answer(false)} class="w-full">
				{t('connect.deny')}
			</Button>
		</div>

		<p class="text-xs text-fc-fg-muted">{t('connect.footer')}</p>
	</div>
{:else}
	<div class="flex flex-col items-center gap-3 text-center">
		<div class="flex size-14 items-center justify-center rounded-fc-2xl bg-fc-field text-fc-fg">
			<Icon icon={icons.plug} size={28} />
		</div>
		<p class="text-sm text-fc-fg-muted">
			{stage === 'signed-out' ? t('connect.signedOut') : t('connect.gone')}
		</p>
	</div>
{/if}
