<script lang="ts">
	import { page } from '$app/state';
	import { Alert, Skeleton } from '@facile/muse';
	import LanguagePicker from '$lib/components/LanguagePicker.svelte';
	import LegalLinks from '$lib/components/LegalLinks.svelte';
	import { locale, t } from '$lib/i18n/i18n.svelte';

	interface LegalDocument {
		title: string;
		version: string;
		html: string;
		draft: boolean;
	}

	let doc = $state<LegalDocument | null>(null);
	let missing = $state(false);

	$effect(() => {
		load(page.params.doc ?? '', locale());
	});

	/**
	 * load fetches one document in one language. The HTML comes from
	 * Registre's own renderer, which escapes every character of the source
	 * before adding its handful of tags, so it is safe to insert.
	 */
	async function load(slug: string, lang: string) {
		missing = false;
		const response = await fetch(`/api/legal/${encodeURIComponent(slug)}?lang=${lang}`).catch(() => null);
		if (!response?.ok) {
			missing = true;
			doc = null;
			return;
		}
		doc = (await response.json()) as LegalDocument;
	}
</script>

<svelte:head>
	<title>{doc?.title ?? t('legal.nav')} — Facile</title>
</svelte:head>

<main class="mx-auto flex min-h-dvh max-w-3xl flex-col gap-8 bg-fc-bg px-4 py-10 sm:px-6">
	<header class="flex flex-wrap items-center justify-between gap-4">
		<a href="/" class="flex items-center gap-3">
			<img src="/logo.svg" alt="" width={24} height={24} />
			<span class="font-heading text-lg font-semibold text-fc-fg">Facile</span>
		</a>
		<LanguagePicker />
	</header>

	{#if missing}
		<Alert tone="danger" title={t('legal.missing')} />
	{:else if !doc}
		<Skeleton class="h-96 w-full" />
	{:else}
		<article class="legal flex flex-col gap-4 text-fc-md text-fc-fg">
			<h1 class="font-heading text-fc-2xl font-semibold">{doc.title}</h1>
			<p class="text-fc-sm text-fc-fg-muted">{t('legal.version', { version: doc.version })}</p>
			{@html doc.html}
		</article>
	{/if}

	<footer class="border-t border-fc-border pt-6">
		<LegalLinks />
	</footer>
</main>

<style>
	.legal :global(h2) {
		margin-top: 1.5rem;
		font-weight: 600;
		font-size: 1.125rem;
	}
	.legal :global(ul) {
		list-style: disc;
		padding-left: 1.5rem;
	}
	.legal :global(a) {
		text-decoration: underline;
		text-underline-offset: 2px;
	}
</style>
