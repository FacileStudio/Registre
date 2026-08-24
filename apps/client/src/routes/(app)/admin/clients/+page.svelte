<script lang="ts">
	import {
		Alert,
		Badge,
		Button,
		ConfirmModal,
		EmptyState,
		Modal,
		Section,
		Skeleton,
		Table,
		icons,
		toast
	} from '@facile/muse';
	import ClientCreateDrawer from '$lib/components/ClientCreateDrawer.svelte';
	import ClientSecretPanel from '$lib/components/ClientSecretPanel.svelte';
	import { admin, errorMessage, formatDate, type ClientView } from '$lib/admin';

	let clients = $state<ClientView[]>([]);
	let loading = $state(true);
	let error = $state('');

	let createOpen = $state(false);
	let creating = $state(false);
	let createError = $state('');
	let newID = $state('');
	let newName = $state('');
	let newKind = $state('service_account');
	let newRedirects = $state('');
	let newRoles = $state('');
	let newLogoutURI = $state('');
	let createdSecret = $state('');
	let createdID = $state('');

	let rotateTarget = $state<ClientView | null>(null);
	let rotateOpen = $state(false);
	let rotatedSecret = $state('');
	let rotatedID = $state('');
	let rotatedOpen = $state(false);

	$effect(() => {
		load();
	});

	async function load() {
		try {
			clients = await admin.listClients();
			error = '';
		} catch (cause) {
			error = errorMessage(cause, 'Could not load the register.');
		} finally {
			loading = false;
		}
	}

	/*
	 * Reset before opening, never after closing: reopening the drawer must not
	 * be able to re-show a secret minted on a previous run.
	 */
	function openCreate() {
		createdSecret = '';
		createdID = '';
		newID = '';
		newName = '';
		newKind = 'service_account';
		createError = '';
		createOpen = true;
	}

	/** lines splits a textarea into a trimmed list, dropping the empties. */
	function lines(raw: string): string[] {
		return raw
			.split('\n')
			.map((line) => line.trim())
			.filter(Boolean);
	}

	async function create(event: SubmitEvent) {
		event.preventDefault();
		if (creating) return;
		creating = true;
		createError = '';
		const isApplication = newKind === 'application';
		try {
			const result = await admin.createClient({
				client_id: newID.trim(),
				name: newName.trim(),
				kind: newKind,
				redirect_uris: isApplication ? lines(newRedirects) : undefined,
				visible_roles: isApplication ? lines(newRoles) : undefined,
				backchannel_logout_uri: isApplication ? newLogoutURI.trim() || undefined : undefined
			});
			createdSecret = result.secret;
			createdID = result.client.client_id;
			await load();
		} catch (cause) {
			createError = errorMessage(cause, 'Could not create the client.');
		} finally {
			creating = false;
		}
	}

	/** finishCreate closes the drawer once the reader says they have the secret. */
	function finishCreate() {
		createOpen = false;
		toast.success(`${createdID} is registered.`);
	}

	async function rotate() {
		const target = rotateTarget;
		if (!target) return;
		try {
			const result = await admin.rotateSecret(target.client_id);
			rotatedSecret = result.secret;
			rotatedID = result.client_id;
			rotatedOpen = true;
			rotateTarget = null;
			await load();
		} catch (cause) {
			toast.danger(errorMessage(cause, 'Could not rotate that secret.'));
			throw cause;
		}
	}

	function forgetRotated() {
		rotatedSecret = '';
		rotatedID = '';
	}
</script>

<Section
	title="Clients and service accounts"
	description="Applications that send people here to sign in, and machine credentials that call the token endpoint directly."
	bodyClass="flex flex-col gap-4"
>
	{#snippet actions()}
		<Button icon={icons.plus} onclick={openCreate}>New client</Button>
	{/snippet}

	{#if error}
		<Alert tone="danger" title="Could not load the register">{error}</Alert>
	{:else if loading}
		<div class="flex flex-col gap-2">
			<Skeleton class="h-10 w-full" />
			<Skeleton class="h-10 w-full" />
			<Skeleton class="h-10 w-full" />
		</div>
	{:else if clients.length === 0}
		<EmptyState
			icon={icons.plug}
			title="Nothing registered yet"
			description="An app cannot start an OIDC flow until it has a client here, and a service cannot mint a token without one."
		>
			<Button variant="outline" icon={icons.plus} onclick={openCreate}>New client</Button>
		</EmptyState>
	{:else}
		<Table>
			<thead>
				<tr>
					<th scope="col">Client ID</th>
					<th scope="col">Name</th>
					<th scope="col">Kind</th>
					<th scope="col">Scopes</th>
					<th scope="col">Status</th>
					<th scope="col">Created</th>
					<th scope="col" class="text-right" aria-label="Actions"></th>
				</tr>
			</thead>
			<tbody>
				{#each clients as client (client.client_id)}
					<tr class={client.disabled_at ? 'opacity-55' : ''}>
						<td class="font-fc-mono text-fc-xs">{client.client_id}</td>
						<td>{client.name}</td>
						<td>
							<Badge tone={client.kind === 'application' ? 'info' : 'neutral'}>
								{client.kind === 'application' ? 'application' : 'service account'}
							</Badge>
						</td>
						<td class="font-fc-mono text-fc-xs text-fc-fg-muted">
							{client.scopes?.join(' ') || '—'}
						</td>
						<td>
							<Badge tone={client.disabled_at ? 'neutral' : 'success'}>
								{client.disabled_at ? 'disabled' : 'active'}
							</Badge>
						</td>
						<td class="whitespace-nowrap text-fc-fg-muted">{formatDate(client.created_at)}</td>
						<td class="text-right">
							<Button
								variant="ghost-danger"
								size="sm"
								icon={icons.refresh}
								aria-label="Rotate the secret for {client.client_id}"
								onclick={() => {
									rotateTarget = client;
									rotateOpen = true;
								}}
							>
								Rotate secret
							</Button>
						</td>
					</tr>
				{/each}
			</tbody>
		</Table>
	{/if}
</Section>

<ClientCreateDrawer
	bind:open={createOpen}
	bind:id={newID}
	bind:name={newName}
	bind:kind={newKind}
	bind:redirects={newRedirects}
	bind:roles={newRoles}
	bind:logoutURI={newLogoutURI}
	{creating}
	error={createError}
	{createdSecret}
	{createdID}
	onSubmit={create}
	onCancel={() => (createOpen = false)}
	onDone={finishCreate}
/>

<ConfirmModal
	bind:open={rotateOpen}
	tone="danger"
	title="Rotate the secret for {rotateTarget?.client_id ?? ''}?"
	description="The current secret stops working the moment the new one is minted, so anything still using it starts failing authentication. Have the new secret ready to deploy before you do this."
	confirmLabel="Rotate the secret"
	cancelLabel="Leave it alone"
	onConfirm={rotate}
	onCancel={() => (rotateTarget = null)}
/>

<Modal
	bind:open={rotatedOpen}
	title="New secret for {rotatedID}"
	dismissible={false}
	size="sm"
	onClose={forgetRotated}
>
	<ClientSecretPanel
		secret={rotatedSecret}
		actionLabel="I have copied it"
		onAction={() => (rotatedOpen = false)}
	>
		The old secret is already dead and this one is shown once. Deploy it before you close this.
	</ClientSecretPanel>
</Modal>
