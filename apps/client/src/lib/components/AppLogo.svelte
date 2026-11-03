<script lang="ts">
	import { Icon } from '@facile/muse';
	import type { SuiteApp } from '$lib/suite';

	let { app }: { app: SuiteApp } = $props();
</script>

<!--
@component
An app's brand glyph on a surface tile. The SVG is used as a mask and filled
with the fg token, so every logo follows the page's theme rather than the
colour scheme baked into its file.
-->

<span
	aria-hidden="true"
	class="flex size-11 shrink-0 items-center justify-center rounded-fc-md bg-fc-surface text-fc-fg"
>
	{#if app.logo}
		<span
			class="block size-6 bg-fc-fg"
			style="mask: url({app.logo}) center / contain no-repeat; -webkit-mask: url({app.logo}) center / contain no-repeat"
		></span>
	{:else if app.icon}
		<Icon icon={app.icon} size={22} />
	{/if}
</span>
