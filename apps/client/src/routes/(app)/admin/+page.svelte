<script lang="ts">
	import {
		Alert,
		Badge,
		Button,
		Checkbox,
		ConfirmModal,
		EmptyState,
		Field,
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

	let createOpen = $state(false);
	let creating = $state(false);
	let createError = $state('');
	let newEmail = $state('');
	let newName = $state('');
	let newPassword = $state('');
	let newAdmin = $state(false);

	let disableTarget = $state<UserView | null>(null);
	let disableOpen = $state(false);

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
</script>

<Section
	title="Users"
	description="People with an account here. Disabling one ends their sessions in every app."
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
				{#each users as user (user.id)}
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
							{#if !user.disabled_at}
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
						</td>
					</tr>
				{/each}
			</tbody>
		</Table>
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

<ConfirmModal
	bind:open={disableOpen}
	tone="danger"
	title="Disable {disableTarget?.email ?? ''}?"
	description="They stop being able to sign in, and every app holding a session for them is told to end it. Nothing is deleted, but re-enabling is not something this dashboard can do yet."
	confirmLabel="Disable the account"
	cancelLabel="Keep it active"
	onConfirm={disable}
	onCancel={() => (disableTarget = null)}
/>
