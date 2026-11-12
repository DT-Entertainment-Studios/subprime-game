<script lang="ts">
	import { preferences } from '#lib/prefs.svelte';
</script>

<button
	class="plain"
	type="button"
	aria-label="Design wechseln"
	onclick={() => preferences.toggleTheme()}
>
	{preferences.current.theme === 'dark' ? '☀️ Hell' : '🌙 Dunkel'}
</button>
