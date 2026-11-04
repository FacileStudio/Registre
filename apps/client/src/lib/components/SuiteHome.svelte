<script lang="ts">
	import { t } from '$lib/i18n/i18n.svelte';
	import { Button, Card, EmptyState, Icon, Input, Page, PageHeader, Section, icons } from '@facile/muse';
	import type { AccountView } from '$lib/account';
	import { spaces, type SpaceSummary } from '$lib/spaces';
	import { filterSuite, suite } from '$lib/suite';
	import AppLogo from '$lib/components/AppLogo.svelte';

	let { me }: { me: AccountView } = $props();

	let query = $state('');
	let mySpaces = $state<SpaceSummary[]>([]);

	$effect(() => {
		spaces
			.list()
			.then((list) => (mySpaces = list.sort((a, b) => a.name.localeCompare(b.name))))
			.catch(() => (mySpaces = []));
	});

	const groups = $derived(filterSuite(suite, query));
	const firstName = $derived((me.name || me.email).split(/[\s@]/)[0]);

	/** focusSearch binds ⌘K and Ctrl+K to the search field. */
	function focusSearch(event: KeyboardEvent) {
		if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
			event.preventDefault();
			document.getElementById('suite-search')?.focus();
		}
	}
</script>

<svelte:window onkeydown={focusSearch} />

<Page width="xl">
	<PageHeader title={t('home.hello', { name: firstName })} description={t('home.subtitle')} />

	<div class="relative">
		<span class="pointer-events-none absolute inset-y-0 left-3.5 flex items-center text-fc-fg-muted">
			<Icon icon={icons.search} size={16} />
		</span>
		<Input
			id="suite-search"
			type="search"
			bind:value={query}
			placeholder={t('home.search')}
			aria-label={t('home.search')}
			class="w-full pl-10"
		/>
	</div>

	{#if groups.length === 0}
		<EmptyState icon={icons.search} title={t('home.noMatch')} description={t('home.noMatchHint', { query })} />
	{/if}

	{#each groups as group (group.title)}
		<Section title={group.title}>
			<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 [&>*]:min-w-0">
				{#each group.apps as app (app.url)}
					<Card href={app.url} target="_blank" rel="noopener noreferrer">
						<div class="flex items-center gap-3">
							<AppLogo {app} />
							<div class="min-w-0">
								<div class="truncate text-fc-sm font-semibold text-fc-fg">{app.name}</div>
								<div class="truncate text-fc-xs text-fc-fg-muted">{app.description}</div>
							</div>
						</div>
					</Card>
				{/each}
			</div>
		</Section>
	{/each}

	{#if mySpaces.length > 0}
		<Section title={t('home.spaces')}>
			{#snippet actions()}
				<Button variant="ghost" size="sm" href="/spaces" iconRight={icons.arrow}>{t('home.manage')}</Button>
			{/snippet}
			<div class="flex flex-wrap gap-2">
				{#each mySpaces as space (space.id)}
					<Button variant="outline" size="sm" href="/spaces/{encodeURIComponent(space.id)}">{space.name}</Button>
				{/each}
			</div>
		</Section>
	{/if}
</Page>
