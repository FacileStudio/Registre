<script lang="ts">
	import { Badge, Button, Icon, icons } from '@facile/muse';
	import type { AccountView } from '$lib/account';
	import { categories, domainColor, getDomain, type PortalApp } from '$lib/portal';

	interface Props {
		me: AccountView;
		onlogout: () => Promise<void>;
	}

	let { me, onlogout }: Props = $props();

	let searchQuery = $state('');
	let paletteOpen = $state(false);
	let paletteQuery = $state('');
	let activeIdx = $state(0);
	let paletteInput: HTMLInputElement | null = $state(null);

	const allLinks = $derived(
		Object.entries(categories).flatMap(([category, apps]) =>
			apps.map((app) => ({ ...app, category, domain: getDomain(app.url) }))
		)
	);

	const filteredCategories = $derived.by(() => {
		const q = searchQuery.toLowerCase().trim();
		if (!q) return categories;
		const result: Record<string, PortalApp[]> = {};
		for (const [cat, apps] of Object.entries(categories)) {
			const matched = apps.filter(
				(a) =>
					a.name.toLowerCase().includes(q) ||
					a.desc.toLowerCase().includes(q) ||
					cat.toLowerCase().includes(q)
			);
			if (matched.length > 0) result[cat] = matched;
		}
		return result;
	});

	const hasResults = $derived(Object.keys(filteredCategories).length > 0);

	const paletteMatches = $derived.by(() => {
		const q = paletteQuery.toLowerCase().trim();
		if (!q) return allLinks.slice(0, 20);
		return allLinks
			.filter(
				(l) =>
					l.name.toLowerCase().includes(q) ||
					l.desc.toLowerCase().includes(q) ||
					l.category.toLowerCase().includes(q)
			)
			.slice(0, 20);
	});

	function openPalette() {
		paletteOpen = true;
		paletteQuery = '';
		activeIdx = 0;
		requestAnimationFrame(() => paletteInput?.focus());
	}

	function closePalette() {
		paletteOpen = false;
	}

	function onKeydown(e: KeyboardEvent) {
		if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
			e.preventDefault();
			paletteOpen ? closePalette() : openPalette();
			return;
		}
		if (!paletteOpen) return;
		if (e.key === 'Escape') {
			e.preventDefault();
			closePalette();
		} else if (e.key === 'ArrowDown') {
			e.preventDefault();
			if (paletteMatches.length) activeIdx = (activeIdx + 1) % paletteMatches.length;
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (paletteMatches.length) {
				activeIdx = (activeIdx - 1 + paletteMatches.length) % paletteMatches.length;
			}
		} else if (e.key === 'Enter') {
			e.preventDefault();
			if (paletteMatches[activeIdx]) {
				window.open(paletteMatches[activeIdx].url, '_blank', 'noopener,noreferrer');
				closePalette();
			}
		}
	}
</script>

<svelte:window onkeydown={onKeydown} />

<header class="sticky top-0 z-30 border-b border-fc-border bg-fc-surface/50 backdrop-blur-sm">
	<div class="mx-auto flex max-w-5xl items-center justify-between px-6 py-3">
		<a href="/" class="flex items-center gap-3">
			<img src="/logo.svg" alt="" width={24} height={24} class="block shrink-0" />
			<span class="text-fc-lg font-semibold tracking-tight">Portail Facile</span>
		</a>
		<div class="flex items-center gap-2">
			{#if me.admin}
				<Button variant="outline" size="sm" href="/admin" icon={icons.shield}>
					Administration
				</Button>
			{/if}
			<Button variant="ghost" size="sm" href="/account" icon={icons.userCircle}>
				{me.name || me.email}
			</Button>
			<Button variant="ghost" size="sm" icon={icons.logout} onclick={onlogout}>
				Sign out
			</Button>
		</div>
	</div>
</header>

<main class="mx-auto max-w-5xl px-6 py-8">
	<div class="relative mb-8">
		<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5 text-fc-fg-muted">
			<Icon icon={icons.search} size={16} />
		</div>
		<input
			type="text"
			bind:value={searchQuery}
			placeholder="Search apps and services..."
			class="w-full rounded-fc-lg border border-fc-border bg-fc-surface py-2.5 pl-10 pr-16 text-fc-sm
				text-fc-fg placeholder-fc-fg-muted outline-none transition-colors focus:border-fc-ring"
		/>
		<button
			type="button"
			onclick={openPalette}
			class="absolute inset-y-0 right-3 my-auto flex h-6 items-center rounded-fc-xs border
				border-fc-border bg-fc-page px-1.5 text-fc-2xs font-medium text-fc-fg-muted hover:text-fc-fg"
		>
			⌘K
		</button>
	</div>

	{#if !hasResults}
		<div class="py-16 text-center text-fc-sm text-fc-fg-muted">
			Nothing found for "{searchQuery}".
		</div>
	{:else}
		<div class="flex flex-col gap-10">
			{#each Object.entries(filteredCategories) as [cat, apps] (cat)}
				<section class="flex flex-col gap-3">
					<div class="flex items-center gap-2.5">
						<h2 class="text-fc-md font-semibold tracking-tight text-fc-fg">{cat}</h2>
						<Badge tone="neutral">{apps.length}</Badge>
					</div>
					<div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
						{#each apps as app (app.url)}
							{@const domain = getDomain(app.url)}
							<a
								href={app.url}
								target="_blank"
								rel="noopener noreferrer"
								class="group flex items-center gap-3.5 rounded-fc-lg border border-fc-border
									bg-fc-surface p-3 transition-all hover:-translate-y-0.5
									hover:border-fc-border-hover hover:bg-fc-surface-hover"
							>
								<div
									class="relative flex size-9 shrink-0 items-center justify-center overflow-hidden
										rounded-fc-md text-fc-xs font-bold text-white shadow-sm"
									style="background: {domainColor(domain)}"
								>
									<span>{app.name.charAt(0)}</span>
									<img
										src={app.icon || `https://${domain}/favicon.ico`}
										alt=""
										class="absolute inset-0 size-full rounded-fc-md bg-fc-surface object-cover
											opacity-0 transition-opacity"
										onload={(e) => {
											const el = e.currentTarget as HTMLImageElement;
											if (el.naturalWidth) el.style.opacity = '1';
										}}
										onerror={(e) => {
											(e.currentTarget as HTMLImageElement).remove();
										}}
									/>
								</div>
								<div class="min-w-0 flex-1">
									<div class="truncate text-fc-sm font-medium text-fc-fg group-hover:text-fc-primary">
										{app.name}
									</div>
									<div class="truncate text-fc-xs text-fc-fg-muted">
										{app.desc}
									</div>
								</div>
							</a>
						{/each}
					</div>
				</section>
			{/each}
		</div>
	{/if}
</main>

{#if paletteOpen}
	<div
		class="fixed inset-0 z-50 flex items-start justify-center bg-black/50 p-4 pt-20 backdrop-blur-xs"
		onclick={(e) => {
			if (e.target === e.currentTarget) closePalette();
		}}
		role="presentation"
	>
		<div class="w-full max-w-lg overflow-hidden rounded-fc-xl border border-fc-border bg-fc-surface shadow-2xl">
			<div class="flex items-center border-b border-fc-border px-4 py-3">
				<Icon icon={icons.search} size={18} class="shrink-0 text-fc-fg-muted" />
				<input
					bind:this={paletteInput}
					bind:value={paletteQuery}
					type="text"
					placeholder="Search links..."
					class="w-full bg-transparent px-3 text-fc-sm text-fc-fg placeholder-fc-fg-muted outline-none"
				/>
				<button type="button" onclick={closePalette} class="text-fc-fg-muted hover:text-fc-fg">
					<Icon icon={icons.close} size={16} />
				</button>
			</div>
			<div class="max-h-80 overflow-y-auto p-2">
				{#if paletteMatches.length === 0}
					<div class="py-6 text-center text-fc-sm text-fc-fg-muted">No matches</div>
				{:else}
					{#each paletteMatches as match, idx (match.url)}
						<a
							href={match.url}
							target="_blank"
							rel="noopener noreferrer"
							onclick={closePalette}
							class="flex items-center gap-3 rounded-fc-md px-3 py-2 text-fc-sm transition-colors
								{idx === activeIdx
								? 'bg-fc-surface-hover text-fc-fg'
								: 'text-fc-fg-muted hover:bg-fc-surface-hover hover:text-fc-fg'}"
						>
							<div
								class="relative flex size-6 shrink-0 items-center justify-center overflow-hidden
									rounded-fc-xs text-fc-2xs font-bold text-white"
								style="background: {domainColor(match.domain)}"
							>
								<span>{match.name.charAt(0)}</span>
								<img
									src={match.icon || `https://${match.domain}/favicon.ico`}
									alt=""
									class="absolute inset-0 size-full object-cover opacity-0"
									onload={(e) => {
										const el = e.currentTarget as HTMLImageElement;
										if (el.naturalWidth) el.style.opacity = '1';
									}}
									onerror={(e) => {
										(e.currentTarget as HTMLImageElement).remove();
									}}
								/>
							</div>
							<span class="font-medium text-fc-fg">{match.name}</span>
							<span class="truncate text-fc-xs text-fc-fg-muted">{match.desc}</span>
							<span class="ml-auto shrink-0 text-fc-2xs text-fc-fg-muted">{match.category}</span>
						</a>
					{/each}
				{/if}
			</div>
			<div class="flex items-center gap-4 border-t border-fc-border px-4 py-2 text-fc-2xs text-fc-fg-muted">
				<span><kbd class="rounded border border-fc-border bg-fc-page px-1">↑↓</kbd> navigate</span>
				<span><kbd class="rounded border border-fc-border bg-fc-page px-1">↵</kbd> open</span>
				<span><kbd class="rounded border border-fc-border bg-fc-page px-1">esc</kbd> close</span>
			</div>
		</div>
	</div>
{/if}
