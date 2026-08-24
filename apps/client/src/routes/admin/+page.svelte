<script lang="ts">
	import { Alert, Badge, Button, Card, Field, Input, Section, Select, Table } from '@facile/muse';
	import {
		authHeader,
		type ClientView,
		type InviteView,
		type KeyState,
		type UserView
	} from '$lib/admin';

	let email = $state('');
	let password = $state('');
	let signedIn = $state(false);
	let busy = $state(false);
	let error = $state('');

	let users = $state<UserView[]>([]);
	let clients = $state<ClientView[]>([]);
	let invites = $state<InviteView[]>([]);
	let keys = $state<KeyState[]>([]);

	let newUserName = $state('');
	let newUserPassword = $state('');

	let newClientID = $state('');
	let newClientName = $state('');
	let newClientKind = $state('service_account');

	let inviteEmail = $state('');

	let shownSecret = $state<{ label: string; value: string } | null>(null);

	async function call(path: string, init: RequestInit = {}): Promise<Response> {
		const headers = new Headers(init.headers);
		headers.set('Authorization', authHeader(email, password));
		if (init.body) headers.set('Content-Type', 'application/json');
		return fetch(path, { ...init, headers });
	}

	async function refreshAll() {
		const [usersRes, clientsRes, invitesRes, keysRes] = await Promise.all([
			call('/api/admin/users'),
			call('/api/admin/clients'),
			call('/api/admin/invites'),
			call('/api/admin/keys')
		]);
		if (!usersRes.ok) throw new Error('not authorised');
		users = await usersRes.json();
		clients = await clientsRes.json();
		invites = await invitesRes.json();
		keys = await keysRes.json();
	}

	async function signIn(event: SubmitEvent) {
		event.preventDefault();
		if (busy) return;
		busy = true;
		error = '';
		try {
			await refreshAll();
			signedIn = true;
		} catch {
			error = 'Wrong operator email or password.';
		} finally {
			busy = false;
		}
	}

	async function createUser(event: SubmitEvent) {
		event.preventDefault();
		const response = await call('/api/admin/users', {
			method: 'POST',
			body: JSON.stringify({ email: newUserName.trim(), password: newUserPassword })
		});
		if (!response.ok) {
			shownSecret = null;
			error = 'Could not create the user. Check the email and password length.';
			return;
		}
		newUserName = '';
		newUserPassword = '';
		await refreshAll();
	}

	async function disableUser(user: UserView) {
		await call(`/api/admin/users/${user.id}/disable`, { method: 'POST' });
		await refreshAll();
	}

	async function createClient(event: SubmitEvent) {
		event.preventDefault();
		const response = await call('/api/admin/clients', {
			method: 'POST',
			body: JSON.stringify({
				client_id: newClientID.trim(),
				name: newClientName.trim(),
				kind: newClientKind
			})
		});
		if (!response.ok) {
			error = 'Could not create the client.';
			return;
		}
		const body = await response.json();
		shownSecret = { label: `Secret for ${body.client.client_id}`, value: body.secret };
		newClientID = '';
		newClientName = '';
		await refreshAll();
	}

	async function rotateSecret(client: ClientView) {
		const response = await call(`/api/admin/clients/${client.client_id}/secret`, { method: 'POST' });
		if (response.ok) {
			const body = await response.json();
			shownSecret = { label: `New secret for ${body.client_id}`, value: body.secret };
		}
	}

	async function inviteUser(event: SubmitEvent) {
		event.preventDefault();
		const response = await call('/api/admin/invites', {
			method: 'POST',
			body: JSON.stringify({ email: inviteEmail.trim() })
		});
		if (!response.ok) {
			error = 'Could not record the invitation.';
			return;
		}
		const body = await response.json();
		shownSecret = { label: `Invitation token for ${body.email}`, value: body.token };
		inviteEmail = '';
		await refreshAll();
	}
</script>

<svelte:head>
	<title>Dashboard — registre</title>
</svelte:head>

<main class="mx-auto min-h-dvh max-w-4xl bg-fc-page px-4 py-8">
	{#if !signedIn}
		<Card class="mx-auto mt-10 w-full max-w-sm p-8">
			<h1 class="mb-1 text-fc-lg font-semibold text-fc-fg">Operator sign-in</h1>
			<p class="mb-6 text-fc-sm text-fc-fg-muted">
				The registre dashboard. Operator accounts only.
			</p>
			<form onsubmit={signIn} class="flex flex-col gap-4">
				{#if error}
					<Alert tone="danger" title={error} />
				{/if}
				<Field label="Email" for="op-email">
					<Input id="op-email" type="email" required bind:value={email} />
				</Field>
				<Field label="Password" for="op-password">
					<Input id="op-password" type="password" required bind:value={password} />
				</Field>
				<Button type="submit" disabled={busy}>
					{busy ? 'Checking…' : 'Sign in'}
				</Button>
			</form>
		</Card>
	{:else}
		<div class="flex flex-col gap-6">
			<header>
				<h1 class="text-fc-lg font-semibold text-fc-fg">registre dashboard</h1>
				<p class="text-fc-sm text-fc-fg-muted">
					Signed in as {email}. Secrets and tokens are shown once — copy them now.
				</p>
			</header>

			{#if error}
				<Alert tone="danger" title={error} />
			{/if}

			{#if shownSecret}
				<Alert tone="info" title={shownSecret.label}>
					<p class="mt-1 break-all font-mono text-fc-sm">{shownSecret.value}</p>
				</Alert>
			{/if}

			<Section title="Users">
				<form onsubmit={createUser} class="mb-4 flex flex-wrap items-end gap-3">
					<Field label="Email" for="new-user-email">
						<Input id="new-user-email" type="email" required bind:value={newUserName} />
					</Field>
					<Field label="Password" for="new-user-password">
						<Input id="new-user-password" type="password" autocomplete="new-password" required bind:value={newUserPassword} />
					</Field>
					<Button type="submit">Create user</Button>
				</form>
				<Table>
					<thead>
						<tr><th>Email</th><th>Admin</th><th>Status</th><th>Created</th><th></th></tr>
					</thead>
					<tbody>
						{#each users as user (user.id)}
							<tr>
								<td>{user.email}</td>
								<td>{user.admin ? 'yes' : 'no'}</td>
								<td>
									<Badge tone={user.disabled_at ? 'neutral' : 'success'}>
										{user.disabled_at ? 'disabled' : 'active'}
									</Badge>
								</td>
								<td>{user.created_at}</td>
								<td>
									{#if !user.disabled_at}
										<Button variant="ghost" onclick={() => disableUser(user)}>Disable</Button>
									{/if}
								</td>
							</tr>
						{/each}
					</tbody>
				</Table>
			</Section>

			<Section title="Invitations">
				<p class="mb-3 text-fc-sm text-fc-fg-muted">
					An invitation is a single-use token the person redeems at /invites/redeem to set their own password.
				</p>
				<form onsubmit={inviteUser} class="mb-4 flex flex-wrap items-end gap-3">
					<Field label="Email" for="invite-email">
						<Input id="invite-email" type="email" required bind:value={inviteEmail} />
					</Field>
					<Button type="submit">Invite</Button>
				</form>
				<Table>
					<thead>
						<tr><th>Email</th><th>Invited by</th><th>Expires</th><th>Status</th></tr>
					</thead>
					<tbody>
						{#each invites as invite (invite.email + invite.created_at)}
							<tr>
								<td>{invite.email}</td>
								<td>{invite.created_by}</td>
								<td>{invite.expires_at}</td>
								<td>
									<Badge tone={invite.redeemed ? 'neutral' : 'info'}>
										{invite.redeemed ? 'redeemed' : 'pending'}
									</Badge>
								</td>
							</tr>
						{/each}
					</tbody>
				</Table>
			</Section>

			<Section title="Clients and service accounts">
				<form onsubmit={createClient} class="mb-4 flex flex-wrap items-end gap-3">
					<Field label="Client ID" for="new-client-id">
						<Input id="new-client-id" required bind:value={newClientID} />
					</Field>
					<Field label="Name" for="new-client-name">
						<Input id="new-client-name" required bind:value={newClientName} />
					</Field>
					<Field label="Kind" for="new-client-kind">
						<Select id="new-client-kind" bind:value={newClientKind}>
							<option value="service_account">service account</option>
							<option value="application">application</option>
						</Select>
					</Field>
					<Button type="submit">Create</Button>
				</form>
				<Table>
					<thead>
						<tr><th>Client ID</th><th>Name</th><th>Kind</th><th>Status</th><th></th></tr>
					</thead>
					<tbody>
						{#each clients as client (client.client_id)}
							<tr>
								<td class="font-mono">{client.client_id}</td>
								<td>{client.name}</td>
								<td>{client.kind}</td>
								<td>
									<Badge tone={client.disabled_at ? 'neutral' : 'success'}>
										{client.disabled_at ? 'disabled' : 'active'}
									</Badge>
								</td>
								<td>
									<Button variant="ghost" onclick={() => rotateSecret(client)}>Rotate secret</Button>
								</td>
							</tr>
						{/each}
					</tbody>
				</Table>
			</Section>

			<Section title="Signing keys">
				<Table>
					<thead>
						<tr><th>ID</th><th>Algorithm</th><th>State</th><th>Created</th><th>Expires</th></tr>
					</thead>
					<tbody>
						{#each keys as key (key.id)}
							<tr>
								<td class="font-mono">{key.id.slice(0, 12)}…</td>
								<td>{key.algorithm}</td>
								<td><Badge tone={key.state === 'active' ? 'success' : key.state === 'published' ? 'info' : 'neutral'}>{key.state}</Badge></td>
								<td>{key.created_at}</td>
								<td>{key.expires_at ?? '—'}</td>
							</tr>
						{/each}
					</tbody>
				</Table>
			</Section>
		</div>
	{/if}
</main>
