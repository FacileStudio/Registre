<script lang="ts">
	import { Icon } from '@facile/muse';
	import type { SuiteApp } from '$lib/suite';

	let { app }: { app: Pick<SuiteApp, 'logo' | 'icon'> } = $props();
</script>

<span
	aria-hidden="true"
	class="flex size-11 shrink-0 items-center justify-center rounded-fc-md bg-fc-surface text-fc-fg"
>
	{#if app.icon}
		<Icon icon={app.icon} size={22} />
	{:else if app.logo}
		<span
			class="block size-6 bg-fc-fg"
			style="mask: url({app.logo}) center / contain no-repeat; -webkit-mask: url({app.logo}) center / contain no-repeat"
		></span>
	{/if}
</span>
