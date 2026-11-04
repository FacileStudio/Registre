<script lang="ts">
	import { Alert, Badge, Button, SettingsRow, SettingsSection, Stack, icons, toast } from '@facile/muse';
	import { page } from '$app/state';
	import { accountError } from '$lib/account';
	import { billing, formatPrice, isStripeURL, type PlanView } from '$lib/billing';

	let { spaceID }: { spaceID: string } = $props();

	let view = $state<PlanView | null>(null);
	let busy = $state('');

	const returned = $derived(page.url.searchParams.get('billing') === 'success');
	const current = $derived(view?.plan?.plan ?? '');

	$effect(() => {
		if (spaceID) load();
	});

	/** load asks Registre for the plan; any failure is drawn as "billing unavailable", never as a broken page. */
	async function load() {
		try {
			view = await billing.get(spaceID);
		} catch {
			view = { enabled: true, available: false, owner: false };
		}
	}

	/** go follows a checkout or portal URL, but only to Stripe. */
	async function go(key: string, start: () => Promise<{ url: string }>) {
		if (busy) return;
		busy = key;
		try {
			const { url } = await start();
			if (!isStripeURL(url)) throw new Error('Billing answered an unexpected address.');
			window.location.assign(url);
		} catch (cause) {
			toast.danger(accountError(cause, 'Billing is unavailable right now.'));
			busy = '';
		}
	}
</script>

{#if view?.enabled}
	<SettingsSection title="Plan" description="Applies to this space in every Facile app.">
		{#if !view.available}
			<Alert tone="warning" title="Billing unavailable">The plan cannot be shown right now. Try again later.</Alert>
		{:else}
			<Stack>
				{#if returned}
					<Alert tone="success" title="Thanks. Your plan updates in a few seconds." />
				{/if}
				<SettingsRow label="Current plan" description={view.plan?.comped ? 'Offered by Facile.' : view.plan?.status}>
					<Badge tone="accent">{current || 'solo'}</Badge>
				</SettingsRow>
				{#if view.owner}
					{#each view.catalog ?? [] as plan (plan.key)}
						{#if plan.key !== current}
							<SettingsRow label={plan.name}>
								<div class="flex flex-wrap gap-2">
									{#each plan.prices as price (price.lookup_key)}
										<Button
											variant="outline"
											disabled={busy !== ''}
											onclick={() => go(price.lookup_key, () => billing.checkout(spaceID, price.lookup_key))}
										>
											{busy === price.lookup_key ? 'Opening…' : `Upgrade, ${formatPrice(price)}`}
										</Button>
									{/each}
								</div>
							</SettingsRow>
						{/if}
					{/each}
					<SettingsRow label="Billing" description="Invoices, payment method and cancellation, on Stripe.">
						<Button
							variant="outline"
							icon={icons.card}
							disabled={busy !== ''}
							onclick={() => go('portal', () => billing.portal(spaceID))}
						>
							{busy === 'portal' ? 'Opening…' : 'Manage billing'}
						</Button>
					</SettingsRow>
				{:else}
					<p class="text-sm text-fc-fg-muted">Only an owner of this space can change its plan.</p>
				{/if}
			</Stack>
		{/if}
	</SettingsSection>
{/if}
