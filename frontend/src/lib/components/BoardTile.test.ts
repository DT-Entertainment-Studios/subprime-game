import { describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render } from '@testing-library/svelte';
import BoardTileView from './BoardTile.svelte';
import BoardTileSource from './BoardTile.svelte?raw';
import type { BoardTile, Player, TileState } from '#lib/types';

const TANKSTELLE_SUED: BoardTile = {
	index: 5,
	name: 'Tankstelle Süd',
	type: 'tankstelle',
	group: 'tankstelle',
	price: 200,
	rents: [25, 50, 100, 200],
	sell: 140
};

const ELEKTRIZITAETSWERK: BoardTile = {
	index: 12,
	name: 'Elektrizitätswerk',
	type: 'utility',
	group: 'versorger',
	price: 150,
	sell: 105
};

const EMPTY_STATE: TileState = { owner: -1, houses: 0, mortgaged: false };

/** Path data of every glyph rendered in the field's colour band. */
function bandPaths(container: HTMLElement): string[] {
	const band = container.querySelector('.band');
	expect(band).not.toBeNull();
	return [...band!.querySelectorAll('svg path')].map((path) => path.getAttribute('d') ?? '');
}

describe('BoardTile', () => {
	it('renders a tankstelle with the gas-pump band icon and the soft-hyphen name', () => {
		const { container } = render(BoardTileView, {
			tile: TANKSTELLE_SUED,
			state: EMPTY_STATE,
			onselect: () => {}
		});

		const band = container.querySelector('.band') as HTMLElement;
		expect(band).not.toBeNull();
		// Exactly the group glyph: tankstellen cannot build, so no houses snippet is drawn.
		expect(band.querySelectorAll('svg')).toHaveLength(1);
		expect(getComputedStyle(band).backgroundColor).toBe('rgb(69, 90, 100)');

		const name = container.querySelector('.name') as HTMLElement;
		// breakName() inserts a soft hyphen after "Tank", so the text is not the plain name.
		expect(name.textContent).toBe('Tank\u00ADstelle Süd');
		expect(name).toHaveClass('long');
	});

	it('keeps the utility band icon instead of the gas-pump glyph', () => {
		const tankstelle = render(BoardTileView, {
			tile: TANKSTELLE_SUED,
			state: EMPTY_STATE,
			onselect: () => {}
		});
		const gasGlyph = bandPaths(tankstelle.container);
		expect(gasGlyph.length).toBeGreaterThan(0);
		cleanup();

		const { container } = render(BoardTileView, {
			tile: ELEKTRIZITAETSWERK,
			state: EMPTY_STATE,
			onselect: () => {}
		});
		const band = container.querySelector('.band') as HTMLElement;
		expect(band.querySelectorAll('svg')).toHaveLength(1);
		const utilityGlyph = bandPaths(container);
		expect(utilityGlyph.length).toBeGreaterThan(0);
		expect(utilityGlyph).not.toEqual(gasGlyph);
	});

	it('opens the detail card on click and on Enter', async () => {
		const onselect = vi.fn();
		const { container } = render(BoardTileView, {
			tile: TANKSTELLE_SUED,
			state: EMPTY_STATE,
			onselect
		});
		const field = container.querySelector('.tile') as HTMLElement;

		await fireEvent.click(field);
		expect(onselect).toHaveBeenCalledTimes(1);

		await fireEvent.keyDown(field, { key: 'Enter' });
		expect(onselect).toHaveBeenCalledTimes(2);
	});
});

describe('BoardTile seat focus', () => {
	const ANNA: Player = {
		id: 'anna',
		name: 'Anna',
		color: 'blau',
		seat: 0,
		cash: 1500,
		liquidationValue: 1500,
		position: 5,
		inJail: false,
		jailAttempts: 0,
		jailCards: 0,
		bankrupt: false,
		connected: true,
		netWorth: 1500,
		stats: {
			rentCollected: 0,
			rentPaid: 0,
			propertiesBought: 0,
			propertiesSold: 0,
			housesBuilt: 0,
			trades: 0
		}
	};
	const OWNED: TileState = { owner: 0, houses: 0, mortgaged: false };

	function props(
		selectedSeat: number | null,
		state: TileState,
		onselect: () => void = () => {}
	) {
		return {
			tile: TANKSTELLE_SUED,
			state,
			owner: state.owner >= 0 ? ANNA : undefined,
			selectedSeat,
			onselect
		};
	}

	function root(container: HTMLElement): HTMLElement {
		return container.querySelector('.tile') as HTMLElement;
	}

	it('highlights the owned tile only for its owner and fades it for every other seat', async () => {
		const { container, rerender } = render(BoardTileView, props(0, OWNED));

		expect(root(container)).toHaveClass('highlighted');
		expect(root(container)).not.toHaveClass('faded');

		await rerender(props(1, OWNED));

		expect(root(container)).toHaveClass('faded');
		expect(root(container)).not.toHaveClass('highlighted');

		await rerender(props(null, OWNED));

		expect(root(container)).not.toHaveClass('faded');
		expect(root(container)).not.toHaveClass('highlighted');
	});

	it('fades an unowned tile while any seat is selected', () => {
		const { container } = render(BoardTileView, props(0, EMPTY_STATE));

		expect(root(container)).toHaveClass('faded');
		expect(root(container)).not.toHaveClass('highlighted');
	});

	it('keeps mortgaged styling together with the focus fade', () => {
		const { container } = render(BoardTileView, props(0, { ...EMPTY_STATE, mortgaged: true }));

		expect(root(container)).toHaveClass('mortgaged', 'faded');
		// The focus rule is declared after the mortgage rule, so it wins.
		expect(BoardTileSource.indexOf('.tile.mortgaged')).toBeLessThan(
			BoardTileSource.indexOf('.tile.faded')
		);
	});

	it('keeps a selected owned mortgaged tile highlighted at 0.72 opacity', () => {
		// vitest does not inject Svelte component CSS in jsdom. Apply the real
		// .mortgaged rule from the source so the computed value is meaningful.
		const rule = BoardTileSource.match(/\.tile\.mortgaged\s*\{[^}]*\}/)?.[0];
		expect(rule).toBeDefined();
		const style = document.createElement('style');
		style.textContent = rule as string;
		document.head.appendChild(style);
		try {
			const { container } = render(BoardTileView, props(0, { ...OWNED, mortgaged: true }));
			const tile = root(container);

			expect(tile).toHaveClass('mortgaged', 'highlighted');
			expect(tile).not.toHaveClass('faded');
			expect(getComputedStyle(tile).opacity).toBe('0.72');
		} finally {
			style.remove();
		}
	});

	it('still selects a faded tile on click', async () => {
		const onselect = vi.fn();
		const { container } = render(BoardTileView, props(1, EMPTY_STATE, onselect));

		await fireEvent.click(root(container));

		expect(root(container)).toHaveClass('faded');
		expect(onselect).toHaveBeenCalledTimes(1);
	});

	it('draws the selection ring as a pseudo-element above the owner strip', () => {
		const rule = BoardTileSource.match(/\.tile\.selectable\.highlighted::after\s*\{[^}]*\}/)?.[0];
		expect(rule).toBeDefined();
		for (const declaration of [
			'inset: 0',
			'border: 2px solid var(--md-sys-color-primary)',
			'pointer-events: none',
			'z-index: 4'
		]) {
			expect(rule).toContain(declaration);
		}
		expect(rule).not.toMatch(/box-shadow/);
		expect(BoardTileSource).not.toMatch(/\.tile\.selectable\.highlighted\s*\{[^}]*box-shadow/);
	});
});
