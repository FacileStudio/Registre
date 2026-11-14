<script lang="ts">
	import { Bar, Cursor, Stage, Window, cursorAt, type Act } from '@facile/mise-en-scene';
	import { Icon, SignInWithFacile, icons } from '@facile/muse';
	import AppLogo from '$lib/components/AppLogo.svelte';
	import { t } from '$lib/i18n/i18n.svelte';
	import { REGISTRE_ICON } from '$lib/icons';
	import { suite, type SuiteApp } from '$lib/suite';

	const NAMES = ['Plume', 'Nuage', 'Sablier', 'Courrier', 'Agenda', 'Opus'];
	const all = suite.flatMap((group) => group.apps);
	const apps = NAMES.map((name) => all.find((app) => app.name === name)).filter((app): app is SuiteApp => !!app);
	const first = apps[0];

	let pending = $state(false);
	let signed = $state(false);
	let opened = $state(0);
	let cursor = $state(cursorAt(560, 420));
	let button = $state<HTMLElement>();

	function reset() {
		pending = false;
		signed = false;
		opened = 0;
		cursor = cursorAt(560, 420);
	}

	function poster() {
		signed = true;
		opened = apps.length;
	}

	async function script(act: Act) {
		await act.wait(700);
		cursor.visible = true;
		await act.move(cursor, act.point(button, 0.62, 0.7));
		await act.click(cursor, () => (pending = true));
		await act.wait(700);
		cursor.visible = false;
		signed = true;
		await act.wait(450);
		await act.count(apps.length, (v) => (opened = v), 320);
		await act.wait(2200);
		reset();
		await act.wait(500);
	}
</script>

{#snippet avatar(size: number)}
	<span class="sd-avatar" style="width:{size}px;height:{size}px;font-size:{Math.round(size * 0.4)}px">C</span>
{/snippet}

{#snippet scene(compact: boolean)}
	<div class="sd-body" class:sd-compact={compact}>
		<div class="sd-login" class:sd-gone={signed}>
			{#if first}<AppLogo app={first} />{/if}
			<span class="font-heading text-[20px] font-bold">{first?.name}</span>
			<div class="mb-2.5 grid w-[220px] justify-items-center gap-2"><Bar w="70%" /><Bar w="45%" /></div>
			<span bind:this={button} class="inline-flex">
				<SignInWithFacile href="/login" loading={pending} tabindex={-1} />
			</span>
		</div>
		<div class="sd-account" class:sd-gone={!signed}>
			<div class="sd-user">
				{@render avatar(38)}
				<span class="sd-who"><b class="font-bold">Camille</b><Bar w="70%" /></span>
				<span class="inline-flex items-center gap-1.5 text-[13px] text-fc-fg-muted"><Icon icon={REGISTRE_ICON} size={16} />Registre</span>
			</div>
			<ul class="sd-apps">
				{#each apps as app, i (app.name)}
					<li class="sd-tile" class:sd-on={opened > i}>
						<AppLogo {app} />
						<span class="sd-tile-text">
							<span class="sd-tile-name">{app.name}</span>
							<span class="sd-tile-state">{t('landing.demoSigned')}</span>
						</span>
						<span class="sd-check">{@render avatar(26)}<Icon icon={icons.check} size={14} /></span>
					</li>
				{/each}
			</ul>
		</div>
	</div>
{/snippet}

<Stage
	width={960}
	height={400}
	compact={{ below: 640, width: 358, height: 576 }}
	{script}
	{reset}
	{poster}
	label={t('landing.demoLabel')}
>
	{#snippet children({ compact })}
		<Window
			title={compact ? undefined : `${first?.name.toLowerCase()}.facile.studio`}
			style={compact ? 'top:12px;left:12px;width:334px;height:552px' : 'top:24px;left:48px;width:864px;height:352px'}
		>
			<div class="sd-wrap">{@render scene(compact)}</div>
		</Window>
		<Cursor {cursor} />
	{/snippet}
</Stage>

<style>
	.sd-wrap {
		position: relative;
		flex: 1;
		min-height: 0;
		color: var(--color-fc-fg);
		font-family: var(--font-fc-body);
	}
	.sd-body {
		position: absolute;
		inset: 0;
	}
	.sd-login,
	.sd-account {
		position: absolute;
		inset: 0;
		display: flex;
		flex-direction: column;
		transition:
			opacity 0.35s ease,
			translate 0.35s ease;
	}
	.sd-gone {
		opacity: 0;
		translate: 0 8px;
	}
	.sd-login {
		align-items: center;
		justify-content: center;
		gap: 14px;
	}
	.sd-account {
		gap: 16px;
		padding: 28px 32px;
	}
	.sd-compact .sd-account {
		gap: 12px;
		padding: 16px;
	}
	.sd-user {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 12px 14px;
		border-radius: 14px;
		background: var(--color-fc-surface);
	}
	.sd-avatar {
		display: flex;
		flex-shrink: 0;
		align-items: center;
		justify-content: center;
		border-radius: 999px;
		background: var(--color-fc-accent);
		color: var(--color-fc-accent-fg);
		font-weight: 700;
	}
	.sd-who {
		display: flex;
		flex: 1;
		flex-direction: column;
		gap: 6px;
		min-width: 0;
		font-size: 14px;
	}
	.sd-apps {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 12px;
		margin: 0;
		padding: 0;
		list-style: none;
	}
	.sd-compact .sd-apps {
		grid-template-columns: minmax(0, 1fr);
		gap: 8px;
	}
	.sd-tile {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 14px;
		border-radius: 10px;
		background: var(--color-fc-component);
		opacity: 0.45;
		transition: opacity 0.3s ease;
	}
	.sd-compact .sd-tile {
		padding: 8px 12px;
	}
	.sd-on {
		opacity: 1;
	}
	.sd-tile-text {
		display: flex;
		flex: 1;
		flex-direction: column;
		min-width: 0;
	}
	.sd-tile-name,
	.sd-tile-state {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.sd-tile-name {
		font-size: 14px;
		font-weight: 700;
	}
	.sd-tile-state {
		color: var(--color-fc-fg-muted);
		font-size: 12px;
		opacity: 0;
		transition: opacity 0.3s ease;
	}
	.sd-on .sd-tile-state {
		opacity: 1;
	}
	.sd-check {
		display: flex;
		align-items: center;
		gap: 6px;
		color: var(--color-fc-success);
		opacity: 0;
		scale: 0.6;
		transition:
			opacity 0.25s ease,
			scale 0.25s ease;
	}
	.sd-on .sd-check {
		opacity: 1;
		scale: 1;
	}
	@media (prefers-reduced-motion: reduce) {
		.sd-login,
		.sd-account,
		.sd-tile,
		.sd-tile-state,
		.sd-check {
			transition: none;
		}
	}
</style>
