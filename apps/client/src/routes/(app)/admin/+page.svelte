<script lang="ts">
	import {
		Alert,
		Badge,
		Button,
		EmptyState,
		Icon,
		Input,
		Section,
		Skeleton,
		Table,
		icons,
		toast
	} from '@facile/muse';
	import { MIN_PASSWORD } from '$lib/policy';
	import { admin, errorMessage, formatDate, type UserView } from '$lib/admin';
	import UserModals from '$lib/components/UserModals.svelte';

	let users = $state<UserView[]>([]);
	let loading = $state(true);
	let error = $state('');
	let search = $state('');

	let createOpen = $state(false);
	let creating = $state(false);
	let createError = $state('');
	let newEmail = $state('');
	let newName = $state('');
	let newPassword = $state('');
	let newAdmin = $state(false);

	let passwordTarget = $state<UserView | null>(null);
	let passwordOpen = $state(false);
	let updatingPassword = $state(false);
	let passwordError = $state('');
	let changePasswordValue = $state('');

	let editTarget = $state<UserView | null>(null);
	let editOpen = $state(false);
	let editing = $state(false);
	let editError = $state('');
	let editName = $state('');
	let editAdmin = $state(false);

	let disableTarget = $state<UserView | null>(null);
	let disableOpen = $state(false);

	let enableTarget = $state<UserView | null>(null);
	let enableOpen = $state(false);

	let deleteTarget = $state<UserView | null>(null);
	let deleteOpen = $state(false);

	const filteredUsers = $derived.by(() => {
		const q = search.toLowerCase().trim();
		if (!q) return users;
		return users.filter(
			(u) => u.email.toLowerCase().includes(q) || (u.name && u.name.toLowerCase().includes(q))
		);
	});

	$effect(() => {
		load();
	});

	async function load() {
		try {
			users = await admin.listUsers();
			error = '';
		} catch (cause) {
			error = errorMessage(cause, 'Could not load the users.');
		} finally {
			loading = false;
		}
	}

	function openCreate() {
		newEmail = '';
		newName = '';
		newPassword = '';
		newAdmin = false;
		createError = '';
		createOpen = true;
	}

	async function create(event: SubmitEvent) {
		event.preventDefault();
		if (creating) return;
		creating = true;
		createError = '';
		try {
			await admin.createUser({
				email: newEmail.trim(),
				name: newName.trim(),
				password: newPassword,
				admin: newAdmin
			});
			createOpen = false;
			toast.success(`${newEmail.trim()} can now sign in.`);
			await load();
		} catch (cause) {
			createError = errorMessage(cause, 'Could not create the user.');
		} finally {
			creating = false;
		}
	}

	function openPassword(user: UserView) {
		passwordTarget = user;
		changePasswordValue = '';
		passwordError = '';
		passwordOpen = true;
	}

	async function submitPassword(event: SubmitEvent) {
		event.preventDefault();
		const target = passwordTarget;
		if (!target || updatingPassword) return;
		updatingPassword = true;
		passwordError = '';
		try {
			const result = await admin.setUserPassword(target.id, changePasswordValue);
			passwordOpen = false;
			toast.success(
				`Password updated for ${target.email}. ${result.logout_targets} app session${result.logout_targets === 1 ? '' : 's'} revoked.`
			);
			passwordTarget = null;
		} catch (cause) {
			passwordError = errorMessage(cause, 'Could not change the password.');
		} finally {
			updatingPassword = false;
		}
	}

	function openEdit(user: UserView) {
		editTarget = user;
		editName = user.name || '';
		editAdmin = user.admin;
		editError = '';
		editOpen = true;
	}

	async function submitEdit(event: SubmitEvent) {
		event.preventDefault();
		const target = editTarget;
		if (!target || editing) return;
		editing = true;
		editError = '';
		try {
			await admin.updateUser(target.id, {
				name: editName.trim(),
				admin: editAdmin
			});
			editOpen = false;
			toast.success(`Account details updated for ${target.email}.`);
			editTarget = null;
			await load();
		} catch (cause) {
			editError = errorMessage(cause, 'Could not update user details.');
		} finally {
			editing = false;
		}
	}

	async function disable() {
		const target = disableTarget;
		if (!target) return;
		try {
			const result = await admin.disableUser(target.id);
			toast.success(
				`${target.email} is disabled. ${result.logout_targets} app${result.logout_targets === 1 ? '' : 's'} told to end the session.`
			);
			disableTarget = null;
			await load();
		} catch (cause) {
			toast.danger(errorMessage(cause, 'Could not disable that account.'));
			throw cause;
		}
	}

	async function enable() {
		const target = enableTarget;
		if (!target) return;
		try {
			await admin.enableUser(target.id);
			toast.success(`${target.email} is now re-enabled.`);
			enableTarget = null;
			await load();
		} catch (cause) {
			toast.danger(errorMessage(cause, 'Could not enable that account.'));
			throw cause;
		}
	}

	async function removeUser() {
		const target = deleteTarget;
		if (!target) return;
		try {
			await admin.deleteUser(target.id);
			toast.success(`${target.email} was deleted.`);
			deleteTarget = null;
			await load();
		} catch (cause) {
			toast.danger(errorMessage(cause, 'Could not delete that account.'));
			throw cause;
		}
	}
</script>

<Section
	title="Users"
	description="People with an account here. Manage credentials, operator status, and authentication access."
	bodyClass="flex flex-col gap-4"
>
	{#snippet actions()}
		<Button icon={icons.plus} onclick={openCreate}>New user</Button>
	{/snippet}

	{#if error}
		<Alert tone="danger" title="Could not load the users">{error}</Alert>
	{:else if loading}
		<div class="flex flex-col gap-2">
			<Skeleton class="h-10 w-full" />
			<Skeleton class="h-10 w-full" />
			<Skeleton class="h-10 w-full" />
		</div>
	{:else if users.length === 0}
		<EmptyState
			icon={icons.usersGroup}
			title="No accounts yet"
			description="Create the first one, or send an invitation and let them choose their own password."
		>
			<Button variant="outline" icon={icons.plus} onclick={openCreate}>New user</Button>
		</EmptyState>
	{:else}
		<div class="flex flex-col gap-3">
			<div class="relative w-full max-w-xs">
				<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-fc-fg-muted">
					<Icon icon={icons.search} size={15} />
				</div>
				<input
					type="text"
					bind:value={search}
					placeholder="Search users..."
					class="w-full rounded-fc-md border border-fc-border bg-fc-surface py-1.5 pl-9 pr-3 text-fc-xs
						text-fc-fg placeholder-fc-fg-muted outline-none transition-colors focus:border-fc-ring"
				/>
			</div>

			<Table>
				<thead>
					<tr>
						<th scope="col">Email</th>
						<th scope="col">Name</th>
						<th scope="col">Role</th>
						<th scope="col">Status</th>
						<th scope="col">Created</th>
						<th scope="col" class="text-right" aria-label="Actions"></th>
					</tr>
				</thead>
				<tbody>
					{#each filteredUsers as user (user.id)}
						<tr class={user.disabled_at ? 'opacity-55' : ''}>
							<td class="font-fc-mono text-fc-xs">{user.email}</td>
							<td>{user.name || '—'}</td>
							<td>
								{#if user.admin}
									<Badge tone="admin">operator</Badge>
								{:else}
									<Badge tone="neutral">member</Badge>
								{/if}
							</td>
							<td>
								<Badge tone={user.disabled_at ? 'neutral' : 'success'}>
									{user.disabled_at ? 'disabled' : 'active'}
								</Badge>
							</td>
							<td class="whitespace-nowrap text-fc-fg-muted">{formatDate(user.created_at)}</td>
							<td class="text-right">
								<div class="flex items-center justify-end gap-1">
									<Button
										variant="ghost"
										size="sm"
										icon={icons.key}
										aria-label="Change password for {user.email}"
										onclick={() => openPassword(user)}
									>
										Password
									</Button>
									<Button
										variant="ghost"
										size="sm"
										icon={icons.edit}
										aria-label="Edit {user.email}"
										onclick={() => openEdit(user)}
									>
										Edit
									</Button>
									{#if user.disabled_at}
										<Button
											variant="ghost"
											size="sm"
											icon={icons.check}
											aria-label="Enable {user.email}"
											onclick={() => {
												enableTarget = user;
												enableOpen = true;
											}}
										>
											Enable
										</Button>
									{:else}
										<Button
											variant="ghost-danger"
											size="sm"
											icon={icons.revoke}
											aria-label="Disable {user.email}"
											onclick={() => {
												disableTarget = user;
												disableOpen = true;
											}}
										>
											Disable
										</Button>
									{/if}
									<Button
										variant="ghost-danger"
										size="sm"
										icon={icons.remove}
										aria-label="Delete {user.email}"
										onclick={() => {
											deleteTarget = user;
											deleteOpen = true;
										}}
									>
										Delete
									</Button>
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</Table>
		</div>
	{/if}
</Section>

<UserModals
	bind:createOpen
	{creating}
	{createError}
	bind:newEmail
	bind:newName
	bind:newPassword
	bind:newAdmin
	onCreate={create}
	bind:passwordOpen
	bind:passwordTarget
	{updatingPassword}
	{passwordError}
	bind:changePasswordValue
	onSubmitPassword={submitPassword}
	bind:editOpen
	bind:editTarget
	{editing}
	{editError}
	bind:editName
	bind:editAdmin
	onSubmitEdit={submitEdit}
	bind:disableOpen
	bind:disableTarget
	onDisable={disable}
	bind:enableOpen
	bind:enableTarget
	onEnable={enable}
	bind:deleteOpen
	bind:deleteTarget
	onDelete={removeUser}
/>
