<!--
	Invitations addressed to me: the space, the role offered and who sent
	it. Accepting is the only way into a space.
-->
<script lang="ts">
	import { Badge, Button, Section, toast } from '@facile/muse';
	import { accountError } from '$lib/account';
	import { t } from '$lib/i18n/i18n.svelte';
	import type { MessageKey } from '$lib/i18n/en';
	import { spaces, type MyInvitation } from '$lib/spaces';

	let { onchange }: { onchange: () => Promise<void> } = $props();

	let invitations = $state<MyInvitation[]>([]);
	let busy = $state('');

	$effect(() => {
		load();
	});

	async function load() {
		invitations = (await spaces.invitations().catch(() => ({ invitations: [] }))).invitations;
	}

	async function answer(invitation: MyInvitation, accept: boolean) {
		busy = invitation.id;
		try {
			if (accept) {
				await spaces.accept(invitation.id);
				toast.success(t('spaces.joined', { space: invitation.space_name }));
			} else {
				await spaces.decline(invitation.id);
			}
			await load();
			await onchange();
		} catch (cause) {
			toast.danger(accountError(cause, t('common.failed')));
		} finally {
			busy = '';
		}
	}
</script>

{#if invitations.length > 0}
	<Section title={t('spaces.forYou')}>
		<div class="flex flex-col gap-3">
			{#each invitations as invitation (invitation.id)}
				<div class="flex flex-wrap items-center justify-between gap-3 rounded-fc-md border border-fc-border p-3">
					<div class="min-w-0">
						<div class="truncate text-fc-sm font-semibold text-fc-fg">{invitation.space_name}</div>
						<div class="text-fc-xs text-fc-fg-muted">{t('spaces.invitedBy', { name: invitation.invited_by })}</div>
					</div>
					<div class="flex items-center gap-2">
						<Badge tone="neutral">{t(`role.${invitation.role}` as MessageKey)}</Badge>
						<Button size="sm" disabled={busy !== ''} onclick={() => answer(invitation, true)}>{t('spaces.accept')}</Button>
						<Button size="sm" variant="ghost" disabled={busy !== ''} onclick={() => answer(invitation, false)}>
							{t('spaces.decline')}
						</Button>
					</div>
				</div>
			{/each}
		</div>
	</Section>
{/if}
