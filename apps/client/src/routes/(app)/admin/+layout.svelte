<script lang="ts">
	import { page } from '$app/state';
	import {
		Alert,
		Button,
		Card,
		Divider,
		Field,
		Input,
		Page,
		PageHeader,
		PageTransition,
		Stack,
		Tabs,
		icons
	} from '@facile/muse';
	import {
		admin,
		clearOperator,
		errorMessage,
		hasOperator,
		operatorEmail,
		setOperator
	} from '$lib/admin';

	let { children } = $props();

	let email = $state('');
	let password = $state('');
	let signedIn = $state(hasOperator());
	let busy = $state(false);
	let error = $state('');

	/*
	 * The credential is proved by using it: /api/admin has no session endpoint,
	 * so the cheapest read doubles as the check, and a refusal clears what was
	 * just held rather than leaving a bad Basic header armed for the next call.
	 */
	async function signIn(event: SubmitEvent) {
		event.preventDefault();
		if (busy) return;
		busy = true;
		error = '';
		setOperator(email, password);
		try {
			await admin.listUsers();
			password = '';
			signedIn = true;
		} catch (cause) {
			clearOperator();
			error = errorMessage(cause, 'Wrong operator email or password.');
		} finally {
			busy = false;
		}
	}

	function signOut() {
		clearOperator();
		signedIn = false;
		email = '';
		password = '';
	}

	const sections = [
		{ id: 'users', label: 'Users', icon: icons.usersGroup, href: '/admin' },
		{ id: 'invitations', label: 'Invitations', icon: icons.mail, href: '/admin/invitations' },
		{ id: 'clients', label: 'Clients', icon: icons.plug, href: '/admin/clients' },
		{ id: 'keys', label: 'Signing keys', icon: icons.key, href: '/admin/keys' }
	];

	const active = $derived(
		sections.find((s) => s.href !== '/admin' && page.url.pathname.startsWith(s.href))?.id ?? 'users'
	);
</script>

<svelte:head>
	<title>Administration — Registre</title>
</svelte:head>

{#if !signedIn}
	<Page width="sm">
		<Card class="p-8">
			<Stack>
				<div class="flex flex-col gap-1">
					<h1 class="text-fc-lg font-semibold text-fc-fg">Operator sign-in</h1>
					<p class="text-fc-sm text-fc-fg-muted">
						Administration answers only to an operator account. Your credential is held in this
						tab's memory and is gone when you reload.
					</p>
				</div>
				<form class="flex flex-col gap-4" onsubmit={signIn}>
					<div aria-live="polite">
						{#if error}
							<Alert tone="danger" title={error} />
						{/if}
					</div>
					<Field label="Email">
						<Input type="email" autocomplete="username" required bind:value={email} />
					</Field>
					<Field label="Password">
						<Input
							type="password"
							autocomplete="current-password"
							required
							bind:value={password}
						/>
					</Field>
					<Button type="submit" disabled={busy}>
						{busy ? 'Checking…' : 'Sign in'}
					</Button>
				</form>
			</Stack>
		</Card>
	</Page>
{:else}
	<Page width="xl">
		<PageHeader
			title="Administration"
			description="Signed in as {operatorEmail()}. Secrets and tokens are shown once, so copy them when they appear."
		>
			{#snippet actions()}
				<Button variant="ghost" icon={icons.logout} onclick={signOut}>Sign out</Button>
			{/snippet}
		</PageHeader>

		<Stack>
			<div class="flex flex-col gap-4">
				<Tabs items={sections} value={active} label="Administration sections" />
				<Divider />
			</div>

			<PageTransition key={active} distance={8} duration={0.25} class="flex flex-col gap-10">
				{@render children()}
			</PageTransition>
		</Stack>
	</Page>
{/if}
