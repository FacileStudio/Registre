<script lang="ts">
	import { t } from '$lib/i18n/i18n.svelte';
	import {
		Button,
		Icon,
		LandingFooter,
		LandingHeader,
		Page,
		TextElevate,
		icons
	} from '@facile/muse';
	import { getContext } from 'svelte';
	import { SESSION_CONTEXT, type ShellSession } from '$lib/account';
	import SuiteHome from '$lib/components/SuiteHome.svelte';
	import { REGISTRE_ICON } from '$lib/icons';

	const shell = getContext<ShellSession>(SESSION_CONTEXT);

	const links = $derived([
		{ href: '#features', label: t('nav.apps') },
		{ href: '#security', label: t('account.tab.security') },
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
		{
			icon: icons.key,
			title: t('landing.f1'),
			description: t('landing.f1Body'),
			highlights: ['Sablier', 'Nuage', 'Courrier', 'Agenda', 'Opus', 'Bulle']
		},
		{
			icon: icons.shield,
			title: t('landing.f2'),
			description: t('landing.f2Body'),
			highlights: ['FIDO2', 'WebAuthn', 'Biometrics', 'Breach Screening']
		},
		{
			icon: icons.usersGroup,
			title: t('landing.f3'),
			description: t('landing.f3Body'),
			highlights: ['Sovereignty', 'Shared Spaces', 'Instant Invite', 'Zero Tracking']
		}
	]);

	const protocols = $derived([
		{
			title: 'RFC 8628 Device Flow',
			body: 'Authorize command-line tools like facile, bulle, or pastille without ever typing a password or secret into your terminal.',
			badge: 'CLI Authorization'
		},
		{
			title: 'OpenID Connect & OAuth 2.0',
			body: 'Industry-standard PKCE authorization flow powering secure web and mobile applications across the entire sovereign suite.',
			badge: 'Standard Protocol'
		},
		{
			title: 'Passkeys & WebAuthn',
			body: 'Cryptographic public-key authentication built right into your hardware. Phishing-proof and hassle-free.',
			badge: 'Zero Passwords'
		}
	]);
</script>

<svelte:head>
	<title>{shell.me ? `${t('nav.apps')} — Facile` : 'Registre · One account for the whole suite'}</title>
</svelte:head>

{#if shell.me}
	<SuiteHome me={shell.me} />
{:else}
	<div class="flex min-h-dvh flex-col bg-fc-page text-fc-fg">
		<LandingHeader appName="Registre" icon={REGISTRE_ICON} homeHref="/" {links} skipHref="#main">
			{#snippet brand()}
				<a href="/" class="flex items-center gap-2.5 font-heading text-fc-lg font-semibold tracking-tight text-fc-fg">
					<img src="/logo.svg" alt="" width={22} height={22} class="block shrink-0" />
					<span>Registre</span>
				</a>
			{/snippet}
			{#snippet actions()}
				<Button variant="ghost" size="sm" href="/login">{t('login.title')}</Button>
				<Button size="sm" href="/signup">{t('signup.title')}</Button>
			{/snippet}
		</LandingHeader>

		<main id="main" class="flex-1">
			<Page width="xl">
				<div class="flex flex-col gap-24 py-16 sm:py-24">
					<section class="flex flex-col items-center text-center">
						<div class="mb-6 inline-flex items-center gap-2 rounded-fc-pill bg-fc-surface px-3.5 py-1.5 text-fc-xs font-medium text-fc-fg-muted shadow-sm">
							<Icon icon={icons.shield} size={14} class="text-fc-fg" />
							<span>Facile Identity & Single Sign-On</span>
						</div>

						<h1 class="font-heading text-4xl font-semibold tracking-tight sm:text-5xl md:text-6xl text-fc-fg max-w-4xl">
							<span class="block"><TextElevate text={t('auth.tagline1')} delay={0.08} /></span>
							<span class="block text-fc-fg-muted"><TextElevate text={t('auth.tagline2')} delay={0.2} /></span>
						</h1>

						<p class="mx-auto mt-6 max-w-2xl text-fc-base sm:text-fc-lg leading-relaxed text-fc-fg-muted">
							{t('landing.pitch')}
						</p>

						<div class="mt-8 flex flex-wrap items-center justify-center gap-3">
							<Button size="lg" href="/signup" iconRight={icons.arrow}>{t('signup.title')}</Button>
							<Button size="lg" variant="ghost" href="/login">{t('login.title')}</Button>
						</div>

						<div class="mt-16 flex flex-wrap items-center justify-center gap-x-8 gap-y-3 text-fc-xs text-fc-fg-muted">
							<span class="flex items-center gap-1.5">
								<Icon icon={icons.check} size={14} class="text-fc-success" />
								Passkeys & Biometrics
							</span>
							<span class="flex items-center gap-1.5">
								<Icon icon={icons.check} size={14} class="text-fc-success" />
								OpenID Connect 1.0
							</span>
							<span class="flex items-center gap-1.5">
								<Icon icon={icons.check} size={14} class="text-fc-success" />
								RFC 8628 Device Flow
							</span>
							<span class="flex items-center gap-1.5">
								<Icon icon={icons.check} size={14} class="text-fc-success" />
								100% Sovereign
							</span>
						</div>
					</section>

					<section id="features" class="flex flex-col gap-10">
						<div class="flex flex-col items-center text-center gap-2">
							<span class="text-fc-xs font-semibold uppercase tracking-wider text-fc-fg-muted">Architecture</span>
							<h2 class="font-heading text-3xl font-semibold tracking-tight text-fc-fg sm:text-4xl">
								{t('landing.title')}
							</h2>
						</div>

						<div class="grid gap-6 md:grid-cols-3">
							{#each features as feature (feature.title)}
								<div class="flex flex-col rounded-fc-2xl bg-fc-surface p-7 sm:p-8 shadow-sm">
									<div class="mb-5 flex size-12 items-center justify-center rounded-fc-xl bg-fc-field text-fc-fg">
										<Icon icon={feature.icon} size={22} />
									</div>
									<h3 class="font-heading text-fc-lg font-semibold text-fc-fg mb-2.5">{feature.title}</h3>
									<p class="text-fc-sm text-fc-fg-muted leading-relaxed mb-6 flex-1">{feature.description}</p>
									<div class="flex flex-wrap gap-1.5 pt-4">
										{#each feature.highlights as tag (tag)}
											<span class="rounded-fc-pill bg-fc-field px-2.5 py-1 text-[11px] font-medium text-fc-fg-muted">
												{tag}
											</span>
										{/each}
									</div>
								</div>
							{/each}
						</div>
					</section>

					<section id="security" class="flex flex-col gap-10">
						<div class="flex flex-col items-center text-center gap-2">
							<span class="text-fc-xs font-semibold uppercase tracking-wider text-fc-fg-muted">Protocols</span>
							<h2 class="font-heading text-3xl font-semibold tracking-tight text-fc-fg sm:text-4xl">
								Built on open security standards
							</h2>
						</div>

						<div class="grid gap-6 md:grid-cols-3">
							{#each protocols as proto (proto.title)}
								<div class="flex flex-col rounded-fc-2xl bg-fc-surface p-7 sm:p-8 shadow-sm">
									<div class="mb-4">
										<span class="rounded-fc-pill bg-fc-field px-2.5 py-1 text-fc-xs font-medium text-fc-fg-muted">
											{proto.badge}
										</span>
									</div>
									<h3 class="font-heading text-fc-base font-semibold text-fc-fg mb-2">{proto.title}</h3>
									<p class="text-fc-sm text-fc-fg-muted leading-relaxed">{proto.body}</p>
								</div>
							{/each}
						</div>
					</section>

					<section class="rounded-fc-3xl bg-fc-surface p-8 sm:p-14 text-center shadow-sm">
						<div class="mx-auto flex max-w-xl flex-col items-center gap-4">
							<div class="flex size-14 items-center justify-center rounded-fc-2xl bg-fc-field text-fc-fg mb-2">
								<Icon icon={icons.shield} size={28} />
							</div>
							<h2 class="font-heading text-3xl font-semibold tracking-tight text-fc-fg sm:text-4xl">
								{t('landing.ready')}
							</h2>
							<p class="text-fc-sm sm:text-fc-base text-fc-fg-muted leading-relaxed">
								{t('landing.readyBody')}
							</p>
							<div class="mt-4 flex flex-wrap items-center justify-center gap-3">
								<Button size="lg" href="/signup" iconRight={icons.arrow}>{t('signup.title')}</Button>
								<Button size="lg" variant="ghost" href="/login">{t('login.title')}</Button>
							</div>
						</div>
					</section>
				</div>
			</Page>
		</main>

		<LandingFooter
			appName="Registre"
			icon={REGISTRE_ICON}
			homeHref="/"
			tagline={t('landing.pitch')}
			{columns}
			legal={{ base: '/legal' }}
		>
			{#snippet brand()}
				<a href="/" class="flex items-center gap-2.5 font-heading text-fc-lg font-semibold tracking-tight text-fc-fg">
					<img src="/logo.svg" alt="" width={22} height={22} class="block shrink-0" />
					<span>Registre</span>
				</a>
			{/snippet}
		</LandingFooter>
	</div>
{/if}
