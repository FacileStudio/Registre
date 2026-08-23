<script lang="ts">
	import { Button, Divider, TextElevate, icons } from '@facile/muse';

	const features = [
		{
			icon: 'solar:key-minimalistic-square-3-linear',
			title: 'One identity, every app',
			description:
				'Sign in once with your Facile account. Registre vouches for you to Sablier, Nuage, Casier and the rest of the suite.'
		},
		{
			icon: 'solar:shield-check-linear',
			title: 'Signed, verifiable tokens',
			description:
				'Every token is a JWT signed by rotating keys and published in an open JWKS. Apps verify it themselves, no phone-home required.'
		},
		{
			icon: icons.usersGroup,
			title: 'People and services',
			description:
				'Human accounts and service credentials live side by side, each scoped to exactly what the calling app is allowed to see.'
		}
	];
</script>

<svelte:head>
	<title>registre — the Facile Suite's identity provider</title>
	<meta
		name="description"
		content="Registre signs the Facile Suite's users and services in. One account, one sign-in, every app."
	/>
</svelte:head>

<div class="min-h-dvh bg-fc-page text-fc-fg">
	<header class="border-b border-fc-border">
		<div class="mx-auto flex max-w-5xl items-center justify-between px-6 py-4">
			<div class="flex h-14 items-center gap-3">
				<iconify-icon
					icon="solar:key-minimalistic-square-3-bold-duotone"
					width="24"
					height="24"
					class="block shrink-0"
				></iconify-icon>
				<span class="font-fc-title text-fc-xl font-semibold tracking-tight">registre</span>
			</div>
			<Button variant="ghost" href="/login" class="rounded-fc-md">Log in</Button>
		</div>
	</header>

	<main>
		<section class="mx-auto max-w-5xl px-6 py-24 text-center">
			<h1 class="font-fc-title text-fc-3xl font-semibold tracking-tight">
				<span class="block"><TextElevate text="One account." delay={0.1} /></span>
				<span class="block"><TextElevate text="Every app." delay={0.25} /></span>
			</h1>
			<p class="mx-auto mt-6 max-w-xl text-fc-md text-fc-fg-muted">
				Registre is the identity provider behind the Facile Suite. Sign in through the app you came
				from — there is nothing to do here by hand.
			</p>
			<div class="mt-10 flex flex-wrap justify-center gap-3">
				<Button size="lg" href="/login" iconRight={icons.arrow} class="rounded-fc-md">
					Sign in
				</Button>
				<Button
					size="lg"
					variant="outline"
					href="https://facile.studio"
					class="rounded-fc-md">About Facile</Button
				>
			</div>
		</section>

		<Divider class="my-0" />

		<section class="mx-auto max-w-5xl px-6 py-20">
			<div class="grid gap-6 md:grid-cols-3">
				{#each features as feature (feature.title)}
					<div class="rounded-fc-lg border border-fc-border p-6">
						<div
							class="mb-3 flex size-10 items-center justify-center rounded-fc-md border border-fc-border"
						>
							<iconify-icon icon={feature.icon} width="20" height="20" class="block shrink-0"
							></iconify-icon>
						</div>
						<h3 class="text-fc-lg font-semibold">{feature.title}</h3>
						<p class="mt-1.5 text-fc-sm text-fc-fg-muted">{feature.description}</p>
					</div>
				{/each}
			</div>
		</section>

		<Divider class="my-0" />

		<section class="mx-auto max-w-5xl px-6 py-20 text-center">
			<h2 class="font-fc-title text-fc-2xl font-semibold tracking-tight">Ready when you are</h2>
			<p class="mt-4 text-fc-sm text-fc-fg-muted">
				Head back to the app you were using; it will bring you here when it needs you.
			</p>
			<div class="mt-8 flex justify-center">
				<Button size="lg" href="/login" class="rounded-fc-md">Sign in</Button>
			</div>
		</section>
	</main>

	<footer class="border-t border-fc-border">
		<div class="mx-auto max-w-5xl px-6 py-6 text-center text-fc-sm text-fc-fg-muted">
			© {new Date().getFullYear()} registre by
			<a
				href="https://facile.studio"
				class="font-semibold underline underline-offset-2 transition-colors hover:text-fc-fg"
				>Facile.</a
			>
		</div>
	</footer>
</div>
