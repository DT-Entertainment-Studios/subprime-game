<script lang="ts">
	import type { BoardTile, GameState, Intent, Player } from '#lib/types';
	import {
		formatMoney,
		groupColor,
		groupLabel,
		mortgageValue,
		STREET_GROUPS,
		unmortgageCost
	} from '#lib/format';
	import IconGasStation from '@tabler/icons-svelte/icons/gas-station';
	import IconBolt from '@tabler/icons-svelte/icons/bolt';
	import IconDroplet from '@tabler/icons-svelte/icons/droplet';

	let {
		game,
		me,
		isMyTurn,
		onIntent
	}: {
		game: GameState;
		me: Player;
		isMyTurn: boolean;
		onIntent: (intent: Intent) => void;
	} = $props();

	interface Section {
		key: string;
		label: string;
		color: string;
		icon: typeof IconGasStation | null;
		total: number;
		tiles: BoardTile[];
	}

	/** Field index awaiting the bank-sale confirmation popup. */
	let pendingSale = $state<number | null>(null);
	let dialogEl = $state<HTMLElement>();

	const owned = $derived(game.board.filter((tile) => game.tiles[tile.index].owner === me.seat));
	const pendingTile = $derived(pendingSale === null ? null : game.board[pendingSale]);

	// Drop the popup as soon as the sale is no longer possible (turn change,
	// houses built, field traded away).
	$effect(() => {
		if (pendingSale === null) return;
		const state = game.tiles[pendingSale];
		if (!state || state.owner !== me.seat || !canSellToBank(pendingSale)) pendingSale = null;
	});

	// Focus the popup and lock the page behind it, like the other dialogs.
	$effect(() => {
		if (pendingSale === null) return;
		dialogEl?.focus();
		document.body.style.overflow = 'hidden';
		return () => {
			document.body.style.overflow = '';
		};
	});

	function closeSale() {
		pendingSale = null;
	}

	function confirmSale() {
		if (pendingSale === null) return;
		onIntent({ type: 'sell_to_bank', tile: pendingSale });
		pendingSale = null;
	}

	function backdrop(event: MouseEvent) {
		if (event.target === event.currentTarget) closeSale();
	}

	const sections = $derived.by(() => {
		const result: Section[] = [];
		const byIndex = (a: BoardTile, b: BoardTile) => a.index - b.index;

		const tankstellen = owned.filter((tile) => tile.type === 'tankstelle').sort(byIndex);
		if (tankstellen.length > 0) {
			result.push({
				key: 'tankstelle',
				label: groupLabel('tankstelle'),
				color: groupColor('tankstelle'),
				icon: IconGasStation,
				total: game.board.filter((tile) => tile.type === 'tankstelle').length,
				tiles: tankstellen
			});
		}

		const utilities = owned.filter((tile) => tile.type === 'utility').sort(byIndex);
		if (utilities.length > 0) {
			result.push({
				key: 'versorger',
				label: groupLabel('versorger'),
				color: groupColor('versorger'),
				icon: IconBolt,
				total: game.board.filter((tile) => tile.type === 'utility').length,
				tiles: utilities
			});
		}

		for (const group of STREET_GROUPS) {
			const tiles = owned.filter((tile) => tile.group === group).sort(byIndex);
			if (tiles.length === 0) continue;
			result.push({
				key: group,
				label: groupLabel(group),
				color: groupColor(group),
				icon: null,
				total: game.board.filter((tile) => tile.group === group).length,
				tiles
			});
		}

		return result;
	});

	function utilityIcon(tile: BoardTile) {
		return tile.name.includes('Wasser') ? IconDroplet : IconBolt;
	}

	function groupTiles(group: string | undefined) {
		if (!group) return [];
		return game.board.filter((tile) => tile.group === group);
	}

	function ownsGroup(group: string | undefined): boolean {
		return groupTiles(group).every((tile) => game.tiles[tile.index].owner === me.seat);
	}

	function canBuild(index: number): boolean {
		if (!isMyTurn) return false;
		const tile = game.board[index];
		const state = game.tiles[index];
		if (!tile.build || !tile.group || state.houses >= 5) return false;
		if (!ownsGroup(tile.group)) return false;
		if (groupTiles(tile.group).some((t) => game.tiles[t.index].mortgaged)) return false;
		const levels = groupTiles(tile.group).map((t) => game.tiles[t.index].houses);
		const min = Math.min(...levels);
		if (state.houses > min) return false;
		return me.cash >= tile.build;
	}

	function canSell(index: number): boolean {
		if (!isMyTurn) return false;
		const tile = game.board[index];
		const state = game.tiles[index];
		if (!tile.build || state.houses === 0) return false;
		const levels = groupTiles(tile.group).map((t) => game.tiles[t.index].houses);
		if (state.houses < Math.max(...levels)) return false;
		return true;
	}

	function canMortgage(index: number): boolean {
		if (!isMyTurn) return false;
		const tile = game.board[index];
		const state = game.tiles[index];
		if (state.mortgaged || state.houses > 0) return false;
		return !groupTiles(tile.group).some((t) => game.tiles[t.index].houses > 0);
	}

	function canSellToBank(index: number): boolean {
		if (!isMyTurn) return false;
		const tile = game.board[index];
		const state = game.tiles[index];
		if (state.houses > 0) return false;
		return !groupTiles(tile.group).some((t) => game.tiles[t.index].houses > 0);
	}

	function stateLabel(houses: number, mortgaged: boolean): string {
		const build = houses === 5 ? 'Hotel' : houses > 0 ? `${houses} ${houses === 1 ? 'Haus' : 'Häuser'}` : 'unbebaut';
		return mortgaged ? `${build} · Hypothek` : build;
	}
</script>

<div class="estate">
	<p class="muted hint">
		Bauen ist nur in deinem Zug vor dem Würfeln möglich. Hypotheken und Verkäufe sind auch
		während einer Kaufentscheidung, bei Schulden oder in der Werkstatt erlaubt, um genug Geld
		zu beschaffen.
	</p>
	{#if owned.length === 0}
		<p class="muted">Du besitzt noch keine Felder.</p>
	{/if}

	{#each sections as section (section.key)}
		<section class="group" style="--accent: {section.color}">
			<header>
				{#if section.icon}
					{@const SectionIcon = section.icon}
					<span class="icon-badge"><SectionIcon stroke={2} /></span>
				{:else}
					<span class="swatch"></span>
				{/if}
				<strong>{section.label}</strong>
				<span class="count">{section.tiles.length}/{section.total}</span>
				{#if section.tiles.length === section.total}
					<span class="complete" title="Alle Felder dieser Gruppe gehören dir">vollständig</span>
				{/if}
			</header>
			<ul>
				{#each section.tiles as tile (tile.index)}
					{@const state = game.tiles[tile.index]}
					<li>
						<div class="head">
							{#if section.key === 'tankstelle'}
								<span class="tile-icon"><IconGasStation stroke={2} /></span>
							{:else if section.key === 'versorger'}
								{@const UtilityIcon = utilityIcon(tile)}
								<span class="tile-icon"><UtilityIcon stroke={2} /></span>
							{/if}
							<strong>{tile.name}</strong>
							<span class="state">{stateLabel(state.houses, state.mortgaged)}</span>
						</div>
						<div class="actions">
							{#if tile.build && !state.mortgaged && state.houses < 5}
								<button class="plain" disabled={!canBuild(tile.index)} onclick={() => onIntent({ type: 'build', tile: tile.index })}>
									Bauen ({formatMoney(tile.build)})
								</button>
							{/if}
							{#if state.houses > 0}
								<button class="plain" disabled={!canSell(tile.index)} onclick={() => onIntent({ type: 'sell_building', tile: tile.index })}>
									Haus verkaufen
								</button>
							{/if}
							{#if !state.mortgaged && state.houses === 0}
								<button class="plain" disabled={!canMortgage(tile.index)} onclick={() => onIntent({ type: 'mortgage', tile: tile.index })}>
									Hypothek ({formatMoney(mortgageValue(tile.price))})
								</button>
							{/if}
							{#if state.mortgaged}
								<button class="plain" disabled={!isMyTurn || me.cash < unmortgageCost(tile.price)} onclick={() => onIntent({ type: 'unmortgage', tile: tile.index })}>
									Ablösen ({formatMoney(unmortgageCost(tile.price))})
								</button>
							{/if}
							{#if state.houses === 0}
								<button class="plain" disabled={!canSellToBank(tile.index)} onclick={() => (pendingSale = tile.index)}>
									An Bank ({state.mortgaged ? '0 GE' : formatMoney(tile.sell)})
								</button>
							{/if}
						</div>
					</li>
				{/each}
			</ul>
		</section>
	{/each}
</div>

<svelte:window onkeydown={(event) => event.key === 'Escape' && closeSale()} />

{#if pendingTile}
	{@const pendingState = game.tiles[pendingTile.index]}
	<!-- The backdrop only closes on click; keyboard users close with Escape or
	     the cancel button, so the pointer-only interaction is intentional here. -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="backdrop" onclick={backdrop}>
		<div
			class="dialog"
			bind:this={dialogEl}
			role="dialog"
			aria-modal="true"
			aria-label="Verkauf an die Bank bestätigen"
			tabindex="-1"
		>
			<h2>An die Bank verkaufen?</h2>
			<p>
				Willst du <strong>{pendingTile.name}</strong> wirklich für
				<strong>{pendingState.mortgaged ? '0 GE' : formatMoney(pendingTile.sell)}</strong> an die Bank
				verkaufen?
			</p>
			{#if pendingState.mortgaged}
				<p class="muted">
					Das Feld ist beliehen. Die Hypothek verfällt und du erhältst keinen Erlös.
				</p>
			{/if}
			<p class="muted">Der Verkauf lässt sich nicht rückgängig machen.</p>
			<div class="dialog-actions">
				<button class="primary danger" onclick={confirmSale}>Ja, verkaufen</button>
				<button class="plain" onclick={closeSale}>Abbrechen</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.estate {
		display: flex;
		flex-direction: column;
		gap: 0.55rem;
	}

	.hint {
		margin: 0;
		font-size: 0.78rem;
	}

	.group {
		border: 1px solid var(--md-sys-color-outline-variant);
		border-radius: var(--md-sys-shape-corner-small);
		overflow: hidden;
		background: var(--md-sys-color-surface);
	}

	header {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		padding: 0.3rem 0.5rem;
		background: color-mix(in srgb, var(--accent) 24%, var(--md-sys-color-surface-container));
		border-bottom: 1px solid color-mix(in srgb, var(--accent) 45%, var(--md-sys-color-outline));
		font-size: 0.86rem;
	}

	.swatch {
		width: 0.9rem;
		height: 0.9rem;
		border-radius: var(--md-sys-shape-corner-extra-small);
		background: var(--accent);
		border: 1px solid color-mix(in srgb, var(--md-sys-color-shadow) 32%, transparent);
		flex: 0 0 auto;
	}

	.icon-badge {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 1.15rem;
		height: 1.15rem;
		border-radius: 50%;
		background: var(--accent);
		color: var(--cd-ral9016);
		flex: 0 0 auto;
	}

	.icon-badge :global(svg) {
		width: 0.85rem;
		height: 0.85rem;
	}

	.count {
		margin-left: auto;
		font-size: 0.74rem;
		font-variant-numeric: tabular-nums;
		color: var(--md-sys-color-on-surface-variant);
	}

	.complete {
		font-size: 0.66rem;
		padding: 0.05rem 0.4rem;
		border-radius: var(--md-sys-shape-corner-full);
		background: var(--cd-success-container);
		color: var(--cd-on-success-container);
		font-weight: 600;
	}

	ul {
		list-style: none;
		margin: 0;
		padding: 0;
	}

	li {
		padding: 0.35rem 0.5rem;
		border-left: 3px solid var(--accent);
		background: color-mix(in srgb, var(--accent) 7%, var(--md-sys-color-surface));
	}

	li + li {
		border-top: 1px solid color-mix(in srgb, var(--md-sys-color-outline-variant) 32%, transparent);
	}

	.head {
		display: flex;
		align-items: baseline;
		gap: 0.35rem;
		flex-wrap: wrap;
		font-size: 0.86rem;
	}

	.tile-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		align-self: center;
		width: 1.1rem;
		height: 1.1rem;
		border-radius: 50%;
		background: var(--accent);
		color: var(--cd-ral9016);
		flex: 0 0 auto;
	}

	.tile-icon :global(svg) {
		width: 0.78rem;
		height: 0.78rem;
	}

	.state {
		margin-left: auto;
		font-size: 0.72rem;
		color: var(--md-sys-color-on-surface-variant);
	}

	.actions {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(6.75rem, 1fr));
		gap: 0.25rem;
		margin-top: 0.3rem;
	}

	.actions button {
		font-size: 0.72rem;
		padding: 0.25rem 0.4rem;
		line-height: 1.15;
	}

	.backdrop {
		position: fixed;
		inset: 0;
		z-index: 70;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 1rem;
		background: color-mix(in srgb, var(--md-sys-color-scrim) 52%, transparent);
	}

	.dialog {
		width: min(24rem, 94vw);
		outline: none;
		padding: 0.9rem 1rem 1rem;
		background: var(--md-sys-color-surface-container);
		border: 1px solid var(--md-sys-color-outline-variant);
		border-radius: var(--md-sys-shape-corner-large);
		box-shadow: 0 var(--md-sys-elevation-4) calc(var(--md-sys-elevation-5) * 3)
			color-mix(in srgb, var(--md-sys-color-shadow) 32%, transparent);
	}

	.dialog h2 {
		margin: 0 0 0.5rem;
		font-size: 1.05rem;
		line-height: 1.2;
	}

	.dialog p {
		margin: 0 0 0.45rem;
		font-size: 0.88rem;
		line-height: 1.4;
	}

	.dialog-actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
		margin-top: 0.8rem;
	}

	.primary.danger {
		background: var(--md-sys-color-error);
		color: var(--md-sys-color-on-error);
	}
</style>
