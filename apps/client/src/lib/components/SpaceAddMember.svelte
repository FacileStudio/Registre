<script lang="ts">
	import { Alert, Button, Field, Input, Select, icons, toast } from '@facile/muse';
	import { accountError } from '$lib/account';
	import { assignable, spaces, type Role, type SpaceDetail } from '$lib/spaces';

	let { space, onchange }: { space: SpaceDetail; onchange: () => Promise<void> } = $props();

	let email = $state('');
	let role = $state<Role>('member');
	let adding = $state(false);
	let error = $state('');

	const grantable = $derived(assignable(space.role));

	/** add puts the person into the space; the API answers 404 when Registre has no account for them. */
	async function add(event: SubmitEvent) {
		event.preventDefault();
		if (adding) return;
		adding = true;
		error = '';
		try {
			const { member } = await spaces.addMember(space.id, email.trim(), role);
			toast.success(`${member.name || member.email} joined ${space.name}.`);
			email = '';
			role = 'member';
			await onchange();
		} catch (cause) {
			error = accountError(cause, 'Could not add the member.');
		} finally {
			adding = false;
		}
	}
</script>

<form class="flex flex-col gap-3" onsubmit={add}>
	<div class="flex flex-col gap-3 sm:flex-row sm:items-end">
		<Field label="Email" class="flex-1">
			<Input type="email" required disabled={adding} bind:value={email} placeholder="mate@facile.studio" />
		</Field>
		<Field label="Role">
			<Select disabled={adding} bind:value={role}>
				{#each grantable as option (option)}
					<option value={option}>{option}</option>
				{/each}
			</Select>
		</Field>
		<Button type="submit" icon={icons.plus} class="w-full sm:w-auto" disabled={adding || !email.trim()}>
			{adding ? 'Adding…' : 'Add'}
		</Button>
	</div>
	<div aria-live="polite">
		{#if error}
			<Alert tone="danger" title={error} />
		{/if}
	</div>
</form>
