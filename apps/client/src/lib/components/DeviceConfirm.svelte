<script lang="ts">
	import { describeScopes, expiresAtLabel, type DeviceRequestView } from '$lib/device';

	const choiceClass =
		'inline-flex h-11 flex-1 items-center justify-center rounded-md px-4 text-sm ' +
		'font-medium transition-colors disabled:pointer-events-none disabled:opacity-50';
	const approveClass = `${choiceClass} bg-primary text-primary-foreground hover:bg-primary/90`;
	const denyClass = `${choiceClass} bg-destructive/10 text-destructive hover:bg-destructive/20`;

	interface Props {
		request: DeviceRequestView;
		code: string;
		busy: boolean;
		error: string;
		approve: () => void;
		deny: () => void;
	}

	let { request, code, busy, error, approve, deny }: Props = $props();

	const scopes = $derived(describeScopes(request.scopes ?? []));
	const deadline = $derived(expiresAtLabel(request.expires_at));
</script>

<div class="mb-8">
	<h1 class="font-heading text-2xl font-bold tracking-tight text-foreground">Is this you?</h1>
	<p class="mt-1.5 text-sm text-muted-foreground">
		Approve this only if you started it yourself, just now.
	</p>
</div>

{#if error}
	<p role="alert" class="mb-4 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
		{error}
	</p>
{/if}

<div class="space-y-3 rounded-md border border-border bg-muted p-4">
	<div class="space-y-1">
		<span class="block text-xs text-muted-foreground">Signing in to</span>
		<span class="block truncate text-base font-semibold text-foreground">
			{request.client_name}
		</span>
	</div>

	<div class="flex items-baseline justify-between gap-3 border-t border-border pt-3">
		<span class="text-xs text-muted-foreground">Code</span>
		<span class="font-mono text-sm tracking-widest text-foreground">{code}</span>
	</div>

	{#if deadline}
		<div class="flex items-baseline justify-between gap-3">
			<span class="text-xs text-muted-foreground">Runs out at</span>
			<span class="text-sm text-foreground">{deadline}</span>
		</div>
	{/if}
</div>

<div class="mt-6 space-y-2">
	<p class="text-sm font-medium text-foreground">It is asking to</p>
	{#if scopes.length === 0}
		<p class="text-sm text-muted-foreground">Sign you in, and nothing more.</p>
	{:else}
		<ul class="space-y-2">
			{#each scopes as scope (scope.name)}
				<li class="flex items-start gap-3 text-sm text-foreground">
					<span class="mt-2 size-1.5 shrink-0 rounded-full bg-muted-foreground"></span>
					<span class="min-w-0">{scope.label}</span>
				</li>
			{/each}
		</ul>
	{/if}
</div>

<div class="mt-8 flex gap-3">
	<button type="button" disabled={busy} aria-busy={busy} onclick={deny} class={denyClass}>
		Deny
	</button>
	<button type="button" disabled={busy} aria-busy={busy} onclick={approve} class={approveClass}>
		Approve
	</button>
</div>

<p class="mt-6 text-sm text-muted-foreground">
	If you did not start this, deny it. Either way your password is never shared with the app that
	asked.
</p>
