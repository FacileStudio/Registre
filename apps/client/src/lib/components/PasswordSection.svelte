<!--
	The change-password block on the security page. It owns the three fields and
	the length and match checks that gate the button; the page owns the call that
	actually changes the password.
-->
<script lang="ts">
	import { t } from '$lib/i18n/i18n.svelte';
	import { Alert, Button, Field, Input, SettingsSection, icons } from '@facile/muse';

	let {
		minLength,
		saving,
		error,
		current = $bindable(),
		next = $bindable(),
		confirm = $bindable(),
		onSubmit
	}: {
		minLength: number;
		saving: boolean;
		error: string;
		current: string;
		next: string;
		confirm: string;
		onSubmit: (event: SubmitEvent) => void;
	} = $props();

	const tooShort = $derived(next.length > 0 && next.length < minLength);
	const mismatch = $derived(confirm.length > 0 && confirm !== next);
</script>

<SettingsSection
	title={t('common.password')}
	description={t('password.sectionHint')}
>
	<form class="flex flex-col gap-4" onsubmit={onSubmit}>
		<div aria-live="polite">
			{#if error}
				<Alert tone="danger" title={error} />
			{/if}
		</div>
		<Field label={t('password.current')}>
			<Input
				type="password"
				autocomplete="current-password"
				required
				disabled={saving}
				bind:value={current}
			/>
		</Field>
		<Field
			label={t('password.new')}
			helper={t('password.rule', { min: minLength })}
			error={tooShort ? t('password.tooShort', { min: minLength }) : undefined}
		>
			<Input
				type="password"
				autocomplete="new-password"
				minlength={minLength}
				required
				disabled={saving}
				bind:value={next}
			/>
		</Field>
		<Field
			label={t('password.confirm')}
			error={mismatch ? t('password.mismatch') : undefined}
		>
			<Input
				type="password"
				autocomplete="new-password"
				required
				disabled={saving}
				bind:value={confirm}
			/>
		</Field>
		<div class="flex justify-end">
			<Button
				type="submit"
				icon={icons.shield}
				disabled={saving || tooShort || mismatch || current.length === 0}
			>
				{saving ? t('common.saving') : t('password.change')}
			</Button>
		</div>
	</form>
</SettingsSection>
