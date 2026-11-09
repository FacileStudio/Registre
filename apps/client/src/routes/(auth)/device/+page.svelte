<script lang="ts">
	import { t } from '$lib/i18n/i18n.svelte';
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { replaceState } from '$app/navigation';
	import { Spinner } from '@facile/muse';
	import DeviceCodeForm from '$lib/components/DeviceCodeForm.svelte';
	import DeviceConfirm from '$lib/components/DeviceConfirm.svelte';
	import DeviceOutcomeView from '$lib/components/DeviceOutcome.svelte';
	import { account, type AccountView } from '$lib/account';
	import {
		device,
		isSignedOut,
		needsConsent,
		CONSENT_PATH,
		outcomeOf,
		stash,
		unstash,
		type DeviceOutcome,
		type DeviceRequestView
	} from '$lib/device';
	import { normalise } from '$lib/usercode';

	type Stage = 'entry' | 'confirm' | 'outcome' | 'signed-out';

	const primaryButtonClass =
		'inline-flex h-11 w-full items-center justify-center rounded-md bg-primary px-4 ' +
		'text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 ' +
		'disabled:pointer-events-none disabled:opacity-50';
	const outlineButtonClass =
		'inline-flex h-11 w-full items-center justify-center rounded-md ' +
		'bg-muted px-4 text-sm font-medium text-foreground transition-colors ' +
		'hover:bg-muted/80 disabled:pointer-events-none ' +
		'disabled:opacity-50';

	let stage = $state<Stage>('entry');
	let code = $state('');
	let prefilled = $state(false);
	let request = $state<DeviceRequestView | null>(null);
	let outcome = $state<DeviceOutcome>('unusable');
	let error = $state('');
	let busy = $state(false);
	let me = $state<AccountView | null>(null);

	/**
	 * answered holds the codes this tab has already decided, in canonical form.
	 *
	 * Canonical is what makes it a set rather than a list of spellings: the same
	 * code arriving from the link, from storage and from the keyboard would
	 * otherwise be three different strings, and the second answer would be sent
	 * to an API that has already spent the flow.
	 */
	const answered = new Set<string>();

	onMount(() => {
		account.me().then((a) => (me = a)).catch(() => {});
		const fromLink = page.url.searchParams.get('user_code') ?? '';
		code = normalise(fromLink || unstash());
		prefilled = code.length > 0;
		if (fromLink) {
			const scrubbed = new URL(page.url);
			scrubbed.searchParams.delete('user_code');
			replaceState(scrubbed, page.state);
		}
		if (code.length > 0) {
			look();
		}
	});

	function settle(reached: DeviceOutcome): void {
		outcome = reached;
		stage = 'outcome';
	}

	function fail(cause: unknown): void {
		if (isSignedOut(cause)) {
			stash(code);
			window.location.assign(device.signInPath);
			return;
		}
		if (needsConsent(cause)) {
			stash(code);
			window.location.assign(CONSENT_PATH);
			return;
		}
		const named = outcomeOf(cause);
		if (named) {
			settle(named);
			return;
		}
		error = cause instanceof Error ? cause.message : '';
	}

	async function run(step: () => Promise<void>): Promise<void> {
		if (busy) return;
		busy = true;
		error = '';
		try {
			await step();
		} catch (cause) {
			fail(cause);
		} finally {
			busy = false;
		}
	}

	/**
	 * look canonicalises the code before it does anything else with it, so the
	 * string this page shows, remembers, stashes and sends is one string.
	 *
	 * The form normalises as the human types, but this is also the entry point
	 * for a code out of the URL and out of sessionStorage, neither of which the
	 * form has touched. Canonicalising in one place here is what keeps the code
	 * on the confirmation screen identical to the code in the request.
	 */
	function look() {
		code = normalise(code);
		if (answered.has(code)) {
			settle('used');
			return;
		}
		run(async () => {
			request = await device.pending(code);
			stage = 'confirm';
		});
	}

	function answer(post: (userCode: string) => Promise<unknown>, reached: DeviceOutcome) {
		run(async () => {
			await post(code);
			answered.add(code);
			settle(reached);
		});
	}

	function retry() {
		code = unstash() || code;
		look();
	}

	function again() {
		code = '';
		request = null;
		prefilled = false;
		error = '';
		stage = 'entry';
	}
</script>

<svelte:head>
	<title>{t('device.title')} — Facile</title>
</svelte:head>

{#key stage}
	<div class="stage">
		{#if stage === 'entry'}
			{#if busy && prefilled}
				<div class="flex flex-col items-center justify-center py-10">
					<Spinner size="lg" />
				</div>
			{:else}
				<DeviceCodeForm bind:code {busy} {error} {prefilled} submit={look} />
			{/if}
		{:else if stage === 'confirm' && request}
			<DeviceConfirm
				{request}
				{code}
				{busy}
				{error}
				{me}
				approve={() => answer(device.approve, 'approved')}
				deny={() => answer(device.deny, 'denied')}
			/>
		{:else if stage === 'outcome'}
			<DeviceOutcomeView {outcome} clientName={request?.client_name} {again} />
		{:else}
			<div class="mb-6 text-center">
				<h1 class="font-heading text-2xl font-bold tracking-tight text-foreground">
					{t('device.signInFirst')}
				</h1>
				<p class="mt-1.5 text-sm text-muted-foreground">
					{t('device.signInFirstBody')}
				</p>
			</div>
			<a href={device.signInPath} class={primaryButtonClass}>{t('login.submit')}</a>
			<button
				type="button"
				onclick={retry}
				disabled={busy}
				aria-busy={busy}
				class="{outlineButtonClass} mt-3"
			>
				{busy ? t('common.checking') : t('device.already')}
			</button>
		{/if}
	</div>
{/key}

<style>
	.stage {
		animation: stage-in 0.25s var(--ease-fc) both;
	}

	@keyframes stage-in {
		from {
			opacity: 0;
			transform: translateY(8px);
		}
		to {
			opacity: 1;
			transform: none;
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.stage {
			animation: none;
		}
	}
</style>
