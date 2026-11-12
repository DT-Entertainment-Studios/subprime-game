<script lang="ts">
	import type { GameState, TradeOffer } from '#lib/types';
	import { tradeSideItems } from '#lib/trade';
	import TradeChips from './TradeChips.svelte';

	let { game, trade }: { game: GameState; trade: TradeOffer } = $props();

	const from = $derived(game.players[trade.from] ?? null);
	const to = $derived(game.players[trade.to] ?? null);
</script>

{#if from && to}
	<div class="details">
		<div class="side">
			<p class="muted">{from.name} gibt</p>
			<TradeChips items={tradeSideItems(game, trade.offerTiles, trade.offerCards, trade.offerCash)} />
		</div>
		<div class="side">
			<p class="muted">{to.name} gibt</p>
			<TradeChips items={tradeSideItems(game, trade.requestTiles, trade.requestCards, trade.requestCash)} />
		</div>
	</div>
{/if}

<style>
	.details {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 0.5rem;
		margin: 0.35rem 0 0.25rem;
	}

	.side p {
		margin: 0 0 0.25rem;
		font-size: 11px; /* label-small metrics, tokens.css:176 */
		line-height: 16px;
	}
</style>
