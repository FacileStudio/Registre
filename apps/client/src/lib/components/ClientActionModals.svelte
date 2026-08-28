<script lang="ts">
	import { ConfirmModal, Modal } from '@facile/muse';
	import type { ClientView } from '$lib/admin';
	import ClientSecretPanel from '$lib/components/ClientSecretPanel.svelte';

	interface Props {
		rotateTarget?: ClientView | null;
		rotateOpen?: boolean;
		rotatedOpen?: boolean;
		rotatedSecret?: string;
		rotatedID?: string;
		onRotate: () => void;
		onForgetRotated: () => void;
		disableTarget?: ClientView | null;
		disableOpen?: boolean;
		onDisable: () => void;
		enableTarget?: ClientView | null;
		enableOpen?: boolean;
		onEnable: () => void;
		deleteTarget?: ClientView | null;
		deleteOpen?: boolean;
		onDelete: () => void;
		kindLabel?: string;
	}

	let {
		rotateTarget = $bindable(null),
		rotateOpen = $bindable(false),
		rotatedOpen = $bindable(false),
		rotatedSecret = $bindable(''),
		rotatedID = $bindable(''),
		onRotate,
		onForgetRotated,
		disableTarget = $bindable(null),
		disableOpen = $bindable(false),
		onDisable,
		enableTarget = $bindable(null),
		enableOpen = $bindable(false),
		onEnable,
		deleteTarget = $bindable(null),
		deleteOpen = $bindable(false),
		onDelete,
		kindLabel = 'client'
	}: Props = $props();
</script>

<ConfirmModal
	bind:open={rotateOpen}
	tone="danger"
	title="Rotate secret for {rotateTarget?.name ?? ''}?"
	description="The current secret stops working immediately. Any caller using the old secret will fail to authenticate."
	confirmLabel="Rotate the secret"
	cancelLabel="Cancel"
	onConfirm={onRotate}
	onCancel={() => (rotateTarget = null)}
/>

<Modal
	bind:open={rotatedOpen}
	title="New secret for {rotatedID}"
	dismissible={false}
	size="sm"
	onClose={onForgetRotated}
>
	<ClientSecretPanel
		secret={rotatedSecret}
		actionLabel="Done"
		onAction={() => (rotatedOpen = false)}
	>
		The previous secret has been invalidated. Copy and deploy this secret immediately.
	</ClientSecretPanel>
</Modal>

<ConfirmModal
	bind:open={disableOpen}
	tone="danger"
	title="Disable {disableTarget?.name ?? ''}?"
	description="This {kindLabel} will immediately stop being able to authenticate or exchange tokens."
	confirmLabel="Disable {kindLabel}"
	cancelLabel="Keep active"
	onConfirm={onDisable}
	onCancel={() => (disableTarget = null)}
/>

<ConfirmModal
	bind:open={enableOpen}
	title="Enable {enableTarget?.name ?? ''}?"
	description="This {kindLabel} will be re-activated and allowed to authenticate."
	confirmLabel="Enable {kindLabel}"
	cancelLabel="Leave disabled"
	onConfirm={onEnable}
	onCancel={() => (enableTarget = null)}
/>

<ConfirmModal
	bind:open={deleteOpen}
	tone="danger"
	title="Delete {deleteTarget?.name ?? ''}?"
	description="This {kindLabel} registration will be permanently deleted."
	confirmLabel="Delete {kindLabel}"
	cancelLabel="Cancel"
	onConfirm={onDelete}
	onCancel={() => (deleteTarget = null)}
/>
