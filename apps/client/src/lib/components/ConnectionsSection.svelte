<!--
	The account's connections: other accounts, such as Discord, that can sign
	in to this one. Only providers an app has opted into are listed, so nothing
	shows until one is in use. Linking and unlinking need a factor proven in
	the last ten minutes; when the server says so the reauthentication modal
	opens and the change resumes after it.

	A deep link, /account/security?link=discord&return_to=<url>, starts the
	link as soon as the connections load and, when the provider is already
	linked, goes straight back to return_to. The server vouches for return_to
	in both cases; the page never navigates anywhere it did not hand back.
-->
<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Badge, Button, Icon, SettingsSection, Spinner, icons, toast } from '@facile/muse';
	import { account, accountError, AccountError, type AccountView, type ConnectionView } from '$lib/account';
	import { locale, t } from '$lib/i18n/i18n.svelte';
	import { PROVIDERS } from '$lib/social';
	import { parseDeepLink, withoutDeepLink, type DeepLink } from '$lib/social-link';
	import ReauthModal from './ReauthModal.svelte';

	let { me, onchange }: { me: AccountView; onchange: () => Promise<void> } = $props();

	let items = $state<ConnectionView[]>([]);
	let busy = $state(false);
	let reauthOpen = $state(false);
	let pending = $state<(() => Promise<void>) | null>(null);
	let connecting = $state('');
	let leaving = $state(false);

	onMount(() => {
		const deep = takeDeepLink();
		load().then(() => deep && followDeepLink(deep));
		announceOutcome();
	});

	$effect(() => {
		if (connecting && !busy && !reauthOpen && !leaving) connecting = '';
	});

	async function load() {
		items = (await account.connections().catch(() => ({ connections: [] }))).connections;
	}

	/** takeDeepLink reads the deep link from the URL once and strips it, so a reload never repeats it. */
	function takeDeepLink(): DeepLink | null {
		const deep = parseDeepLink(page.url.searchParams);
		if (!deep) return null;
		history.replaceState(history.state, '', withoutDeepLink(new URL(window.location.href)));
		return deep;
	}

	/** followDeepLink links the provider, or goes back when it is already linked. */
	function followDeepLink(deep: DeepLink) {
		const item = items.find((i) => i.provider === deep.provider);
		if (!item) return;
		if (!item.linked) {
			connecting = item.provider;
			link(item, deep.returnTo);
			return;
		}
		if (!deep.returnTo) {
			toast.success(t('connections.alreadyLinked', { provider: label(item.provider) }));
			return;
		}
		guarded(async () => {
			const back = await account.connectionReturn(item.provider, deep.returnTo);
			leaving = true;
			window.location.assign(back.url);
		}).then(() => {
			if (!leaving) toast.success(t('connections.alreadyLinked', { provider: label(item.provider) }));
		});
	}

	/** announceOutcome tells the person how the link they just made ended, then clears it from the URL. */
	function announceOutcome() {
		const outcome = page.url.searchParams.get('connection');
		if (!outcome) return;
		if (outcome === 'linked') toast.success(t('connections.linked', { provider: t('connections.discord') }));
		else if (outcome === 'taken') toast.danger(t('connections.taken'));
		else toast.danger(t('connections.failed'));
		const clean = new URL(window.location.href);
		clean.searchParams.delete('connection');
		history.replaceState(history.state, '', clean);
	}

	function label(provider: string): string {
		const known = PROVIDERS[provider];
		return known ? t(known.title) : provider;
	}

	/** guarded runs change, and on reauth_required asks for a factor and runs it again. */
	async function guarded(change: () => Promise<void>) {
		busy = true;
		try {
			await change();
		} catch (cause) {
			if (cause instanceof AccountError && cause.message === 'reauth_required') {
				pending = change;
				reauthOpen = true;
			} else if (cause instanceof AccountError && cause.message === 'last_factor') {
				toast.danger(t('connections.lastFactor'));
			} else {
				toast.danger(accountError(cause, t('connections.failed')));
			}
		} finally {
			busy = false;
		}
	}

	function link(item: ConnectionView, returnTo = '') {
		guarded(async () => {
			const started = await account.linkConnection(item.provider, returnTo);
			leaving = true;
			window.location.assign(started.url);
		});
	}

	function unlink(item: ConnectionView) {
		guarded(async () => {
			await account.unlinkConnection(item.provider);
			await load();
			await onchange();
			toast.success(t('connections.unlinked', { provider: label(item.provider) }));
		});
	}

	function resume() {
		const change = pending;
		pending = null;
		if (change) guarded(change);
	}

	const date = (value: string) => new Date(value).toLocaleDateString(locale());
</script>

{#if connecting && !reauthOpen}
	<div class="fixed inset-0 z-40 flex flex-col items-center justify-center gap-4 bg-fc-page px-4 text-center" role="status">
		<Spinner size="lg" label={t('connections.connecting', { provider: label(connecting) })} />
		<p class="text-fc-base font-medium text-fc-fg">{t('connections.connecting', { provider: label(connecting) })}</p>
		<p class="max-w-sm text-fc-sm text-fc-fg-muted">{t('connections.connectingHint')}</p>
	</div>
{/if}

{#if items.length > 0}
	<SettingsSection title={t('connections.title')} description={t('connections.description')}>
		<ul class="flex flex-col gap-3">
			{#each items as item (item.provider)}
				<li class="flex flex-col gap-3 rounded-fc-md border border-fc-border p-4 sm:flex-row sm:items-center sm:justify-between">
					<div class="flex min-w-0 items-center gap-3">
						<span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-fc-md border border-fc-border bg-fc-surface text-fc-fg">
							<Icon icon={PROVIDERS[item.provider]?.icon ?? icons.plus} size={20} />
						</span>
						<div class="flex min-w-0 flex-col gap-1">
							<div class="flex flex-wrap items-center gap-2">
								<span class="text-fc-sm font-medium text-fc-fg">{label(item.provider)}</span>
								<Badge tone={item.linked ? 'success' : 'neutral'}>
									{item.linked ? t('connections.statusLinked') : t('connections.notLinked')}
								</Badge>
							</div>
							{#if item.linked}
								<p class="text-fc-xs text-fc-fg-muted">
									{t('connections.linkedAs', { name: item.display_name ?? '' })}{item.linked_at ? ` · ${date(item.linked_at)}` : ''}
								</p>
							{/if}
						</div>
					</div>
					{#if item.linked}
						<Button variant="ghost-danger" icon={icons.remove} disabled={busy} onclick={() => unlink(item)}>
							{t('connections.unlink')}
						</Button>
					{:else}
						<Button
							variant="primary"
							icon={PROVIDERS[item.provider]?.icon ?? icons.plus}
							disabled={busy}
							onclick={() => link(item)}
						>
							{t('connections.link')} {label(item.provider)}
						</Button>
					{/if}
				</li>
			{/each}
		</ul>
	</SettingsSection>
{/if}

<ReauthModal bind:open={reauthOpen} {me} onDone={resume} />
