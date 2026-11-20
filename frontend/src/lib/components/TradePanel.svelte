<script lang="ts">
	import type { BoardTile, GameState, Intent, Player, TradeOffer } from '#lib/types';
	import { formatMoney, groupColor, groupLabel, STREET_GROUPS, tokenColor } from '#lib/format';
	import { CARD_COLOR, tradeSideItems } from '#lib/trade';
	import { tradeUI } from '#lib/trade-ui.svelte';
	import Dropdown from './Dropdown.svelte';
	import TradeChips from './TradeChips.svelte';
	import TradeDetails from './TradeDetails.svelte';
	import IconGasStation from '@tabler/icons-svelte/icons/gas-station';
	import IconBolt from '@tabler/icons-svelte/icons/bolt';
	import IconDroplet from '@tabler/icons-svelte/icons/droplet';
	import IconTool from '@tabler/icons-svelte/icons/tool';

	let {
		game,
		me,
		onIntent
	}: {
		game: GameState;
		me: Player;
		onIntent: (intent: Intent) => void;
	} = $props();

	// Objects selected per side: tile indices, card list slots and cash.
	let partnerSeat = $state(-1);
	let offerTiles = $state<number[]>([]);
	let requestTiles = $state<number[]>([]);
	let offerCards = $state<number[]>([]);
	let requestCards = $state<number[]>([]);
	let offerCash = $state(0);
	let requestCash = $state(0);
	/** Id of the open offer replaced on submit (counter-offer mode). */
	let counterTo = $state<string | null>(null);

	const partners = $derived(game.players.filter((p) => p.seat !== me.seat && !p.bankrupt));
	const partner = $derived(game.players[partnerSeat] ?? null);
	const openTrades = $derived(game.trades.filter((t) => t.status === 'open'));

	interface TradeSection {
		key: string;
		label: string;
		color: string;
		icon: '' | 'gas' | 'bolt' | 'tool';
		tiles: BoardTile[];
		cards: number;
	}

	function tradeable(seat: number) {
		return game.board.filter((tile) => game.tiles[tile.index].owner === seat && game.tiles[tile.index].houses === 0);
	}

	/** All selectable trade objects of a seat, grouped by colour group. */
	function sectionsFor(seat: number): TradeSection[] {
		const owned = [...tradeable(seat)].sort((a, b) => a.index - b.index);
		const result: TradeSection[] = [];
		if (owned.some((t) => t.type === 'tankstelle')) {
			result.push({ key: 'tankstelle', label: groupLabel('tankstelle'), color: groupColor('tankstelle'), icon: 'gas', tiles: owned.filter((t) => t.type === 'tankstelle'), cards: 0 });
		}
		if (owned.some((t) => t.type === 'utility')) {
			result.push({ key: 'versorger', label: groupLabel('versorger'), color: groupColor('versorger'), icon: 'bolt', tiles: owned.filter((t) => t.type === 'utility'), cards: 0 });
		}
		for (const group of STREET_GROUPS) {
			const tiles = owned.filter((t) => t.group === group);
			if (tiles.length > 0) {
				result.push({ key: group, label: groupLabel(group), color: groupColor(group), icon: '', tiles, cards: 0 });
			}
		}
		const cards = game.players[seat]?.jailCards ?? 0;
		if (cards > 0) {
			result.push({ key: 'cards', label: 'Pannenhilfen', color: CARD_COLOR, icon: 'tool', tiles: [], cards });
		}
		return result;
	}

	function sectionCount(section: TradeSection, tileSel: number[], cardSel: number[]): { have: number; total: number } {
		let have = section.tiles.filter((t) => tileSel.includes(t.index)).length;
		if (section.cards > 0) have += cardSel.filter((i) => i < section.cards).length;
		return { have, total: section.tiles.length + section.cards };
	}

	function toggleTile(target: 'offer' | 'request', index: number): void {
		if (target === 'offer') offerTiles = toggle(offerTiles, index);
		else requestTiles = toggle(requestTiles, index);
	}

	function toggleCard(target: 'offer' | 'request', slot: number): void {
		if (target === 'offer') offerCards = toggle(offerCards, slot);
		else requestCards = toggle(requestCards, slot);
	}

	function toggle(list: number[], value: number): number[] {
		return list.includes(value) ? list.filter((v) => v !== value) : [...list, value];
	}

	function setCash(target: 'offer' | 'request', raw: string): void {
		const parsed = Number(String(raw).replace(',', '.'));
		const value = Number.isFinite(parsed) ? Math.max(0, Math.trunc(parsed)) : 0;
		if (target === 'offer') offerCash = value;
		else requestCash = value;
	}

	function num(value: number): number {
		return Number.isFinite(value) ? Math.max(0, Math.trunc(value)) : 0;
	}

	function clearForm(): void {
		offerTiles = [];
		requestTiles = [];
		offerCards = [];
		requestCards = [];
		offerCash = 0;
		requestCash = 0;
		counterTo = null;
	}

	function resetForm(): void {
		clearForm();
		partnerSeat = -1;
	}

	function cardSlots(count: number): number[] {
		return Array.from({ length: Math.max(0, Math.trunc(count)) }, (_, i) => i);
	}

	/** Prefills the form with the sides of the original offer swapped. */
	function startCounter(trade: TradeOffer): void {
		if (!(me && (trade.from === me.seat || trade.to === me.seat))) return;
		const other = game.players[trade.from === me.seat ? trade.to : trade.from];
		if (!other || other.bankrupt) return;
		// I get what I was originally offered and give what was requested of me.
		const iWasTarget = trade.to === me.seat;
		const myTileList = (iWasTarget ? trade.requestTiles : trade.offerTiles) ?? [];
		const theirTileList = (iWasTarget ? trade.offerTiles : trade.requestTiles) ?? [];

		clearForm();
		partnerSeat = other.seat;
		const myTradeable = new Set(tradeable(me.seat).map((t) => t.index));
		const theirTradeable = new Set(tradeable(other.seat).map((t) => t.index));
		offerTiles = myTileList.filter((i) => myTradeable.has(i));
		requestTiles = theirTileList.filter((i) => theirTradeable.has(i));
		offerCash = Math.min(iWasTarget ? trade.requestCash : trade.offerCash, me.cash);
		requestCash = Math.min(iWasTarget ? trade.offerCash : trade.requestCash, other.cash);
		offerCards = cardSlots(Math.min(iWasTarget ? trade.requestCards : trade.offerCards, me.jailCards));
		requestCards = cardSlots(Math.min(iWasTarget ? trade.offerCards : trade.requestCards, other.jailCards));
		counterTo = trade.id;
	}

	// Consumes a counter request that was opened from a popup.
	$effect(() => {
		const request = tradeUI.counterRespond;
		if (!request) return;
		tradeUI.counterRespond = null;
		const trade = game.trades.find((t) => t.id === request);
		if (trade && trade.status === 'open') startCounter(trade);
	});

	function submit(): void {
		if (!(partner && (offerTiles.length || requestTiles.length || offerCards.length || requestCards.length || num(offerCash) || num(requestCash)))) return;
		onIntent({
			type: 'propose_trade',
			trade: {
				to: partner.seat,
				offerTiles,
				requestTiles,
				offerCash: num(offerCash),
				requestCash: num(requestCash),
				offerCards: offerCards.length,
				requestCards: requestCards.length,
				...(counterTo ? { counterTo } : {})
			}
		});
		resetForm();
	}
</script>

<div class="trade">
	{#if openTrades.length > 0}
		<h4>Offene Angebote</h4>
		{#each openTrades as trade (trade.id)}
			<div class="offer">
				<div class="row between">
					<strong>{game.players[trade.from]?.name}</strong>
					<span class="muted">→ {game.players[trade.to]?.name}</span>
				</div>
				<TradeDetails {game} {trade} />
				{#if trade.to === me.seat}
					<div class="row">
						<button class="plain" onclick={() => onIntent({ type: 'respond_trade', tradeId: trade.id, accept: false })}>Ablehnen</button>
						<button class="plain" onclick={() => startCounter(trade)}>Gegenangebot</button>
						<button class="primary" onclick={() => onIntent({ type: 'respond_trade', tradeId: trade.id, accept: true })}>Annehmen</button>
					</div>
				{:else if trade.from === me.seat}
					<p class="muted waiting">Wartet auf die Antwort von {game.players[trade.to]?.name}.</p>
				{/if}
			</div>
		{/each}
	{/if}

	<h4>{counterTo ? 'Gegengebot' : 'Neuer Handel'}</h4>
	{#if counterTo}<p class="muted hint">Du arbeitest das offene Angebot um. Es wird beim Absenden ersetzt.</p>{/if}

	<label for="trade-partner">Mitspieler</label>
	<Dropdown
		id="trade-partner"
		options={[
			{ value: -1, label: '– auswählen –' },
			...partners.map((p) => ({ value: p.seat, label: p.name, color: tokenColor(p.color) }))
		]}
		bind:value={partnerSeat}
		onselect={() => (counterTo = null)}
	/>

	{#if partner}
		<div class="sides">
			{@render tradeSide('Du gibst', me, 'offer', offerTiles, offerCards, offerCash, (v) => setCash('offer', v))}
			{@render tradeSide(`${partner.name} gibt`, partner, 'request', requestTiles, requestCards, requestCash, (v) => setCash('request', v))}
		</div>
		<div class="row">
			<button class="plain" onclick={resetForm}>Zurücksetzen</button>
			<button class="primary" onclick={submit}>Angebot senden</button>
		</div>
	{/if}
</div>

{#snippet tradeSide(
	heading: string,
	owner: Player,
	target: 'offer' | 'request',
	tileSelection: number[],
	cardSelection: number[],
	cash: number,
	onCash: (raw: string) => void
)}
	<div class="side-block">
		<p class="muted">{heading}</p>
		<div class="summary">
			<TradeChips items={tradeSideItems(game, tileSelection, cardSelection.length, num(cash))} />
		</div>
		{#each sectionsFor(owner.seat) as section (section.key)}
			{@const picked = sectionCount(section, tileSelection, cardSelection)}
			<section class="obj-group" style="--accent: {section.color}">
				<header>
					{#if section.icon === 'gas'}
						<span class="badge icon"><IconGasStation stroke={2} /></span>
					{:else if section.icon === 'bolt'}
						<span class="badge icon"><IconBolt stroke={2} /></span>
					{:else if section.icon === 'tool'}
						<span class="badge icon"><IconTool stroke={2} /></span>
					{:else}
						<span class="badge swatch"></span>
					{/if}
					<strong>{section.label}</strong>
					<span class="count">{picked.have}/{picked.total}</span>
				</header>
				<ul>
					{#each section.tiles as tile (tile.index)}
						<li>
							<label class="check">
								<input
									type="checkbox"
									checked={tileSelection.includes(tile.index)}
									onchange={() => toggleTile(target, tile.index)}
								/>
								{#if tile.type === 'utility'}
									<span class="badge icon small" title={groupLabel('versorger')}>
										{#if tile.name.includes('Wasser')}<IconDroplet stroke={2} />{:else}<IconBolt stroke={2} />{/if}
									</span>
								{:else if tile.type === 'tankstelle'}
									<span class="badge icon small" title={groupLabel('tankstelle')}><IconGasStation stroke={2} /></span>
								{:else}
									<span class="mini-swatch" style="background: {section.color}"></span>
								{/if}
								<span class="check-label">{tile.name}</span>
								<span class="price muted">{formatMoney(tile.price ?? 0)}</span>
							</label>
						</li>
					{/each}
					{#if section.cards > 0}
						{#each Array(section.cards) as _, slot (slot)}
							<li>
								<label class="check">
									<input type="checkbox" checked={cardSelection.includes(slot)} onchange={() => toggleCard(target, slot)} />
									<span class="badge icon small pass" title="Pannenhilfe"><IconTool stroke={2} /></span>
									<span class="check-label">Pannenhilfe {section.cards > 1 ? slot + 1 : ''}</span>
								</label>
							</li>
						{/each}
					{/if}
				</ul>
			</section>
		{:else}
			<p class="muted empty">– nichts zu tauschen –</p>
		{/each}
		<label for="trade-{target}-cash">Bargeld</label>
		<input
			id="trade-{target}-cash"
			type="number"
			min="0"
			step="10"
			max={owner.cash}
			value={num(cash) || ''}
			oninput={(e) => onCash(e.currentTarget.value)}
		/>
	</div>
{/snippet}

<style>
	h4 {
		margin: 0.5rem 0 0.35rem;
		font-size: 14px; /* title-small, tokens.css:170 */
		line-height: 20px;
		font-weight: 500;
	}

	.offer {
		background: var(--md-sys-color-surface);
		border: 1px solid var(--md-sys-color-outline-variant);
		border-radius: var(--md-sys-shape-corner-small);
		padding: 0.4rem 0.5rem;
		margin-bottom: 0.4rem;
	}

	.waiting {
		font-size: 0.76rem;
		margin: 0.1rem 0 0;
	}

	.hint {
		font-size: 0.76rem;
		margin: 0 0 0.35rem;
	}

	.sides {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 0.75rem;
		margin: 0.5rem 0;
	}

	.row.between {
		justify-content: space-between;
	}

	.side-block p {
		margin: 0 0 0.3rem;
	}

	.side-block > label {
		margin-top: 0.4rem;
	}

	.summary {
		margin-bottom: 0.4rem;
	}

	.obj-group {
		border: 1px solid var(--md-sys-color-outline-variant);
		border-radius: var(--md-sys-shape-corner-small);
		overflow: hidden;
		background: var(--md-sys-color-surface);
		margin-bottom: 0.35rem;
	}

	.obj-group header {
		display: flex;
		align-items: center;
		gap: 0.35rem;
		padding: 0.25rem 0.4rem;
		background: color-mix(in srgb, var(--accent) 26%, var(--md-sys-color-surface-container));
		border-bottom: 1px solid color-mix(in srgb, var(--accent) 45%, var(--md-sys-color-outline));
		font-size: 0.76rem;
	}

	.obj-group ul {
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.obj-group li {
		padding: 0.22rem 0.4rem;
		border-top: 1px solid color-mix(in srgb, var(--md-sys-color-outline-variant) 32%, transparent);
	}

	.badge {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		border: 1px solid color-mix(in srgb, var(--md-sys-color-shadow) 28%, transparent);
		flex: 0 0 auto;
		padding: 0;
	}

	.badge.icon {
		width: 1.05rem;
		height: 1.05rem;
		border-radius: 50%;
		background: var(--accent);
		color: var(--cd-ral9016);
	}

	.badge.icon :global(svg) {
		width: 0.7rem;
		height: 0.7rem;
	}

	.badge.swatch {
		width: 0.85rem;
		height: 0.85rem;
		border-radius: var(--md-sys-shape-corner-extra-small);
		background: var(--accent);
	}

	.badge.icon.small {
		width: 0.95rem;
		height: 0.95rem;
	}

	.check {
		display: flex;
		align-items: center;
		gap: 0.3rem;
		font-size: 12px; /* label-medium, tokens.css:175 */
		line-height: 16px;
		font-weight: 500;
		color: var(--md-sys-color-on-surface);
	}

	.check input {
		width: auto;
	}

	.check-label {
		flex: 1 1 auto;
		min-width: 0;
	}

	.price {
		font-size: 0.7rem;
		white-space: nowrap;
	}

	.mini-swatch {
		width: 0.7rem;
		height: 0.7rem;
		border-radius: var(--md-sys-shape-corner-extra-small);
		border: 1px solid color-mix(in srgb, var(--md-sys-color-shadow) 28%, transparent);
		flex: 0 0 auto;
	}

	.empty {
		font-size: 0.78rem;
	}
</style>
