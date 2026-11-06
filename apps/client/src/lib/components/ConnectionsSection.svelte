<!--
	The account's connections: other accounts, such as Discord, that can sign
	in to this one. Only providers an app has opted into are listed, so nothing
	shows until one is in use. Linking and unlinking need a factor proven in
	the last ten minutes; when the server says so the reauthentication modal
	opens and the change resumes after it.
-->
<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Button, SettingsRow, SettingsSection, icons, toast } from '@facile/muse';
	import { account, accountError, AccountError, type AccountView, type ConnectionView } from '$lib/account';
	import { locale, t } from '$lib/i18n/i18n.svelte';
	import { PROVIDERS } from '$lib/social';
	import ReauthModal from './ReauthModal.svelte';

	let { me, onchange }: { me: AccountView; onchange: () => Promise<void> } = $props();

	let items = $state<ConnectionView[]>([]);
	let busy = $state(false);
	let reauthOpen = $state(false);
	let pending = $state<(() => Promise<void>) | null>(null);

	onMount(() => {
		load();
		announceOutcome();
	});

	async function load() {
		items = (await account.connections().catch(() => ({ connections: [] }))).connections;
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

	function link(item: ConnectionView) {
		guarded(async () => {
			const started = await account.linkConnection(item.provider);
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

{#if items.length > 0}
	<SettingsSection title={t('connections.title')} description={t('connections.description')}>
		{#each items as item (item.provider)}
			<SettingsRow
				label={label(item.provider)}
				description={item.linked
					? `${t('connections.linkedAs', { name: item.display_name ?? '' })}${item.linked_at ? ` · ${date(item.linked_at)}` : ''}`
					: t('connections.notLinked')}
			>
				{#if item.linked}
					<Button variant="ghost-danger" icon={icons.remove} disabled={busy} onclick={() => unlink(item)}>
						{t('connections.unlink')}
					</Button>
				{:else}
					<Button
						variant="outline"
						icon={PROVIDERS[item.provider]?.icon ?? icons.plus}
						disabled={busy}
						onclick={() => link(item)}
					>
						{t('connections.link')}
					</Button>
				{/if}
			</SettingsRow>
		{/each}
	</SettingsSection>
{/if}

<ReauthModal bind:open={reauthOpen} {me} onDone={resume} />
