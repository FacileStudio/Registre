<script lang="ts">
	import {
		Button,
		Card,
		Divider,
		Icon,
		Inline,
		Page,
		Stack,
		TextElevate,
		icons
	} from '@facile/muse';
	import { account, type AccountView } from '$lib/account';
	import SuiteHome from '$lib/components/SuiteHome.svelte';

	const features = [
		{
			icon: icons.key,
			title: 'One identity, every app',
			description:
				'Sign in once with your Facile account. Registre vouches for you to ' +
				'Sablier, Nuage, Casier and the rest of the suite.'
		},
		{
			icon: icons.shield,
			title: 'Signed, verifiable tokens',
			description:
				'Every token is a JWT signed by rotating keys and published in an open JWKS. ' +
				'Apps verify it themselves, no phone-home required.'
		},
		{
			icon: icons.usersGroup,
			title: 'People and services',
			description:
				'Human accounts and service credentials live side by side, each scoped ' +
				'to exactly what the calling app is allowed to see.'
		}
	];

	let me = $state<AccountView | null>(null);
	let loading = $state(true);

	$effect(() => {
		account
			.me()
			.then((view) => {
				me = view;
				loading = false;
			})
			.catch(() => {
				me = null;
				loading = false;
			});
	});

	async function logout() {
		await account.logout().catch(() => {});
		me = null;
	}
</script>

<svelte:head>
	<title>{me ? 'Facile' : 'Registre — the Facile Suite’s identity provider'}</title>
</svelte:head>

<div class="min-h-dvh bg-fc-page text-fc-fg">
	{#if loading}
		<div class="flex min-h-dvh items-center justify-center">
			<div class="size-6 animate-spin rounded-full border-2 border-fc-ring border-t-transparent"></div>
		</div>
	{:else if me}
		<SuiteHome {me} onlogout={logout} />
	{:else}
		<header class="border-b border-fc-border">
			<div class="mx-auto flex max-w-fc-xl items-center justify-between px-6 py-4">
				<div class="flex h-14 items-center gap-3">
					<img src="/logo.svg" alt="" width={24} height={24} class="block shrink-0" />
					<span class="text-fc-xl font-semibold tracking-tight">Registre</span>
				</div>
				<Button variant="ghost" href="/login">Log in</Button>
			</div>
		</header>

		<main>
			<Page width="xl">
				<Stack gap="content" class="items-center py-12 text-center">
					<h1 class="text-fc-3xl font-semibold tracking-tight">
						<span class="block"><TextElevate text="One account." delay={0.1} /></span>
						<span class="block"><TextElevate text="Every app." delay={0.25} /></span>
					</h1>
					<p class="max-w-xl text-fc-md text-fc-fg-muted">
						Registre is the identity provider behind the Facile Suite. Sign in through the app you
						came from — there is nothing to do here by hand.
					</p>
					<Inline gap="tight" class="justify-center pt-4">
						<Button size="lg" href="/login" iconRight={icons.arrow}>Sign in</Button>
						<Button size="lg" variant="outline" href="https://facile.studio">About Facile</Button>
					</Inline>
				</Stack>

				<Divider />

				<div class="grid gap-4 md:grid-cols-3 [&>*]:min-w-0">
					{#each features as feature (feature.title)}
						<Card>
							<Stack gap="bound">
								<span
									class="mb-2 flex size-10 items-center justify-center rounded-fc-md bg-fc-surface text-fc-fg"
								>
									<Icon icon={feature.icon} size={20} />
								</span>
								<h2 class="text-fc-lg font-semibold">{feature.title}</h2>
								<p class="text-fc-sm text-fc-fg-muted">{feature.description}</p>
							</Stack>
						</Card>
					{/each}
				</div>

				<Divider />

				<Stack gap="content" class="items-center py-8 text-center">
					<h2 class="text-fc-2xl font-semibold tracking-tight">Ready when you are</h2>
					<p class="text-fc-sm text-fc-fg-muted">
						Head back to the app you were using; it will bring you here when it needs you.
					</p>
					<Button size="lg" href="/login">Sign in</Button>
				</Stack>
			</Page>
		</main>
	{/if}

	<footer class="border-t border-fc-border">
		<div class="mx-auto max-w-fc-xl px-6 py-6 text-center text-fc-sm text-fc-fg-muted">
			© {new Date().getFullYear()} Registre by
			<a
				href="https://facile.studio"
				class="rounded-fc-xs font-semibold underline underline-offset-2 transition-colors
					hover:text-fc-fg focus-visible:outline-2 focus-visible:outline-offset-2
					focus-visible:outline-fc-ring"
			>
				Facile.
			</a>
		</div>
	</footer>
</div>
