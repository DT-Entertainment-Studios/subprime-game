import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import TileCard from './TileCard.svelte';
import type { BoardTile, GameState, Player, TileState } from '#lib/types';

const RENTS_STREET = [2, 10, 30, 90, 160, 250];

const board = [
	{ index: 0, name: 'Los', type: 'go' },
	{ index: 1, name: 'Zollgasse', type: 'street', group: 'braun', price: 60, rents: RENTS_STREET, build: 50, sell: 42 },
	{ index: 2, name: 'Münzgasse', type: 'street', group: 'braun', price: 60, rents: [4, 20, 60, 180, 320, 450], build: 50, sell: 42 },
	{ index: 4, name: 'Einkommensteuer', type: 'tax', taxPercent: 10 },
	{ index: 11, name: 'Gartenstraße', type: 'street', group: 'rosa', price: 120, rents: [4, 20, 60, 180, 320, 450], build: 60, sell: 50 },
	{ index: 5, name: 'Tankstelle Süd', type: 'tankstelle', group: 'tankstelle', price: 200, rents: [25, 50, 100, 200], sell: 140 },
	{ index: 7, name: 'Ereignisfeld', type: 'chance' },
	{ index: 12, name: 'Elektrizitätswerk', type: 'utility', group: 'versorger', price: 150, sell: 105 },
	{ index: 15, name: 'Tankstelle West', type: 'tankstelle', group: 'tankstelle', price: 200, rents: [25, 50, 100, 200], sell: 140 },
	{ index: 38, name: 'Zusatzsteuer', type: 'tax', tax: 100 }
] as GameState['board'];

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
			auctionSeconds: 15,
		},
		board,
		players: [player(0, 'Anton'), player(1, 'Berta')],
		tiles: [],
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

function tileState(_index: number, over: Partial<TileState> = {}): TileState {
	return { owner: -1, houses: 0, mortgaged: false, ...over };
}

/** Full TileState array (positions = board indexes) with the given overrides. */
function withTiles(g: GameState, entries: Record<number, Partial<TileState>>): GameState {
	g.tiles = Array.from({ length: 40 }, () => tileState(-1));
	for (const [idx, over] of Object.entries(entries)) {
		Object.assign(g.tiles[Number(idx)], over);
	}
	return g;
}

function propsFor(tileIndex: number, state: Partial<TileState> = {}, owner?: Player) {
	const g = withTiles(game(), { [tileIndex]: state });
	const tile = board.find((t) => t.index === tileIndex) as BoardTile;
	return { tile, tileState: g.tiles[tileIndex], game: g, owner, onclose: () => {} };
}

describe('TileCard', () => {
	it('shows the rent ladder and marks the currently applicable rent', () => {
		const owner = player(0, 'Anton');
		render(TileCard, propsFor(1, tileState(1, { owner: 0, houses: 2 }), owner));

		// The build level is both a status chip and a rent row label.
		expect(screen.getAllByText('2 Häuser')).toHaveLength(2);
		expect(screen.getByText('90 GE')).toBeInTheDocument();
		expect(screen.getByText('kein Haus')).toBeInTheDocument();
		expect(screen.getAllByText('jetzt fällig')).toHaveLength(1);
		expect(screen.getByText(/Besitzer:/)).toHaveTextContent('Anton');
	});

	it('doubles the unbuilt rent when the owner holds the complete group', () => {
		const g = withTiles(game(), { 1: { owner: 0 }, 2: { owner: 0 } });
		render(TileCard, { tile: board[1], tileState: g.tiles[1], game: g, owner: g.players[0], onclose: () => {} });

		expect(screen.getByText('4 GE')).toBeInTheDocument();
		expect(screen.getByText(/ganze Farbgruppe/)).toBeInTheDocument();
		// The "kein Haus" row is the current one.
		expect(screen.getAllByText('jetzt fällig')).toHaveLength(1);
	});

	it('explains mortgage state, redeem price and the zero bank sale', () => {
		const owner = player(0, 'Anton');
		render(TileCard, propsFor(1, tileState(1, { owner: 0, mortgaged: true }), owner));

		expect(screen.getByText(/hypothekiert/)).toBeInTheDocument();
		expect(screen.getByText('Ablöse (60 %)')).toBeInTheDocument();
		expect(screen.getByText('36 GE')).toBeInTheDocument();
		// The bank takes the field for nothing while mortgaged.
		expect(screen.getByText(/Hypothek verfällt/)).toBeInTheDocument();
		// No rent is due while mortgaged.
		expect(screen.queryByText('jetzt fällig')).toBeNull();
	});

	it('shows house cost, bank sale price and ownership chips', () => {
		const owner = player(0, 'Anton');
		render(TileCard, propsFor(1, tileState(1, { owner: 0 }), owner));

		// The build cost appears once as a fact and once as a rent row label.
		expect(screen.getAllByText('1 Haus')).toHaveLength(2);
		expect(screen.getByText('50 GE')).toBeInTheDocument();
		expect(screen.getByText('Verkauf an die Bank')).toBeInTheDocument();
		expect(screen.getByText('42 GE')).toBeInTheDocument();
		expect(screen.getByText('unbaut')).toBeInTheDocument();
		expect(screen.getByText(/Besitzer:/)).toHaveTextContent('Anton');
	});

	it('shows tankstelle rent ranks without house costs', () => {
		const owner = player(0, 'Anton');
		render(TileCard, propsFor(5, { owner: 0, houses: 0, mortgaged: false }, owner));

		expect(screen.getByText('1 Tankstelle')).toBeInTheDocument();
		expect(screen.getByText('4 Tankstellen')).toBeInTheDocument();
		expect(screen.queryByText('1 Haus')).toBeNull();
		expect(screen.queryByText(/Hotel/)).toBeNull();
	});

	it('shows the percentage for a percentage tax field', () => {
		render(TileCard, propsFor(4, tileState(4)));
		expect(screen.getByText(/10 % des Bargelds/)).toBeInTheDocument();
		expect(screen.queryByText('kein Haus')).toBeNull();
	});

	it('shows the fixed amount for a fixed tax field', () => {
		render(TileCard, propsFor(38, tileState(38)));
		expect(screen.getByText(/100 GE/)).toBeInTheDocument();
	});

	it('mentions the chance card deck rule on chance fields', () => {
		render(TileCard, propsFor(7, tileState(7)));
		expect(screen.getByText(/Ereigniskarte/)).toBeInTheDocument();
	});

	it('shows no share hint and full rent when the matching utility is unowned', () => {
		render(TileCard, propsFor(11, tileState(11, { owner: 0 }), player(0, 'Anton')));
		expect(screen.getByText(/unverkauft/)).toHaveTextContent(/kein Nutzungsanteil/);
		expect(screen.getByText(/volle Miete/)).toBeInTheDocument();
		expect(screen.queryByText(/fließen an die Bank/)).toBeNull();
	});
});
