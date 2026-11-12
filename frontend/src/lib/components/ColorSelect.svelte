<script lang="ts">
	import { colorLabel, tokenColor, TOKEN_COLORS_LIST } from '#lib/format';
	import Dropdown from './Dropdown.svelte';

	let { value = $bindable(), id }: { value?: string; id: string } = $props();

	const options = TOKEN_COLORS_LIST.map((c) => ({
		value: c,
		label: colorLabel(c),
		color: tokenColor(c)
	}));
</script>

<Dropdown {id} {options} bind:value />
