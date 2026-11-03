<script lang="ts">
	import { page } from '$app/state';
	import {
		Alert,
		Avatar,
		Badge,
		Button,
		Card,
		Checkbox,
		EmptyState,
		Section,
		Skeleton,
		Switch,
		icons,
		toast
	} from '@facile/muse';
	import AppLogo from '$lib/components/AppLogo.svelte';
	import { admin, errorMessage, formatDate, type AppAccess, type UserDetail } from '$lib/admin';
	import { appLogo, displayName, relativeTime, toggleRole } from '$lib/access';

	let detail = $state<UserDetail | null>(null);
	let error = $state('');
	let busy = $state<string | null>(null);

	const userID = $derived(page.params.id ?? '');

	/** load reads the person, their access to every app and their spaces. */
	async function load() {
		try {
			detail = await admin.getUser(userID);
			error = '';
		} catch (cause) {
			error = errorMessage(cause, 'Could not load this person.');
		}
	}

	$effect(() => {
		if (userID) void load();
	});

	/** setAccess grants or revokes the person's access to one app. */
	async function setAccess(app: AppAccess, on: boolean) {
		if (!detail) return;
		busy = app.client_id;
		try {
			if (on) await admin.grant(app.client_id, detail.user.id, app.roles);
			else await admin.revoke(app.client_id, detail.user.id);
			toast.success(on ? `Access to ${app.name} granted.` : `Access to ${app.name} removed.`);
			await load();
		} catch (cause) {
			toast.danger(errorMessage(cause, 'Could not change that access.'));
		} finally {
			busy = null;
		}
	}

	/** setRole turns one role on or off for the person in one app. */
	async function setRole(app: AppAccess, role: string) {
		if (!detail) return;
		busy = app.client_id;
		try {
			await admin.grant(app.client_id, detail.user.id, toggleRole(app.roles, role));
			await load();
		} catch (cause) {
			toast.danger(errorMessage(cause, 'Could not change that role.'));
		} finally {
			busy = null;
		}
	}
</script>

<svelte:head>
	<title>{detail ? displayName(detail.user) : 'Person'} — Registre</title>
</svelte:head>

<div class="flex flex-col gap-6">
	<div>
		<Button variant="ghost" size="sm" href="/admin" icon={icons.arrow}>All users</Button>
	</div>

	{#if error}
		<Alert tone="danger" title="Could not load this person">{error}</Alert>
	{:else if !detail}
		<Skeleton class="h-24 w-full" />
		<Skeleton class="h-64 w-full" />
	{:else}
		{@const user = detail.user}
		<Card class="flex flex-col gap-4 p-5 sm:flex-row sm:items-center">
			<Avatar name={displayName(user)} src={user.picture_url || undefined} size="lg" />
			<div class="flex min-w-0 flex-1 flex-col gap-1">
				<h1 class="truncate text-fc-xl font-semibold text-fc-fg">{displayName(user)}</h1>
				<p class="truncate font-fc-mono text-fc-xs text-fc-fg-muted">{user.email}</p>
				<div class="mt-1 flex flex-wrap gap-1">
					{#if user.admin}<Badge tone="admin">operator</Badge>{/if}
					<Badge tone={user.disabled_at ? 'neutral' : 'success'}>{user.disabled_at ? 'disabled' : 'active'}</Badge>
					{#if user.totp}<Badge tone="info">2FA</Badge>{/if}
					{#if !user.email_verified}<Badge tone="warning">email unverified</Badge>{/if}
				</div>
			</div>
			<dl class="grid grid-cols-2 gap-x-6 gap-y-1 text-fc-xs sm:text-right">
				<dt class="text-fc-fg-muted">Last sign-in</dt>
				<dd class="text-fc-fg">{relativeTime(user.last_sign_in_at)}</dd>
				<dt class="text-fc-fg-muted">Member since</dt>
				<dd class="text-fc-fg">{formatDate(user.created_at)}</dd>
			</dl>
		</Card>

		<Section
			title="App access"
			description="Apps open to everyone admit this person anyway. Restrict an app from its page to admit only the people switched on here. Roles are what the app receives in its roles claim."
			bodyClass="flex flex-col gap-2"
		>
			{#each detail.apps as app (app.client_id)}
				<Card class="flex flex-col gap-3 p-4 {app.disabled ? 'opacity-55' : ''}">
					<div class="flex items-center gap-3">
						<AppLogo app={{ logo: appLogo(app.name), icon: icons.widget }} />
						<a href="/admin/apps/{encodeURIComponent(app.client_id)}" class="flex min-w-0 flex-1 flex-col">
							<span class="truncate font-medium text-fc-fg">{app.name}</span>
							<span class="text-fc-xs text-fc-fg-muted">
								{app.access_policy === 'assigned' ? 'Only people with access' : 'Open to everyone'}
							</span>
						</a>
						{#if app.access_policy === 'assigned'}
							<Switch
								label="Access to {app.name}"
								checked={app.granted}
								disabled={busy === app.client_id}
								onchange={(event) => setAccess(app, (event.currentTarget as HTMLInputElement).checked)}
							/>
						{:else}
							<Badge tone="success">allowed</Badge>
						{/if}
					</div>
					{#if app.visible_roles.length > 0}
						<div class="flex flex-wrap items-center gap-3 border-t border-fc-border pt-3">
							<span class="text-fc-xs text-fc-fg-muted">Roles</span>
							{#each app.visible_roles as role (role)}
								<Checkbox
									label={role}
									checked={app.roles.includes(role)}
									disabled={busy === app.client_id || !app.allowed}
									onchange={() => setRole(app, role)}
								/>
							{/each}
						</div>
					{/if}
				</Card>
			{:else}
				<EmptyState icon={icons.widget} title="No apps registered" description="Register an app to manage access to it." />
			{/each}
		</Section>

		<Section title="Spaces" description="Shared spaces this person belongs to. Members are managed on each space's page.">
			{#if detail.spaces.length === 0}
				<p class="text-fc-sm text-fc-fg-muted">Not in any shared space.</p>
			{:else}
				<div class="flex flex-wrap gap-2">
					{#each detail.spaces as space (space.id)}
						<a href="/spaces/{space.id}" class="flex items-center gap-2 rounded-fc-pill border border-fc-border px-3 py-1.5 text-fc-sm text-fc-fg hover:bg-fc-surface">
							{space.name}
							<Badge tone={space.role === 'owner' ? 'owner' : space.role === 'admin' ? 'admin' : 'neutral'}>{space.role}</Badge>
						</a>
					{/each}
				</div>
			{/if}
		</Section>
	{/if}
</div>
