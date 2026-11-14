<script lang="ts">
	import {
		Button,
		FeatureSplit,
		ForDevelopers,
		LANDING_HEADING,
		LandingCta,
		LandingFooter,
		LandingHeader,
		PricingPlans,
		TextElevate,
		WorksWith,
		icons,
		scopeLocale,
		type DeveloperRow,
		type FeatureItem
	} from '@facile/muse';
	import { locale, t } from '$lib/i18n/i18n.svelte';
	import { REGISTRE_ICON } from '$lib/icons';
	import SignInDemo from './SignInDemo.svelte';
	import { openApps } from './apps';
	import { PLANS } from './plans';

	scopeLocale(locale);

	const width = 'mx-auto w-full max-w-5xl px-4 sm:px-6';

	const links = $derived([
		{ href: '#features', label: t('landing.features') },
		{ href: '#pricing', label: t('landing.plans') },
		{ href: '#apps', label: t('landing.apps') }
	]);

	const columns = $derived([
		{
			title: 'Registre',
			links: [
				{ href: '/signup', label: t('signup.title') },
				{ href: '/login', label: t('login.title') },
				{ href: '/device', label: t('device.title') }
			]
		},
		{
			title: 'Suite',
			links: [
				{ href: 'https://suite.facile.studio', label: 'Facile', external: true },
				{ href: 'https://suite.facile.studio/#tarifs', label: t('landing.plans'), external: true },
				{ href: 'https://github.com/FacileStudio/Registre', label: 'GitHub', external: true }
			]
		}
	]);

	const features: FeatureItem[] = $derived([
		{ icon: icons.usersGroup, title: t('landing.f1'), text: t('landing.f1Body') },
		{ icon: icons.key, title: t('landing.f2'), text: t('landing.f2Body') },
		{ icon: icons.card, title: t('landing.f3'), text: t('landing.f3Body') },
		{ icon: icons.shield, title: t('landing.f4'), text: t('landing.f4Body') }
	]);

	const apps = $derived(openApps(locale()));

	const notes = $derived({ solo: t('landing.planFree'), '*': t('landing.planPaid') });

	$effect(() => {
		document.querySelector('meta[name="description"]')?.setAttribute('content', t('landing.meta'));
	});

	const developers: DeveloperRow[] = $derived([
		{ kind: 'cli', text: t('landing.devCli'), command: 'curl -fsSL https://get.facile.studio | sh', href: 'https://github.com/FacileStudio/facile' },
		{ kind: 'source', text: t('landing.devSource'), href: 'https://github.com/FacileStudio/Registre' }
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
		{/snippet}
	</LandingHeader>

	<main id="main" class="flex flex-1 flex-col gap-20 pb-20 sm:gap-28">
		<section class="{width} pt-16 text-center sm:pt-24">
			<h1 class="font-heading text-fc-3xl font-bold tracking-tight sm:text-5xl">
				<span class="block"><TextElevate text={t('auth.tagline1')} delay={0.08} /></span>
				<span class="block text-fc-fg-muted"><TextElevate text={t('auth.tagline2')} delay={0.2} /></span>
			</h1>
			<p class="mx-auto mt-6 max-w-xl text-fc-md text-fc-fg-muted">{t('landing.pitch')}</p>
			<LandingCta class="mt-8" align="center" signUpHref="/signup" signInHref="/login" />
			<div class="mt-12 text-left">
				<SignInDemo />
			</div>
		</section>

		<FeatureSplit
			id="features"
			class="{width} scroll-mt-20"
			title={t('landing.featuresTitle')}
			description={t('landing.featuresLine')}
			{features}
		/>

		<PricingPlans
			id="pricing"
			class="{width} scroll-mt-20"
			title={t('landing.plansTitle')}
			headingClass={LANDING_HEADING}
			catalog={PLANS}
			{notes}
			recommended="pro"
			ctaHref="/signup"
			suiteHref="https://suite.facile.studio/#tarifs"
		/>

		<WorksWith
			id="apps"
			class="{width} scroll-mt-20"
			title={t('landing.appsTitle')}
			description={t('landing.appsLine')}
			{apps}
		/>

		<div class={width}>
			<ForDevelopers rows={developers} description={t('landing.devLine')} selfHostHref="https://suite.facile.studio/auto-hebergement" />
		</div>

		<section class="{width} text-center">
			<h2 class={LANDING_HEADING}>{t('landing.ready')}</h2>
			<p class="mt-4 text-fc-sm text-fc-fg-muted">{t('landing.readyBody')}</p>
			<LandingCta class="mt-8" align="center" signUpHref="/signup" signInHref="/login" />
		</section>
	</main>

	<LandingFooter
		appName="Registre"
		icon={REGISTRE_ICON}
		homeHref="/"
		tagline={t('landing.footer')}
		{columns}
		legal={{ base: '/legal' }}
		{brand}
	/>
</div>
