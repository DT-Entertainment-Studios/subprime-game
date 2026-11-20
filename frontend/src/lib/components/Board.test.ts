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
				}
			}
		],
		tiles: Array.from({ length: 40 }, () => ({ owner: -1, houses: 0, mortgaged: false })),
		current: 0,
		debtor: -1,
		turnPhase: 'awaiting_roll',
		dice: [3, 4],
		doubles: 0,
		trades: [],
		events: [],
		winner: -1
	};
}

function auctionGame(liquidationValue: number, cash = 20): GameState {
	const state = game();
	state.players[0].cash = cash;
	state.players[0].liquidationValue = liquidationValue;
	state.auction = {
		tile: 1,
		bid: 0,
		bidder: -1,
		minBid: 20,
		endsAt: new Date(Date.now() + 15_000).toISOString(),
		history: []
	};
	return state;
}

/** Two seats, one of them in debt; `debtorSeat` selects who owes. */
function debtGame(debtorSeat: number): GameState {
	const state = game();
	state.turnPhase = 'awaiting_debt';
	state.debtor = debtorSeat;
	state.players.push({
		...state.players[0],
		id: 'user-1',
		name: 'Berta',
		seat: 1,
		cash: -100,
		liquidationValue: 200
	});
	if (debtorSeat === 0) {
		state.players[0].cash = -100;
		state.players[0].liquidationValue = 200;
	}
	return state;
}

describe('Board', () => {
	it('opens the tile detail card when a field is clicked', async () => {
		render(Board, { game: game(), isMyTurn: false, isDebtor: false, me: null, onIntent: () => {} });

		// 'Straße 5' carries no soft hyphen from breakName().
		fireEvent.click(screen.getByText('Straße 5'));

		const dialog = await screen.findByRole('dialog', { name: 'Feld Straße 5' });
		expect(dialog).toBeInTheDocument();
		expect(dialog.textContent).toContain('Mieten');
		expect(dialog.textContent).toContain('2 GE');
	});

	it('offers the Pannenhilfe action in the jail phase', () => {
		const state = game();
		state.turnPhase = 'awaiting_jail';
		state.players[0].inJail = true;
		state.players[0].jailCards = 1;
		render(Board, { game: state, isMyTurn: true, isDebtor: false, me: state.players[0], onIntent: () => {} });

		expect(screen.getByRole('button', { name: 'Pannenhilfe verwenden' })).toBeInTheDocument();
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
		render(Board, { game: state, isMyTurn: false, isDebtor: false, me: state.players[0], onIntent });

		const minus = screen.getByRole('button', { name: 'Gebot um 20 senken' });
		const plus = screen.getByRole('button', { name: 'Gebot um 20 erhöhen' });
		expect(minus).toBeDisabled();

		await fireEvent.click(plus);
		expect(minus).toBeEnabled();
		await fireEvent.click(screen.getByRole('button', { name: 'Bieten' }));

		expect(onIntent).toHaveBeenCalledWith({ type: 'bid', amount: 40 });
	});

	it('allows a bid above cash when the liquidation value covers it', async () => {
		const state = auctionGame(200);
		const onIntent = vi.fn();
		render(Board, { game: state, isMyTurn: false, isDebtor: false, me: state.players[0], onIntent });

		const plus = screen.getByRole('button', { name: 'Gebot um 20 erhöhen' });
		await fireEvent.click(plus);
		await fireEvent.click(plus); // 60 GE, above the 20 GE cash balance
		expect(plus).toBeEnabled();
		await fireEvent.click(screen.getByRole('button', { name: 'Bieten' }));

		expect(onIntent).toHaveBeenCalledWith({ type: 'bid', amount: 60 });
		expect(screen.getByText(/Verkauf und Hypotheken zählen mit/)).toBeInTheDocument();
	});

	it('limits the +20 step by liquidation value, not by cash', async () => {
		const state = auctionGame(40);
		render(Board, {
			game: state,
			isMyTurn: false,
			isDebtor: false,
			me: state.players[0],
			onIntent: () => {}
		});

		const plus = screen.getByRole('button', { name: 'Gebot um 20 erhöhen' });
		expect(plus).toBeEnabled();
		await fireEvent.click(plus); // bid 40, liquidation value exhausted
		expect(plus).toBeDisabled();
	});

	it('hides bid controls when the liquidation value is below the minimum bid', () => {
		const state = auctionGame(10, 10);
		render(Board, {
			game: state,
			isMyTurn: false,
			isDebtor: false,
			me: state.players[0],
			onIntent: () => {}
		});

		expect(screen.queryByRole('button', { name: 'Bieten' })).toBeNull();
		expect(screen.getByText('Du kannst gerade nicht bieten.')).toBeInTheDocument();
	});

	it('names a foreign debtor and reports the blocked game', () => {
		const state = debtGame(1);
		render(Board, {
			game: state,
			isMyTurn: true,
			isDebtor: false,
			me: state.players[0],
			debtor: state.players[1],
			onIntent: () => {}
		});

		expect(screen.getByText(/Berta muss Schulden begleichen/)).toBeInTheDocument();
		expect(screen.getByText(/Das Spiel ist blockiert/)).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Aufgeben' })).toBeNull();
	});

	it('offers the two-step give-up only to the debtor', async () => {
		const state = debtGame(0);
		render(Board, {
			game: state,
			isMyTurn: false,
			isDebtor: true,
			me: state.players[0],
			debtor: state.players[0],
			onIntent: () => {}
		});

		expect(screen.getByText(/Du hast 100 GE Schulden/)).toBeInTheDocument();
		await fireEvent.click(screen.getByRole('button', { name: 'Aufgeben' }));
		expect(screen.getByText(/Wirklich aufgeben/)).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Ja, aufgeben' })).toBeInTheDocument();
	});

	it('resets the give-up confirmation when the debtor changes', async () => {
		const state = debtGame(0);
		const { rerender } = render(Board, {
			game: state,
			isMyTurn: false,
			isDebtor: true,
			me: state.players[0],
			debtor: state.players[0],
			onIntent: () => {}
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Aufgeben' }));
		expect(screen.getByRole('button', { name: 'Ja, aufgeben' })).toBeInTheDocument();

		await rerender({
			game: state,
			isMyTurn: true,
			isDebtor: false,
			me: state.players[0],
			debtor: state.players[1],
			onIntent: () => {}
		});

		expect(screen.queryByRole('button', { name: 'Ja, aufgeben' })).toBeNull();
		expect(screen.queryByText(/Wirklich aufgeben/)).toBeNull();
		expect(screen.getByText(/Berta muss Schulden begleichen/)).toBeInTheDocument();
	});
});
