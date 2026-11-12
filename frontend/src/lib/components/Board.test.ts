import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/svelte';
import Board from './Board.svelte';
import type { BoardTile, GameState } from '#lib/types';

/** A minimal-but-complete 40-tile board (corners + buyable fields). */
function board(): BoardTile[] {
	const tiles: BoardTile[] = [];
	for (let index = 0; index < 40; index++) {
		if (index % 10 === 0) {
			const type = index === 0 ? 'go' : index === 10 ? 'jail' : index === 20 ? 'parking' : 'gotojail';
			tiles.push({ index, name: `Feld ${index}`, type });
		} else {
			tiles.push({
				index,
				name: index === 1 ? 'Zollgasse' : `Straße ${index}`,
				type: 'street',
				group: 'braun',
				price: 60,
				rents: [2, 10, 30, 90, 160, 250],
				build: 50,
				sell: 42
			});
		}
	}
	return tiles;
}

function game(): GameState {
	return {
		phase: 'playing',
		rules: {
			minPlayers: 2,
			maxPlayers: 8,
			startingCash: 1500,
			losSalary: 200,
			jailFine: 50,
			auctionSeconds: 15,
		},
		board: board(),
		players: [
			{
				id: 'user-0',
				name: 'Anton',
				color: 'blau',
				seat: 0,
				cash: 1500,
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
				}
			}
		],
		tiles: Array.from({ length: 40 }, () => ({ owner: -1, houses: 0, mortgaged: false })),
		current: 0,
		turnPhase: 'awaiting_roll',
		dice: [3, 4],
		doubles: 0,
		trades: [],
		events: [],
		winner: -1
	};
}

describe('Board', () => {
	it('opens the tile detail card when a field is clicked', async () => {
		render(Board, { game: game(), isMyTurn: false, me: null, onIntent: () => {} });

		// 'Straße 5' carries no soft hyphen from breakName().
		fireEvent.click(screen.getByText('Straße 5'));

		const dialog = await screen.findByRole('dialog', { name: 'Feld Straße 5' });
		expect(dialog).toBeInTheDocument();
		expect(dialog.textContent).toContain('Mieten');
		expect(dialog.textContent).toContain('2 GE');
	});

	it('bids in 20-steps with separate minus and plus buttons', async () => {
		const state = game();
		state.auction = {
			tile: 1,
			bid: 0,
			bidder: -1,
			minBid: 20,
			endsAt: new Date(Date.now() + 15_000).toISOString(),
			history: []
		};
		const onIntent = vi.fn();
		render(Board, { game: state, isMyTurn: false, me: state.players[0], onIntent });

		const minus = screen.getByRole('button', { name: 'Gebot um 20 senken' });
		const plus = screen.getByRole('button', { name: 'Gebot um 20 erhöhen' });
		expect(minus).toBeDisabled();

		await fireEvent.click(plus);
		expect(minus).toBeEnabled();
		await fireEvent.click(screen.getByRole('button', { name: 'Bieten' }));

		expect(onIntent).toHaveBeenCalledWith({ type: 'bid', amount: 40 });
	});
});
