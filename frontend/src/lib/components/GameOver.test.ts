import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import GameOver from './GameOver.svelte';
import type { GameEvent, GameState, Player } from '#lib/types';

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
		stats: {
			rentCollected: 0,
			rentPaid: 0,
			propertiesBought: 0,
			propertiesSold: 0,
			housesBuilt: 0,
			trades: 0
		},
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

	it('sorts bankrupt players last', () => {
		const state = game({
			winner: 2,
			players: [
				player(0, 'Anna', { bankrupt: true, netWorth: 9999 }),
				player(1, 'Ben', { netWorth: 100 }),
				player(2, 'Clara', { netWorth: 500 })
			]
		});

		render(GameOver, { game: state });

		const names = [...document.querySelectorAll('tbody tr')].map(
			(row) => row.querySelector('td')?.textContent?.trim() ?? ''
		);
		expect(names).toEqual(['Clara', 'Ben', 'Anna']);
	});
});
