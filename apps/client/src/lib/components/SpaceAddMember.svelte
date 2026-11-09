<script lang="ts">
	import { Alert, Badge, Button, Field, Input, Select, icons, toast } from '@facile/muse';
	import { accountError } from '$lib/account';
	import { locale, t } from '$lib/i18n/i18n.svelte';
	import { assignable, spaces, type PendingInvitation, type Role, type SpaceDetail } from '$lib/spaces';
	import type { MessageKey } from '$lib/i18n/en';

	let { space, onchange }: { space: SpaceDetail; onchange: () => Promise<void> } = $props();

	let email = $state('');
	let role = $state<Role>('member');
	let inviting = $state(false);
	let error = $state('');

	const grantable = $derived(assignable(space.role));

	/**
	 * invite sends an invitation. The answer is the same whether or not the
	 * address has an account, and nobody joins until they accept.
	 */
	async function invite(event: SubmitEvent) {
		event.preventDefault();
		if (inviting) return;
		inviting = true;
		error = '';
		try {
			await spaces.invite(space.id, email.trim(), role);
			toast.success(t('spaces.invited', { email: email.trim() }));
			email = '';
			role = 'member';
			await onchange();
		} catch (cause) {
			error = accountError(cause, t('spaces.inviteFailed'));
		} finally {
			inviting = false;
		}
	}

	async function revoke(invitation: PendingInvitation) {
		try {
			await spaces.revokeInvitation(space.id, invitation.id);
			toast.success(t('spaces.revoked', { email: invitation.email }));
			await onchange();
		} catch (cause) {
			toast.danger(accountError(cause, t('common.failed')));
		}
	}
</script>

<form class="flex flex-col gap-3" onsubmit={invite}>
	<p class="text-fc-sm text-fc-fg-muted">{t('spaces.inviteHint')}</p>
	<div class="flex flex-col gap-3 sm:flex-row sm:items-end">
		<Field label={t('common.email')} class="flex-1">
			<Input type="email" required disabled={inviting} bind:value={email} placeholder="mate@example.com" />
		</Field>
		<Field label={t('spaces.role')}>
			<Select disabled={inviting} bind:value={role}>
				{#each grantable as option (option)}
					<option value={option}>{t(`role.${option}` as MessageKey)}</option>
				{/each}
			</Select>
		</Field>
		<Button type="submit" icon={icons.mail} class="w-full sm:w-auto" disabled={inviting || !email.trim()}>
			{inviting ? t('common.sending') : t('spaces.invite')}
		</Button>
	</div>
	<div aria-live="polite">
		{#if error}
			<Alert tone="danger" title={error} />
		{/if}
	</div>
</form>

{#if space.invitations && space.invitations.length > 0}
	<div class="mt-4 flex flex-col gap-2">
		<h3 class="text-fc-sm font-semibold text-fc-fg">{t('spaces.pending')}</h3>
		{#each space.invitations as invitation (invitation.id)}
			<div class="flex flex-wrap items-center justify-between gap-2 rounded-fc-md bg-fc-surface px-3 py-2">
				<div class="min-w-0">
					<div class="truncate font-fc-mono text-fc-sm text-fc-fg">{invitation.email}</div>
					<div class="text-fc-xs text-fc-fg-muted">
						{t('spaces.expires', { date: new Date(invitation.expires_at).toLocaleDateString(locale()) })}
					</div>
				</div>
				<div class="flex items-center gap-2">
					<Badge tone="neutral">{t(`role.${invitation.role}` as MessageKey)}</Badge>
					<Button variant="ghost-danger" size="sm" icon={icons.revoke} onclick={() => revoke(invitation)}>
						{t('spaces.revoke')}
					</Button>
				</div>
			</div>
		{/each}
	</div>
{/if}
