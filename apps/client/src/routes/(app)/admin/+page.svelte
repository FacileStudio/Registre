<script lang="ts">
	import {
		Alert,
		Badge,
		Button,
		Checkbox,
		ConfirmModal,
		EmptyState,
		Field,
		Icon,
		Input,
		Modal,
		Section,
		Skeleton,
		Table,
		icons,
		toast
	} from '@facile/muse';
	import { MIN_PASSWORD } from '$lib/policy';
	import { admin, errorMessage, formatDate, type UserView } from '$lib/admin';

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

<Modal bind:open={createOpen} title="New user" showClose size="sm">
	<form class="flex flex-col gap-4" onsubmit={create}>
		<div aria-live="polite">
			{#if createError}
				<Alert tone="danger" title={createError} />
			{/if}
		</div>
		<Field label="Email" helper="Verified by construction — you are attesting the address.">
			<Input type="email" required disabled={creating} bind:value={newEmail} />
		</Field>
		<Field label="Name" helper="Optional. Shown in the apps they sign in to.">
			<Input disabled={creating} bind:value={newName} />
		</Field>
		<Field label="Password" helper="At least {MIN_PASSWORD} characters. Tell them to change it.">
			<Input
				type="password"
				autocomplete="new-password"
				minlength={MIN_PASSWORD}
				required
				disabled={creating}
				bind:value={newPassword}
			/>
		</Field>
		<Checkbox
			bind:checked={newAdmin}
			disabled={creating}
			label="Operator — can reach this dashboard"
		/>
		<div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
			<Button
				type="button"
				variant="ghost"
				class="w-full sm:w-auto"
				disabled={creating}
				onclick={() => (createOpen = false)}
			>
				Cancel
			</Button>
			<Button
				type="submit"
				icon={icons.plus}
				class="w-full sm:w-auto"
				disabled={creating || newEmail.trim().length === 0 || newPassword.length < MIN_PASSWORD}
			>
				{creating ? 'Creating…' : 'Create user'}
			</Button>
		</div>
	</form>
</Modal>

<Modal bind:open={passwordOpen} title="Change password for {passwordTarget?.email ?? ''}" showClose size="sm">
	<form class="flex flex-col gap-4" onsubmit={submitPassword}>
		<div aria-live="polite">
			{#if passwordError}
				<Alert tone="danger" title={passwordError} />
			{/if}
		</div>
		<p class="text-fc-xs text-fc-fg-muted">
			Setting a new password immediately ends all active sessions for this account across all apps and services.
		</p>
		<Field label="New password" helper="At least {MIN_PASSWORD} characters.">
			<Input
				type="password"
				autocomplete="new-password"
				minlength={MIN_PASSWORD}
				required
				disabled={updatingPassword}
				bind:value={changePasswordValue}
			/>
		</Field>
		<div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
			<Button
				type="button"
				variant="ghost"
				class="w-full sm:w-auto"
				disabled={updatingPassword}
				onclick={() => (passwordOpen = false)}
			>
				Cancel
			</Button>
			<Button
				type="submit"
				icon={icons.key}
				class="w-full sm:w-auto"
				disabled={updatingPassword || changePasswordValue.length < MIN_PASSWORD}
			>
				{updatingPassword ? 'Updating…' : 'Update password'}
			</Button>
		</div>
	</form>
</Modal>

<Modal bind:open={editOpen} title="Edit user" showClose size="sm">
	<form class="flex flex-col gap-4" onsubmit={submitEdit}>
		<div aria-live="polite">
			{#if editError}
				<Alert tone="danger" title={editError} />
			{/if}
		</div>
		<Field label="Email">
			<Input type="email" disabled value={editTarget?.email ?? ''} />
		</Field>
		<Field label="Name" helper="Shown in the apps they sign in to.">
			<Input disabled={editing} bind:value={editName} />
		</Field>
		<Checkbox
			bind:checked={editAdmin}
			disabled={editing}
			label="Operator — can reach this administration dashboard"
		/>
		<div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
			<Button
				type="button"
				variant="ghost"
				class="w-full sm:w-auto"
				disabled={editing}
				onclick={() => (editOpen = false)}
			>
				Cancel
			</Button>
			<Button
				type="submit"
				class="w-full sm:w-auto"
				disabled={editing}
			>
				{editing ? 'Saving…' : 'Save changes'}
			</Button>
		</div>
	</form>
</Modal>

<ConfirmModal
	bind:open={disableOpen}
	tone="danger"
	title="Disable {disableTarget?.email ?? ''}?"
	description="They stop being able to sign in, and every app holding a session for them is told to end it."
	confirmLabel="Disable the account"
	cancelLabel="Keep it active"
	onConfirm={disable}
	onCancel={() => (disableTarget = null)}
/>

<ConfirmModal
	bind:open={enableOpen}
	title="Enable {enableTarget?.email ?? ''}?"
	description="The account will be re-activated and the user will be able to sign in again."
	confirmLabel="Enable the account"
	cancelLabel="Leave disabled"
	onConfirm={enable}
	onCancel={() => (enableTarget = null)}
/>

<ConfirmModal
	bind:open={deleteOpen}
	tone="danger"
	title="Delete {deleteTarget?.email ?? ''}?"
	description="This account and its sessions will be permanently removed. This cannot be undone."
	confirmLabel="Delete account"
	cancelLabel="Cancel"
	onConfirm={removeUser}
	onCancel={() => (deleteTarget = null)}
/>
