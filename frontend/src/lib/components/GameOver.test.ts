import { describe, expect, it } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/svelte';
import GameOver from './GameOver.svelte';
import gameOverSource from './GameOver.svelte?raw';
import type { GameEvent, GameState, Player, PlayerStats } from '#lib/types';

function stats(overrides: Partial<PlayerStats> = {}): PlayerStats {
	return {
		rentCollected: 0,
		rentPaid: 0,
		rentPaidTo: {},
		propertiesBought: 0,
		propertiesSold: 0,
		housesBuilt: 0,
		trades: 0,
		...overrides
	};
}

function player(seat: number, name: string, overrides: Partial<Player> = {}): Player {
	return {
		id: `user-${seat}`,
		name,
		color: 'blau',
		seat,
		cash: 1500,
		liquidationValue: 1500,
		position: 0,
		inJail: false,
		jailAttempts: 0,
		jailCards: 0,
		bankrupt: false,
		connected: true,
		netWorth: 1500,
		stats: stats(),
		eliminatedAt: 0,
		...overrides
	};
}

function game(overrides: Partial<GameState> = {}): GameState {
	return {
		phase: 'finished',
		rules: {
			minPlayers: 2,
			maxPlayers: 8,
			startingCash: 1500,
			losSalary: 200,
			jailFine: 50,
			auctionSeconds: 15
		},
		board: [],
		players: [player(0, 'Anna'), player(1, 'Ben')],
		tiles: [],
		current: 0,
		debtor: -1,
		turnPhase: 'finished',
		dice: [1, 2],
		doubles: 0,
		trades: [],
		events: [],
		winner: 0,
		...overrides
	};
}

function bustEvent(playerSeat: number): GameEvent {
	return {
		seq: 1,
		type: 'bankruptcy',
		player: playerSeat,
		cause: 'miete',
		debt: 420,
		realizable: 180,
		shortfall: 240,
		text: 'Ben ist pleite. Miete an Berta: 420 GE. Realisierbar: 180 GE. Fehlbetrag: 240 GE. Seine Felder gehen an die Bank.',
		at: '2026-01-01T00:00:00Z'
	};
}

function eliminationEvent(type: string, playerSeat: number, text: string, seq = 1): GameEvent {
	return { seq, type, player: playerSeat, text, at: '2026-01-01T00:00:00Z' };
}

function renderBenReason(events: GameEvent[]): Element | null {
	render(
		GameOver,
		{
			game: game({
				players: [player(0, 'Anna'), player(1, 'Ben', { bankrupt: true, netWorth: 500 })],
				events
			})
		}
	);
	return screen.getByText('Ben').closest('tr')?.querySelector('.reason') ?? null;
}

describe('GameOver', () => {
	it('shows the elimination reason for bankrupt players', () => {
		render(
			GameOver,
			{
				game: game({
					players: [player(0, 'Anna'), player(1, 'Ben', { bankrupt: true, netWorth: 500 })],
					events: [bustEvent(1)]
				})
			}
		);

		expect(screen.getByText('Ausscheiden')).toBeInTheDocument();

		const benReason = screen.getByText('Ben').closest('tr')?.querySelector('.reason');
		expect(benReason).toHaveTextContent('Miete');
		const title = benReason?.getAttribute('title') ?? '';
		expect(title).toContain('Miete');
		expect(title).toContain('240 GE');

		const annaReason = screen.getByText('Anna').closest('tr')?.querySelector('.reason');
		expect(annaReason).toHaveTextContent('–');
	});

	it('shows Aufgegeben for a give_up elimination', () => {
		const reason = renderBenReason([
			eliminationEvent('give_up', 1, 'Ben gibt auf und scheidet aus dem Spiel aus.')
		]);
		expect(reason).toHaveTextContent('Aufgegeben');
		expect(reason?.getAttribute('title')).toContain('Ben gibt auf und scheidet aus dem Spiel aus.');
	});

	it('shows Verlassen for a leave elimination', () => {
		const reason = renderBenReason([
			eliminationEvent('leave', 1, 'Ben verlässt das Spiel.')
		]);
		expect(reason).toHaveTextContent('Verlassen');
		expect(reason?.getAttribute('title')).toContain('Ben verlässt das Spiel.');
	});

	it('keeps the Zahlung fallback for a generic bankruptcy with no cause', () => {
		const reason = renderBenReason([eliminationEvent('bankruptcy', 1, 'Ben ist pleite.')]);
		expect(reason).toHaveTextContent('Zahlung');
	});

	it('sorts bankrupt players last', () => {
		const state = game({
			winner: 2,
			players: [
				player(0, 'Anna', { bankrupt: true, netWorth: 9999, eliminatedAt: 2 }),
				player(1, 'Ben', { netWorth: 100 }),
				player(2, 'Clara', { netWorth: 500 })
			]
		});

		render(GameOver, { game: state });

		const names = [...document.querySelectorAll('tbody tr')].map(
			(row) => row.querySelectorAll('td')[1]?.textContent?.trim() ?? ''
		);
		expect(names).toEqual(['Clara', 'Ben', 'Anna']);
	});

	it('shows a Platz column with ordered placement cells', () => {
		const state = game({
			winner: 3,
			players: [
				player(0, 'Anna', { bankrupt: true, netWorth: 9999, eliminatedAt: 2 }),
				player(1, 'Ben', { bankrupt: true, netWorth: 9999, eliminatedAt: 5 }),
				player(2, 'Clara', { bankrupt: true, netWorth: 9999, eliminatedAt: 5 }),
				player(3, 'Dora', { netWorth: 300 })
			]
		});

		render(GameOver, { game: state });

		expect(screen.getByText('Platz')).toBeInTheDocument();
		const places = [...document.querySelectorAll('tbody tr')].map(
			(row) => row.querySelector('td')?.textContent?.trim() ?? ''
		);
		expect(places).toEqual(['1', '2', '2', '3']);
	});

	function renderRentCell() {
		render(
			GameOver,
			{
				game: game({
					winner: 0,
					players: [
						player(0, 'Anna', {
							stats: stats({ rentPaid: 150, rentPaidTo: { 1: 100, 2: 50 } })
						}),
						player(1, 'Ben'),
						player(2, 'Clara')
					]
				})
			}
		);
		return screen.getByText('Anna').closest('tr')?.querySelector('.rent-cell') as HTMLElement;
	}

	it('keeps the rent-paid total visible without details or tooltip until interaction', () => {
		const rentCell = renderRentCell();
		expect(rentCell).not.toBeNull();
		expect(rentCell).toHaveTextContent('150 GE');
		expect(document.querySelector('details')).toBeNull();
		expect(screen.queryByRole('tooltip')).toBeNull();
	});

	it('shows per-opponent rent lines on hover and hides them on mouseleave', async () => {
		const rentCell = renderRentCell();

		await fireEvent.mouseEnter(rentCell);

		const tooltip = screen.getByRole('tooltip');
		expect(tooltip).toHaveTextContent('an Ben: 100 GE');
		expect(tooltip).toHaveTextContent('an Clara: 50 GE');
		expect(rentCell).toHaveAttribute('aria-describedby', tooltip.id);

		await fireEvent.mouseLeave(rentCell);
		expect(screen.queryByRole('tooltip')).toBeNull();
	});

	it('toggles the rent tooltip on focus and focusout', async () => {
		const rentCell = renderRentCell();

		await fireEvent.focusIn(rentCell);
		expect(screen.getByRole('tooltip')).toBeInTheDocument();

		await fireEvent.focusOut(rentCell);
		expect(screen.queryByRole('tooltip')).toBeNull();
	});

	it('keeps the focused cell tooltip open after hovering and leaving another cell', async () => {
		const annaCell = renderRentCell();
		const benCell = screen.getByText('Ben').closest('tr')?.querySelector('.rent-cell') as HTMLElement;

		await fireEvent.focusIn(annaCell);
		await fireEvent.mouseEnter(benCell);
		await fireEvent.mouseLeave(benCell);

		const tooltip = screen.getByRole('tooltip');
		expect(tooltip).toHaveTextContent('an Ben: 100 GE');
		expect(annaCell).toHaveAttribute('aria-describedby', tooltip.id);
		expect(screen.getAllByRole('tooltip')).toHaveLength(1);
	});

	it('keeps the rent tooltip fully opaque on bankrupt rows', async () => {
		render(
			GameOver,
			{
				game: game({
					players: [
						player(0, 'Anna'),
						player(1, 'Ben', { bankrupt: true, netWorth: 500 })
					],
					events: [bustEvent(1)]
				})
			}
		);
		const benRow = screen.getByText('Ben').closest('tr') as HTMLElement;
		const benCell = benRow.querySelector('.rent-cell') as HTMLElement;
		await fireEvent.mouseEnter(benCell);
		const tooltip = screen.getByRole('tooltip');

		// Documents the browser invariant: opacity composes through ancestors, so any
		// ancestor < 1 makes the tooltip translucent. jsdom in this setup does not inject
		// Svelte's scoped CSS, so this loop cannot detect the rule; the source assertion
		// below is the actual regression guard.
		for (let el: HTMLElement | null = tooltip; el; el = el.parentElement) {
			expect(Number(getComputedStyle(el).opacity)).toBe(1);
		}

		// jsdom does not inject Svelte's scoped <style> block, so inspect the source:
		// the bankrupt row must not dim its descendants via opacity, and the visual
		// distinction must come from muted text and a dimmed dot instead (AC1, AC3).
		const bankruptRule = gameOverSource.match(/tr\.bankrupt\s*\{[^}]*\}/)?.[0] ?? '';
		expect(bankruptRule).not.toMatch(/opacity\s*:/);
		expect(bankruptRule).toContain('var(--md-sys-color-on-surface-variant)');
		expect(gameOverSource).toMatch(/tr\.bankrupt\s+\.dot\s*\{[^}]*opacity:\s*0\.5/);
	});
});
