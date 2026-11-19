<script lang="ts">
	import type { GameState, Intent, Player } from '#lib/types';
	import { formatMoney, secondsUntil, turnPhaseLabel } from '#lib/format';
	import BoardTile from './BoardTile.svelte';
	import TileCard from './TileCard.svelte';
	import RoadCars from './RoadCars.svelte';
	import { EDGE_WIDTH, MARK_WIDTH, ROAD_PATHS, ROAD_WIDTH } from '#lib/road';

	let {
		game,
		isMyTurn,
		isDebtor,
		debtor = null,
		me = null,
		onIntent
	}: {
		game: GameState;
		isMyTurn: boolean;
		isDebtor: boolean;
		debtor?: Player | null;
		me?: Player | null;
		onIntent: (intent: Intent) => void;
	} = $props();

	let now = $state(Date.now());
	let bidAmount = $state(0);
	/** Board index of the tile whose detail card is open (click on a field). */
	let selectedTile = $state<number | null>(null);
	/** Two-step confirmation for forfeiting while in debt. */
	let confirmGiveUp = $state(false);

	$effect(() => {
		const timer = setInterval(() => (now = Date.now()), 500);
		return () => clearInterval(timer);
	});

	const current = $derived(game.players[game.current]);
	const occupants = $derived.by(() => {
		const map = new Map<number, Player[]>();
		for (const player of game.players) {
			if (player.bankrupt) continue;
			const list = map.get(player.position) ?? [];
			list.push(player);
			map.set(player.position, list);
		}
		return map;
	});
	const auctionTile = $derived(game.auction ? game.board[game.auction.tile] : undefined);
	const pendingTile = $derived(game.pending ? game.board[game.pending.tile] : undefined);
	const auctionSeconds = $derived(game.auction ? secondsUntil(game.auction.endsAt, now) : 0);
	const bidStep = $derived(game.auction && game.auction.bid > 0 ? game.auction.bid + 20 : game.auction?.minBid ?? 0);
	/** The bid currently selected in the stepper; 0 means "use the minimum". */
	const bidValue = $derived(bidAmount > bidStep ? bidAmount : bidStep);
	const canBid = $derived(
		game.auction !== undefined &&
			me !== null &&
			!me.bankrupt &&
			game.turnPhase !== 'awaiting_debt' &&
			game.auction.bidder !== me.seat &&
			(me.liquidationValue ?? me.cash) >= bidStep
	);
	const debtAmount = $derived(debtor && debtor.cash < 0 ? -debtor.cash : 0);
	const pendingShortfall = $derived(
		game.pending && me ? Math.max(0, game.pending.price - me.cash) : 0
	);

	$effect(() => {
		if (game.turnPhase !== 'awaiting_debt' || !isDebtor) confirmGiveUp = false;
	});

	// Keep the selected bid valid: reset between auctions and follow the
	// minimum after someone else raises the current bid.
	$effect(() => {
		if (!game.auction) {
			bidAmount = 0;
		} else if (bidAmount !== 0 && bidAmount < bidStep) {
			bidAmount = bidStep;
		}
	});

	function placeBid() {
		onIntent({ type: 'bid', amount: bidValue });
		bidAmount = 0;
	}

	function giveUp() {
		confirmGiveUp = false;
		onIntent({ type: 'give_up' });
	}
</script>

<div class="board" aria-label="Spielbrett">
	{#each game.board as tile (tile.index)}
		<BoardTile
			{tile}
			state={game.tiles[tile.index]}
			owner={game.tiles[tile.index].owner >= 0 ? game.players[game.tiles[tile.index].owner] : undefined}
			occupants={occupants.get(tile.index) ?? []}
			losSalary={game.rules.losSalary}
			onselect={() => (selectedTile = tile.index)}
		/>
	{/each}

	{#if selectedTile !== null}
		<TileCard
			tile={game.board[selectedTile]}
			tileState={game.tiles[selectedTile]}
			game={game}
			owner={
				game.tiles[selectedTile].owner >= 0
					? game.players[game.tiles[selectedTile].owner]
					: undefined
			}
			occupants={occupants.get(selectedTile) ?? []}
			onclose={() => (selectedTile = null)}
		/>
	{/if}

	<div class="center" style="--rw: {ROAD_WIDTH}%">
		<svg class="road" viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden="true">
			<path class="asphalt" d={ROAD_PATHS.area} fill-rule="evenodd" />
			<path class="edge" d={ROAD_PATHS.outer} style="stroke-width: {EDGE_WIDTH}" />
			<path class="edge" d={ROAD_PATHS.inner} style="stroke-width: {EDGE_WIDTH}" />
			<path class="mark" d={ROAD_PATHS.ring} style="stroke-width: {MARK_WIDTH}" />
			<path class="mark" d={ROAD_PATHS.spurs} style="stroke-width: {MARK_WIDTH}" />
			<!-- Painted after the road lines: the lot surface cleanly covers the
			     kerb lines where the driveway runs into it. -->
			<path class="asphalt" d={ROAD_PATHS.parking} />
			<path class="bay" d={ROAD_PATHS.parkingBays} style="stroke-width: {EDGE_WIDTH}" />
		</svg>
		<RoadCars players={game.players} currentSeat={game.current} events={game.events} />
		<div class="panel">
			<div class="turn">
				<strong>{current?.name ?? '–'}</strong>
				<span class="muted">{turnPhaseLabel(game.turnPhase)}</span>
				{#if game.doubles > 0}
					<span class="badge">Pasch {game.doubles}</span>
				{/if}
			</div>

			{#if game.turnPhase !== 'auction'}
				<div class="dice" aria-label="Würfel">
					<span class="die">{game.dice[0] || '–'}</span>
					<span class="die">{game.dice[1] || '–'}</span>
				</div>
			{/if}

			{#if game.pending && pendingTile}
				<div class="decision">
					<p>
						<strong>{pendingTile.name}</strong>
						{#if game.pending.doubled}
							<span class="badge">Doppelter Preis</span>
						{/if}
					</p>
					<p class="muted">
						{formatMoney(game.pending.price)}
					</p>
					{#if isMyTurn}
						{#if pendingShortfall > 0}
							<p class="warning">
								Dir fehlen {formatMoney(pendingShortfall)}. Hypothekiere oder verkaufe Felder unter
								„Bauen und Verwalten“ – oder starte die Auktion.
							</p>
						{/if}
						<div class="row">
							<button class="plain" onclick={() => onIntent({ type: 'decline' })}>Auktion starten</button>
							<button
								class="cta"
								disabled={pendingShortfall > 0}
								title={pendingShortfall > 0 ? 'Du kannst dieses Feld gerade nicht bezahlen.' : undefined}
								onclick={() => onIntent({ type: 'buy' })}
							>
								Kaufen
							</button>
						</div>
					{:else}
						<p class="muted">Warte auf {current?.name} …</p>
					{/if}
				</div>
			{/if}

			{#if game.auction && auctionTile}
				<div class="decision">
					<p>
						Auktion: <strong>{auctionTile.name}</strong>
					</p>
					<p class="muted">
						{game.auction.bid > 0
							? `${formatMoney(game.auction.bid)} von ${game.players[game.auction.bidder]?.name ?? '–'}`
							: `Mindestgebot ${formatMoney(game.auction.minBid)}`}
						· {auctionSeconds}s
					</p>
					{#if canBid}
						<div class="bid-controls">
							<div class="stepper">
								<button
									class="step"
									type="button"
									aria-label="Gebot um 20 senken"
									disabled={bidValue <= bidStep}
									onclick={() => (bidAmount = bidValue - 20)}
								>
									−20
								</button>
								<div class="bid-amount" aria-live="polite">
									<span class="label">Dein Gebot</span>
									<strong>{formatMoney(bidValue)}</strong>
								</div>
								<button
									class="step"
									type="button"
									aria-label="Gebot um 20 erhöhen"
									disabled={me !== null && bidValue + 20 > (me.liquidationValue ?? me.cash)}
									onclick={() => (bidAmount = bidValue + 20)}
								>
									+20
								</button>
							</div>
							<button class="cta bid-submit" type="button" onclick={placeBid}>Bieten</button>
						</div>
						<p class="muted">
							Gebote dürfen über dein Bargeld hinausgehen: Verkauf und Hypotheken zählen mit.
						</p>
					{:else}
						<p class="muted">Du kannst gerade nicht bieten.</p>
					{/if}
				</div>
			{/if}

			{#if game.turnPhase === 'awaiting_jail' && isMyTurn && me}
				<div class="decision">
					<p>Dein Auto ist in der Werkstatt (Versuch {me.jailAttempts} von 3).</p>
					{#if me.cash < game.rules.jailFine}
						<p class="warning">
							Dir fehlen {formatMoney(game.rules.jailFine - me.cash)} für die Reparatur.
							Hypothekiere oder verkaufe Felder unter „Bauen und Verwalten“.
						</p>
					{/if}
					<div class="row">
						<button
							class="plain"
							disabled={me.cash < game.rules.jailFine}
							onclick={() => onIntent({ type: 'jail_option', option: 'pay' })}
						>
							Reparatur {formatMoney(game.rules.jailFine)}
						</button>
						<button
							class="plain"
							disabled={me.jailCards === 0}
							onclick={() => onIntent({ type: 'jail_option', option: 'card' })}
						>
							Gutschein einlösen
						</button>
						<button class="cta" onclick={() => onIntent({ type: 'jail_option', option: 'roll' })}>
							Startversuch (Pasch)
						</button>
					</div>
				</div>
			{/if}

			{#if game.turnPhase === 'awaiting_debt'}
				<div class="decision debt">
					{#if isDebtor && debtor}
						<p>
							<strong>Du hast {formatMoney(debtAmount)} Schulden.</strong>
						</p>
						<p class="muted">
							Hypothekiere oder verkaufe Felder, bis dein Geldbestand nicht mehr negativ ist.
							Bauen ist währenddessen nicht möglich.
						</p>
						{#if confirmGiveUp}
							<p class="warning">
								Wirklich aufgeben? Du scheidest endgültig aus dem Spiel aus und verlierst.
							</p>
							<div class="row">
								<button class="plain" onclick={() => (confirmGiveUp = false)}>Abbrechen</button>
								<button class="primary danger" onclick={giveUp}>Ja, aufgeben</button>
							</div>
						{:else}
							<button class="plain danger" onclick={() => (confirmGiveUp = true)}>Aufgeben</button>
						{/if}
					{:else}
						<p class="muted">{debtor?.name ?? current?.name ?? 'Ein Spieler'} muss Schulden begleichen …</p>
						<p class="muted">Das Spiel ist blockiert, bis die Schulden beglichen sind.</p>
					{/if}
				</div>
			{/if}

			{#if game.turnPhase === 'awaiting_roll' && isMyTurn}
				<div class="decision">
					<button class="cta" onclick={() => onIntent({ type: 'roll' })}>Würfeln</button>
					<p class="muted">Du kannst vorher bauen und handeln.</p>
				</div>
			{/if}

			{#if !isMyTurn && game.turnPhase === 'awaiting_roll'}
				<p class="muted waiting">{current?.name} ist am Zug.</p>
			{/if}
		</div>
	</div>
</div>

<style>
	.board {
		display: grid;
		/* Thicker edge tracks: the fields grow while the middle shrinks. */
		grid-template-columns: 1.5fr repeat(9, minmax(0, 1fr)) 1.5fr;
		grid-template-rows: 1.5fr repeat(9, minmax(0, 1fr)) 1.5fr;
		gap: 2px;
		aspect-ratio: 1;
		width: 100%;
		min-width: 560px;
		max-width: min(calc(100vh - 5.5rem), 1320px);
		margin: 0 auto;
		padding: 6px;
		background: var(--md-sys-color-surface-container-low);
		border: 1px solid var(--md-sys-color-outline-variant);
		border-radius: var(--md-sys-shape-corner-large);
		container-type: inline-size;
	}

	/* The dark table keeps a hero surface (RAL 7016 is intentionally allowed
	   here); light mode stays a near-white table. */
	:global(:root[data-theme='dark']) .board {
		background: var(--md-sys-color-surface-container-highest);
	}

	.center {
		--road-asphalt: #2d3137;
		--road-edge: rgba(255, 255, 255, 0.52);
		--road-mark: rgba(255, 255, 255, 0.4);
		grid-area: 2 / 2 / 11 / 11;
		position: relative;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: calc(var(--rw) + 0.5rem);
		overflow: visible;
		text-align: center;
	}

	/* The road is a fixed ring around the middle panel. At the corners it runs
	   into the corner fields, so a car that ends its move there turns off. */
	.road {
		position: absolute;
		inset: 0;
		width: 100%;
		height: 100%;
		overflow: visible;
		pointer-events: none;
	}

	.road .asphalt {
		fill: var(--road-asphalt);
	}

	.road .edge {
		fill: none;
		stroke: var(--road-edge);
		stroke-linejoin: round;
		stroke-linecap: round;
	}

	.road .mark {
		fill: none;
		stroke: var(--road-mark);
		stroke-dasharray: 1.7 1.5;
		stroke-linecap: butt;
	}

	/* Parking bays are solid white lines, like painted markings on a real lot. */
	.road .bay {
		fill: none;
		stroke: var(--road-edge);
		stroke-linecap: butt;
	}

	:global(:root[data-theme='dark']) .center {
		--road-asphalt: #24282e;
		--road-edge: rgba(255, 255, 255, 0.62);
		--road-mark: rgba(255, 255, 255, 0.46);
	}

	.panel {
		position: relative;
		z-index: 2;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.75rem;
		max-width: min(32rem, 100%);
		max-height: 100%;
		overflow: auto;
		padding: 1rem 1.25rem;
		background: var(--md-sys-color-surface);
		border: 1px solid var(--md-sys-color-outline-variant);
		border-radius: var(--md-sys-shape-corner-large);
		box-shadow: 0 var(--md-sys-elevation-2) calc(var(--md-sys-elevation-5))
			color-mix(in srgb, var(--md-sys-color-shadow) 22%, transparent);
	}

	.turn {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		flex-wrap: wrap;
		justify-content: center;
	}

	.dice {
		display: flex;
		gap: 0.5rem;
	}

	.die {
		width: 2.9rem;
		height: 2.9rem;
		border-radius: var(--md-sys-shape-corner-small);
		background: var(--md-sys-color-surface-container);
		border: 1px solid var(--md-sys-color-outline-variant);
		display: grid;
		place-items: center;
		font-size: 1.4rem;
		font-weight: 700;
	}

	.decision {
		background: var(--md-sys-color-surface-container);
		border: 1px solid var(--md-sys-color-outline-variant);
		border-radius: var(--md-sys-shape-corner-medium);
		padding: 0.75rem 1rem;
		width: 100%;
		max-width: 26rem;
	}

	.decision.debt {
		border-color: var(--md-sys-color-error);
		background: var(--md-sys-color-error-container);
		color: var(--md-sys-color-on-error-container);
	}

	.decision p {
		margin: 0 0 0.35rem;
	}

	.decision .warning {
		color: var(--cd-warning);
		font-weight: 600;
	}

	.bid-controls {
		display: flex;
		align-items: stretch;
		gap: 0.5rem;
		flex-wrap: wrap;
	}

	.stepper {
		display: grid;
		grid-template-columns: auto minmax(5.5rem, 1fr) auto;
		gap: 0.4rem;
		flex: 1 1 14rem;
	}

	.step {
		min-width: 3.25rem;
		min-height: 3rem;
		padding: 0 0.5rem;
		font-size: 1.2rem;
		font-weight: 700;
		background: var(--md-sys-color-surface);
		color: var(--md-sys-color-on-surface);
		border: 1px solid var(--md-sys-color-outline);
		border-radius: var(--md-sys-shape-corner-medium);
		cursor: pointer;
	}

	.step:hover:not(:disabled) {
		background: var(--md-sys-color-surface-container-high);
	}

	.step:disabled {
		opacity: 0.45;
		cursor: not-allowed;
	}

	.bid-amount {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: 0.25rem 0.6rem;
		background: var(--md-sys-color-surface);
		border: 1px solid var(--md-sys-color-outline);
		border-radius: var(--md-sys-shape-corner-medium);
	}

	.bid-amount .label {
		font-size: 0.7rem;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--md-sys-color-on-surface-variant);
	}

	.bid-amount strong {
		font-size: 1.1rem;
	}

	.bid-submit {
		min-height: 3rem;
		padding-inline: 1.25rem;
	}

	.danger {
		color: var(--md-sys-color-error);
		border-color: var(--md-sys-color-error);
	}

	.primary.danger {
		background: var(--md-sys-color-error);
		color: var(--md-sys-color-on-error);
	}

	.waiting {
		margin: 0;
	}
</style>
