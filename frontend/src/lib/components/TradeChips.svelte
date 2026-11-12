<script lang="ts">
	import type { TradeItem } from '#lib/trade';
	import IconGasStation from '@tabler/icons-svelte/icons/gas-station';
	import IconBolt from '@tabler/icons-svelte/icons/bolt';
	import IconDroplet from '@tabler/icons-svelte/icons/droplet';
	import IconTool from '@tabler/icons-svelte/icons/tool';
	import IconCoins from '@tabler/icons-svelte/icons/coins';

	let { items }: { items: TradeItem[] } = $props();

	const ICONS = {
		gas: IconGasStation,
		bolt: IconBolt,
		droplet: IconDroplet,
		tool: IconTool,
		coin: IconCoins
	} as const;
</script>

{#if items.length === 0}
	<span class="nothing">– nichts –</span>
{:else}
	<ul class="chips">
		{#each items as item, i (item.kind + item.label + i)}
			{@const Icon = item.icon ? ICONS[item.icon] : null}
			<li class="chip" title={item.title}>
				{#if Icon}
					<span class="medal" style="background: {item.color}">
						<Icon stroke={2} />
					</span>
				{:else}
					<span class="swatch" style="background: {item.color}"></span>
				{/if}
				<span class="label">{item.label}</span>
			</li>
		{/each}
	</ul>
{/if}

<style>
	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: 0.25rem;
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.chip {
		display: inline-flex;
		align-items: center;
		gap: 0.3rem;
		font-size: 12px; /* label-medium metrics, tokens.css:175 */
		line-height: 16px;
		padding: 0.15rem 0.45rem 0.15rem 0.2rem;
		border: 1px solid var(--md-sys-color-outline-variant);
		border-radius: var(--md-sys-shape-corner-full);
		background: var(--md-sys-color-surface-container);
	}

	/* Icon entries show a round colour badge, plain tiles a square swatch. */
	.medal {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 1rem;
		height: 1rem;
		border-radius: 50%;
		border: 1px solid var(--md-sys-color-outline-variant);
		color: var(--cd-ral9016);
		flex: 0 0 auto;
	}

	.chip :global(svg) {
		width: 0.72rem;
		height: 0.72rem;
	}

	.swatch {
		display: inline-block;
		width: 0.85rem;
		height: 0.85rem;
		border-radius: var(--md-sys-shape-corner-extra-small);
		border: 1px solid var(--md-sys-color-outline-variant);
		flex: 0 0 auto;
	}

	.nothing {
		font-size: 0.78rem;
		color: var(--md-sys-color-on-surface-variant);
	}
</style>
