import { describe, expect, it } from 'vitest';
import { tradeSideItems, type TradeItem } from './trade';
import type { GameState } from './types';

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
		players: [],
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

describe('tradeSideItems', () => {
	it('treats null tiles as an empty list and still returns cash and card chips', () => {
		let items: TradeItem[] = [];
		expect(() => {
			items = tradeSideItems(game(), null, 1, 10);
		}).not.toThrow();
		expect(items.map((item) => item.kind)).toEqual(['cash', 'card']);
		expect(items[0].label).toBe('10 GE');
		expect(items[1].label).toBe('1 Pannenhilfe');
	});

	it('formats a grouped cash amount with the narrow no-break separator', () => {
		const items = tradeSideItems(game(), null, 0, 1500);
		expect(items.map((item) => item.kind)).toEqual(['cash']);
		expect(items[0].label).toBe('1\u202F500 GE');
	});

	it('treats undefined tiles as an empty list', () => {
		let items: TradeItem[] = [];
		expect(() => {
			items = tradeSideItems(game(), undefined, 1, 10);
		}).not.toThrow();
		expect(items.map((item) => item.kind)).toEqual(['cash', 'card']);
	});

	it('produces the same chips for null, undefined and an empty array', () => {
		const fromNull = tradeSideItems(game(), null, 2, 25);
		const fromUndefined = tradeSideItems(game(), undefined, 2, 25);
		const fromEmpty = tradeSideItems(game(), [], 2, 25);
		expect(fromNull).toEqual(fromEmpty);
		expect(fromUndefined).toEqual(fromEmpty);
	});
});
