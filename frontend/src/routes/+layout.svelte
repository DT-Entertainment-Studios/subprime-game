<script lang="ts">
	import '../app.css';
	import '@fontsource/dejavu-sans';
	import favicon from '#lib/assets/favicon.svg';
	import { preferences } from '#lib/prefs.svelte';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();

	preferences.apply();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>Subprime</title>
	<meta name="description" content="Subprime – das Brettspiel für Immobilienhaie" />
</svelte:head>

<div class="app-shell" class:animate-off={!preferences.current.animations}>
	{@render children()}
</div>
