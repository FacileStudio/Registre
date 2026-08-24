<!--
	The new-client drawer. It shows the registration form, then the secret once
	the client exists. Application fields appear only for the application kind,
	because a service account has no redirect, no visible roles and no
	back-channel logout. The page creates the client and owns the secret.
-->
<script lang="ts">
	import { Alert, Button, Drawer, Field, Input, Select, Textarea, icons } from '@facile/muse';
	import ClientSecretPanel from './ClientSecretPanel.svelte';

	let {
		open = $bindable(),
		id = $bindable(),
		name = $bindable(),
		kind = $bindable(),
		redirects = $bindable(),
		roles = $bindable(),
		logoutURI = $bindable(),
		creating,
		error,
		createdSecret,
		createdID,
		onSubmit,
		onCancel,
		onDone
	}: {
		open: boolean;
		id: string;
		name: string;
		kind: string;
		redirects: string;
		roles: string;
		logoutURI: string;
		creating: boolean;
		error: string;
		createdSecret: string;
		createdID: string;
		onSubmit: (event: SubmitEvent) => void;
		onCancel: () => void;
		onDone: () => void;
	} = $props();

	const isApplication = $derived(kind === 'application');
</script>

<Drawer bind:open title="New client" showClose>
	{#if createdSecret}
		<ClientSecretPanel
			secret={createdSecret}
			helper="Paired with client ID {createdID}."
			actionLabel="Done"
			onAction={onDone}
		>
			This is the only time the secret is shown. Registre stores a hash, so losing it means
			rotating.
		</ClientSecretPanel>
	{:else}
		<form class="flex flex-col gap-4" onsubmit={onSubmit}>
			<div aria-live="polite">
				{#if error}
					<Alert tone="danger" title={error} />
				{/if}
			</div>
			<Field label="Client ID" helper="What the app sends as client_id. Lowercase, no spaces.">
				<Input required disabled={creating} class="font-fc-mono" bind:value={id} />
			</Field>
			<Field label="Name" helper="Shown to people on the consent screen.">
				<Input required disabled={creating} bind:value={name} />
			</Field>
			<Field label="Kind">
				<Select disabled={creating} bind:value={kind}>
					<option value="service_account">service account</option>
					<option value="application">application</option>
				</Select>
			</Field>
			{#if isApplication}
				<Field
					label="Redirect URIs"
					helper="One per line, matched exactly. Without one the app can never finish a login."
				>
					<Textarea
						required
						rows={3}
						disabled={creating}
						class="font-fc-mono"
						placeholder="https://sablier.facile.studio/auth/oidc/callback"
						bind:value={redirects}
					/>
				</Field>
				<Field
					label="Visible roles"
					helper="One per line. An app sees no roles unless they are listed here, and its own authorization checks deny without them."
				>
					<Textarea
						rows={2}
						disabled={creating}
						class="font-fc-mono"
						placeholder="sablier-admin"
						bind:value={roles}
					/>
				</Field>
				<Field
					label="Back-channel logout URI"
					helper="Where registre posts a logout token. Empty means the app never learns a session was terminated."
				>
					<Input
						disabled={creating}
						class="font-fc-mono"
						placeholder="https://sablier.facile.studio/auth/backchannel-logout"
						bind:value={logoutURI}
					/>
				</Field>
			{/if}
			<div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
				<Button
					type="button"
					variant="ghost"
					class="w-full sm:w-auto"
					disabled={creating}
					onclick={onCancel}
				>
					Cancel
				</Button>
				<Button
					type="submit"
					icon={icons.plus}
					class="w-full sm:w-auto"
					disabled={creating || id.trim().length === 0 || name.trim().length === 0}
				>
					{creating ? 'Creating…' : 'Create client'}
				</Button>
			</div>
		</form>
	{/if}
</Drawer>
