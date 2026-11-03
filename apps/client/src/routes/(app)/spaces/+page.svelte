<script lang="ts">
	import { goto } from '$app/navigation';
	import {
		Alert,
		Badge,
		Button,
		Card,
		EmptyState,
		Field,
		Icon,
		Input,
		Page,
		PageHeader,
		Section,
		Skeleton,
		Stack,
		icons,
		toast
	} from '@facile/muse';
	import { accountError, isUnauthorized } from '$lib/account';
	import { ROLE_TONE, spaces, type SpaceSummary } from '$lib/spaces';

	let list = $state<SpaceSummary[]>([]);
	let loading = $state(true);
	let signedOut = $state(false);
	let error = $state('');

	let name = $state('');
	let creating = $state(false);
	let createError = $state('');

	$effect(() => {
		load();
	});

	/** load fetches the spaces I am in, sorted by name. */
	async function load() {
		try {
			list = (await spaces.list()).sort((a, b) => a.name.localeCompare(b.name));
			error = '';
		} catch (cause) {
			signedOut = isUnauthorized(cause);
			error = accountError(cause, 'Could not load your spaces.');
		} finally {
			loading = false;
		}
	}

	/** create makes a space I own and opens it, so the next step is adding people. */
	async function create(event: SubmitEvent) {
		event.preventDefault();
		if (creating) return;
		creating = true;
		createError = '';
		try {
			const space = await spaces.create(name.trim());
			toast.success(`${space.name} is ready. Add the people who share it.`);
			await goto(`/spaces/${encodeURIComponent(space.id)}`);
		} catch (cause) {
			createError = accountError(cause, 'Could not create the space.');
		} finally {
			creating = false;
		}
	}

	const plural = (count: number) => (count === 1 ? '1 member' : `${count} members`);
</script>

<svelte:head>
	<title>Spaces — Registre</title>
</svelte:head>

<Page width="lg">
	<PageHeader
		title="Spaces"
		description="A space and its members here are the same in every Facile app, so you set them up once."
	/>

	{#if loading}
		<Stack>
			<Skeleton class="h-16 w-full" />
			<Skeleton class="h-16 w-full" />
		</Stack>
	{:else if signedOut}
		<EmptyState
			icon={icons.userCircle}
			title="You are not signed in"
			description="Open a Facile app, sign in there, and come back to manage your spaces."
		/>
	{:else if error}
		<Alert tone="danger" title="Could not load your spaces">{error}</Alert>
	{:else}
		<Section title="Your spaces">
			{#if list.length === 0}
				<EmptyState
					icon={icons.layers}
					title="No spaces yet"
					description="Create one below and add your team. It appears in every app at once."
				/>
			{:else}
				<div class="grid grid-cols-1 gap-3 sm:grid-cols-2 [&>*]:min-w-0">
					{#each list as space (space.id)}
						<Card href="/spaces/{encodeURIComponent(space.id)}">
							<div class="flex items-center gap-3">
								<span class="flex size-10 shrink-0 items-center justify-center rounded-fc-md bg-fc-surface text-fc-fg">
									<Icon icon={icons.layers} size={18} />
								</span>
								<div class="min-w-0 flex-1">
									<div class="truncate text-fc-sm font-semibold text-fc-fg">{space.name}</div>
									<div class="text-fc-xs text-fc-fg-muted">{plural(space.member_count)}</div>
								</div>
								<Badge tone={ROLE_TONE[space.role]}>{space.role}</Badge>
							</div>
						</Card>
					{/each}
				</div>
			{/if}
		</Section>

		<Section title="New space" description="You become its owner. Names are unique across the suite.">
			<form class="flex flex-col gap-3 sm:flex-row sm:items-end" onsubmit={create}>
				<Field label="Name" class="flex-1">
					<Input required maxlength={80} disabled={creating} bind:value={name} placeholder="Design team" />
				</Field>
				<Button type="submit" icon={icons.plus} class="w-full sm:w-auto" disabled={creating || !name.trim()}>
					{creating ? 'Creating…' : 'Create space'}
				</Button>
			</form>
			<div aria-live="polite" class="mt-3">
				{#if createError}
					<Alert tone="danger" title={createError} />
				{/if}
			</div>
		</Section>
	{/if}
</Page>
