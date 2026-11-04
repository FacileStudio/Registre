<script lang="ts">
	import { Select } from '@facile/muse';
	import { LOCALES, LOCALE_NAMES, locale, setLocale, t, type Locale } from '$lib/i18n/i18n.svelte';

	let { onchoose }: { onchoose?: (next: Locale) => void } = $props();

	let value = $state<string>(locale());

	$effect(() => {
		value = locale();
	});

	/** choose switches the page language and tells the caller. */
	function choose(event: Event) {
		const next = (event.currentTarget as HTMLSelectElement).value as Locale;
		setLocale(next);
		onchoose?.(next);
	}
</script>

<Select aria-label={t('common.language')} class="h-9 w-auto min-w-32" bind:value onchange={choose}>
	{#each LOCALES as option (option)}
		<option value={option}>{LOCALE_NAMES[option]}</option>
	{/each}
</Select>
