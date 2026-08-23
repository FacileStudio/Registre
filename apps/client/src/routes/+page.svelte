<script lang="ts">
	import { Card } from '@facile/muse';
</script>

<svelte:head>
	<title>registre — the Facile Suite's identity provider</title>
	<meta
		name="description"
		content="Registre signs the suite's users and services in. Sign-in starts from the app you are trying to reach."
	/>
</svelte:head>

<main class="grid min-h-dvh place-items-center bg-fc-page px-4">
	<Card class="w-full max-w-md p-10 text-center">
		<svg
			width="64"
			height="64"
			viewBox="0 0 24 24"
			fill="none"
			aria-hidden="true"
			class="mx-auto mb-6 text-fc-fg"
		>
			<circle cx="12" cy="12" r="4" fill="currentColor" />
			<g stroke="currentColor" stroke-width="2" stroke-linecap="round">
				<path d="M12 2v2" /><path d="M12 20v2" />
				<path d="m4.93 4.93 1.41 1.41" /><path d="m17.66 17.66 1.41 1.41" />
				<path d="M2 12h2" /><path d="M20 12h2" />
				<path d="m6.34 17.66-1.41 1.41" /><path d="m19.07 4.93-1.41 1.41" />
			</g>
		</svg>
		<h1 class="mb-2 text-fc-lg font-semibold text-fc-fg">registre</h1>
		<p class="text-fc-base text-fc-fg-muted">
			The Facile Suite's identity provider. There is nothing to do here by hand:
			sign-in starts from the app you are trying to reach, and lands back there
			when you are done.
		</p>
	</Card>
</main>
