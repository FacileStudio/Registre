<script lang="ts">
	import { t } from '$lib/i18n/i18n.svelte';
	import { getContext } from 'svelte';
	import { SESSION_CONTEXT, type ShellSession } from '$lib/account';
	import SuiteHome from '$lib/components/SuiteHome.svelte';
	import Landing from '$lib/components/landing/Landing.svelte';

	const shell = getContext<ShellSession>(SESSION_CONTEXT);
</script>

<svelte:head>
	<title>{shell.me ? `${t('nav.apps')} — Facile` : `Registre · ${t('landing.title')}`}</title>
</svelte:head>

{#if shell.me}
	<SuiteHome me={shell.me} />
{:else}
	<Landing />
{/if}
