<script lang="ts">
	import { Button, Avatar, Icon, icons } from '@facile/muse';
	import { t } from '$lib/i18n/i18n.svelte';
	import { describeScopes, expiresAtLabel, type DeviceRequestView } from '$lib/device';
	import type { AccountView } from '$lib/account';

	interface Props {
		request: DeviceRequestView;
		code: string;
		busy: boolean;
		error: string;
		me?: AccountView | null;
		approve: () => void;
		deny: () => void;
	}

	let { request, code, busy, error, me, approve, deny }: Props = $props();

	const scopes = $derived(describeScopes(request.scopes ?? []));
	const deadline = $derived(expiresAtLabel(request.expires_at));
</script>

<div class="flex flex-col gap-6 text-center">
	{#if error}
		<p role="alert" class="rounded-fc-md bg-fc-danger/10 px-3 py-2 text-sm text-fc-danger">
			{error}
		</p>
	{/if}

	<div class="flex flex-col items-center gap-2">
		<div class="flex size-14 items-center justify-center rounded-fc-2xl bg-fc-field text-fc-fg">
			<Icon icon={icons.code} size={28} />
		</div>
		<h1 class="font-heading text-2xl font-semibold tracking-tight text-fc-fg">
			{request.client_name}
		</h1>
		<p class="text-sm text-fc-fg-muted">
			{t('device.signingInTo')}
		</p>
	</div>

	{#if me}
		<div class="flex items-center justify-center gap-2.5 rounded-fc-pill bg-fc-field px-3.5 py-1.5 self-center">
			<Avatar size="sm" src={me.picture} name={me.name || me.email} />
			<div class="flex flex-col text-left">
				<span class="text-fc-xs font-medium text-fc-fg">{me.name || me.email}</span>
				{#if me.name && me.email}
					<span class="text-[11px] text-fc-fg-muted">{me.email}</span>
				{/if}
			</div>
		</div>
	{/if}

	<div class="flex flex-col gap-3 rounded-fc-xl bg-fc-field p-4 text-left">
		<div class="flex items-center justify-between text-xs text-fc-fg-muted">
			<span>{t('device.code')}</span>
			<span class="font-mono text-sm tracking-widest text-fc-fg">{code}</span>
		</div>
		{#if deadline}
			<div class="flex items-center justify-between text-xs text-fc-fg-muted">
				<span>{t('device.runsOut')}</span>
				<span class="text-fc-fg">{deadline}</span>
			</div>
		{/if}
	</div>

	<div class="flex flex-col gap-2 text-left">
		<p class="text-xs font-semibold uppercase tracking-wider text-fc-fg-muted">
			{t('device.asking')}
		</p>
		{#if scopes.length === 0}
			<p class="text-sm text-fc-fg-muted">{t('device.nothingMore')}</p>
		{:else}
			<ul class="flex flex-col gap-2.5">
				{#each scopes as scope (scope.name)}
					<li class="flex items-start gap-2.5 text-sm text-fc-fg">
						<span class="mt-1 size-2 shrink-0 rounded-full bg-fc-accent"></span>
						<span class="min-w-0">{scope.label}</span>
					</li>
				{/each}
			</ul>
		{/if}
	</div>

	<div class="flex flex-col gap-2.5 pt-2">
		<Button type="button" variant="primary" size="lg" disabled={busy} onclick={approve} class="w-full">
			{t('device.approve')}
		</Button>
		<Button type="button" variant="secondary" size="lg" disabled={busy} onclick={deny} class="w-full">
			{t('device.deny')}
		</Button>
	</div>

	<p class="text-xs text-fc-fg-muted">
		{t('device.footer')}
	</p>
</div>
