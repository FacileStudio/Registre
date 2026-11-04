<script lang="ts">
	import { t } from '$lib/i18n/i18n.svelte';
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
	import { getContext } from 'svelte';
	import { SESSION_CONTEXT, type ShellSession } from '$lib/account';
	import SuiteHome from '$lib/components/SuiteHome.svelte';
	import LegalLinks from '$lib/components/LegalLinks.svelte';

	const features = $derived([
		{ icon: icons.key, title: t('landing.f1'), description: t('landing.f1Body') },
		{ icon: icons.shield, title: t('landing.f2'), description: t('landing.f2Body') },
		{ icon: icons.usersGroup, title: t('landing.f3'), description: t('landing.f3Body') }
	]);

	const shell = getContext<ShellSession>(SESSION_CONTEXT);
</script>

<svelte:head>
	<title>{shell.me ? `${t('nav.apps')} — Facile` : t('landing.title')}</title>
</svelte:head>

{#if shell.me}
	<SuiteHome me={shell.me} />
{:else}
	<div class="min-h-dvh bg-fc-page text-fc-fg">
			<header class="border-b border-fc-border">
				<div class="mx-auto flex max-w-fc-xl items-center justify-between px-6 py-4">
					<div class="flex h-14 items-center gap-3">
						<img src="/logo.svg" alt="" width={24} height={24} class="block shrink-0" />
						<span class="text-fc-xl font-semibold tracking-tight">Facile</span>
					</div>
					<Button variant="ghost" href="/login">{t('login.submit')}</Button>
				</div>
			</header>

			<main>
				<Page width="xl">
					<Stack gap="content" class="items-center py-12 text-center">
						<h1 class="text-fc-3xl font-semibold tracking-tight">
							<span class="block"><TextElevate text={t('auth.tagline1')} delay={0.1} /></span>
							<span class="block"><TextElevate text={t('auth.tagline2')} delay={0.25} /></span>
						</h1>
						<p class="max-w-xl text-fc-md text-fc-fg-muted">
							{t('landing.pitch')}
						</p>
						<Inline gap="tight" class="justify-center pt-4">
							<Button size="lg" href="/login" iconRight={icons.arrow}>{t('login.title')}</Button>
							<Button size="lg" variant="outline" href="/signup">{t('signup.title')}</Button>
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
						<h2 class="text-fc-2xl font-semibold tracking-tight">{t('landing.ready')}</h2>
						<p class="text-fc-sm text-fc-fg-muted">
							{t('landing.readyBody')}
						</p>
						<Button size="lg" href="/login">{t('login.title')}</Button>
					</Stack>
				</Page>
			</main>

		<footer class="border-t border-fc-border">
			<div class="mx-auto max-w-fc-xl px-6 py-6 text-center text-fc-sm text-fc-fg-muted">
				<div class="flex flex-col items-center gap-3">
					<LegalLinks />
					<span>© {new Date().getFullYear()} Facile Studio</span>
				</div>
			</div>
		</footer>
	</div>
{/if}
