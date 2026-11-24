<script lang="ts">
	import type { BoardTile, Player, TileState } from '#lib/types';
	import {
		formatMoney,
		gridPosition,
		groupColor,
		rentLabel,
		tileTypeLabel,
		tokenColor,
		unmortgageCost
	} from '#lib/format';
	import IconArrowBigLeftFilled from '@tabler/icons-svelte/icons/arrow-big-left-filled';
	import IconTool from '@tabler/icons-svelte/icons/tool';
	import IconCarCrash from '@tabler/icons-svelte/icons/car-crash';
	import IconParking from '@tabler/icons-svelte/icons/parking';
	import IconHelpCircle from '@tabler/icons-svelte/icons/help-circle';
	import IconGift from '@tabler/icons-svelte/icons/gift';
	import IconReceipt from '@tabler/icons-svelte/icons/receipt';
	import IconGasStation from '@tabler/icons-svelte/icons/gas-station';
	import IconBolt from '@tabler/icons-svelte/icons/bolt';
	import IconDroplet from '@tabler/icons-svelte/icons/droplet';

	let {
		tile,
		state,
		owner = undefined,
		occupants = [],
		losSalary = 200,
		selectedSeat = null,
		onselect
	}: {
		tile: BoardTile;
		state: TileState;
		owner?: Player;
		occupants?: Player[];
		losSalary?: number;
		/** Seat focused in the player list; null means no board focus. */
		selectedSeat?: number | null;
		/** Opens the tile detail card; every rendered tile is clickable. */
		onselect: () => void;
	} = $props();

	/** Enter/Space open the tile card of this field, mirroring a real button. */
	function handleKey(event: KeyboardEvent) {
		if (event.key === 'Enter' || event.key === ' ') {
			event.preventDefault();
			onselect();
		}
	}

	const faded = $derived(selectedSeat !== null && state.owner !== selectedSeat);
	const highlighted = $derived(selectedSeat !== null && state.owner === selectedSeat);

	const pos = $derived(gridPosition(tile.index));
	const isCorner = $derived(tile.index % 10 === 0);
	const cornerKey = $derived(
		isCorner ? (['br', 'bl', 'tl', 'tr'] as const)[tile.index / 10] : ''
	);
	const side = $derived(
		tile.index < 10 ? 'bottom' : tile.index < 20 ? 'left' : tile.index < 30 ? 'top' : 'right'
	);

	const hasBand = $derived(!!tile.group);
	const bandColor = $derived(tile.group ? groupColor(tile.group) : 'transparent');
	const bandIcon = $derived(
		tile.type === 'tankstelle'
			? IconGasStation
			: tile.type === 'utility'
				? tile.name.includes('Wasser')
					? IconDroplet
					: IconBolt
				: null
	);

	// Special fields get their own icon, accent colour and short label so they
	// are recognisable at a glance without reading the name.
	const special = $derived.by(() => {
		switch (tile.type) {
			case 'go':
				return { icon: IconArrowBigLeftFilled, label: 'Los', sub: `${losSalary} GE`, accent: '#2e7d32' };
			case 'jail':
				return { icon: IconTool, label: 'Werkstatt', sub: 'Zur Reparatur', accent: '#a15c00' };
			case 'gotojail':
				return { icon: IconCarCrash, label: 'Unfall', sub: '', accent: '#b3261e' };
			case 'parking':
				return { icon: IconParking, label: 'Frei-Parken', sub: '', accent: '#00796b' };
			case 'chance':
				return { icon: IconHelpCircle, label: 'Ereignis', sub: '', accent: '#c2410c' };
			case 'chest':
				return { icon: IconGift, label: 'Gemeinschaft', sub: '', accent: '#3f51b5' };
			case 'tax':
				return {
					icon: IconReceipt,
					label: 'Steuer',
					sub: tile.taxPercent ? `${tile.taxPercent} %` : `${tile.tax ?? 0} GE`,
					accent: '#546e7a'
				};
			default:
				return null;
		}
	});

	const title = $derived(
		[
			tile.name,
			tile.price ? formatMoney(tile.price) : '',
			owner ? `Besitzer: ${owner.name}` : '',
			state.mortgaged
				? `Hypothek (Ablöse ${formatMoney(unmortgageCost(tile.price ?? 0))})`
				: '',
			rentLabel(tile),
			occupants.length ? `Hier: ${occupants.map((p) => p.name).join(', ')}` : '',
			'Klicken für Details'
		]
			.filter(Boolean)
			.join(' · ')
	);

	/** Family word of the four fuel-station fields (Tankstelle Süd/West/Nord/Ost). */
	const TANKSTELLE = 'Tankstelle';

	// German compound names get soft hyphens at their natural joints so narrow
	// side fields never break words at a random last letter.
	function breakName(name: string): string {
		if (name === 'Elektrizitätswerk') return 'Elektri\u00ADzitäts\u00ADwerk';
		if (name === 'Gemeinschaftsfeld') return 'Gemein\u00ADschafts\u00ADfeld';
		if (name.startsWith(`${TANKSTELLE} `)) return 'Tank\u00ADstelle' + name.slice(TANKSTELLE.length);
		return name.replace(
			/(steuer|promenade|platz|straße|gasse|weg|ring|garten|aktie|werk)$/i,
			'\u00AD$1'
		);
	}

	const displayName = $derived(breakName(tile.name));
</script>

{#snippet houses()}
	<span
		class="houses"
		title={state.houses === 5 ? 'Hotel' : `${state.houses} ${state.houses === 1 ? 'Haus' : 'Häuser'}`}
	>
		{#if state.houses === 5}
			<!-- Hotel: a tall multi-storey building with a rooftop "H" sign, so it
			     never gets confused with the pointed roofs of the houses. -->
			<svg class="hotel" viewBox="0 0 28 30" aria-hidden="true">
				<rect
					x="7"
					y="0.4"
					width="14"
					height="7.4"
					rx="1.2"
					fill="#1a1a1a"
					stroke="#ffffff"
					stroke-width="0.8"
				/>
				<text
					x="14"
					y="6.1"
					text-anchor="middle"
					font-size="6.2"
					font-weight="800"
					font-family="system-ui, sans-serif"
					fill="#ffffff">H</text
				>
				<rect
					x="1.8"
					y="7.4"
					width="24.4"
					height="21.6"
					rx="1.2"
					fill="#ffffff"
					stroke="#1a1a1a"
					stroke-width="2.2"
				/>
				<g fill="#1a1a1a">
					<rect x="4.6" y="10" width="3.8" height="3" rx="0.5" />
					<rect x="12.1" y="10" width="3.8" height="3" rx="0.5" />
					<rect x="19.6" y="10" width="3.8" height="3" rx="0.5" />
					<rect x="4.6" y="14.6" width="3.8" height="3" rx="0.5" />
					<rect x="12.1" y="14.6" width="3.8" height="3" rx="0.5" />
					<rect x="19.6" y="14.6" width="3.8" height="3" rx="0.5" />
					<rect x="4.6" y="19.2" width="3.8" height="3" rx="0.5" />
					<rect x="19.6" y="19.2" width="3.8" height="3" rx="0.5" />
					<rect x="10.9" y="22" width="6.2" height="7" rx="0.5" />
				</g>
			</svg>
		{:else}
			{#each Array(state.houses) as _, i (i)}
				<!-- House: filled body with a bold roof and a door, readable even when
				     four of them sit side by side on a narrow field. -->
				<svg class="house" viewBox="0 0 24 22" aria-hidden="true">
					<path
						d="M2.2 9.8 L12 2 L21.8 9.8 Z"
						fill="#ffffff"
						stroke="#1a1a1a"
						stroke-width="2.4"
						stroke-linejoin="round"
					/>
					<rect
						x="4.8"
						y="8.9"
						width="14.4"
						height="11.3"
						rx="0.9"
						fill="#ffffff"
						stroke="#1a1a1a"
						stroke-width="2.4"
					/>
					<rect x="10.2" y="14.2" width="3.6" height="6" rx="0.5" fill="#1a1a1a" />
				</svg>
			{/each}
		{/if}
	</span>
{/snippet}

<div
	class="tile side-{side} selectable"
	class:corner={isCorner}
	class:corner-br={cornerKey === 'br'}
	class:corner-bl={cornerKey === 'bl'}
	class:corner-tl={cornerKey === 'tl'}
	class:corner-tr={cornerKey === 'tr'}
	class:special={special !== null}
	class:parking={tile.type === 'parking'}
	class:mortgaged={state.mortgaged}
	class:faded={faded}
	class:highlighted={highlighted}
	data-index={tile.index}
	role="button"
	tabindex="0"
	aria-haspopup="dialog"
	onclick={() => onselect()}
	onkeydown={handleKey}
	style="grid-row: {pos.row}; grid-column: {pos.col}; --accent: {special?.accent ??
		'transparent'}; --owner: {owner ? tokenColor(owner.color) : 'transparent'};"
	{title}
>
	{#if hasBand}
		<div class="band" style="background: {bandColor}">
			{#if bandIcon}
				{@const BandIcon = bandIcon}
				<BandIcon stroke={2} />
			{/if}
			{#if state.houses > 0}
				{@render houses()}
			{/if}
		</div>
	{/if}

	{#if special}
		{@const SpecialIcon = special.icon}
		<div class="special-body">
			<span class="glyph"><SpecialIcon stroke={1.7} /></span>
			<span class="special-label">{special.label}</span>
			{#if special.sub}
				<span class="special-sub">{special.sub}</span>
			{/if}
		</div>
	{:else}
		<div class="body">
			<span class="name" class:long={tile.name.length > 12}>{displayName}</span>
			{#if state.mortgaged}
				<span class="price mortgage">HYPO</span>
			{:else if tile.price}
				<span class="price">{formatMoney(tile.price)}</span>
			{:else if tileTypeLabel(tile)}
				<span class="price">{tileTypeLabel(tile)}</span>
			{/if}
		</div>
	{/if}

	{#if owner}
		<span class="owner-strip" title={`Besitzer: ${owner.name}`}>
			<span class="owner-initial">{owner.name.slice(0, 1).toUpperCase()}</span>
		</span>
	{/if}
</div>

<style>
	.tile {
		position: relative;
		display: flex;
		flex-direction: column;
		background: var(--md-sys-color-surface);
		border: 1px solid var(--md-sys-color-outline-variant);
		border-radius: var(--md-sys-shape-corner-extra-small);
		overflow: hidden;
		min-width: 0;
		min-height: 0;
		font-size: clamp(0.56rem, 1.18cqw, 0.95rem);
		line-height: 1.08;
		letter-spacing: normal;
	}

	/* The colour band always faces the middle of the board. */
	.side-top {
		flex-direction: column-reverse;
	}

	.side-left {
		flex-direction: row-reverse;
	}

	.side-right {
		flex-direction: row;
	}

	.tile.corner {
		flex-direction: column;
		background: var(--md-sys-color-surface-container);
	}

	.tile.mortgaged {
		opacity: 0.72;
	}

	/* Declared after .mortgaged so board focus wins while it is active. */
	.tile.faded {
		opacity: 0.45;
	}

	/* Every field opens the detail card on click; keyboard users focus it. */
	.tile.selectable {
		cursor: pointer;
	}

	.tile.selectable:hover,
	.tile.selectable:focus-visible {
		border-color: var(--md-sys-color-primary);
		box-shadow: 0 0 0 1px var(--md-sys-color-primary);
		z-index: 2;
	}

	.tile.selectable:focus-visible {
		outline: var(--cd-focus-ring-width) solid var(--cd-focus-ring-color);
		outline-offset: var(--cd-focus-ring-offset);
		z-index: 3;
	}

	/* Overlay ring above the owner strip (z-index 3) so all four edges stay
	   continuous. pointer-events: none keeps the field fully clickable. */
	.tile.selectable.highlighted::after {
		content: '';
		position: absolute;
		inset: 0;
		border: 2px solid var(--md-sys-color-primary);
		border-radius: inherit;
		pointer-events: none;
		z-index: 4;
	}

	.band {
		position: relative;
		flex: 0 0 27%;
		display: flex;
		align-items: center;
		justify-content: center;
		border-bottom: 1px solid color-mix(in srgb, var(--md-sys-color-shadow) 28%, transparent);
	}

	.side-top .band {
		border-top: 1px solid color-mix(in srgb, var(--md-sys-color-shadow) 28%, transparent);
		border-bottom: none;
	}

	.side-left .band,
	.side-right .band {
		border-bottom: none;
	}

	.side-left .band {
		border-left: 1px solid color-mix(in srgb, var(--md-sys-color-shadow) 28%, transparent);
	}

	.side-right .band {
		border-right: 1px solid color-mix(in srgb, var(--md-sys-color-shadow) 28%, transparent);
	}

	.band > :global(svg) {
		width: 62%;
		height: 62%;
		max-width: 2.1em;
		max-height: 2.1em;
		color: var(--cd-ral9016);
	}

	.houses {
		position: absolute;
		inset: 0;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 0.1em;
		padding: 0.08em;
		pointer-events: none;
	}

	.side-left .houses,
	.side-right .houses {
		flex-direction: column;
		gap: 0.08em;
	}

	.house,
	.hotel {
		flex: 0 0 auto;
		height: auto;
		max-width: none;
		max-height: none;
		filter: drop-shadow(0 1px 1px rgba(0, 0, 0, 0.35));
	}

	/* Four houses share the band width, so the size is capped by a percentage
	   as well: at small viewports the board shrinks faster than the font.
	   Top/bottom bands are width-limited, left/right bands are height-limited. */
	.houses .house {
		width: min(1.5em, 23%);
	}

	.side-left .houses .house,
	.side-right .houses .house {
		/* Four houses stack vertically here, so height is the limiting axis:
		   4 × 23 % plus gaps/padding stays below 100 %. */
		height: min(1.5em, 23%);
		width: auto;
	}

	/* The hotel is a single multi-storey building: it fills the band instead of
	   being lined up, which makes it stand out from any number of houses. */
	.houses .hotel {
		height: 92%;
		width: auto;
	}

	.side-left .houses .hotel,
	.side-right .houses .hotel {
		height: auto;
		width: 92%;
	}

	/* Ownership is shown as a solid strip on the outer edge of the field,
	   opposite the colour band and the middle of the board. */
	.owner-strip {
		position: absolute;
		display: flex;
		align-items: center;
		justify-content: center;
		background: var(--owner);
		box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--md-sys-color-shadow) 32%, transparent);
		z-index: 3;
		pointer-events: none;
	}

	.owner-initial {
		font-size: 0.62em;
		font-weight: 800;
		line-height: 1;
		color: var(--cd-ral9016);
		text-shadow: 0 1px 1px color-mix(in srgb, var(--md-sys-color-shadow) 70%, transparent);
	}

	.side-bottom .owner-strip {
		left: 0;
		right: 0;
		bottom: 0;
		height: 0.85em;
	}

	.side-top .owner-strip {
		left: 0;
		right: 0;
		top: 0;
		height: 0.85em;
	}

	.side-left .owner-strip {
		top: 0;
		bottom: 0;
		left: 0;
		width: 0.85em;
	}

	.side-right .owner-strip {
		top: 0;
		bottom: 0;
		right: 0;
		width: 0.85em;
	}

	.body {
		flex: 1;
		display: flex;
		flex-direction: column;
		gap: 0.12em;
		padding: 0.3em 0.34em;
		min-width: 0;
		min-height: 0;
		overflow: hidden;
	}

	.side-bottom .body {
		padding-bottom: calc(0.3em + 0.85em);
	}

	.side-top .body {
		padding-top: calc(0.3em + 0.85em);
	}

	.side-left .body {
		padding-left: calc(0.22em + 0.85em);
	}

	.side-right .body {
		padding-right: calc(0.22em + 0.85em);
	}

	.name {
		font-weight: 700;
		font-size: 1em;
		overflow-wrap: break-word;
		hyphens: manual;
	}

	.name.long {
		font-size: 0.8em;
	}

	.price {
		color: var(--md-sys-color-on-surface);
		font-size: 0.94em;
		font-weight: 600;
		font-variant-numeric: tabular-nums;
		letter-spacing: 0.01em;
	}

	.price.mortgage {
		color: var(--md-sys-color-error);
		font-weight: 700;
		letter-spacing: 0.05em;
	}

	/* Special fields: Los, Werkstatt, Unfall, Frei-Parken, Ereignis, Gemeinschaft, Steuer. */
	.tile.special {
		background: linear-gradient(
			150deg,
			color-mix(in srgb, var(--accent) 20%, var(--md-sys-color-surface)),
			var(--md-sys-color-surface)
		);
		border-color: color-mix(in srgb, var(--accent) 55%, var(--md-sys-color-outline-variant));
	}

	.tile.corner.special {
		background: linear-gradient(
			145deg,
			color-mix(in srgb, var(--accent) 30%, var(--md-sys-color-surface)),
			color-mix(in srgb, var(--accent) 8%, var(--md-sys-color-surface))
		);
		border-width: 2px;
	}

	.special-body {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 0.15em;
		padding: 0.3em;
		text-align: center;
		min-width: 0;
		min-height: 0;
		overflow: hidden;
		color: var(--accent);
	}

	.glyph {
		display: flex;
		flex: 0 0 auto;
	}

	.glyph :global(svg) {
		width: 2.4em;
		height: 2.4em;
	}

	.tile.corner .glyph :global(svg) {
		width: 2.6em;
		height: 2.6em;
	}

	.special-label {
		font-weight: 700;
		font-size: 1em;
		color: var(--md-sys-color-on-surface);
		line-height: 1.1;
	}

	.corner .special-label {
		font-size: 1.15em;
	}

	.special-sub {
		font-size: 0.78em;
		color: var(--md-sys-color-on-surface-variant);
		line-height: 1.1;
	}

	.side-bottom .special-body {
		padding-bottom: calc(0.3em + 0.85em);
	}

	.side-top .special-body {
		padding-top: calc(0.3em + 0.85em);
	}

	.side-left .special-body {
		padding-left: calc(0.2em + 0.85em);
	}

	.side-right .special-body {
		padding-right: calc(0.2em + 0.85em);
	}

	/* The corner spur enters at the inner corner, so the corner content is
	   anchored to the outer corner and stays clear of the road and the car. */
	.tile.corner-br .special-body {
		justify-content: flex-end;
		align-items: flex-end;
	}

	.tile.corner-bl .special-body {
		justify-content: flex-end;
		align-items: flex-start;
	}

	.tile.corner-tl .special-body {
		justify-content: flex-start;
		align-items: flex-start;
	}

	.tile.corner-tr .special-body {
		justify-content: flex-start;
		align-items: flex-end;
	}

	/* The Frei-Parken field carries a painted parking lot in its lower half, so
	   the label keeps to a single line at the outer corner and drops the big
	   glyph; the bay markings itself make the field recognisable. */
	.tile.parking .glyph {
		display: none;
	}

	.tile.parking .special-body {
		justify-content: flex-start;
		padding: 0.35em 0.4em;
	}
</style>
