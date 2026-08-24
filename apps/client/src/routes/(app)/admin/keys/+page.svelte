<script lang="ts">
	import {
		Alert,
		Badge,
		EmptyState,
		Section,
		Skeleton,
		Table,
		icons,
		type BadgeProps
	} from '@facile/muse';
	import { admin, errorMessage, formatDate, type KeyState } from '$lib/admin';

	let keys = $state<KeyState[]>([]);
	let loading = $state(true);
	let error = $state('');

	$effect(() => {
		load();
	});

	async function load() {
		try {
			keys = await admin.listKeys();
			error = '';
		} catch (cause) {
			error = errorMessage(cause, 'Could not read the keyring.');
		} finally {
			loading = false;
		}
	}

	/*
	 * published is the state that has to be visible before active: a key must
	 * appear in JWKS before anything signs with it, and a retired public key
	 * has to stay there for a full token lifetime after its last use. Getting
	 * that order wrong 401s every app at once, so the states are toned rather
	 * than left as plain words.
	 */
	function tone(state: string): BadgeProps['tone'] {
		if (state === 'active') return 'success';
		if (state === 'published') return 'info';
		if (state === 'retired') return 'warning';
		return 'neutral';
	}
</script>

<Section
	title="Signing keys"
	description="Rotation runs on its own loop. A key is published in JWKS before it signs anything, and its public half stays there for a full token lifetime after its last use."
	bodyClass="flex flex-col gap-4"
>
	{#if error}
		<Alert tone="danger" title="Could not read the keyring">{error}</Alert>
	{:else if loading}
		<div class="flex flex-col gap-2">
			<Skeleton class="h-10 w-full" />
			<Skeleton class="h-10 w-full" />
		</div>
	{:else if keys.length === 0}
		<EmptyState
			icon={icons.key}
			title="No signing keys"
			description="Registre mints its first key at boot. An empty keyring means nothing can sign a token, so this instance is not serving."
		/>
	{:else}
		<Table>
			<thead>
				<tr>
					<th scope="col">Key ID</th>
					<th scope="col">Algorithm</th>
					<th scope="col">State</th>
					<th scope="col">Created</th>
					<th scope="col">Activated</th>
					<th scope="col">Retired</th>
					<th scope="col">Expires</th>
				</tr>
			</thead>
			<tbody>
				{#each keys as key (key.id)}
					<tr>
						<td class="font-fc-mono text-fc-xs">{key.id}</td>
						<td class="font-fc-mono text-fc-xs">{key.algorithm}</td>
						<td><Badge tone={tone(key.state)}>{key.state}</Badge></td>
						<td class="whitespace-nowrap text-fc-fg-muted">{formatDate(key.created_at)}</td>
						<td class="whitespace-nowrap text-fc-fg-muted">{formatDate(key.activated_at)}</td>
						<td class="whitespace-nowrap text-fc-fg-muted">{formatDate(key.retired_at)}</td>
						<td class="whitespace-nowrap text-fc-fg-muted">{formatDate(key.expires_at)}</td>
					</tr>
				{/each}
			</tbody>
		</Table>
	{/if}
</Section>
