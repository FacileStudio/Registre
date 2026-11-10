<script lang="ts">
	import { onMount } from 'svelte';
	import { Button, ConfirmModal, EmptyState, SettingsSection, Skeleton, Stack, icons, toast } from '@facile/muse';
	import { locale, t } from '$lib/i18n/i18n.svelte';
	import { connect, describe, type ConnectedApp } from '$lib/connect';

	let apps = $state<ConnectedApp[]>([]);
	let loading = $state(true);
	let leaving = $state<ConnectedApp | null>(null);
	let confirmOpen = $state(false);

	/** day is a date as the person's locale writes it. */
	function day(value: string): string {
		return new Date(value).toLocaleDateString(locale(), { day: 'numeric', month: 'short', year: 'numeric' });
	}

	/** nameOf is what a grant is called on screen. */
	function nameOf(app: ConnectedApp): string {
		return app.client_name || app.client_id;
	}

	async function load() {
		try {
			apps = await connect.apps();
		} catch {
			toast.danger(t('common.failed'));
		} finally {
			loading = false;
		}
	}

	function ask(app: ConnectedApp) {
		leaving = app;
		confirmOpen = true;
	}

	/** disconnect revokes the grant; the client loses access within ten minutes. */
	async function disconnect() {
		if (!leaving) return;
		const gone = leaving;
		try {
			await connect.disconnect(gone.id);
			apps = apps.filter((app) => app.id !== gone.id);
			toast.success(t('apps.disconnected', { client: nameOf(gone) }));
		} catch {
			toast.danger(t('common.failed'));
		} finally {
			leaving = null;
		}
	}

	onMount(load);
</script>

<SettingsSection title={t('apps.title')} description={t('apps.hint')}>
	{#if loading}
		<Stack>
			<Skeleton class="h-20 w-full" />
		</Stack>
	{:else if apps.length === 0}
		<EmptyState icon={icons.plug} title={t('apps.empty')} description={t('apps.emptyHint')} />
	{:else}
		<ul class="flex flex-col gap-3">
			{#each apps as app (app.id)}
				<li class="flex flex-col gap-3 rounded-fc-xl bg-fc-field p-4 sm:flex-row sm:items-center sm:justify-between">
					<div class="flex min-w-0 flex-col gap-1">
						<span class="truncate text-sm font-semibold text-fc-fg">{nameOf(app)}</span>
						<span class="truncate text-xs text-fc-fg-muted">
							{t('apps.access', { app: app.app_name || app.resource })}{app.redirect_host ? ` · ${app.redirect_host}` : ''}
						</span>
						<span class="text-xs text-fc-fg-muted">
							{describe(app.scopes, app.app_name).map((scope) => scope.label).join(' · ')}
						</span>
						<span class="text-xs text-fc-fg-muted">
							{t('apps.since', { date: day(app.created_at) })} ·
							{app.last_used_at ? t('apps.lastUsed', { date: day(app.last_used_at) }) : t('apps.neverUsed')}
						</span>
					</div>
					<Button variant="outline" icon={icons.revoke} onclick={() => ask(app)} class="shrink-0 self-start sm:self-center">
						{t('apps.disconnect')}
					</Button>
				</li>
			{/each}
		</ul>
	{/if}
</SettingsSection>

<ConfirmModal
	bind:open={confirmOpen}
	tone="danger"
	icon={icons.revoke}
	title={t('apps.disconnect')}
	description={leaving ? `${nameOf(leaving)} · ${t('apps.hint')}` : ''}
	confirmLabel={t('apps.disconnect')}
	cancelLabel={t('common.cancel')}
	onConfirm={disconnect}
	onCancel={() => (leaving = null)}
/>
