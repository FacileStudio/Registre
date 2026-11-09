<script lang="ts">
	import type { Snippet } from 'svelte';
	import LanguagePicker from '$lib/components/LanguagePicker.svelte';
	import LegalLinks from '$lib/components/LegalLinks.svelte';

	let { children }: { children?: Snippet } = $props();
</script>

<div class="flex min-h-dvh flex-col items-center justify-center bg-fc-page px-4 py-8 sm:px-6">
	<div class="flex w-full max-w-[440px] flex-col items-center sm:max-w-[480px]">
		<div class="flex w-full flex-col items-center rounded-fc-2xl bg-fc-surface p-6 shadow-sm sm:rounded-fc-3xl sm:p-10">
			<a href="/" class="mb-6 flex items-center gap-2.5 transition-opacity hover:opacity-90">
				<img src="/logo.svg" alt="" width={36} height={36} class="block shrink-0" />
				<span class="font-heading text-2xl font-semibold tracking-tight text-fc-fg">Facile</span>
			</a>
			<div class="w-full">
				{@render children?.()}
			</div>
		</div>

		<div class="mt-6 flex w-full flex-col items-center justify-between gap-4 px-2 text-center sm:flex-row sm:text-left">
			<LanguagePicker />
			<LegalLinks />
		</div>
	</div>
</div>
