<script lang="ts">
	import { Avatar, Button, SettingsRow, SettingsSection, icons, toast } from '@facile/muse';
	import { account, accountError, type AccountView } from '$lib/account';
	import { squarePicture } from '$lib/picture';

	interface Props {
		me: AccountView;
		onchange: () => Promise<void>;
	}

	let { me, onchange }: Props = $props();

	let busy = $state(false);
	let picker: HTMLInputElement | null = $state(null);

	/** run wraps one picture change in the busy state and the reload after it. */
	async function run(change: () => Promise<unknown>, done: string, failed: string) {
		busy = true;
		try {
			await change();
			await onchange();
			toast.success(done);
		} catch (cause) {
			toast.danger(accountError(cause, failed));
		} finally {
			busy = false;
		}
	}

	/** upload crops the chosen file and makes it the account's picture. */
	function upload(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		input.value = '';
		if (!file) return;
		run(
			async () => account.setPicture(await squarePicture(file)),
			'Picture updated. Apps show it the next time you sign in to them.',
			'Could not use that picture.'
		);
	}

	/** remove clears the account's picture. */
	function remove() {
		run(() => account.removePicture(), 'Picture removed.', 'Could not remove the picture.');
	}
</script>

<SettingsSection
	title="Picture"
	description="One picture for every app in the suite. Each app picks it up the next time you sign in to it."
>
	<SettingsRow label="Profile picture" description="PNG, JPEG or GIF. It is cropped to a square.">
		<div class="flex items-center gap-3">
			<Avatar src={me.picture || undefined} name={me.name || me.email} size="lg" />
			<input
				bind:this={picker}
				type="file"
				accept="image/*"
				class="sr-only"
				tabindex="-1"
				aria-hidden="true"
				onchange={upload}
			/>
			<Button variant="outline" icon={icons.upload} disabled={busy} onclick={() => picker?.click()}>
				{me.picture ? 'Change' : 'Upload'}
			</Button>
			{#if me.picture}
				<Button variant="ghost" icon={icons.remove} disabled={busy} onclick={remove}>Remove</Button>
			{/if}
		</div>
	</SettingsRow>
</SettingsSection>
