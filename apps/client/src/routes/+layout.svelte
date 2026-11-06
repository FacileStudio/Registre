<script lang="ts">
	import '../app.css';
	import '$lib/icons';
	import { Toaster } from '@facile/muse';

	let { children } = $props();
</script>

{@render children()}

<!-- One Toaster for the whole app, outside the router, so a navigation cannot
     unmount a toast mid-flight. The bottom padding clears MobileNav. -->
<Toaster class="pb-28 md:pb-6" />
