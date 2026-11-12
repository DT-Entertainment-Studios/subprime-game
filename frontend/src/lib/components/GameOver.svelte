<script lang="ts">
	import type { GameState } from '#lib/types';
	import { formatMoney, tokenColor } from '#lib/format';

	let { game }: { game: GameState } = $props();

	const standings = $derived(
		[...game.players].sort((a, b) => {
			if (a.bankrupt !== b.bankrupt) return a.bankrupt ? 1 : -1;
			return b.netWorth - a.netWorth;
		})
	);
	const winner = $derived(game.winner >= 0 ? game.players[game.winner] : null);
</script>

<div class="over">
	<div class="panel">
		<h2>Spiel beendet</h2>
		{#if winner}
			<p class="winner">
				<span class="dot" style="background: {tokenColor(winner.color)}"></span>
				{winner.name} gewinnt!
			</p>
		{/if}
		<table>
			<thead>
				<tr>
					<th>Spieler</th>
					<th>Vermögen</th>
					<th>Miete erhalten</th>
					<th>Miete gezahlt</th>
					<th>Käufe</th>
					<th>Häuser</th>
					<th>Handel</th>
				</tr>
			</thead>
			<tbody>
				{#each standings as player (player.id)}
					<tr class:bankrupt={player.bankrupt}>
						<td>
							<span class="dot" style="background: {tokenColor(player.color)}"></span>
							{player.name}
						</td>
						<td>{formatMoney(player.netWorth)}</td>
						<td>{formatMoney(player.stats.rentCollected)}</td>
						<td>{formatMoney(player.stats.rentPaid)}</td>
						<td>{player.stats.propertiesBought}</td>
						<td>{player.stats.housesBuilt}</td>
						<td>{player.stats.trades}</td>
					</tr>
				{/each}
			</tbody>
		</table>
		<a class="cta" href="/">Zur Startseite</a>
	</div>
</div>

<style>
	.over {
		position: fixed;
		inset: 0;
		background: color-mix(in srgb, var(--md-sys-color-scrim) 55%, transparent);
		display: grid;
		place-items: center;
		padding: 1rem;
		z-index: 20;
		overflow: auto;
	}

	.panel {
		background: var(--md-sys-color-surface-container);
		border: 1px solid var(--md-sys-color-outline-variant);
		border-radius: var(--md-sys-shape-corner-large);
		padding: 1.25rem;
		max-width: 48rem;
		width: 100%;
		box-shadow: 0 var(--md-sys-elevation-4) calc(var(--md-sys-elevation-5) * 3)
			color-mix(in srgb, var(--md-sys-color-shadow) 32%, transparent);
	}

	.winner {
		font-size: 16px; /* title-medium, tokens.css:169 */
		line-height: 24px;
		font-weight: 500;
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	table {
		width: 100%;
		border-collapse: collapse;
		font-size: 12px; /* body-small, tokens.css:173 */
		line-height: 16px;
	}

	th,
	td {
		text-align: right;
		padding: 0.3rem 0.4rem;
		border-bottom: 1px solid var(--md-sys-color-outline-variant);
	}

	th:first-child,
	td:first-child {
		text-align: left;
	}

	tr.bankrupt {
		opacity: 0.6;
	}

	.dot {
		display: inline-block;
		width: 0.8rem;
		height: 0.8rem;
		border-radius: 50%;
		margin-right: 0.3rem;
	}

	.cta {
		margin-top: 1rem;
	}
</style>
