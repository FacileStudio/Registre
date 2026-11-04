<script lang="ts">
	import { t } from '$lib/i18n/i18n.svelte';
	import type { MessageKey } from '$lib/i18n/en';
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
			toast.success(t('spaces.roleChanged', { name: member.name || member.email, role: t(`role.${role}` as MessageKey) }));
		} catch (cause) {
			toast.danger(accountError(cause, t('common.failed')));
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
			toast.success(t('spaces.removed', { name: target.name || target.email, space: space.name }));
		} catch (cause) {
			toast.danger(accountError(cause, t('common.failed')));
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
			<th scope="col">{t('spaces.member')}</th>
			<th scope="col">{t('spaces.role')}</th>
			<th scope="col"><span class="sr-only">{t('spaces.actions')}</span></th>
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
							aria-label={t('spaces.roleOf', { name: member.name || member.email })}
							class="min-w-28"
							value={member.role}
							disabled={busy !== ''}
							onchange={(event) => setRole(member, event.currentTarget.value as Role)}
						>
							{#each grantable as role (role)}
								<option value={role}>{t(`role.${role}` as MessageKey)}</option>
							{/each}
						</Select>
					{:else}
						<Badge tone={ROLE_TONE[member.role]}>{t(`role.${member.role}` as MessageKey)}</Badge>
					{/if}
				</td>
				<td class="text-right">
					{#if editable(member)}
						<IconButton
							variant="ghost"
							aria-label={t('spaces.removeName', { name: member.name || member.email })}
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
	title={t('spaces.removeTitle', { name: removeTarget?.name || removeTarget?.email || '' })}
	description={t('spaces.removeBody', { space: space.name })}
	confirmLabel={t('passkeys.remove')}
	cancelLabel={t('common.keep')}
	onConfirm={remove}
	onCancel={() => (removeTarget = null)}
/>
