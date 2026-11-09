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
	import SpaceInvitations from '$lib/components/SpaceInvitations.svelte';
	import { t } from '$lib/i18n/i18n.svelte';
	import type { MessageKey } from '$lib/i18n/en';

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
			error = accountError(cause, t('spaces.loadFailed'));
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
			toast.success(t('spaces.created', { name: space.name }));
			await goto(`/spaces/${encodeURIComponent(space.id)}`);
		} catch (cause) {
			createError = accountError(cause, t('common.failed'));
		} finally {
			creating = false;
		}
	}

	const plural = (count: number) => (count === 1 ? t('spaces.oneMember') : t('spaces.members', { count }));
</script>

<svelte:head>
	<title>{t('nav.spaces')} — Facile</title>
</svelte:head>

<Page width="lg">
	<PageHeader
		title={t('nav.spaces')}
		description={t('spaces.description')}
	/>

	{#if loading}
		<Stack>
			<Skeleton class="h-16 w-full" />
			<Skeleton class="h-16 w-full" />
		</Stack>
	{:else if signedOut}
		<EmptyState
			icon={icons.userCircle}
			title={t('account.signedOut')}
			description={t('spaces.signedOutHint')}
		/>
	{:else if error}
		<Alert tone="danger" title={t('spaces.loadFailed')}>{error}</Alert>
	{:else}
		<SpaceInvitations onchange={load} />

		<Section title={t('home.spaces')}>
			{#if list.length === 0}
				<EmptyState
					icon={icons.space}
					title={t('spaces.none')}
					description={t('spaces.noneHint')}
				/>
			{:else}
				<div class="grid grid-cols-1 gap-3 sm:grid-cols-2 [&>*]:min-w-0">
					{#each list as space (space.id)}
						<Card href="/spaces/{encodeURIComponent(space.id)}">
							<div class="flex items-center gap-3">
								<span class="flex size-10 shrink-0 items-center justify-center rounded-fc-md bg-fc-surface text-fc-fg">
									<Icon icon={icons.space} size={18} />
								</span>
								<div class="min-w-0 flex-1">
									<div class="truncate text-fc-sm font-semibold text-fc-fg">{space.name}</div>
									<div class="text-fc-xs text-fc-fg-muted">{plural(space.member_count)}</div>
								</div>
								<Badge tone={ROLE_TONE[space.role]}>{t(`role.${space.role}` as MessageKey)}</Badge>
							</div>
						</Card>
					{/each}
				</div>
			{/if}
		</Section>

		<Section title={t('spaces.new')} description={t('spaces.newHint')}>
			<form class="flex flex-col gap-3 sm:flex-row sm:items-end" onsubmit={create}>
				<Field label={t('spaces.name')} class="flex-1">
					<Input required maxlength={80} disabled={creating} bind:value={name} placeholder={t('spaces.namePlaceholder')} />
				</Field>
				<Button type="submit" size="lg" icon={icons.plus} class="w-full sm:w-auto" disabled={creating || !name.trim()}>
					{creating ? t('common.saving') : t('spaces.create')}
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
