<script lang="ts">
	import { page } from '$app/state';
	import {
		Alert,
		Avatar,
		Badge,
		Button,
		Card,
		Checkbox,
		OptionCards,
		Section,
		Select,
		Skeleton,
		icons,
		toast
	} from '@facile/muse';
	import AppLogo from '$lib/components/AppLogo.svelte';
	import {
		admin,
		errorMessage,
		type AccessPolicy,
		type ClientAccess,
		type ClientView,
		type GrantView,
		type UserView
	} from '$lib/admin';
	import { appLogo, displayName, relativeTime, toggleRole } from '$lib/access';

	let client = $state<ClientView | null>(null);
	let access = $state<ClientAccess | null>(null);
	let users = $state<UserView[]>([]);
	let error = $state('');
	let busy = $state(false);
	let adding = $state('');

	const clientID = $derived(page.params.id ?? '');
	const candidates = $derived(
		users.filter((u) => !u.disabled_at && !access?.grants.some((g) => g.user.id === u.id))
	);

	/** load reads the app, who can use it and every account to pick from. */
	async function load() {
		try {
			[client, access, users] = await Promise.all([
				admin.getClient(clientID),
				admin.clientAccess(clientID),
				admin.listUsers()
			]);
			error = '';
		} catch (cause) {
			error = errorMessage(cause, 'Could not load this app.');
		}
	}

	$effect(() => {
		if (clientID) void load();
	});

	/** run performs one change, then reloads, toasting the outcome. */
	async function run(change: () => Promise<unknown>, done: string) {
		busy = true;
		try {
			await change();
			toast.success(done);
			await load();
		} catch (cause) {
			toast.danger(errorMessage(cause, 'That change did not go through.'));
		} finally {
			busy = false;
		}
	}

	function setPolicy(policy: string) {
		if (!access || policy === access.access_policy) return;
		void run(
			() => admin.setPolicy(clientID, policy as AccessPolicy),
			policy === 'assigned' ? 'Only people with access can sign in now.' : 'Everyone can sign in now.'
		);
	}

	function addPerson() {
		const user = users.find((u) => u.id === adding);
		if (!user) return;
		adding = '';
		void run(() => admin.grant(clientID, user.id, []), `${displayName(user)} has access now.`);
	}

	function setRole(grant: GrantView, role: string) {
		void run(() => admin.grant(clientID, grant.user.id, toggleRole(grant.roles, role)), 'Roles updated.');
	}

	function remove(grant: GrantView) {
		void run(() => admin.revoke(clientID, grant.user.id), `${displayName(grant.user)} no longer has access.`);
	}
</script>

<svelte:head>
	<title>{client?.name ?? 'App'} — Registre</title>
</svelte:head>

<div class="flex flex-col gap-6">
	<div>
		<Button variant="ghost" size="sm" href="/admin/apps" icon={icons.arrow}>All apps</Button>
	</div>

	{#if error}
		<Alert tone="danger" title="Could not load this app">{error}</Alert>
	{:else if !client || !access}
		<Skeleton class="h-20 w-full" />
		<Skeleton class="h-64 w-full" />
	{:else}
		<Card class="flex items-center gap-4 p-5">
			<AppLogo app={{ logo: appLogo(client.name), icon: icons.widget }} />
			<div class="flex min-w-0 flex-1 flex-col">
				<h1 class="truncate text-fc-xl font-semibold text-fc-fg">{client.name}</h1>
				<p class="truncate font-fc-mono text-fc-xs text-fc-fg-muted">{client.client_id}</p>
			</div>
			<Badge tone={client.disabled_at ? 'neutral' : 'success'}>{client.disabled_at ? 'disabled' : 'active'}</Badge>
		</Card>

		<Section
			title="Who can sign in"
			description="Everyone: every active Registre account. People with access: only those listed below, and anyone else is signed out within one token lifetime."
		>
			{#if access.self}
				<Alert tone="info" title="Registre stays open to everyone">
					It is the door to this dashboard, so restricting it could lock every operator out.
				</Alert>
			{:else}
				<OptionCards
					label="Who can sign in"
					value={access.access_policy}
					onSelect={(value: string) => setPolicy(value)}
					options={[
						{ value: 'everyone', label: 'Everyone', icon: icons.globe },
						{ value: 'assigned', label: 'People with access', icon: icons.shield }
					]}
				/>
			{/if}
		</Section>

		<Section
			title="People with access"
			description={access.visible_roles.length
				? `Roles this app can read: ${access.visible_roles.join(', ')}. Tick a role to give it.`
				: 'This app declares no roles. Add visible roles in its configuration to hand out permissions.'}
			bodyClass="flex flex-col gap-3"
		>
			<form
				class="flex flex-col gap-2 sm:flex-row"
				onsubmit={(event) => {
					event.preventDefault();
					addPerson();
				}}
			>
				<Select bind:value={adding} aria-label="Person to give access to" class="sm:max-w-sm">
					<option value="">Choose a person…</option>
					{#each candidates as user (user.id)}
						<option value={user.id}>{displayName(user)} · {user.email}</option>
					{/each}
				</Select>
				<Button type="submit" icon={icons.plus} disabled={!adding || busy}>Give access</Button>
			</form>

			{#each access.grants as grant (grant.user.id)}
				<Card class="flex flex-col gap-3 p-4 sm:flex-row sm:items-center">
					<a href="/admin/users/{grant.user.id}" class="flex min-w-0 flex-1 items-center gap-3">
						<Avatar name={displayName(grant.user)} src={grant.user.picture_url || undefined} size="sm" />
						<span class="flex min-w-0 flex-col">
							<span class="truncate font-medium text-fc-fg">{displayName(grant.user)}</span>
							<span class="truncate text-fc-xs text-fc-fg-muted">
								Since {relativeTime(grant.granted_at).toLowerCase()}{grant.granted_by ? ` · by ${grant.granted_by}` : ''}
							</span>
						</span>
					</a>
					{#if access.visible_roles.length}
						<div class="flex flex-wrap gap-3">
							{#each access.visible_roles as role (role)}
								<Checkbox
									label={role}
									checked={grant.roles.includes(role)}
									disabled={busy}
									onchange={() => setRole(grant, role)}
								/>
							{/each}
						</div>
					{/if}
					<Button
						variant="ghost-danger"
						size="sm"
						icon={icons.remove}
						disabled={busy}
						aria-label="Remove {displayName(grant.user)}'s access"
						onclick={() => remove(grant)}
					>
						Remove
					</Button>
				</Card>
			{:else}
				<p class="text-fc-sm text-fc-fg-muted">
					{access.access_policy === 'assigned'
						? 'Nobody can sign in yet: give someone access above.'
						: 'Nobody holds a role here yet. Everyone can still sign in.'}
				</p>
			{/each}
		</Section>
	{/if}
</div>
