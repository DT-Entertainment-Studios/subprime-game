<script lang="ts">
	import type { GameState } from '#lib/types';
	import { formatMoney, tokenColor } from '#lib/format';
	import { eliminationInfo } from '#lib/bankruptcy';
	import { rankedStandings } from '#lib/placement';

	let { game }: { game: GameState } = $props();

	const standings = $derived(rankedStandings(game.players));
	const winner = $derived(game.winner >= 0 ? game.players[game.winner] : null);
	let hoverRent = $state<number | null>(null);
	let focusRent = $state<number | null>(null);
	let openRent = $derived(hoverRent ?? focusRent);
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
					<th>Platz</th>
					<th>Spieler</th>
					<th>Ausscheiden</th>
					<th>Vermögen</th>
					<th>Miete erhalten</th>
					<th>Miete gezahlt</th>
					<th>Käufe</th>
					<th>Häuser</th>
					<th>Handel</th>
				</tr>
			</thead>
			<tbody>
				{#each standings as entry (entry.player.id)}
					{@const player = entry.player}
					{@const info = player.bankrupt ? eliminationInfo(game.events, player.seat) : undefined}
					<tr class:bankrupt={player.bankrupt}>
						<td>{entry.place}</td>
						<td>
							<span class="dot" style="background: {tokenColor(player.color)}"></span>
							{player.name}
						</td>
						<td class="reason" title={info?.tooltip ?? ''}>
							{player.bankrupt ? info!.label : '–'}
						</td>
						<td>{formatMoney(player.netWorth)}</td>
						<td>{formatMoney(player.stats.rentCollected)}</td>
						<td>
							<span class="rent-wrap">
								<button
									type="button"
									class="rent-cell"
									aria-describedby={openRent === player.seat ? `rent-tip-${player.seat}` : undefined}
									onmouseenter={() => (hoverRent = player.seat)}
									onmouseleave={() => (hoverRent = null)}
									onfocusin={() => (focusRent = player.seat)}
									onfocusout={() => (focusRent = null)}
								>
									{formatMoney(player.stats.rentPaid)}
								</button>
								{#if openRent === player.seat}
									<span class="rent-tooltip" role="tooltip" id={`rent-tip-${player.seat}`}>
										<ul class="rent-list">
											{#each game.players.filter((o) => o.seat !== player.seat) as other (other.seat)}
												<li>an {other.name}: {formatMoney(player.stats.rentPaidTo?.[other.seat] ?? 0)}</li>
											{/each}
										</ul>
									</span>
								{/if}
							</span>
						</td>
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

	th:nth-child(2),
	th:nth-child(3),
	td:nth-child(2),
	td.reason {
		text-align: left;
	}

	/* Keep bankrupt rows muted without dimming the tooltip (a row descendant). */
	tr.bankrupt {
		color: var(--md-sys-color-on-surface-variant);
	}

	tr.bankrupt .dot {
		opacity: 0.5;
	}

	.dot {
		display: inline-block;
		width: 0.8rem;
		height: 0.8rem;
		border-radius: 50%;
		margin-right: 0.3rem;
	}

	.rent-wrap {
		position: relative;
		display: inline-block;
	}

	.rent-cell {
		background: none;
		border: 0;
		padding: 0;
		color: inherit;
		font: inherit;
		text-align: right;
		cursor: help;
	}

	.rent-tooltip {
		position: absolute;
		top: 100%;
		right: 0;
		z-index: 1;
		min-width: max-content;
		white-space: nowrap;
		padding: 0.5rem 0.75rem;
		border-radius: var(--md-sys-shape-corner-small);
		background: var(--md-sys-color-inverse-surface);
		color: var(--md-sys-color-inverse-on-surface);
		box-shadow: 0 var(--md-sys-elevation-2) calc(var(--md-sys-elevation-3) * 2)
			color-mix(in srgb, var(--md-sys-color-shadow) 32%, transparent);
	}

	.rent-list {
		list-style: none;
		margin: 0;
		padding: 0;
		text-align: left;
	}

	.cta {
		margin-top: 1rem;
	}
</style>
