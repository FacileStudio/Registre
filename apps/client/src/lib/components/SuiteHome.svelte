<script lang="ts">
	import { Card, EmptyState, Icon, Input, Page, PageHeader, Section, icons } from '@facile/muse';
	import type { AccountView } from '$lib/account';
	import { filterSuite, suite } from '$lib/suite';
	import AppLogo from '$lib/components/AppLogo.svelte';

	let { me }: { me: AccountView } = $props();

	let query = $state('');

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
	<PageHeader title="Hello, {firstName}" description="Every app in the suite, signed in with the same account." />

	<div class="relative">
		<span class="pointer-events-none absolute inset-y-0 left-3.5 flex items-center text-fc-fg-muted">
			<Icon icon={icons.search} size={16} />
		</span>
		<Input
			id="suite-search"
			type="search"
			bind:value={query}
			placeholder="Search apps"
			aria-label="Search apps"
			class="w-full pl-10"
		/>
	</div>

	{#if groups.length === 0}
		<EmptyState icon={icons.search} title="No app matches" description="Nothing in the suite is called “{query}”." />
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
</Page>
