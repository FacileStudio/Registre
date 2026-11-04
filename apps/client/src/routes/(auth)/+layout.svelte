<script lang="ts">
	import type { Snippet } from 'svelte';
	import LanguagePicker from '$lib/components/LanguagePicker.svelte';
	import LegalLinks from '$lib/components/LegalLinks.svelte';
	import { t } from '$lib/i18n/i18n.svelte';

	let { children }: { children?: Snippet } = $props();
</script>

<div class="flex min-h-dvh">
	<div class="hidden flex-col border-r border-border bg-black px-12 py-10 lg:flex lg:w-1/2">
		<a href="/" class="mb-auto flex items-center gap-3">
			<img src="/logo.svg" alt="" width={28} height={28} class="block shrink-0" />
			<span class="font-heading text-xl font-semibold tracking-tight text-white">Facile</span>
		</a>

		<div class="mb-auto">
			<h2 class="font-heading text-4xl leading-tight font-semibold tracking-tight text-white">
				{t('auth.tagline1')}<br />{t('auth.tagline2')}
			</h2>
			<p class="mt-4 max-w-xs text-sm leading-relaxed text-white/50">{t('auth.pitch')}</p>
		</div>

		<p class="text-xs text-white/30">© {new Date().getFullYear()} Facile Studio</p>
	</div>

	<div class="flex w-full flex-col items-center justify-center bg-background px-6 py-12 lg:w-1/2">
		<div class="w-full max-w-sm">
			<a href="/" class="mb-8 flex items-center gap-3 lg:hidden">
				<img src="/logo.svg" alt="" width={28} height={28} class="block shrink-0" />
				<span class="font-heading text-xl font-semibold tracking-tight text-foreground">Facile</span>
			</a>
			{@render children?.()}
			<div class="mt-10 flex flex-col gap-4">
				<LanguagePicker />
				<LegalLinks />
			</div>
		</div>
	</div>
</div>
