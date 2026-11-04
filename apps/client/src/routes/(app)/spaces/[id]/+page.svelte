<script lang="ts">
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
			error = accountError(cause, 'Could not load the space.');
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
			toast.success(`Renamed to ${saved.name}.`);
			await load();
		} catch (cause) {
			toast.danger(accountError(cause, 'Could not rename the space.'));
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
			toast.danger(accountError(cause, 'Could not delete the space.'));
		}
	}

	/** leave takes me out of the space; the last owner is refused with a reason. */
	async function leave() {
		if (!space) return;
		try {
			await spaces.leave(space.id);
			toast.success(`You left ${space.name}.`);
			await goto('/spaces');
		} catch (cause) {
			toast.danger(accountError(cause, 'Could not leave the space.'));
		}
	}
</script>

<svelte:head>
	<title>{space?.name ?? 'Space'} — Registre</title>
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
			title="No such space"
			description="It does not exist, or you are not one of its members."
		>
			<Button variant="outline" href="/spaces">Your spaces</Button>
		</EmptyState>
	{:else if error || !space}
		<Alert tone="danger" title="Could not load the space">{error}</Alert>
	{:else}
		<PageHeader
			title={space.name}
			description="You are {space.role}. Changes here reach every Facile app within seconds."
			back={{ href: '/spaces', label: 'Spaces' }}
		/>

		{#if manager}
			<SettingsSection title="Name" description="Unique across the suite.">
				<form class="flex flex-col gap-3 sm:flex-row" onsubmit={rename}>
					<Input aria-label="Space name" required maxlength={80} disabled={renaming} bind:value={name} class="flex-1" />
					<Button
						type="submit"
						variant="outline"
						icon={icons.edit}
						class="w-full sm:w-auto"
						disabled={renaming || !name.trim() || name.trim() === space.name}
					>
						{renaming ? 'Saving…' : 'Rename'}
					</Button>
				</form>
			</SettingsSection>
		{/if}

		<SettingsSection title="Members" description="Owners and admins add people and set their roles.">
			<SpaceMembers {space} myEmail={shell.me?.email ?? ''} onchange={load} />
			{#if manager}
				<div class="mt-4">
					<SpaceAddMember {space} onchange={load} />
				</div>
			{/if}
		</SettingsSection>

		<SpacePlan spaceID={space.id} />

		<SettingsSection title="Leave">
			<SettingsRow label="Leave {space.name}" description="You lose access in every app. The last owner cannot leave.">
				<Button variant="outline" icon={icons.logout} onclick={() => (leaveOpen = true)}>Leave</Button>
			</SettingsRow>
		</SettingsSection>

		{#if space.role === 'owner'}
			<SettingsSection title="Delete">
				<SettingsRow
					label="Delete {space.name}"
					description="Removes the space and everyone in it from every Facile app. Files, events and tasks inside the apps are kept."
				>
					<Button variant="danger" icon={icons.remove} onclick={() => (deleteOpen = true)}>Delete</Button>
				</SettingsRow>
			</SettingsSection>
		{/if}
	{/if}
</Page>

<ConfirmModal
	bind:open={leaveOpen}
	tone="danger"
	title="Leave {space?.name ?? ''}?"
	description="You stop seeing it in every Facile app. Someone with admin or above has to add you back."
	confirmLabel="Leave"
	cancelLabel="Stay"
	onConfirm={leave}
/>

<ConfirmModal
	bind:open={deleteOpen}
	tone="danger"
	title="Delete {space?.name ?? ''}?"
	description="Everyone loses this space in every Facile app within seconds. This cannot be undone here."
	confirmLabel="Delete"
	cancelLabel="Keep"
	onConfirm={destroy}
/>
