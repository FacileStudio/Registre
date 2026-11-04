<script lang="ts">
	import { locale, t } from '$lib/i18n/i18n.svelte';

	const documents = [
		['cgu', 'legal.cgu'],
		['privacy', 'legal.privacy'],
		['cgv', 'legal.cgv'],
		['legal-notice', 'legal.notice']
	] as const;
</script>

<nav aria-label={t('legal.nav')} class="flex flex-wrap gap-x-4 gap-y-1 text-fc-xs text-fc-fg-muted">
	{#each documents as [slug, label] (slug)}
		<a
			href="/legal/{slug}?lang={locale()}"
			class="rounded-fc-xs underline-offset-2 hover:text-fc-fg hover:underline focus-visible:outline-2 focus-visible:outline-fc-ring"
		>
			{t(label)}
		</a>
	{/each}
</nav>
