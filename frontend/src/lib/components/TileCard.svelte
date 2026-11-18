<script lang="ts">
	// Detail card for one board field, opened by clicking it. Every player sees
	// the same information — rents, prices, mortgage status and the mark of the
	// currently applicable state (Spec.md §3.1 board data + §4.5/§4.6/§4.7 rules).
	import type { BoardTile, GameState, Player, TileState } from '#lib/types';
	import {
		formatMoney,
		groupColor,
		groupLabel,
		mortgageValue,
		tokenColor,
		unmortgageCost
	} from '#lib/format';
	import IconGasStation from '@tabler/icons-svelte/icons/gas-station';
	import IconBolt from '@tabler/icons-svelte/icons/bolt';
	import IconDroplet from '@tabler/icons-svelte/icons/droplet';
	import IconX from '@tabler/icons-svelte/icons/x';

	let {
		tile,
		tileState,
		game,
		owner = undefined,
		occupants = [],
		onclose
	}: {
		tile: BoardTile;
		tileState: TileState;
		game: GameState;
		owner?: Player | undefined;
		occupants?: Player[];
		onclose: () => void;
	} = $props();

	let cardEl = $state<HTMLElement | undefined>();

	// Focus the card for keyboard use and lock the page behind the overlay.
	$effect(() => {
		cardEl?.focus();
		document.body.style.overflow = 'hidden';
		return () => {
			document.body.style.overflow = '';
		};
	});

	function backdrop(event: MouseEvent) {
		if (event.target === event.currentTarget) onclose();
	}

	const ownable = $derived(
		tile.type === 'street' || tile.type === 'tankstelle' || tile.type === 'utility'
	);
	const bandColor = $derived(tile.group ? groupColor(tile.group) : null);
	const bandIcon = $derived(
		tile.type === 'tankstelle'
			? IconGasStation
			: tile.type === 'utility'
				? tile.name.includes('Wasser')
					? IconDroplet
					: IconBolt
				: null
	);
	const kindLabel = $derived(
		tile.type === 'street'
			? 'Straße'
			: tile.type === 'tax'
				? 'Steuerfeld'
				: tile.type === 'chance'
					? 'Ereignisfeld'
					: tile.type === 'chest'
						? 'Gemeinschaftsfeld'
						: ''
	);

	const groupTiles = $derived(tile.group ? game.board.filter((t) => t.group === tile.group) : []);
	const ownedInGroup = $derived(
		owner ? groupTiles.filter((t) => game.tiles[t.index].owner === owner.seat).length : 0
	);
	// Rent doubling and build rules need the whole *street* group (never
	// tankstelle/versorger), matching the engine's ownsAllStreets().
	const completeGroup = $derived(
		tile.type === 'street' &&
			owner !== undefined &&
			groupTiles.length > 0 &&
			ownedInGroup === groupTiles.length
	);

	/** While the owner is mortgaged or in the workshop no rent is due. */
	const rentBlocked = $derived(
		owner !== undefined && (tileState.mortgaged || owner.inJail || owner.bankrupt)
	);
	const rentBlockedReason = $derived.by(() => {
		if (owner === undefined) return '';
		if (tileState.mortgaged) return 'Das Feld ist hypothekiert — es fällt keine Miete an.';
		if (owner.inJail)
			return `${owner.name} sitzt in der Werkstatt — solange fällt keine Miete an.`;
		return '';
	});

	interface RentRow {
		label: string;
		amount?: number;
		doubled?: boolean;
		formula?: string;
		active: boolean;
	}

	/** The rent ladder with the row that would be charged right now. */
	const rentRows = $derived.by((): RentRow[] => {
		if (!tile.rents) return [];
		const inactive = owner === undefined || rentBlocked;
		if (tile.type === 'street') {
			const labels = ['kein Haus', '1 Haus', '2 Häuser', '3 Häuser', '4 Häuser', 'Hotel'];
			return tile.rents.map((rent, i) => ({
				label: labels[i],
				amount: i === 0 && completeGroup ? rent * 2 : rent,
				doubled: i === 0 && completeGroup,
				active: !inactive && tileState.houses === i
			}));
		}
		// Tankstellen: the rent grows with the number of tankstellen the owner holds.
		if (tile.type === 'tankstelle') {
			const labels = ['1 Tankstelle', '2 Tankstellen', '3 Tankstellen', '4 Tankstellen'];
			return tile.rents.map((rent, i) => ({
				label: labels[i],
				amount: rent,
				active: !inactive && ownedInGroup === i + 1
			}));
		}
		return [];
	});

	/** Utilities show two flat multiplier rows instead of a rent ladder. */
	const utilityRows = $derived.by((): RentRow[] => {
		if (tile.type !== 'utility') return [];
		const inactive = owner === undefined || rentBlocked;
		return [
			{ label: 'ein Werk', formula: '4 × Wurfsumme', active: !inactive && ownedInGroup < 2 },
			{ label: 'beide Werke', formula: '10 × Wurfsumme', active: !inactive && ownedInGroup === 2 }
		];
	});

	const shownRentRows = $derived(tile.type === 'utility' ? utilityRows : rentRows);

	/** Subprime rule: 10 % of rosa/orange/rot/gelb rent flows to the utility. */
	const shareNote = $derived.by(() => {
		if (tile.type !== 'street') return '';
		if (tile.group === 'rosa' || tile.group === 'orange') {
			return shareText('Elektrizitätswerk');
		}
		if (tile.group === 'rot' || tile.group === 'gelb') {
			return shareText('Wasserwerk');
		}
		return '';
	});

	function shareText(utilityName: string): string {
		const utility = game.board.find(
			(t) => t.group === 'versorger' && t.name.includes(utilityName.slice(0, 5))
		);
		if (!utility) return `10 % der Miete fließen an das ${utilityName}.`;
		const utilityOwner = game.tiles[utility.index].owner;
		if (utilityOwner < 0 || !game.players[utilityOwner]) {
			return `10 % der Miete fließen an die Bank (das ${utilityName} ist unverkauft).`;
		}
		if (owner && utilityOwner === owner.seat) {
			return `${owner.name} besitzt beide Werke — der Nutzungsanteil fließt nicht ab.`;
		}
		const name = game.players[utilityOwner].name;
		return `10 % der Miete fließen an ${name} (das ${utilityName}).`;
	}

	const statusChips = $derived.by(() => {
		if (!ownable) return [];
		const chips: string[] = [];
		if (tileState.houses === 5) chips.push('Hotel');
		else if (tileState.houses > 0)
			chips.push(`${tileState.houses} ${tileState.houses === 1 ? 'Haus' : 'Häuser'}`);
		else chips.push('unbaut');
		if (tileState.mortgaged) chips.push('Hypothek');
		return chips;
	});

	const groupLine = $derived.by(() => {
		if (!tile.group) return '';
		const size =
			ownedInGroup > 0
				? `${ownedInGroup}/${groupTiles.length}`
				: `${groupTiles.length} Felder`;
		return `${groupLabel(tile.group)} · ${size}${owner ? ' im Besitz' : ''}`;
	});
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<!-- The backdrop only closes on click; keyboard users close with Escape or the
     close button, so the pointer-only interaction is intentional here. -->
<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="backdrop" onclick={backdrop}>
	<div
		class="panel"
		bind:this={cardEl}
		role="dialog"
		aria-modal="true"
		aria-label={`Feld ${tile.name}`}
		tabindex="-1"
	>
		<header>
			<div class="head-row">
				{#if bandIcon}
					{@const KindIcon = bandIcon}
					<span class="icon-badge" style="background: {bandColor ?? 'var(--cd-gray-800)'}">
						<KindIcon stroke={2} />
					</span>
				{:else if bandColor}
					<span class="swatch" style="background: {bandColor}"></span>
				{/if}
				<div class="names">
					<h2>{tile.name}</h2>
					<span class="kind">{kindLabel}</span>
				</div>
				<button class="plain close" aria-label="Feldinfo schließen" onclick={onclose}>
					<IconX stroke={2} />
				</button>
			</div>
			{#if groupLine}
				<p class="sub muted">{groupLine}</p>
			{/if}
			<p class="owner-line">
				{#if owner}
					<span class="dot" style="background: {tokenColor(owner.color)}"></span>
					<span>Besitzer: <strong>{owner.name}</strong></span>
				{:else if ownable}
					<span>Besitzer: <strong>die Bank</strong></span>
				{/if}
				{#each statusChips as chip (chip)}
					<span class="badge" class:mortgage={chip === 'Hypothek'}>{chip}</span>
				{/each}
			</p>
			{#if occupants.length > 0}
				<p class="sub muted">Hier gerade: {occupants.map((p) => p.name).join(', ')}</p>
			{/if}
		</header>

		{#if rentBlocked && rentBlockedReason}
			<p class="warn">{rentBlockedReason}</p>
		{/if}

		{#if shownRentRows.length > 0}
			<h3>Mieten</h3>
			<div class="rents">
				{#each shownRentRows as row (row.label)}
					<div class="rent" class:active={row.active}>
						<span class="label">
							{row.label}{#if row.doubled}<em> · ganze Farbgruppe (2×)</em>{/if}
						</span>
						<span class="amount">{row.formula ?? formatMoney(row.amount)}</span>
						{#if row.active}
							<span class="now">jetzt fällig</span>
						{:else}
							<span class="spacer" aria-hidden="true"></span>
						{/if}
					</div>
				{/each}
			</div>
			{#if tile.type === 'street' && !completeGroup}
				<p class="hint">Gehört die ganze Farbgruppe einem Spieler, kostet die Miete der ungebauten Straße das Doppelte.</p>
			{:else if tile.type === 'tankstelle'}
				<p class="hint">Die Miete steigt mit der Anzahl der Tankstellen, die der Besitzer hält.</p>
			{:else if tile.type === 'utility'}
				<p class="hint">Die Miete ist ein Vielfaches der Wurfsumme des Mieters.</p>
			{/if}
		{/if}

		{#if ownable}
			<h3>Preise</h3>
			<dl class="facts">
				<div>
					<dt>Kaufpreis</dt>
					<dd>{formatMoney(tile.price)}</dd>
				</div>
				{#if tile.build}
					<div>
						<dt>1 Haus</dt>
						<dd>{formatMoney(tile.build)}<small> · Hotel kostet denselben Preis und ersetzt vier Häuser</small></dd>
					</div>
				{/if}
				<div class:dim={tileState.mortgaged}>
					<dt>{tileState.mortgaged ? 'Hypothek genommen' : 'Beleihung (50 %)'}</dt>
					<dd>{formatMoney(mortgageValue(tile.price))}</dd>
				</div>
				<div class:em={tileState.mortgaged}>
					<dt>Ablöse (60 %)</dt>
					<dd>{formatMoney(unmortgageCost(tile.price))}</dd>
				</div>
				<div>
					<dt>Verkauf an die Bank</dt>
					<dd>{tileState.mortgaged ? '0 GE' : formatMoney(tile.sell)}<small>{tileState.mortgaged ? ' · Hypothek verfällt' : ''}</small></dd>
				</div>
			</dl>
		{/if}

		{#if tile.type === 'tax'}
			<h3>Steuer</h3>
			<p class="hint">Landet hier jemand, zahlt {tile.taxPercent
					? `${tile.taxPercent} % des Bargelds`
					: formatMoney(tile.tax)} direkt an die Bank — Frei-Parken füllt keinen Topf.</p>
		{:else if tile.type === 'chance'}
			<h3>Ereignisfeld</h3>
			<p class="hint">Es wird eine Ereigniskarte gezogen und sofort ausgeführt. Kartenzahlungen gehen an die Bank — nur die Geburtstags- und Vorstandskarten richten sich an die anderen Spieler.</p>
		{:else if tile.type === 'chest'}
			<h3>Gemeinschaftsfeld</h3>
			<p class="hint">Es wird eine Gemeinschaftskarte gezogen und sofort ausgeführt. Kartenzahlungen gehen an die Bank — nur die Geburtstags- und Vorstandskarten richten sich an die anderen Spieler.</p>
		{:else if tile.type === 'go'}
			<h3>Los</h3>
			<p class="hint">Wer Los überquert, zieht das Gehalt ({formatMoney(game.rules.losSalary)}) ein; beim Landen nur einmal, nie doppelt.</p>
		{:else if tile.type === 'jail'}
			<h3>Werkstatt</h3>
			<p class="hint">Nach drei fehlgeschlagenen Versuchen müssen {formatMoney(game.rules.jailFine)} Reparaturkosten gezahlt werden. Auswege: Pasch würfeln, Kaution von {formatMoney(game.rules.jailFine)} zahlen oder einen Werkstatt-Gutschein einlösen. Fehlt das Geld, können vorher Hypotheken aufgenommen oder Felder verkauft werden. Besucher zahlen nichts.</p>
		{:else if tile.type === 'gotojail'}
			<h3>Unfall</h3>
			<p class="hint">Wer hier landet oder dorthin geschickt wird, muss direkt in die Werkstatt — ohne Los-Gehalt.</p>
		{:else if tile.type === 'parking'}
			<h3>Frei-Parken</h3>
			<p class="hint">Wer hier landet, stellt sein Auto auf einem der markierten Stellplätze ab. Geld gibt es dafür nicht — Steuern und Strafen wandern in die Bank, nicht in einen Topf.</p>
		{/if}

		{#if tile.type === 'street'}
			<p class="hint">Kauft ein Spieler die letzte Straße dieser Farbgruppe, kostet der Kauf den doppelten Preis (bei der Auktion 120 % des Gebots).</p>
			{#if shareNote}
				<p class="hint">{shareNote}</p>
			{/if}
			<p class="hint">Innerhalb der Farbgruppe darf sich der Bauzustand höchstens um eine Stufe unterscheiden (Hotel = Stufe 5); Hypotheken verhindern Bauen und Miete.</p>
		{/if}
	</div>
</div>

<style>
	.backdrop {
		position: fixed;
		inset: 0;
		z-index: 60;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 1rem;
		background: color-mix(in srgb, var(--md-sys-color-scrim) 52%, transparent);
	}

	.panel {
		width: min(26rem, 94vw);
		max-height: min(46rem, 92vh);
		overflow-y: auto;
		outline: none;
		padding: 0.9rem 1rem 1rem;
		background: var(--md-sys-color-surface-container);
		border: 1px solid var(--md-sys-color-outline-variant);
		border-radius: var(--md-sys-shape-corner-large);
		box-shadow: 0 var(--md-sys-elevation-4) calc(var(--md-sys-elevation-5) * 3)
			color-mix(in srgb, var(--md-sys-color-shadow) 32%, transparent);
	}

	header {
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
		margin-bottom: 0.6rem;
	}

	.head-row {
		display: flex;
		align-items: center;
		gap: 0.55rem;
	}

	.names {
		flex: 1;
		min-width: 0;
	}

	h2 {
		margin: 0;
		font-size: 22px; /* title-large, tokens.css:168 */
		line-height: 28px;
		font-weight: 400;
	}

	.kind {
		font-size: 12px; /* label-medium, tokens.css:175 */
		line-height: 16px;
		font-weight: 500;
		color: var(--md-sys-color-on-surface-variant);
	}

	.close {
		width: 1.9rem;
		height: 1.9rem;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 0;
	}

	.close :global(svg) {
		width: 0.95rem;
		height: 0.95rem;
	}

	.swatch {
		width: 1.3rem;
		height: 1.3rem;
		border-radius: var(--md-sys-shape-corner-extra-small);
		border: 1px solid var(--md-sys-color-outline-variant);
		flex: 0 0 auto;
	}

	.icon-badge {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 1.7rem;
		height: 1.7rem;
		border-radius: 50%;
		color: var(--cd-ral9016);
		flex: 0 0 auto;
	}

	.icon-badge :global(svg) {
		width: 1rem;
		height: 1rem;
	}

	.sub {
		margin: 0;
		font-size: 0.8rem;
	}

	.owner-line {
		margin: 0;
		display: flex;
		align-items: center;
		gap: 0.4rem;
		flex-wrap: wrap;
		font-size: 0.9rem;
	}

	.dot {
		width: 0.75rem;
		height: 0.75rem;
		border-radius: 50%;
		border: 1px solid var(--md-sys-color-outline-variant);
		flex: 0 0 auto;
	}

	.badge.mortgage {
		color: var(--md-sys-color-on-error-container);
		background: var(--md-sys-color-error-container);
		font-weight: 700;
	}

	h3 {
		margin: 0.7rem 0 0.35rem;
		font-size: 12px; /* label-medium, tokens.css:175 */
		line-height: 16px;
		font-weight: 500;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--md-sys-color-on-surface-variant);
	}

	.rents {
		display: flex;
		flex-direction: column;
		border: 1px solid var(--md-sys-color-outline-variant);
		border-radius: var(--md-sys-shape-corner-small);
		overflow: hidden;
	}

	.rent {
		display: grid;
		grid-template-columns: 1fr auto auto;
		align-items: baseline;
		gap: 0.5rem;
		padding: 0.28rem 0.6rem;
		font-size: 0.86rem;
	}

	.rent + .rent {
		border-top: 1px solid color-mix(in srgb, var(--md-sys-color-outline-variant) 35%, transparent);
	}

	.rent .label em {
		font-style: normal;
		font-size: 0.76rem;
		color: var(--md-sys-color-on-surface-variant);
	}

	.rent .amount {
		font-weight: 700;
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
	}

	.rent.active {
		background: var(--md-sys-color-secondary-container);
		color: var(--md-sys-color-on-secondary-container);
	}

	.now {
		font-size: 0.7rem;
		font-weight: 700;
		color: var(--md-sys-color-on-primary);
		background: var(--md-sys-color-primary);
		border-radius: var(--md-sys-shape-corner-full);
		padding: 0.08rem 0.5rem;
		white-space: nowrap;
	}

	.facts {
		margin: 0;
		display: flex;
		flex-direction: column;
		border: 1px solid var(--md-sys-color-outline-variant);
		border-radius: var(--md-sys-shape-corner-small);
		overflow: hidden;
	}

	.facts > div {
		display: flex;
		justify-content: space-between;
		align-items: baseline;
		gap: 0.75rem;
		padding: 0.28rem 0.6rem;
		font-size: 0.86rem;
	}

	.facts > div + div {
		border-top: 1px solid color-mix(in srgb, var(--md-sys-color-outline-variant) 35%, transparent);
	}

	.facts dt {
		color: var(--md-sys-color-on-surface-variant);
	}

	.facts dd {
		margin: 0;
		font-weight: 700;
		font-variant-numeric: tabular-nums;
		text-align: right;
	}

	.facts dd small {
		font-size: 0.72rem;
		font-weight: 500;
		color: var(--md-sys-color-on-surface-variant);
	}

	.facts .dim {
		opacity: 0.55;
	}

	.facts .em {
		background: var(--md-sys-color-error-container);
	}

	.warn {
		margin: 0 0 0.4rem;
		padding: 0.4rem 0.6rem;
		border-radius: var(--md-sys-shape-corner-small);
		background: var(--cd-warning-container);
		border: 1px solid var(--cd-warning);
		color: var(--cd-on-warning-container);
		font-size: 0.8rem;
	}

	.hint {
		margin: 0.35rem 0 0;
		font-size: 12px; /* body-small, tokens.css:173 */
		line-height: 16px;
		color: var(--md-sys-color-on-surface-variant);
	}
</style>
