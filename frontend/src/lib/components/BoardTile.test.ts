import { describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render } from '@testing-library/svelte';
import BoardTileView from './BoardTile.svelte';
import type { BoardTile, TileState } from '#lib/types';

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
