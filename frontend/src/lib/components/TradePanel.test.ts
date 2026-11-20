import { afterEach, describe, expect, it } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import TradePanel from './TradePanel.svelte';
import { tradeUI } from '#lib/trade-ui.svelte';
import type { GameState, Player, TradeOffer } from '#lib/types';

function player(seat: number, name: string): Player {
	return {
		id: `user-${seat}`,
		name,
		color: 'blau',
		seat,
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
	};
}

// The recipient is "me"; the sender's offer has cash on each side but null
// tile lists, the exact snapshot that used to crash startCounter's .filter().
function offer(): TradeOffer {
	return {
		id: 'trade-1',
		from: 1,
		to: 0,
		offerTiles: null,
		requestTiles: null,
		offerCash: 10,
		requestCash: 5,
		offerCards: 0,
		requestCards: 0,
		status: 'open',
		createdAt: '2026-01-01T00:00:00Z'
	} as unknown as TradeOffer;
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
			auctionSeconds: 15
		},
		board: [],
		players: [player(0, 'Anton'), player(1, 'Berta')],
		tiles: [],
		current: 0,
		turnPhase: 'awaiting_roll',
		dice: [1, 2],
		doubles: 0,
		trades: [offer()],
		events: [],
		winner: -1
	};
}

describe('TradePanel startCounter', () => {
	afterEach(() => {
		tradeUI.counterRespond = null;
	});

	it('prefills a counter-offer from an offer with null tile lists via the button', async () => {
		render(TradePanel, { game: game(), me: player(0, 'Anton'), onIntent: () => {} });

		await fireEvent.click(screen.getByRole('button', { name: 'Gegenangebot' }));

		expect(screen.getByRole('heading', { name: 'Gegengebot' })).toBeInTheDocument();
	});

	it('prefills a counter-offer with null tile lists via a counter request', async () => {
		tradeUI.counterRespond = 'trade-1';
		render(TradePanel, { game: game(), me: player(0, 'Anton'), onIntent: () => {} });

		await waitFor(() => {
			expect(screen.getByRole('heading', { name: 'Gegengebot' })).toBeInTheDocument();
		});
	});
});
