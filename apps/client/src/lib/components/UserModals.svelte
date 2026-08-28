<script lang="ts">
	import { Alert, Button, Checkbox, ConfirmModal, Field, Input, Modal, icons } from '@facile/muse';
	import { MIN_PASSWORD } from '$lib/policy';
	import type { UserView } from '$lib/admin';

	interface Props {
		createOpen?: boolean;
		creating?: boolean;
		createError?: string;
		newEmail?: string;
		newName?: string;
		newPassword?: string;
		newAdmin?: boolean;
		onCreate: (e: SubmitEvent) => void;
		passwordOpen?: boolean;
		passwordTarget?: UserView | null;
		updatingPassword?: boolean;
		passwordError?: string;
		changePasswordValue?: string;
		onSubmitPassword: (e: SubmitEvent) => void;
		editOpen?: boolean;
		editTarget?: UserView | null;
		editing?: boolean;
		editError?: string;
		editName?: string;
		editAdmin?: boolean;
		onSubmitEdit: (e: SubmitEvent) => void;
		disableOpen?: boolean;
		disableTarget?: UserView | null;
		onDisable: () => void;
		enableOpen?: boolean;
		enableTarget?: UserView | null;
		onEnable: () => void;
		deleteOpen?: boolean;
		deleteTarget?: UserView | null;
		onDelete: () => void;
	}

	let {
		createOpen = $bindable(false),
		creating = false,
		createError = '',
		newEmail = $bindable(''),
		newName = $bindable(''),
		newPassword = $bindable(''),
		newAdmin = $bindable(false),
		onCreate,
		passwordOpen = $bindable(false),
		passwordTarget = $bindable(null),
		updatingPassword = false,
		passwordError = '',
		changePasswordValue = $bindable(''),
		onSubmitPassword,
		editOpen = $bindable(false),
		editTarget = $bindable(null),
		editing = false,
		editError = '',
		editName = $bindable(''),
		editAdmin = $bindable(false),
		onSubmitEdit,
		disableOpen = $bindable(false),
		disableTarget = $bindable(null),
		onDisable,
		enableOpen = $bindable(false),
		enableTarget = $bindable(null),
		onEnable,
		deleteOpen = $bindable(false),
		deleteTarget = $bindable(null),
		onDelete
	}: Props = $props();
</script>

<Modal bind:open={createOpen} title="New user" showClose size="sm">
	<form class="flex flex-col gap-4" onsubmit={onCreate}>
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

<Modal
	bind:open={passwordOpen}
	title="Change password for {passwordTarget?.email ?? ''}"
	showClose
	size="sm"
>
	<form class="flex flex-col gap-4" onsubmit={onSubmitPassword}>
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
	<form class="flex flex-col gap-4" onsubmit={onSubmitEdit}>
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
	onConfirm={onDisable}
	onCancel={() => (disableTarget = null)}
/>

<ConfirmModal
	bind:open={enableOpen}
	title="Enable {enableTarget?.email ?? ''}?"
	description="The account will be re-activated and the user will be able to sign in again."
	confirmLabel="Enable the account"
	cancelLabel="Leave disabled"
	onConfirm={onEnable}
	onCancel={() => (enableTarget = null)}
/>

<ConfirmModal
	bind:open={deleteOpen}
	tone="danger"
	title="Delete {deleteTarget?.email ?? ''}?"
	description="This account and its sessions will be permanently removed. This cannot be undone."
	confirmLabel="Delete account"
	cancelLabel="Cancel"
	onConfirm={onDelete}
	onCancel={() => (deleteTarget = null)}
/>
