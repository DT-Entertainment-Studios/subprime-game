import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, within } from '@testing-library/svelte';
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
		debtor: -1,
		turnPhase: 'awaiting_roll',
		dice: [3, 4],
		doubles: 0,
		trades: [],
		events: [],
		winner: -1
	};
}

function gameWithLevels(tile1: number, tile2: number): GameState {
	const g = game();
	g.tiles[1] = { owner: 0, houses: tile1, mortgaged: false };
	g.tiles[2] = { owner: 0, houses: tile2, mortgaged: false };
	return g;
}

/** A debtor with a house, a mortgaged field and a free field to liquidate. */
function debtorGame(): GameState {
	const state = game();
	state.tiles[1] = { owner: 0, houses: 1, mortgaged: false };
	state.tiles[2] = { owner: 0, houses: 0, mortgaged: true };
	state.board = [
		...state.board,
		{
			index: 3,
			name: 'Kanalgasse',
			type: 'street',
			group: 'hellblau',
			price: 100,
			rents: [6, 30, 90, 270, 400, 550],
			build: 50,
			sell: 70
		}
	];
	state.tiles = [...state.tiles, { owner: 0, houses: 0, mortgaged: false }];
	return state;
}

describe('EstatePanel', () => {
	it('asks for confirmation before selling a field to the bank', async () => {
		const onIntent = vi.fn();
		render(EstatePanel, { game: game(), me: player(), isMyTurn: true, isDebtor: false, onIntent });

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
		render(EstatePanel, { game: game(), me: player(), isMyTurn: true, isDebtor: false, onIntent });

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
			isDebtor: false,
			onIntent: () => {}
		});

		await fireEvent.click(screen.getByRole('button', { name: 'An Bank (0 GE)' }));

		const dialog = screen.getByRole('dialog', { name: 'Verkauf an die Bank bestätigen' });
		expect(dialog.textContent).toContain('0 GE');
		expect(dialog.textContent).toContain('Hypothek verfällt');
	});

	it('disables bank sale when another street of the color group has buildings', () => {
		render(EstatePanel, {
			game: gameWithLevels(1, 0),
			me: player(),
			isMyTurn: true,
			isDebtor: false,
			onIntent: () => {}
		});

		const buttons = screen.getAllByRole('button', { name: 'An Bank (42 GE)' });
		expect(buttons).toHaveLength(2);
		expect(buttons[0]).toBeDisabled();
		expect(buttons[1]).toBeDisabled();
	});

	it('drops the bank-sale dialog when the color group becomes built', async () => {
		const onIntent = vi.fn();
		const { rerender } = render(EstatePanel, {
			game: game(),
			me: player(),
			isMyTurn: true,
			isDebtor: false,
			onIntent
		});

		await fireEvent.click(screen.getByRole('button', { name: 'An Bank (42 GE)' }));
		expect(screen.getByRole('dialog', { name: 'Verkauf an die Bank bestätigen' })).toBeInTheDocument();

		const built = game();
		built.tiles[2] = { owner: 0, houses: 1, mortgaged: false };
		await rerender({ game: built, me: player(), isMyTurn: true, isDebtor: false, onIntent });

		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
		expect(onIntent).not.toHaveBeenCalled();
	});

	it('only enables selling from the highest-level street in the group', () => {
		render(EstatePanel, {
			game: gameWithLevels(2, 1),
			me: player(),
			isMyTurn: true,
			isDebtor: false,
			onIntent: () => {}
		});

		const buttons = screen.getAllByRole('button', { name: 'Haus verkaufen' });
		expect(buttons).toHaveLength(2);
		expect(buttons[0]).toBeEnabled(); // tile 1, level 2 (group maximum)
		expect(buttons[1]).toBeDisabled(); // tile 2, level 1
	});

	it('treats a hotel as the highest level when enabling selling', () => {
		render(EstatePanel, {
			game: gameWithLevels(5, 4),
			me: player(),
			isMyTurn: true,
			isDebtor: false,
			onIntent: () => {}
		});

		const buttons = screen.getAllByRole('button', { name: 'Haus verkaufen' });
		expect(buttons).toHaveLength(2);
		expect(buttons[0]).toBeEnabled(); // tile 1, hotel (level 5, maximum)
		expect(buttons[1]).toBeDisabled(); // tile 2, level 4
	});

	it('lets a debtor liquidate while another player holds the turn', () => {
		render(EstatePanel, {
			game: debtorGame(),
			me: player(),
			isMyTurn: false,
			isDebtor: true,
			onIntent: () => {}
		});

		expect(screen.getByRole('button', { name: 'Hypothek (50 GE)' })).toBeEnabled();
		const sellButtons = screen.getAllByRole('button', { name: 'Haus verkaufen' });
		expect(sellButtons).toHaveLength(3);
		expect(sellButtons[0]).toBeEnabled(); // tile 1, the only house
		expect(sellButtons[1]).toBeDisabled(); // tile 2
		expect(sellButtons[2]).toBeDisabled(); // tile 3
		// Main's building-free-group rule still applies to debtors: tile 2 shares
		// the braun group with tile 1's house. The clean hellblau field can still be
		// liquidated out of turn.
		expect(screen.getByRole('button', { name: 'An Bank (0 GE)' })).toBeDisabled();
		expect(screen.getByRole('button', { name: 'An Bank (70 GE)' })).toBeEnabled();

		const buildButtons = screen.getAllByRole('button', { name: /Bauen/ });
		expect(buildButtons.length).toBeGreaterThan(0);
		for (const button of buildButtons) expect(button).toBeDisabled();
		expect(screen.getByRole('button', { name: /Ablösen/ })).toBeDisabled();
		expect(screen.getByText(/darf Felder auch liquidieren/)).toBeInTheDocument();
	});

	it('keeps the action row in place when a field is mortgaged', async () => {
		const { rerender } = render(EstatePanel, {
			game: game(),
			me: player(),
			isMyTurn: true,
			isDebtor: false,
			onIntent: () => {}
		});

		// game() does not own the whole braun group, so canBuild(1) fails.
		expect(screen.getByRole('button', { name: 'Bauen (50 GE)' })).toBeDisabled();
		expect(screen.getByRole('button', { name: 'Hypothek (30 GE)' })).toBeEnabled();
		expect(screen.queryByRole('button', { name: /Ablösen/ })).not.toBeInTheDocument();

		await rerender({
			game: game(true),
			me: player(),
			isMyTurn: true,
			isDebtor: false,
			onIntent: () => {}
		});

		expect(screen.getByRole('button', { name: 'Bauen (50 GE)' })).toBeDisabled();
		expect(screen.getByRole('button', { name: 'Ablösen (36 GE)' })).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: /Hypothek/ })).toBeNull();
		expect(screen.getByRole('button', { name: 'Haus verkaufen' })).toBeDisabled();
		expect(screen.getByRole('button', { name: 'An Bank (0 GE)' })).toBeEnabled();
	});

	it('renders no building buttons for tiles without a build value', () => {
		const tankstelle = game();
		tankstelle.board = [
			...tankstelle.board,
			{
				index: 3,
				name: 'Tankstelle Nord',
				type: 'tankstelle',
				price: 200,
				sell: 140
			}
		];
		tankstelle.tiles = [...tankstelle.tiles, { owner: 0, houses: 0, mortgaged: false }];

		render(EstatePanel, {
			game: tankstelle,
			me: player(),
			isMyTurn: true,
			isDebtor: false,
			onIntent: () => {}
		});

		const li = screen.getByText('Tankstelle Nord').closest('li') as HTMLElement;
		expect(within(li).queryByRole('button', { name: /Bauen/ })).not.toBeInTheDocument();
		expect(within(li).queryByRole('button', { name: 'Haus verkaufen' })).not.toBeInTheDocument();
		expect(within(li).getByRole('button', { name: 'Hypothek (100 GE)' })).toBeInTheDocument();
		expect(within(li).getByRole('button', { name: 'An Bank (140 GE)' })).toBeInTheDocument();
	});
});
