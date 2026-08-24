<script lang="ts">
	import { page } from '$app/state';
	import { PageTransition } from '@facile/muse';

	let { children } = $props();
</script>

<div class="flex min-h-dvh flex-col items-center justify-center gap-8 bg-fc-page px-4 py-12">
	<a
		href="/"
		class="flex items-center gap-3 rounded-fc-md focus-visible:outline-2
			focus-visible:outline-offset-2 focus-visible:outline-fc-ring"
	>
		<img src="/logo.svg" alt="" width={24} height={24} class="block shrink-0" />
		<span class="text-fc-xl font-semibold tracking-tight text-fc-fg">Registre</span>
	</a>

	<PageTransition key={page.url.pathname} class="w-full max-w-sm">
		{@render children()}
	</PageTransition>

	<p class="text-fc-xs text-fc-fg-muted">The Facile Suite's identity provider.</p>
</div>
