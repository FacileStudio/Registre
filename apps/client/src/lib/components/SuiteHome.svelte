<script lang="ts">
	import { Avatar, Button, Card, EmptyState, Icon, Input, Page, PageHeader, Section, Topbar, icons } from '@facile/muse';
	import type { AccountView } from '$lib/account';
	import { filterSuite, suite } from '$lib/suite';
	import AppLogo from '$lib/components/AppLogo.svelte';

	interface Props {
		me: AccountView;
		onlogout: () => Promise<void>;
	}

	let { me, onlogout }: Props = $props();

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

<Topbar>
	<a href="/" class="flex items-center gap-3 rounded-fc-sm focus-visible:outline-2 focus-visible:outline-fc-ring">
		<img src="/logo.svg" alt="" width={24} height={24} class="block shrink-0" />
		<span class="text-fc-lg font-semibold tracking-tight">Facile</span>
	</a>
	<div class="flex items-center gap-2">
		{#if me.admin}
			<Button variant="ghost" size="sm" href="/admin" icon={icons.shield} aria-label="Administration">
				<span class="hidden sm:inline">Administration</span>
			</Button>
		{/if}
		<Button variant="ghost" size="sm" icon={icons.logout} onclick={onlogout} aria-label="Sign out">
			<span class="hidden sm:inline">Sign out</span>
		</Button>
		<a
			href="/account"
			aria-label="Your account"
			class="flex size-11 items-center justify-center rounded-fc-pill focus-visible:outline-2 focus-visible:outline-fc-ring"
		>
			<Avatar src={me.picture || undefined} name={me.name || me.email} size="sm" />
		</a>
	</div>
</Topbar>

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
