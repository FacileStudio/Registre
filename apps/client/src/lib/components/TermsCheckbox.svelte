<script lang="ts">
	import { Checkbox } from '@facile/muse';
	import { locale, t } from '$lib/i18n/i18n.svelte';

	let { checked = $bindable(false), disabled = false }: { checked?: boolean; disabled?: boolean } = $props();
</script>

<div class="flex items-start gap-3">
	<Checkbox id="accept-terms" bind:checked {disabled} required />
	<label for="accept-terms" class="text-fc-sm text-fc-fg">
		{t('consent.accept1')}
		<a href="/legal/cgu?lang={locale()}" target="_blank" rel="noopener" class="underline underline-offset-2">
			{t('legal.cgu')}
		</a>
		{t('consent.and')}
		<a href="/legal/privacy?lang={locale()}" target="_blank" rel="noopener" class="underline underline-offset-2">
			{t('legal.privacy')}
		</a>.
	</label>
</div>
