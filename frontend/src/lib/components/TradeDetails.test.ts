import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import TradeDetails from './TradeDetails.svelte';
import type { GameState, Player, TradeOffer } from '#lib/types';

function player(seat: number, name: string): Player {
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
		}
	};
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
		debtor: -1,
		turnPhase: 'awaiting_roll',
		dice: [1, 2],
		doubles: 0,
		trades: [],
		events: [],
		winner: -1
	};
}

function trade(overrides: Partial<TradeOffer> = {}): TradeOffer {
	return {
		id: 'trade-1',
		from: 0,
		to: 1,
		offerTiles: [],
		requestTiles: [],
		offerCash: 10,
		requestCash: 5,
		offerCards: 0,
		requestCards: 0,
		status: 'open',
		createdAt: '2026-01-01T00:00:00Z',
		...overrides
	};
}

// Old snapshots may carry null instead of [] for empty tile lists.
const nullTiles = { offerTiles: null, requestTiles: null } as unknown as Partial<TradeOffer>;

describe('TradeDetails', () => {
	it('renders an offer with null tile lists without a TypeError and shows the cash chips', () => {
		expect(() => {
			render(TradeDetails, { game: game(), trade: trade(nullTiles) });
		}).not.toThrow();

		expect(screen.getByText('Anton gibt')).toBeInTheDocument();
		expect(screen.getByText('Berta gibt')).toBeInTheDocument();
		expect(screen.getByText('10 GE')).toBeInTheDocument();
		expect(screen.getByText('5 GE')).toBeInTheDocument();
	});

	it('renders empty tile lists the same way as null tile lists', () => {
		render(TradeDetails, { game: game(), trade: trade() });

		expect(screen.getByText('10 GE')).toBeInTheDocument();
		expect(screen.getByText('5 GE')).toBeInTheDocument();
	});
});
