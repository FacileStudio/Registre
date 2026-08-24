<!--
	The two-step sign-in status block: whether the factor is on, how many recovery
	codes are left, and the buttons that open the enrolment, regeneration and
	disable flows. The page owns those flows and passes them in as callbacks.
-->
<script lang="ts">
	import { Badge, Button, SettingsRow, SettingsSection, icons } from '@facile/muse';

	let {
		enrolled,
		remaining,
		onEnrol,
		onRegenerate,
		onDisable
	}: {
		enrolled: boolean;
		remaining: number;
		onEnrol: () => void;
		onRegenerate: () => void;
		onDisable: () => void;
	} = $props();
</script>

<SettingsSection
	title="Two-step sign-in"
	description="An authenticator app generates a six-digit code that Registre asks for after your password."
>
	<SettingsRow label="Status" description="Applies to every app you sign in to through Registre.">
		{#if enrolled}
			<Badge tone="success">on</Badge>
		{:else}
			<Badge tone="neutral">off</Badge>
		{/if}
	</SettingsRow>

	{#if enrolled}
		<SettingsRow
			label="Recovery codes"
			description="Single-use codes for when you do not have your authenticator."
		>
			<div class="flex items-center gap-3">
				<span class="text-fc-sm tabular-nums text-fc-fg-muted">{remaining} left</span>
				<Button variant="outline" icon={icons.refresh} onclick={onRegenerate}>
					Issue new codes
				</Button>
			</div>
		</SettingsRow>
		<SettingsRow
			label="Turn off two-step sign-in"
			description="Your password becomes the only thing guarding the account."
		>
			<Button variant="ghost-danger" icon={icons.revoke} onclick={onDisable}>Turn off</Button>
		</SettingsRow>
	{:else}
		<SettingsRow
			label="Turn on two-step sign-in"
			description="You will scan a QR code and confirm one code to prove it works."
		>
			<Button icon={icons.shield} onclick={onEnrol}>Turn on</Button>
		</SettingsRow>
	{/if}
</SettingsSection>
