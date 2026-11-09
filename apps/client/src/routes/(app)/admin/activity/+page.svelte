<script lang="ts">
	import { Alert, Avatar, Button, EmptyState, Section, Skeleton, icons } from '@facile/muse';
	import { admin, errorMessage, type AdminEvent } from '$lib/admin';
	import { describeAction, displayName, relativeTime } from '$lib/access';

	let events = $state<AdminEvent[]>([]);
	let loading = $state(true);
	let more = $state(false);
	let error = $state('');

	/** load reads one page of events, older than the last one shown when appending. */
	async function load(append = false) {
		loading = true;
		try {
			const before = append ? events.at(-1)?.id : undefined;
			const page = await admin.listEvents(before);
			events = append ? [...events, ...page] : page;
			more = page.length === 50;
			error = '';
		} catch (cause) {
			error = errorMessage(cause, 'Could not load the activity.');
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		void load();
	});

	/** target names what an event changed, readably when the detail allows it. */
	function target(event: AdminEvent): string {
		const app = typeof event.detail?.client_id === 'string' ? ` in ${event.detail.client_id}` : '';
		const email = typeof event.detail?.email === 'string' ? event.detail.email : '';
		return `${email || event.target_id}${app}`;
	}
</script>

<Section title="Activity" description="Every change an operator made from this dashboard, newest first." bodyClass="flex flex-col gap-2">
	{#if error}
		<Alert tone="danger" title="Could not load the activity">{error}</Alert>
	{:else if loading && events.length === 0}
		<Skeleton class="h-12 w-full" />
		<Skeleton class="h-12 w-full" />
	{:else if events.length === 0}
		<EmptyState icon={icons.history} title="Nothing yet" description="Changes to people, apps and access will show up here." />
	{:else}
		<ol class="flex flex-col gap-2">
			{#each events as event (event.id)}
				<li class="flex items-center gap-3 py-1.5">
					<Avatar
						name={event.actor ? displayName(event.actor) : '?'}
						src={event.actor?.picture_url || undefined}
						size="sm"
					/>
					<p class="min-w-0 flex-1 text-fc-sm text-fc-fg">
						<span class="font-medium">{event.actor ? displayName(event.actor) : 'A deleted operator'}</span>
						{describeAction(event)}
						<span class="font-fc-mono text-fc-xs">{target(event)}</span>
					</p>
					<time class="whitespace-nowrap text-fc-xs text-fc-fg-muted" datetime={event.at} title={event.at}>
						{relativeTime(event.at)}
					</time>
				</li>
			{/each}
		</ol>
		{#if more}
			<div>
				<Button variant="outline" disabled={loading} onclick={() => load(true)}>Show older</Button>
			</div>
		{/if}
	{/if}
</Section>
