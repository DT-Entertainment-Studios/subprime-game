import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/svelte';
import EstatePanel from './EstatePanel.svelte';
import type { BoardTile, GameState, Player } from '#lib/types';

const board = [
	{ index: 0, name: 'Los', type: 'go' },
	{
		index: 1,
		name: 'Zollgasse',
		type: 'street',
		group: 'braun',
		price: 60,
		rents: [2, 10, 30, 90, 160, 250],
		build: 50,
		sell: 42
	},
	{
		index: 2,
		name: 'Münzgasse',
		type: 'street',
		group: 'braun',
		price: 60,
		rents: [4, 20, 60, 180, 320, 450],
		build: 50,
		sell: 42
	}
] as GameState['board'];

function player(): Player {
	return {
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
	};
}

function game(mortgaged = false): GameState {
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
		board,
		players: [player()],
		tiles: [
			{ owner: -1, houses: 0, mortgaged: false },
			{ owner: 0, houses: 0, mortgaged },
			{ owner: -1, houses: 0, mortgaged: false }
		],
		current: 0,
		turnPhase: 'awaiting_roll',
		dice: [3, 4],
		doubles: 0,
		trades: [],
		events: [],
		winner: -1
	};
}

describe('EstatePanel', () => {
	it('asks for confirmation before selling a field to the bank', async () => {
		const onIntent = vi.fn();
		render(EstatePanel, { game: game(), me: player(), isMyTurn: true, onIntent });

		await fireEvent.click(screen.getByRole('button', { name: /An Bank/ }));

		const dialog = screen.getByRole('dialog', { name: 'Verkauf an die Bank bestätigen' });
		expect(dialog.textContent).toContain('Zollgasse');
		expect(dialog.textContent).toContain('42 GE');
		expect(onIntent).not.toHaveBeenCalled();

		await fireEvent.click(screen.getByRole('button', { name: 'Ja, verkaufen' }));
		expect(onIntent).toHaveBeenCalledWith({ type: 'sell_to_bank', tile: 1 });
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
	});

	it('keeps the field when the sale is cancelled', async () => {
		const onIntent = vi.fn();
		render(EstatePanel, { game: game(), me: player(), isMyTurn: true, onIntent });

		await fireEvent.click(screen.getByRole('button', { name: /An Bank/ }));
		await fireEvent.click(screen.getByRole('button', { name: 'Abbrechen' }));

		expect(onIntent).not.toHaveBeenCalled();
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
	});

	it('warns that a mortgaged field sells for nothing', async () => {
		render(EstatePanel, {
			game: game(true),
			me: player(),
			isMyTurn: true,
			onIntent: () => {}
		});

		await fireEvent.click(screen.getByRole('button', { name: 'An Bank (0 GE)' }));

		const dialog = screen.getByRole('dialog', { name: 'Verkauf an die Bank bestätigen' });
		expect(dialog.textContent).toContain('0 GE');
		expect(dialog.textContent).toContain('Hypothek verfällt');
	});
});
