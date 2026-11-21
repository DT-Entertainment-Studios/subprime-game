import { describe, expect, it } from 'vitest';
import { rankedStandings } from './placement';
import type { Player } from './types';

function player(seat: number, overrides: Partial<Player> = {}): Player {
	return {
		id: `user-${seat}`,
		name: `Spieler ${seat}`,
		color: 'blau',
		seat,
		cash: 1000,
		liquidationValue: 1000,
		position: 0,
		inJail: false,
		jailAttempts: 0,
		jailCards: 0,
		bankrupt: false,
		connected: true,
		netWorth: 1000,
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

describe('rankedStandings', () => {
	it('ranks a survivor ahead of bankrupt players', () => {
		const result = rankedStandings([
			player(0, { bankrupt: true, eliminatedAt: 9 }),
			player(1, { netWorth: 500 })
		]);

		expect(result.map((entry) => entry.player.seat)).toEqual([1, 0]);
		expect(result.map((entry) => entry.place)).toEqual([1, 2]);
	});

	it('ranks a later bankruptcy ahead of an earlier one', () => {
		const result = rankedStandings([
			player(0, { bankrupt: true, eliminatedAt: 3 }),
			player(1, { bankrupt: true, eliminatedAt: 7 })
		]);

		expect(result.map((entry) => entry.player.seat)).toEqual([1, 0]);
		expect(result.map((entry) => entry.place)).toEqual([1, 2]);
	});

	it('gives players with equal eliminatedAt the same place', () => {
		const result = rankedStandings([
			player(0, { bankrupt: true, eliminatedAt: 5 }),
			player(1, { bankrupt: true, eliminatedAt: 5 })
		]);

		expect(result.map((entry) => entry.place)).toEqual([1, 1]);
	});

	it('uses dense numbering across survivors and tied bankrupt groups', () => {
		const result = rankedStandings([
			player(0, { bankrupt: true, eliminatedAt: 2 }),
			player(1, { bankrupt: true, eliminatedAt: 4 }),
			player(2, { netWorth: 1500 }),
			player(3, { netWorth: 500 })
		]);

		expect(result.map((entry) => entry.player.seat)).toEqual([2, 3, 1, 0]);
		expect(result.map((entry) => entry.place)).toEqual([1, 1, 2, 3]);
	});

	it('handles an empty input', () => {
		expect(rankedStandings([])).toEqual([]);
	});

	it('ranks all-bankrupt players from 1 without a survivor', () => {
		const result = rankedStandings([
			player(0, { bankrupt: true, eliminatedAt: 1 }),
			player(1, { bankrupt: true, eliminatedAt: 3 }),
			player(2, { bankrupt: true, eliminatedAt: 2 })
		]);

		expect(result.map((entry) => entry.player.seat)).toEqual([1, 2, 0]);
		expect(result.map((entry) => entry.place)).toEqual([1, 2, 3]);
	});
});
