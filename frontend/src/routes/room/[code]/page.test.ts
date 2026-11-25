import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import type { BoardTile, GameState, Player, RoomSnapshot } from '#lib/types';
import Page from './+page.svelte';

// The room page is a wiring layer: session, presence, prefs and the router are
// mocked so the test drives the real Board/PlayersList components without
// sockets or REST. `session.room` and `page.params.code` are backed by Svelte
// signals because the page caches `$derived(...)` of both; a plain object would
// never invalidate them.
interface SessionMock {
	room: RoomSnapshot | null;
	me: Player | null;
	isMyTurn: boolean;
	isDebtor: boolean;
	connected: boolean;
	error: string | null;
	init: ReturnType<typeof vi.fn>;
	load: ReturnType<typeof vi.fn>;
	connect: ReturnType<typeof vi.fn>;
	disconnect: ReturnType<typeof vi.fn>;
	send: ReturnType<typeof vi.fn>;
	join: ReturnType<typeof vi.fn>;
	leave: ReturnType<typeof vi.fn>;
	clearError: ReturnType<typeof vi.fn>;
}

const mocks = vi.hoisted(() => ({
	goto: vi.fn(),
	page: null as unknown as { params: { code: string } },
	setCode: null as unknown as (code: string) => void,
	session: null as unknown as SessionMock,
	setRoom: null as unknown as (room: RoomSnapshot | null) => void,
	presence: {
		online: [],
		sent: [],
		error: null as string | null,
		connectRoom: vi.fn(),
		disconnectRoom: vi.fn(),
		invite: vi.fn(),
		cancel: vi.fn(),
		clearError: vi.fn()
	},
	preferences: {
		current: { theme: 'light', zoom: 1, estatePanelOpen: false, hideCards: false },
		update: vi.fn(),
		toggleTheme: vi.fn()
	}
}));

vi.mock('$app/navigation', () => ({ goto: mocks.goto }));
vi.mock('$app/state', async () => {
	// Same isolated internal-signal workaround as for session.room below.
	// @ts-expect-error untyped internal entry point
	const { get, set, state } = await import('svelte/internal/client');
	const codeSource = state('abcdef');
	mocks.page = {
		params: {
			get code() {
				return get(codeSource);
			}
		}
	};
	mocks.setCode = (code) => set(codeSource, code);
	return { page: mocks.page };
});
vi.mock('#lib/presence.svelte', () => ({ presence: mocks.presence }));
vi.mock('#lib/prefs.svelte', () => ({ preferences: mocks.preferences }));
vi.mock('#lib/session.svelte', async () => {
	// Internal API is used only to make the mocked store reactive: runes are not
	// available in a .test.ts module, and Svelte ships no types for this entry.
	// @ts-expect-error untyped internal entry point
	const { get, set, state } = await import('svelte/internal/client');
	const roomSource = state(null);
	const session: SessionMock = {
		get room() {
			return get(roomSource);
		},
		me: null,
		isMyTurn: false,
		isDebtor: false,
		connected: true,
		error: null,
		init: vi.fn().mockResolvedValue(undefined),
		load: vi.fn().mockResolvedValue(undefined),
		connect: vi.fn(),
		disconnect: vi.fn(),
		send: vi.fn(),
		join: vi.fn().mockResolvedValue(undefined),
		leave: vi.fn().mockResolvedValue(true),
		clearError: vi.fn()
	};
	mocks.session = session;
	mocks.setRoom = (room) => set(roomSource, room);
	return { session };
});

function player(id: string, name: string, seat: number, netWorth: number): Player {
	return {
		id,
		name,
		color: 'blau',
		seat,
		cash: 1500,
		liquidationValue: 1500,
		position: seat + 1,
		inJail: false,
		jailAttempts: 0,
		jailCards: 0,
		bankrupt: false,
		connected: seat !== 2,
		netWorth,
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

const BOARD: BoardTile[] = [
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
		name: 'Bergstraße',
		type: 'street',
		group: 'braun',
		price: 60,
		rents: [2, 10, 30, 90, 160, 250],
		build: 50,
		sell: 42
	}
];

function snapshot(players: Player[], owners: number[] = [0, 1, -1]): RoomSnapshot {
	const game: GameState = {
		phase: 'playing',
		rules: {
			minPlayers: 2,
			maxPlayers: 8,
			startingCash: 1500,
			losSalary: 200,
			jailFine: 50,
			auctionSeconds: 15
		},
		board: BOARD,
		players,
		tiles: owners.map((owner) => ({ owner, houses: 0, mortgaged: false })),
		current: 0,
		debtor: -1,
		turnPhase: 'awaiting_roll',
		dice: [3, 4],
		doubles: 0,
		trades: [],
		events: [],
		winner: -1
	};
	return {
		code: 'ABCDEF',
		phase: 'playing',
		hostId: 'anna',
		createdAt: '2026-01-01T00:00:00Z',
		seats: [],
		kickVotes: [],
		you: { id: 'anna', name: 'Anna', role: 'player', seat: 0, isHost: true },
		canStart: false,
		game
	};
}

const PLAYERS = [
	player('anna', 'Anna', 0, 3000),
	player('ben', 'Ben', 1, 2000),
	player('carla', 'Carla', 2, 1000)
];

function nameButton(name: string): HTMLElement {
	return screen.getByRole('button', { name }) as HTMLElement;
}

function classes() {
	return {
		selected: document.querySelectorAll('.player.selected').length,
		tilesFaded: document.querySelectorAll('.tile.faded').length,
		tilesHighlighted: document.querySelectorAll('.tile.highlighted').length,
		carsFaded: document.querySelectorAll('.car.faded').length
	};
}

beforeEach(() => {
	mocks.goto.mockReset();
	mocks.setCode('abcdef');
	mocks.session.error = null;
	mocks.session.isMyTurn = false;
	mocks.session.isDebtor = false;
	mocks.session.me = PLAYERS[0];
	mocks.session.send.mockClear();
	mocks.presence.connectRoom.mockClear();
	mocks.presence.disconnectRoom.mockClear();
	mocks.preferences.current = { theme: 'light', zoom: 1, estatePanelOpen: false, hideCards: false };
	mocks.setRoom(snapshot(PLAYERS));
});

describe('room page player focus', () => {
	it('starts without a selection or faded fields and cars', () => {
		render(Page);

		expect(nameButton('Anna')).toHaveAttribute('aria-pressed', 'false');
		expect(classes()).toEqual({ selected: 0, tilesFaded: 0, tilesHighlighted: 0, carsFaded: 0 });
	});

	it('selects a seat from its name and fades everything else', async () => {
		render(Page);

		await fireEvent.click(nameButton('Anna'));

		expect(nameButton('Anna')).toHaveAttribute('aria-pressed', 'true');
		expect(nameButton('Anna').closest('li')).toHaveClass('selected');
		expect(classes()).toEqual({ selected: 1, tilesFaded: 2, tilesHighlighted: 1, carsFaded: 2 });
	});

	it('selects from the row background and fades other fields and cars', async () => {
		render(Page);
		const row = nameButton('Anna').closest('li') as HTMLElement;

		await fireEvent.click(row);

		expect(nameButton('Anna')).toHaveAttribute('aria-pressed', 'true');
		expect(row).toHaveClass('selected');
		expect(classes()).toEqual({ selected: 1, tilesFaded: 2, tilesHighlighted: 1, carsFaded: 2 });
	});

	it('deselects on a second click and moves the focus to another name', async () => {
		render(Page);

		await fireEvent.click(nameButton('Anna'));
		expect(classes()).toEqual({ selected: 1, tilesFaded: 2, tilesHighlighted: 1, carsFaded: 2 });

		await fireEvent.click(nameButton('Anna'));
		expect(classes()).toEqual({ selected: 0, tilesFaded: 0, tilesHighlighted: 0, carsFaded: 0 });
		expect(nameButton('Anna')).toHaveAttribute('aria-pressed', 'false');

		await fireEvent.click(nameButton('Ben'));
		expect(nameButton('Ben')).toHaveAttribute('aria-pressed', 'true');
		expect(nameButton('Ben').closest('li')).toHaveClass('selected');
		expect(classes()).toEqual({ selected: 1, tilesFaded: 2, tilesHighlighted: 1, carsFaded: 2 });
	});

	it('keeps the selection when votekick is clicked', async () => {
		render(Page);
		await fireEvent.click(nameButton('Anna'));
		const before = classes();

		await fireEvent.click(screen.getByRole('button', { name: 'Votekick gegen Carla' }));

		expect(mocks.session.send).toHaveBeenCalledWith({ type: 'votekick', targetId: 'carla' });
		expect(nameButton('Anna')).toHaveAttribute('aria-pressed', 'true');
		expect(classes()).toEqual(before);
	});

	it('clears the selection when the focused seat disappears', async () => {
		render(Page);
		await fireEvent.click(nameButton('Ben'));
		expect(classes()).toEqual({ selected: 1, tilesFaded: 2, tilesHighlighted: 1, carsFaded: 2 });

		mocks.setRoom(snapshot([PLAYERS[0], PLAYERS[2]], [0, -1, -1]));
		await waitFor(() => expect(screen.queryByRole('button', { name: 'Ben' })).toBeNull());

		expect(classes()).toEqual({ selected: 0, tilesFaded: 0, tilesHighlighted: 0, carsFaded: 0 });
	});

	it('clears the selection when the room code changes', async () => {
		render(Page);
		await fireEvent.click(nameButton('Anna'));
		expect(classes()).toEqual({ selected: 1, tilesFaded: 2, tilesHighlighted: 1, carsFaded: 2 });

		mocks.setCode('uvwxyz');

		await waitFor(() =>
			expect(classes()).toEqual({
				selected: 0,
				tilesFaded: 0,
				tilesHighlighted: 0,
				carsFaded: 0
			})
		);
	});
});
