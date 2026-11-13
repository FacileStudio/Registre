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
		toast,
		Tooltip
	} from '@facile/muse';
	import ClientCreateDrawer from '$lib/components/ClientCreateDrawer.svelte';
	import ClientEditDrawer from '$lib/components/ClientEditDrawer.svelte';
	import ClientActionModals from '$lib/components/ClientActionModals.svelte';
	import { admin, errorMessage, formatDate, type ClientView } from '$lib/admin';
	import AppLogo from '$lib/components/AppLogo.svelte';
	import { appIcon, appLogo } from '$lib/access';

	let clients = $state<ClientView[]>([]);
	let loading = $state(true);
	let error = $state('');
	let search = $state('');

	let createOpen = $state(false);
	let creating = $state(false);
	let createError = $state('');
	let newID = $state('');
	let newName = $state('');
	let newKind = $state('application');
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

	const apps = $derived(clients.filter((c) => c.kind === 'application'));

	const filteredApps = $derived.by(() => {
		const q = search.toLowerCase().trim();
		if (!q) return apps;
		return apps.filter(
			(a) =>
				a.name.toLowerCase().includes(q) ||
				a.client_id.toLowerCase().includes(q) ||
				(a.redirect_uris && a.redirect_uris.some((u) => u.toLowerCase().includes(q)))
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
			error = errorMessage(cause, 'Could not load applications.');
		} finally {
			loading = false;
		}
	}

	function openCreate() {
		createdSecret = '';
		createdID = '';
		newID = '';
		newName = '';
		newKind = 'application';
		newRedirects = '';
		newRoles = '';
		newLogoutURI = '';
		createError = '';
		createOpen = true;
	}

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
		try {
			const result = await admin.createClient({
				client_id: newID.trim(),
				name: newName.trim(),
				kind: 'application',
				redirect_uris: lines(newRedirects),
				visible_roles: lines(newRoles),
				backchannel_logout_uri: newLogoutURI.trim() || undefined
			});
			createdSecret = result.secret;
			createdID = result.client.client_id;
			await load();
		} catch (cause) {
			createError = errorMessage(cause, 'Could not create the application.');
		} finally {
			creating = false;
		}
	}

	function finishCreate() {
		createOpen = false;
		toast.success(`${createdID} is registered.`);
	}

	function openEdit(app: ClientView) {
		editTarget = app;
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
			toast.danger(errorMessage(cause, 'Could not rotate secret.'));
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
			toast.success(`${target.name} is now disabled.`);
			disableTarget = null;
			await load();
		} catch (cause) {
			toast.danger(errorMessage(cause, 'Could not disable application.'));
			throw cause;
		}
	}

	async function enable() {
		const target = enableTarget;
		if (!target) return;
		try {
			await admin.enableClient(target.client_id);
			toast.success(`${target.name} is now re-enabled.`);
			enableTarget = null;
			await load();
		} catch (cause) {
			toast.danger(errorMessage(cause, 'Could not enable application.'));
			throw cause;
		}
	}

	async function removeApp() {
		const target = deleteTarget;
		if (!target) return;
		try {
			await admin.deleteClient(target.client_id);
			toast.success(`${target.name} was removed.`);
			deleteTarget = null;
			await load();
		} catch (cause) {
			toast.danger(errorMessage(cause, 'Could not delete application.'));
			throw cause;
		}
	}

	function getAppOrigin(uris?: string[]): string | null {
		if (!uris || uris.length === 0) return null;
		try {
			const parsed = new URL(uris[0]);
			return `${parsed.protocol}//${parsed.host}`;
		} catch {
			return null;
		}
	}
</script>

<Section
	title="Applications"
	description="Suite applications that send users here to authenticate through OpenID Connect."
	bodyClass="flex flex-col gap-4"
>
	{#snippet actions()}
		<Button icon={icons.plus} onclick={openCreate}>New app</Button>
	{/snippet}

	{#if error}
		<Alert tone="danger" title="Could not load applications">{error}</Alert>
	{:else if loading}
		<div class="flex flex-col gap-2">
			<Skeleton class="h-10 w-full" />
			<Skeleton class="h-10 w-full" />
			<Skeleton class="h-10 w-full" />
		</div>
	{:else if apps.length === 0}
		<EmptyState
			icon={icons.widget}
			title="No applications registered"
			description="Register suite apps here to enable single sign-on across the platform."
		>
			<Button variant="outline" icon={icons.plus} onclick={openCreate}>New app</Button>
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
					placeholder="Search apps..."
					class="w-full rounded-fc-md bg-fc-surface py-1.5 pl-9 pr-3 text-fc-xs text-fc-fg placeholder-fc-fg-muted outline-none transition-colors focus-visible:outline-2 focus-visible:outline-fc-ring"
				/>
			</div>

			<Table>
				<thead>
					<tr>
						<th scope="col">Application</th>
						<th scope="col">Who can sign in</th>
						<th scope="col">Visible Roles</th>
						<th scope="col">Logout Hook</th>
						<th scope="col">Status</th>
						<th scope="col" class="text-right" aria-label="Actions"></th>
					</tr>
				</thead>
				<tbody>
					{#each filteredApps as app (app.client_id)}
						{@const origin = getAppOrigin(app.redirect_uris)}
						<tr class={app.disabled_at ? 'opacity-55' : ''}>
							<td>
								<div class="flex items-center gap-3">
									<AppLogo app={{ icon: appIcon(app.name) ?? icons.widget, logo: appLogo(app.name) }} />
									<a href="/admin/apps/{encodeURIComponent(app.client_id)}" class="flex min-w-0 flex-col">
										<span class="font-medium text-fc-fg hover:underline">{app.name}</span>
										<Tooltip text={(app.redirect_uris || []).join(', ')} disabled={!app.redirect_uris?.length} class="contents">
											<span class="truncate font-fc-mono text-fc-xs text-fc-fg-muted">
												{app.client_id}
											</span>
										</Tooltip>
									</a>
									{#if app.managed}
										<Badge tone="neutral">managed</Badge>
									{/if}
									{#if origin}
										<Tooltip text="Open app" describe={false} class="contents">
											<a
												href={origin}
												target="_blank"
												rel="noopener noreferrer"
												aria-label="Open app"
												class="inline-flex size-11 shrink-0 items-center justify-center rounded-fc-pill text-fc-fg-muted transition-colors hover:bg-fc-surface hover:text-fc-fg focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-fc-ring"
											>
												<Icon icon={icons.globe} size={14} />
											</a>
										</Tooltip>
									{/if}
								</div>
							</td>
							<td>
								{#if app.access_policy === 'assigned'}
									<Badge tone="warning">people with access</Badge>
								{:else}
									<Badge tone="success">everyone</Badge>
								{/if}
							</td>
							<td>
								{#if app.visible_roles && app.visible_roles.length > 0}
									<div class="flex flex-wrap gap-1">
										{#each app.visible_roles as role}
											<Badge tone="admin">{role}</Badge>
										{/each}
									</div>
								{:else}
									<span class="text-fc-fg-muted text-fc-xs">none</span>
								{/if}
							</td>
							<td class="text-fc-xs">
								{#if app.backchannel_logout_uri}
									<Badge tone="success">enabled</Badge>
								{:else}
									<Badge tone="neutral">none</Badge>
								{/if}
							</td>
							<td>
								<Badge tone={app.disabled_at ? 'neutral' : 'success'}>
									{app.disabled_at ? 'disabled' : 'active'}
								</Badge>
							</td>
							<td class="text-right">
								<div class="flex items-center justify-end gap-1">
									<Button
										variant="ghost"
										size="sm"
										icon={icons.usersGroup}
										href="/admin/apps/{encodeURIComponent(app.client_id)}"
										aria-label="Manage access to {app.name}"
									>
										Access
									</Button>
									<Button
										variant="ghost"
										size="sm"
										icon={icons.edit}
										aria-label="Edit {app.name}"
										onclick={() => openEdit(app)}
									>
										Edit
									</Button>
									<Button
										variant="ghost"
										size="sm"
										icon={icons.refresh}
										aria-label="Rotate secret for {app.name}"
										onclick={() => {
											rotateTarget = app;
											rotateOpen = true;
										}}
									>
										Secret
									</Button>
									{#if app.disabled_at}
										<Button
											variant="ghost"
											size="sm"
											icon={icons.check}
											aria-label="Enable {app.name}"
											onclick={() => {
												enableTarget = app;
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
											aria-label="Disable {app.name}"
											onclick={() => {
												disableTarget = app;
												disableOpen = true;
											}}
										>
											Disable
										</Button>
									{/if}
									{#if !app.managed}
										<Button
											variant="ghost-danger"
											size="sm"
											icon={icons.remove}
											aria-label="Delete {app.name}"
											onclick={() => {
												deleteTarget = app;
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
	onDelete={removeApp}
	kindLabel="app"
/>
