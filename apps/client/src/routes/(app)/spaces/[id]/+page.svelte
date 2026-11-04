<script lang="ts">
	import { t } from '$lib/i18n/i18n.svelte';
	import type { MessageKey } from '$lib/i18n/en';
	import { getContext } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import {
		Alert,
		Button,
		ConfirmModal,
		EmptyState,
		Input,
		Page,
		PageHeader,
		SettingsRow,
		SettingsSection,
		Skeleton,
		Stack,
		icons,
		toast
	} from '@facile/muse';
	import { AccountError, accountError, SESSION_CONTEXT, type ShellSession } from '$lib/account';
	import { canManage, spaces, type SpaceDetail } from '$lib/spaces';
	import SpaceAddMember from '$lib/components/SpaceAddMember.svelte';
	import SpaceMembers from '$lib/components/SpaceMembers.svelte';
	import SpacePlan from '$lib/components/SpacePlan.svelte';

	const shell = getContext<ShellSession>(SESSION_CONTEXT);

	let space = $state<SpaceDetail | null>(null);
	let loading = $state(true);
	let missing = $state(false);
	let error = $state('');

	let name = $state('');
	let renaming = $state(false);
	let leaveOpen = $state(false);
	let deleteOpen = $state(false);

	const id = $derived(page.params.id ?? '');
	const manager = $derived(space ? canManage(space.role) : false);

	$effect(() => {
		if (id) load();
	});

	/** load fetches the space; a 404 means it does not exist or I am not in it, which the API does not tell apart. */
	async function load() {
		try {
			space = await spaces.get(id);
			name = space.name;
			error = '';
		} catch (cause) {
			missing = cause instanceof AccountError && (cause.status === 404 || cause.status === 401);
			error = accountError(cause, t('spaces.loadOneFailed'));
		} finally {
			loading = false;
		}
	}

	/** rename saves a new name, which every app picks up on its next sync. */
	async function rename(event: SubmitEvent) {
		event.preventDefault();
		if (!space || renaming) return;
		renaming = true;
		try {
			const saved = await spaces.rename(space.id, name.trim());
			toast.success(t('spaces.renamed', { name: saved.name }));
			await load();
		} catch (cause) {
			toast.danger(accountError(cause, t('common.failed')));
		} finally {
			renaming = false;
		}
	}

	/** destroy deletes the space for everybody in every app; owners only. */
	async function destroy() {
		if (!space) return;
		try {
			await spaces.remove(space.id);
			toast.success(`${space.name} is deleted in every app.`);
			await goto('/spaces');
		} catch (cause) {
			toast.danger(accountError(cause, t('common.failed')));
		}
	}

	/** leave takes me out of the space; the last owner is refused with a reason. */
	async function leave() {
		if (!space) return;
		try {
			await spaces.leave(space.id);
			toast.success(t('spaces.left', { name: space.name }));
			await goto('/spaces');
		} catch (cause) {
			toast.danger(accountError(cause, t('common.failed')));
		}
	}
</script>

<svelte:head>
	<title>{space?.name ?? t('spaces.space')} — Facile</title>
</svelte:head>

<Page width="lg">
	{#if loading}
		<Stack>
			<Skeleton class="h-9 w-64" />
			<Skeleton class="h-40 w-full" />
		</Stack>
	{:else if missing}
		<EmptyState
			icon={icons.space}
			title={t('spaces.noSuch')}
			description={t('spaces.noSuchHint')}
		>
			<Button variant="outline" href="/spaces">{t('home.spaces')}</Button>
		</EmptyState>
	{:else if error || !space}
		<Alert tone="danger" title={t('spaces.loadOneFailed')}>{error}</Alert>
	{:else}
		<PageHeader
			title={space.name}
			description={t('spaces.youAre', { role: t(`role.${space.role}` as MessageKey) })}
			back={{ href: '/spaces', label: t('nav.spaces') }}
		/>

		{#if manager}
			<SettingsSection title={t('spaces.name')} description={t('spaces.unique')}>
				<form class="flex flex-col gap-3 sm:flex-row" onsubmit={rename}>
					<Input aria-label={t('spaces.name')} required maxlength={80} disabled={renaming} bind:value={name} class="flex-1" />
					<Button
						type="submit"
						variant="outline"
						icon={icons.edit}
						class="w-full sm:w-auto"
						disabled={renaming || !name.trim() || name.trim() === space.name}
					>
						{renaming ? t('common.saving') : t('passkeys.rename')}
					</Button>
				</form>
			</SettingsSection>
		{/if}

		<SettingsSection title={t('spaces.membersTitle')} description={t('spaces.membersHint')}>
			<SpaceMembers {space} myEmail={shell.me?.email ?? ''} onchange={load} />
			{#if manager}
				<div class="mt-4">
					<SpaceAddMember {space} onchange={load} />
				</div>
			{/if}
		</SettingsSection>

		<SpacePlan spaceID={space.id} />

		<SettingsSection title={t('spaces.leave')}>
			<SettingsRow label={t('spaces.leaveName', { name: space.name })} description={t('spaces.leaveHint')}>
				<Button variant="outline" icon={icons.logout} onclick={() => (leaveOpen = true)}>{t('spaces.leave')}</Button>
			</SettingsRow>
		</SettingsSection>

		{#if space.role === 'owner'}
			<SettingsSection title={t('spaces.delete')}>
				<SettingsRow
					label={t('spaces.deleteName', { name: space.name })}
					description={t('spaces.deleteHint')}
				>
					<Button variant="danger" icon={icons.remove} onclick={() => (deleteOpen = true)}>{t('spaces.delete')}</Button>
				</SettingsRow>
			</SettingsSection>
		{/if}
	{/if}
</Page>

<ConfirmModal
	bind:open={leaveOpen}
	tone="danger"
	title={t('spaces.leaveTitle', { name: space?.name ?? '' })}
	description={t('spaces.leaveBody')}
	confirmLabel={t('spaces.leave')}
	cancelLabel={t('spaces.stay')}
	onConfirm={leave}
/>

<ConfirmModal
	bind:open={deleteOpen}
	tone="danger"
	title={t('spaces.deleteTitle', { name: space?.name ?? '' })}
	description={t('spaces.deleteBody')}
	confirmLabel={t('spaces.delete')}
	cancelLabel={t('common.keep')}
	onConfirm={destroy}
/>
