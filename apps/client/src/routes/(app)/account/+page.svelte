<script lang="ts">
	import { getContext } from 'svelte';
	import {
		Badge,
		Button,
		SettingsRow,
		SettingsSection,
		icons,
		toast
	} from '@facile/muse';
	import { account, accountError, ACCOUNT_CONTEXT, type AccountContext } from '$lib/account';
	import PictureSection from '$lib/components/PictureSection.svelte';

	const ctx = getContext<AccountContext>(ACCOUNT_CONTEXT);

	let leaving = $state(false);

	async function signOut() {
		leaving = true;
		try {
			await account.logout();
			window.location.assign('/');
		} catch (cause) {
			toast.danger(accountError(cause, 'Could not sign you out.'));
			leaving = false;
		}
	}
</script>

{#if ctx.me}
	<PictureSection me={ctx.me} onchange={ctx.refresh} />
{/if}

<SettingsSection
	title="Details"
	description="Your name and address come from the account Registre holds. Ask an operator to change them."
>
	<SettingsRow label="Email">
		<span class="font-fc-mono text-fc-sm text-fc-fg">{ctx.me?.email ?? '—'}</span>
	</SettingsRow>
	<SettingsRow label="Name">
		<span class="text-fc-sm text-fc-fg">{ctx.me?.name || '—'}</span>
	</SettingsRow>
	<SettingsRow label="Two-step sign-in" description="Managed under Security.">
		{#if ctx.me?.totp_enrolled}
			<Badge tone="success">on</Badge>
		{:else}
			<Badge tone="neutral">off</Badge>
		{/if}
	</SettingsRow>
</SettingsSection>

<SettingsSection
	title="This session"
	description="Signing out ends this browser's session with Registre. The apps you are already signed in to keep their own sessions until they expire."
>
	<SettingsRow label="Sign out" description="You will need to sign in again from an app.">
		<Button variant="outline" icon={icons.logout} disabled={leaving} onclick={signOut}>
			{leaving ? 'Signing out…' : 'Sign out'}
		</Button>
	</SettingsRow>
</SettingsSection>
