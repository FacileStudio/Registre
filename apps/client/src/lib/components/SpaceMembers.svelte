<script lang="ts">
	import { Avatar, Badge, ConfirmModal, Icon, IconButton, Select, Table, icons, toast } from '@facile/muse';
	import { accountError } from '$lib/account';
	import {
		ROLE_TONE,
		assignable,
		canEdit,
		ownerCount,
		spaces,
		type Role,
		type SpaceDetail,
		type SpaceMember
	} from '$lib/spaces';

	let {
		space,
		myEmail,
		onchange
	}: { space: SpaceDetail; myEmail: string; onchange: () => Promise<void> } = $props();

	let busy = $state('');
	let removeOpen = $state(false);
	let removeTarget = $state<SpaceMember | null>(null);

	const owners = $derived(ownerCount(space.members));
	const grantable = $derived(assignable(space.role));
	const members = $derived(
		[...space.members].sort((a, b) => (a.name || a.email).localeCompare(b.name || b.email))
	);

	/** isMe reports whether a row is the signed-in person, who leaves rather than being removed. */
	const isMe = (member: SpaceMember) => member.email.toLowerCase() === myEmail.toLowerCase();

	/** editable reports whether this page offers to change or remove a row. */
	const editable = (member: SpaceMember) => !isMe(member) && canEdit(space.role, member, owners);

	/** setRole asks the API for the new role and reloads, putting the select back if it refuses. */
	async function setRole(member: SpaceMember, role: Role) {
		if (role === member.role) return;
		busy = member.user_id;
		try {
			await spaces.setRole(space.id, member.user_id, role);
			toast.success(`${member.name || member.email} is now ${role}.`);
		} catch (cause) {
			toast.danger(accountError(cause, 'Could not change the role.'));
		} finally {
			await onchange();
			busy = '';
		}
	}

	/** remove takes the confirmed member out of the space in every app. */
	async function remove() {
		const target = removeTarget;
		if (!target) return;
		busy = target.user_id;
		try {
			await spaces.removeMember(space.id, target.user_id);
			toast.success(`${target.name || target.email} was removed from ${space.name}.`);
		} catch (cause) {
			toast.danger(accountError(cause, 'Could not remove the member.'));
		} finally {
			removeTarget = null;
			await onchange();
			busy = '';
		}
	}
</script>

<Table>
	<thead>
		<tr>
			<th scope="col">Member</th>
			<th scope="col">Role</th>
			<th scope="col"><span class="sr-only">Actions</span></th>
		</tr>
	</thead>
	<tbody>
		{#each members as member (member.user_id)}
			<tr>
				<td>
					<div class="flex min-w-0 items-center gap-3">
						<Avatar src={member.picture || undefined} name={member.name || member.email} size="sm" />
						<div class="min-w-0">
							<div class="truncate text-fc-sm font-medium text-fc-fg">
								{member.name || member.email}
								{#if isMe(member)}<span class="text-fc-fg-muted">(you)</span>{/if}
							</div>
							{#if member.name}
								<div class="truncate font-fc-mono text-fc-xs text-fc-fg-muted">{member.email}</div>
							{/if}
						</div>
					</div>
				</td>
				<td>
					{#if editable(member)}
						<Select
							aria-label="Role of {member.name || member.email}"
							class="min-w-28"
							value={member.role}
							disabled={busy !== ''}
							onchange={(event) => setRole(member, event.currentTarget.value as Role)}
						>
							{#each grantable as role (role)}
								<option value={role}>{role}</option>
							{/each}
						</Select>
					{:else}
						<Badge tone={ROLE_TONE[member.role]}>{member.role}</Badge>
					{/if}
				</td>
				<td class="text-right">
					{#if editable(member)}
						<IconButton
							variant="ghost"
							aria-label="Remove {member.name || member.email}"
							disabled={busy !== ''}
							onclick={() => {
								removeTarget = member;
								removeOpen = true;
							}}
						>
							<Icon icon={icons.remove} size={16} />
						</IconButton>
					{/if}
				</td>
			</tr>
		{/each}
	</tbody>
</Table>

<ConfirmModal
	bind:open={removeOpen}
	tone="danger"
	title="Remove {removeTarget?.name || removeTarget?.email || ''}?"
	description="They lose access to {space.name} in every Facile app within a few seconds."
	confirmLabel="Remove"
	cancelLabel="Keep"
	onConfirm={remove}
	onCancel={() => (removeTarget = null)}
/>
