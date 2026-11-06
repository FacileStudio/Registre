<script lang="ts">
	import { Alert, Button, Drawer, Field, Input, Select, Textarea, icons } from '@facile/muse';
	import { admin, errorMessage, type ClientView } from '$lib/admin';

	let {
		open = $bindable(false),
		client,
		onSaved,
		onCancel
	}: {
		open: boolean;
		client: ClientView | null;
		onSaved: (updated: ClientView) => void;
		onCancel: () => void;
	} = $props();

	let name = $state('');
	let kind = $state('application');
	let redirects = $state('');
	let roles = $state('');
	let logoutURI = $state('');
	let deletionURI = $state('');
	let scopes = $state('');
	let audiences = $state('');
	let saving = $state(false);
	let error = $state('');

	$effect(() => {
		if (client && open) {
			name = client.name || '';
			kind = client.kind || 'application';
			redirects = (client.redirect_uris || []).join('\n');
			roles = (client.visible_roles || []).join('\n');
			logoutURI = client.backchannel_logout_uri || '';
			deletionURI = client.deletion_uri || '';
			scopes = (client.scopes || []).join(' ');
			audiences = (client.audiences || []).join(' ');
			error = '';
		}
	});

	function lines(raw: string): string[] {
		return raw
			.split('\n')
			.map((l) => l.trim())
			.filter(Boolean);
	}

	function tokens(raw: string): string[] {
		return raw
			.split(/[\s,]+/)
			.map((t) => t.trim())
			.filter(Boolean);
	}

	const isApplication = $derived(kind === 'application');

	async function save(event: SubmitEvent) {
		event.preventDefault();
		if (!client || saving) return;
		saving = true;
		error = '';
		try {
			const updated = await admin.updateClient(client.client_id, {
				name: name.trim(),
				kind,
				redirect_uris: isApplication ? lines(redirects) : undefined,
				visible_roles: isApplication ? lines(roles) : undefined,
				backchannel_logout_uri: isApplication ? logoutURI.trim() || undefined : undefined,
				deletion_uri: isApplication ? deletionURI.trim() || undefined : undefined,
				scopes: tokens(scopes),
				audiences: tokens(audiences)
			});
			open = false;
			onSaved(updated);
		} catch (cause) {
			error = errorMessage(cause, 'Could not update the client.');
		} finally {
			saving = false;
		}
	}
</script>

<Drawer bind:open title="Edit {client?.client_id ?? 'client'}" showClose>
	<form class="flex flex-col gap-4" onsubmit={save}>
		<div aria-live="polite">
			{#if error}
				<Alert tone="danger" title={error} />
			{/if}
		</div>
		<Field label="Client ID">
			<Input disabled class="font-fc-mono" value={client?.client_id ?? ''} />
		</Field>
		<Field label="Name">
			<Input required disabled={saving} bind:value={name} />
		</Field>
		<Field label="Kind">
			<Select disabled={saving} bind:value={kind}>
				<option value="application">application</option>
				<option value="service_account">service account</option>
			</Select>
		</Field>
		{#if isApplication}
			<Field
				label="Redirect URIs"
				helper="One per line, matched exactly."
			>
				<Textarea
					required
					rows={3}
					disabled={saving}
					class="font-fc-mono"
					placeholder="https://sablier.facile.studio/auth/oidc/callback"
					bind:value={redirects}
				/>
			</Field>
			<Field
				label="Visible roles"
				helper="One per line. An app sees no roles unless listed here."
			>
				<Textarea
					rows={2}
					disabled={saving}
					class="font-fc-mono"
					placeholder="admin"
					bind:value={roles}
				/>
			</Field>
			<Field
				label="Back-channel logout URI"
				helper="Where registre posts a logout token."
			>
				<Input
					disabled={saving}
					class="font-fc-mono"
					placeholder="https://sablier.facile.studio/auth/backchannel-logout"
					bind:value={logoutURI}
				/>
			</Field>
			<Field
				label="Deletion URI"
				helper="Where Registre pushes the signed user.deleted event. Not the sync URI. Empty: the app polls /api/directory/deletions."
			>
				<Input
					type="url"
					disabled={saving}
					class="font-fc-mono"
					placeholder="https://sablier.facile.studio/api/registre/deleted"
					bind:value={deletionURI}
				/>
			</Field>
		{/if}
		<Field label="Scopes" helper="Space-separated list of scopes.">
			<Input disabled={saving} class="font-fc-mono" bind:value={scopes} />
		</Field>
		{#if !isApplication}
			<Field label="Audiences" helper="Space-separated list of audiences.">
				<Input disabled={saving} class="font-fc-mono" bind:value={audiences} />
			</Field>
		{/if}
		<div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
			<Button
				type="button"
				variant="ghost"
				class="w-full sm:w-auto"
				disabled={saving}
				onclick={onCancel}
			>
				Cancel
			</Button>
			<Button
				type="submit"
				class="w-full sm:w-auto"
				disabled={saving || name.trim().length === 0}
			>
				{saving ? 'Saving…' : 'Save changes'}
			</Button>
		</div>
	</form>
</Drawer>
