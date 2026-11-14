<script lang="ts">
	import { Button, Icon, LandingFooter, LandingHeader, TextElevate, icons, scopeLocale } from '@facile/muse';
	import { locale, t } from '$lib/i18n/i18n.svelte';
	import { REGISTRE_ICON } from '$lib/icons';
	import SignInDemo from './SignInDemo.svelte';

	scopeLocale(locale);

	const links = $derived([
		{ href: '#features', label: t('landing.features') },
		{ href: '/device', label: 'CLI' }
	]);

	const columns = $derived([
		{
			title: 'Registre',
			links: [
				{ href: '/login', label: t('login.title') },
				{ href: '/signup', label: t('signup.title') },
				{ href: '/device', label: t('device.title') }
			]
		},
		{
			title: 'Suite',
			links: [
				{ href: 'https://facile.studio', label: 'Facile Studio', external: true },
				{ href: 'https://github.com/FacileStudio/Registre', label: 'GitHub', external: true }
			]
		}
	]);

	const features = $derived([
		{ icon: icons.key, title: t('landing.f1'), body: t('landing.f1Body') },
		{ icon: icons.shield, title: t('landing.f2'), body: t('landing.f2Body') },
		{ icon: 'solar:code-square-bold-duotone', title: t('landing.f3'), body: t('landing.f3Body') },
		{ icon: icons.usersGroup, title: t('landing.f4'), body: t('landing.f4Body') }
	]);
</script>

{#snippet brand()}
	<a href="/" class="flex items-center gap-2.5 font-heading text-fc-lg font-bold tracking-tight text-fc-fg">
		<img src="/logo.svg" alt="" width={22} height={22} class="block shrink-0" />
		<span>Registre</span>
	</a>
{/snippet}

<div class="flex min-h-dvh flex-col bg-fc-page text-fc-fg" lang={locale()}>
	<LandingHeader appName="Registre" icon={REGISTRE_ICON} homeHref="/" {links} skipHref="#main" {brand}>
		{#snippet actions()}
			<Button variant="ghost" size="sm" href="/login">{t('login.title')}</Button>
			<Button size="sm" href="/signup">{t('signup.title')}</Button>
		{/snippet}
	</LandingHeader>

	<main id="main" class="flex-1">
		<section class="mx-auto max-w-5xl px-4 pt-16 pb-10 text-center sm:px-6 sm:pt-24">
			<h1 class="font-heading text-fc-3xl font-bold tracking-tight sm:text-5xl">
				<span class="block"><TextElevate text={t('auth.tagline1')} delay={0.08} /></span>
				<span class="block text-fc-fg-muted"><TextElevate text={t('auth.tagline2')} delay={0.2} /></span>
			</h1>
			<p class="mx-auto mt-6 max-w-xl text-fc-md text-fc-fg-muted">{t('landing.pitch')}</p>
			<div class="mt-8 flex flex-wrap items-center justify-center gap-3">
				<Button size="lg" href="/signup" iconRight={icons.arrow}>{t('signup.title')}</Button>
				<Button size="lg" variant="ghost" href="/login">{t('login.title')}</Button>
			</div>
		</section>

		<section class="mx-auto max-w-5xl px-4 pb-8 sm:px-6">
			<SignInDemo />
		</section>

		<section id="features" class="mx-auto max-w-5xl scroll-mt-20 px-4 py-12 sm:px-6">
			<ul class="grid gap-4 sm:grid-cols-2">
				{#each features as feature (feature.title)}
					<li class="rounded-fc-lg bg-fc-component p-6">
						<div class="mb-4 flex size-10 items-center justify-center rounded-fc-md bg-fc-surface text-fc-fg">
							<Icon icon={feature.icon} size={20} />
						</div>
						<h2 class="text-fc-lg font-bold">{feature.title}</h2>
						<p class="mt-1.5 text-fc-sm text-fc-fg-muted">{feature.body}</p>
					</li>
				{/each}
			</ul>
		</section>

		<section class="mx-auto max-w-5xl px-4 py-20 text-center sm:px-6">
			<h2 class="font-heading text-fc-2xl font-bold tracking-tight">{t('landing.ready')}</h2>
			<p class="mt-4 text-fc-sm text-fc-fg-muted">{t('landing.readyBody')}</p>
			<div class="mt-8 flex flex-wrap items-center justify-center gap-3">
				<Button size="lg" href="/signup" iconRight={icons.arrow}>{t('signup.title')}</Button>
				<Button size="lg" variant="ghost" href="/login">{t('login.title')}</Button>
			</div>
		</section>
	</main>

	<LandingFooter
		appName="Registre"
		icon={REGISTRE_ICON}
		homeHref="/"
		tagline={t('landing.pitch')}
		{columns}
		legal={{ base: '/legal' }}
		{brand}
	/>
</div>
