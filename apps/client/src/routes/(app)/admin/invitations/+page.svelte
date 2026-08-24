<script lang="ts">
	import {
		Alert,
		Badge,
		Button,
		Drawer,
		EmptyState,
		Field,
		Input,
		SecretField,
		Section,
		Skeleton,
		Table,
		icons,
		toast
	} from '@facile/muse';
	import { admin, errorMessage, formatDate, type InviteView } from '$lib/admin';

	let invites = $state<InviteView[]>([]);
	let loading = $state(true);
	let error = $state('');

	let createOpen = $state(false);
	let creating = $state(false);
	let createError = $state('');
	let inviteEmail = $state('');
	let issuedToken = $state('');
	let issuedEmail = $state('');

	const redeemLink = $derived(
		issuedToken
			? `${window.location.origin}/invites/redeem?token=${encodeURIComponent(issuedToken)}`
			: ''
	);

	$effect(() => {
		load();
	});

	async function load() {
		try {
			invites = await admin.listInvites();
			error = '';
		} catch (cause) {
			error = errorMessage(cause, 'Could not load the invitations.');
		} finally {
			loading = false;
		}
	}

	/*
	 * Reset before opening, never after closing: reopening the drawer must not
	 * be able to re-show a token issued on a previous run.
	 */
	function openCreate() {
		issuedToken = '';
		issuedEmail = '';
		inviteEmail = '';
		createError = '';
		createOpen = true;
	}

	async function create(event: SubmitEvent) {
		event.preventDefault();
		if (creating) return;
		creating = true;
		createError = '';
		try {
			const result = await admin.createInvite(inviteEmail.trim());
			issuedToken = result.token;
			issuedEmail = result.email;
			inviteEmail = '';
			await load();
		} catch (cause) {
			createError = errorMessage(cause, 'Could not record the invitation.');
		} finally {
			creating = false;
		}
	}
</script>

<Section
	title="Invitations"
	description="A single-use token the person redeems at /invites/redeem to set their own password."
	bodyClass="flex flex-col gap-4"
>
	{#snippet actions()}
		<Button icon={icons.plus} onclick={openCreate}>New invitation</Button>
	{/snippet}

	{#if error}
		<Alert tone="danger" title="Could not load the invitations">{error}</Alert>
	{:else if loading}
		<div class="flex flex-col gap-2">
			<Skeleton class="h-10 w-full" />
			<Skeleton class="h-10 w-full" />
		</div>
	{:else if invites.length === 0}
		<EmptyState
			icon={icons.mail}
			title="No invitations yet"
			description="Invite someone and they pick their own password, so no operator ever holds it."
		>
			<Button variant="outline" icon={icons.plus} onclick={openCreate}>New invitation</Button>
		</EmptyState>
	{:else}
		<Table>
			<thead>
				<tr>
					<th scope="col">Email</th>
					<th scope="col">Invited by</th>
					<th scope="col">Expires</th>
					<th scope="col">Status</th>
				</tr>
			</thead>
			<tbody>
				{#each invites as invite (invite.email + invite.created_at)}
					<tr class={invite.redeemed ? 'opacity-55' : ''}>
						<td class="font-fc-mono text-fc-xs">{invite.email}</td>
						<td class="font-fc-mono text-fc-xs">{invite.created_by}</td>
						<td class="whitespace-nowrap text-fc-fg-muted">{formatDate(invite.expires_at)}</td>
						<td>
							<Badge tone={invite.redeemed ? 'neutral' : 'info'}>
								{invite.redeemed ? 'redeemed' : 'pending'}
							</Badge>
						</td>
					</tr>
				{/each}
			</tbody>
		</Table>
	{/if}
</Section>

<Drawer bind:open={createOpen} title="New invitation" showClose>
	{#if issuedToken}
		<div class="flex flex-col gap-4">
			<Alert tone="warning" title="Copy the link now">
				Registre stores a hash of this token, so it cannot be shown again. Losing it means issuing
				another invitation.
			</Alert>

			<SecretField
				value={redeemLink}
				visible
				autoHideMs={0}
				label="Redemption link"
				helper="Send this to {issuedEmail}. It works once and then expires."
			/>

			<div class="flex justify-end">
				<Button
					onclick={() => {
						createOpen = false;
						toast.success(`Invitation recorded for ${issuedEmail}.`);
					}}
				>
					Done
				</Button>
			</div>
		</div>
	{:else}
		<form class="flex flex-col gap-4" onsubmit={create}>
			<div aria-live="polite">
				{#if createError}
					<Alert tone="danger" title={createError} />
				{/if}
			</div>
			<Field label="Email" helper="Where the invitation is going. Registre does not send it for you.">
				<Input type="email" required disabled={creating} bind:value={inviteEmail} />
			</Field>
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
					icon={icons.mail}
					class="w-full sm:w-auto"
					disabled={creating || inviteEmail.trim().length === 0}
				>
					{creating ? 'Recording…' : 'Create invitation'}
				</Button>
			</div>
		</form>
	{/if}
</Drawer>
