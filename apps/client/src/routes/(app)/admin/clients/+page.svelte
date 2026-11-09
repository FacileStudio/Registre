<script lang="ts">
	import {
		Alert,
		Badge,
		Button,
		EmptyState,
		Icon,
		Section,
		Skeleton,
		Table,
		icons,
		toast
	} from '@facile/muse';
	import ClientCreateDrawer from '$lib/components/ClientCreateDrawer.svelte';
	import ClientEditDrawer from '$lib/components/ClientEditDrawer.svelte';
	import ClientActionModals from '$lib/components/ClientActionModals.svelte';
	import { admin, errorMessage, formatDate, type ClientView } from '$lib/admin';

	let clients = $state<ClientView[]>([]);
	let loading = $state(true);
	let error = $state('');
	let search = $state('');

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

	let editTarget = $state<ClientView | null>(null);
	let editOpen = $state(false);

	let rotateTarget = $state<ClientView | null>(null);
	let rotateOpen = $state(false);
	let rotatedSecret = $state('');
	let rotatedID = $state('');
	let rotatedOpen = $state(false);

	let disableTarget = $state<ClientView | null>(null);
	let disableOpen = $state(false);

	let enableTarget = $state<ClientView | null>(null);
	let enableOpen = $state(false);

	let deleteTarget = $state<ClientView | null>(null);
	let deleteOpen = $state(false);

	const serviceAccounts = $derived(clients.filter((c) => c.kind === 'service_account'));

	const filteredClients = $derived.by(() => {
		const q = search.toLowerCase().trim();
		if (!q) return serviceAccounts;
		return serviceAccounts.filter(
			(c) =>
				c.name.toLowerCase().includes(q) ||
				c.client_id.toLowerCase().includes(q) ||
				(c.audiences && c.audiences.some((a) => a.toLowerCase().includes(q)))
		);
	});

	$effect(() => {
		load();
	});

	async function load() {
		try {
			clients = await admin.listClients();
			error = '';
		} catch (cause) {
			error = errorMessage(cause, 'Could not load service accounts.');
		} finally {
			loading = false;
		}
	}

	function openCreate() {
		createdSecret = '';
		createdID = '';
		newID = '';
		newName = '';
		newKind = 'service_account';
		newRedirects = '';
		newRoles = '';
		newLogoutURI = '';
		createError = '';
		createOpen = true;
	}

	async function create(event: SubmitEvent) {
		event.preventDefault();
		if (creating) return;
		creating = true;
		createError = '';
		try {
			const result = await admin.createClient({
				client_id: newID.trim(),
				name: newName.trim(),
				kind: 'service_account'
			});
			createdSecret = result.secret;
			createdID = result.client.client_id;
			await load();
		} catch (cause) {
			createError = errorMessage(cause, 'Could not create the service account.');
		} finally {
			creating = false;
		}
	}

	function finishCreate() {
		createOpen = false;
		toast.success(`${createdID} is registered.`);
	}

	function openEdit(client: ClientView) {
		editTarget = client;
		editOpen = true;
	}

	function onSavedEdit(updated: ClientView) {
		toast.success(`${updated.name} updated.`);
		editTarget = null;
		load();
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

	async function disable() {
		const target = disableTarget;
		if (!target) return;
		try {
			await admin.disableClient(target.client_id);
			toast.success(`${target.name} is disabled.`);
			disableTarget = null;
			await load();
		} catch (cause) {
			toast.danger(errorMessage(cause, 'Could not disable service account.'));
			throw cause;
		}
	}

	async function enable() {
		const target = enableTarget;
		if (!target) return;
		try {
			await admin.enableClient(target.client_id);
			toast.success(`${target.name} is re-enabled.`);
			enableTarget = null;
			await load();
		} catch (cause) {
			toast.danger(errorMessage(cause, 'Could not enable service account.'));
			throw cause;
		}
	}

	async function removeClient() {
		const target = deleteTarget;
		if (!target) return;
		try {
			await admin.deleteClient(target.client_id);
			toast.success(`${target.name} was deleted.`);
			deleteTarget = null;
			await load();
		} catch (cause) {
			toast.danger(errorMessage(cause, 'Could not delete service account.'));
			throw cause;
		}
	}
</script>

<Section
	title="Service accounts"
	description="Machine credentials that authenticate via client_credentials and call APIs directly."
	bodyClass="flex flex-col gap-4"
>
	{#snippet actions()}
		<Button icon={icons.plus} onclick={openCreate}>New service account</Button>
	{/snippet}

	{#if error}
		<Alert tone="danger" title="Could not load service accounts">{error}</Alert>
	{:else if loading}
		<div class="flex flex-col gap-2">
			<Skeleton class="h-10 w-full" />
			<Skeleton class="h-10 w-full" />
			<Skeleton class="h-10 w-full" />
		</div>
	{:else if serviceAccounts.length === 0}
		<EmptyState
			icon={icons.server}
			title="No service accounts yet"
			description="Service accounts allow bots, tools, and background services to authenticate with Registre."
		>
			<Button variant="outline" icon={icons.plus} onclick={openCreate}>New service account</Button>
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
					placeholder="Search service accounts..."
					class="w-full rounded-fc-md bg-fc-surface py-1.5 pl-9 pr-3 text-fc-xs text-fc-fg placeholder-fc-fg-muted outline-none transition-colors focus-visible:outline-2 focus-visible:outline-fc-ring"
				/>
			</div>

			<Table>
				<thead>
					<tr>
						<th scope="col">Client ID</th>
						<th scope="col">Name</th>
						<th scope="col">Scopes</th>
						<th scope="col">Audiences</th>
						<th scope="col">Status</th>
						<th scope="col">Created</th>
						<th scope="col" class="text-right" aria-label="Actions"></th>
					</tr>
				</thead>
				<tbody>
					{#each filteredClients as client (client.client_id)}
						<tr class={client.disabled_at ? 'opacity-55' : ''}>
							<td class="font-fc-mono text-fc-xs">
								<div class="flex items-center gap-2">
									<span>{client.client_id}</span>
									{#if client.managed}
										<Badge tone="neutral">managed</Badge>
									{/if}
								</div>
							</td>
							<td>{client.name}</td>
							<td class="font-fc-mono text-fc-xs text-fc-fg-muted">
								{client.scopes?.join(' ') || '—'}
							</td>
							<td class="font-fc-mono text-fc-xs text-fc-fg-muted">
								{client.audiences?.join(' ') || '—'}
							</td>
							<td>
								<Badge tone={client.disabled_at ? 'neutral' : 'success'}>
									{client.disabled_at ? 'disabled' : 'active'}
								</Badge>
							</td>
							<td class="whitespace-nowrap text-fc-fg-muted">{formatDate(client.created_at)}</td>
							<td class="text-right">
								<div class="flex items-center justify-end gap-1">
									<Button
										variant="ghost"
										size="sm"
										icon={icons.edit}
										aria-label="Edit {client.name}"
										onclick={() => openEdit(client)}
									>
										Edit
									</Button>
									<Button
										variant="ghost"
										size="sm"
										icon={icons.refresh}
										aria-label="Rotate secret for {client.client_id}"
										onclick={() => {
											rotateTarget = client;
											rotateOpen = true;
										}}
									>
										Secret
									</Button>
									{#if client.disabled_at}
										<Button
											variant="ghost"
											size="sm"
											icon={icons.check}
											aria-label="Enable {client.client_id}"
											onclick={() => {
												enableTarget = client;
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
											aria-label="Disable {client.client_id}"
											onclick={() => {
												disableTarget = client;
												disableOpen = true;
											}}
										>
											Disable
										</Button>
									{/if}
									{#if !client.managed}
										<Button
											variant="ghost-danger"
											size="sm"
											icon={icons.remove}
											aria-label="Delete {client.client_id}"
											onclick={() => {
												deleteTarget = client;
												deleteOpen = true;
											}}
										>
											Delete
										</Button>
									{/if}
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</Table>
		</div>
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

<ClientEditDrawer
	bind:open={editOpen}
	client={editTarget}
	onSaved={onSavedEdit}
	onCancel={() => (editOpen = false)}
/>

<ClientActionModals
	bind:rotateTarget
	bind:rotateOpen
	bind:rotatedOpen
	bind:rotatedSecret
	bind:rotatedID
	onRotate={rotate}
	onForgetRotated={forgetRotated}
	bind:disableTarget
	bind:disableOpen
	onDisable={disable}
	bind:enableTarget
	bind:enableOpen
	onEnable={enable}
	bind:deleteTarget
	bind:deleteOpen
	onDelete={removeClient}
	kindLabel="service account"
/>
