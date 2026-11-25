import { describe, expect, it } from 'vitest';
import { render } from '@testing-library/svelte';
import RoadCars from './RoadCars.svelte';
import RoadCarsSource from './RoadCars.svelte?raw';
import type { Player } from '#lib/types';

function player(id: string, name: string, seat: number, position: number): Player {
	return {
		id,
		name,
		color: 'blau',
		seat,
		cash: 1500,
		liquidationValue: 1500,
		position,
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
}

/** The car span of a player (the board only shows names as the span title). */
function carOf(container: HTMLElement, name: string): HTMLElement | null {
	return container.querySelector<HTMLElement>(`.cars-layer .car[title="${name}"]`);
}

describe('RoadCars turn marker', () => {
	it('lights only the car of the current seat', () => {
		const { container } = render(RoadCars, {
			props: {
				players: [player('anna', 'Anna', 0, 3), player('ben', 'Ben', 1, 7)],
				currentSeat: 1,
				events: []
			}
		});

		// jsdom has no layout, so the road is never measured and every car sits
		// on its fallback point; the layer still renders one span per player.
		expect(container.querySelectorAll('.cars-layer .car')).toHaveLength(2);
		expect(container.querySelectorAll('.car.active')).toHaveLength(1);

		const active = container.querySelector<HTMLElement>('.car.active');
		expect(active?.title).toBe('Ben');
		expect(active?.querySelector('svg')?.dataset.lit).toBe('true');
		expect(container.querySelectorAll('svg[data-lit="true"]')).toHaveLength(1);
		expect(container.querySelectorAll('[data-beam]')).toHaveLength(1);
		expect(container.querySelectorAll('[data-pool]')).toHaveLength(0);
	});

	it('lights the current seat in its parking bay', () => {
		const { container } = render(RoadCars, {
			props: {
				// Seat 1 stands on Frei-Parken, seat 0 is on the ring.
				players: [player('anna', 'Anna', 0, 3), player('ben', 'Ben', 1, 20)],
				currentSeat: 1,
				events: []
			}
		});

		const parked = carOf(container, 'Ben');
		expect(parked).not.toBeNull();
		expect(parked).toHaveClass('active');
		expect(parked).toHaveClass('bay');
		expect(parked?.querySelector('svg')?.dataset.lit).toBe('true');

		expect(container.querySelectorAll('svg[data-lit="true"]')).toHaveLength(1);
		expect(document.querySelectorAll('.cars-layer .car.active')).toHaveLength(1);
	});

	it('moves the lit state when the current seat changes', async () => {
		const players = [player('anna', 'Anna', 0, 3), player('ben', 'Ben', 1, 7)];
		const { container, rerender } = render(RoadCars, {
			props: { players, currentSeat: 0, events: [] }
		});

		expect(carOf(container, 'Anna')?.querySelector('svg')?.dataset.lit).toBe('true');
		expect(carOf(container, 'Ben')?.querySelector('svg')?.dataset.lit).toBe('false');

		await rerender({ currentSeat: 1 });

		expect(carOf(container, 'Anna')?.querySelector('svg')?.dataset.lit).toBe('false');
		expect(carOf(container, 'Ben')?.querySelector('svg')?.dataset.lit).toBe('true');
		expect(container.querySelectorAll('svg[data-lit="true"]')).toHaveLength(1);

		const filterId = carOf(container, 'Ben')?.querySelector('defs filter')?.getAttribute('id');
		expect(filterId).toBeTruthy();

		await rerender({ currentSeat: 0 });
		await rerender({ currentSeat: 1 });

		// The car instance is keyed by player id, so its per-instance filter id
		// must survive the on → off → on toggle.
		expect(carOf(container, 'Ben')?.querySelector('defs filter')?.getAttribute('id')).toBe(filterId);
		expect(container.querySelectorAll('svg[data-lit="true"]')).toHaveLength(1);
	});

	it('does not reintroduce an active-car glow', () => {
		expect(RoadCarsSource).not.toMatch(/\.car\.active\s*\{[^}]*filter/);
		expect(RoadCarsSource).not.toContain('0.35rem');
	});
});

describe('RoadCars seat focus', () => {
	const players = [player('anna', 'Anna', 0, 3), player('ben', 'Ben', 1, 7)];
	const props = (selectedSeat: number | null) => ({
		players,
		currentSeat: 0,
		events: [],
		selectedSeat
	});

	it('fades every car except the selected player’s', () => {
		const { container } = render(RoadCars, { props: props(1) });

		expect(carOf(container, 'Ben')).not.toHaveClass('faded');
		expect(carOf(container, 'Anna')).toHaveClass('faded');
	});

	it('clears the fade without a selection and moves it with the seat', async () => {
		const { container, rerender } = render(RoadCars, { props: props(1) });

		await rerender({ selectedSeat: null });

		expect(container.querySelectorAll('.car.faded')).toHaveLength(0);

		await rerender({ selectedSeat: 0 });

		expect(carOf(container, 'Anna')).not.toHaveClass('faded');
		expect(carOf(container, 'Ben')).toHaveClass('faded');
	});

	it('never fades the layer itself and renders no car for a bankrupt player', () => {
		const { container } = render(RoadCars, {
			props: {
				players: [...players, { ...player('carla', 'Carla', 2, 11), bankrupt: true }],
				currentSeat: 0,
				events: [],
				selectedSeat: 0
			}
		});

		expect(container.querySelector('.cars-layer')).not.toHaveClass('faded');
		expect(carOf(container, 'Carla')).toBeNull();
		expect(carOf(container, 'Ben')).toHaveClass('faded');
	});
});
