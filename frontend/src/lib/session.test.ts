import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { session } from './session.svelte';
import type { ActiveRoom, GameState, Player, RoomSnapshot } from './types';

const room = { code: 'ABCDEF', phase: 'lobby' } as RoomSnapshot;
const activeRoom: ActiveRoom = { code: 'ABCDEF', phase: 'playing' };

function response(status: number, body: unknown = null): Response {
	return {
		ok: status >= 200 && status < 300,
		status,
		text: async () => (body === null ? '' : JSON.stringify(body))
	} as Response;
}

function stubFetch(result: Response) {
	const fetchMock = vi.fn().mockResolvedValue(result);
	vi.stubGlobal('fetch', fetchMock);
	return fetchMock;
}

beforeEach(() => {
	session.room = null;
	session.user = null;
	session.activeRoom = null;
	session.error = null;
	session.connected = false;
	session.loading = false;
});

afterEach(() => {
	vi.unstubAllGlobals();
});

function gamePlayer(id: string, seat: number, cash: number): Player {
	return {
		id,
		name: id,
		color: 'blau',
		seat,
		cash,
		liquidationValue: cash,
		position: 0,
		inJail: false,
		jailAttempts: 0,
		jailCards: 0,
		bankrupt: false,
		connected: true,
		netWorth: cash,
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

function gameWithDebt(current: number, debtor: number): GameState {
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
		players: [gamePlayer('user-0', 0, -50), gamePlayer('user-1', 1, 1500)],
		tiles: [],
		current,
		debtor,
		turnPhase: 'awaiting_debt',
		dice: [0, 0],
		doubles: 0,
		trades: [],
		events: [],
		winner: -1
	};
}

function viewerRoom(viewerId: string, game: GameState): RoomSnapshot {
	return {
		...room,
		you: { id: viewerId, name: viewerId, role: 'player', seat: 0, isHost: false },
		game
	};
}

describe('session.isDebtor', () => {
	it('follows game.debtor, not game.current', () => {
		session.room = viewerRoom('user-0', gameWithDebt(1, 0));

		expect(session.isMyTurn).toBe(false);
		expect(session.isDebtor).toBe(true);
	});

	it('is false when the current player is not the debtor', () => {
		session.room = viewerRoom('user-0', gameWithDebt(0, 1));

		expect(session.isMyTurn).toBe(true);
		expect(session.isDebtor).toBe(false);
	});

	it('is false when no debtor is set', () => {
		session.room = viewerRoom('user-0', gameWithDebt(0, -1));

		expect(session.isDebtor).toBe(false);
	});

	it('is false without a game', () => {
		session.room = room;

		expect(session.isDebtor).toBe(false);
	});
});

describe('session.leave', () => {
	it('frees the room state and returns true on success', async () => {
		const fetchMock = stubFetch(response(204));
		session.room = room;
		session.error = 'Alter Fehler.';

		await expect(session.leave('ABCDEF')).resolves.toBe(true);

		expect(fetchMock).toHaveBeenCalledTimes(1);
		expect(session.room).toBeNull();
		expect(session.error).toBeNull();
	});

	it('returns true when the room is already gone', async () => {
		stubFetch(
			response(404, { error: { code: 'room_not_found', message: 'Unbekannter Raum.' } })
		);
		session.room = room;

		await expect(session.leave('ABCDEF')).resolves.toBe(true);
		expect(session.room).toBeNull();
	});

	it('keeps the seat and shows the error on a real failure', async () => {
		stubFetch(response(500, { error: { code: 'internal', message: 'Serverfehler.' } }));
		session.room = room;
		const seat = session.room;

		await expect(session.leave('ABCDEF')).resolves.toBe(false);

		expect(session.room).toBe(seat);
		expect(session.error).toBe('Serverfehler.');
	});
});

describe('session.refreshActiveRoom', () => {
	it('loads the blocking room', async () => {
		stubFetch(response(200, { room: activeRoom }));

		await session.refreshActiveRoom();

		expect(session.activeRoom).toEqual(activeRoom);
		expect(session.error).toBeNull();
	});

	it('clears a stale banner when the server reports no room', async () => {
		stubFetch(response(200, { room: null }));
		session.activeRoom = activeRoom;

		await session.refreshActiveRoom();

		expect(session.activeRoom).toBeNull();
	});

	it('keeps the previous banner and shows the error on a transport error', async () => {
		stubFetch(response(500, { error: { code: 'internal', message: 'Serverfehler.' } }));
		session.activeRoom = activeRoom;

		await session.refreshActiveRoom();

		expect(session.activeRoom).toEqual(activeRoom);
		expect(session.error).toBe('Serverfehler.');
	});

	it('ignores a stale answer that resolves after a newer refresh', async () => {
		const first = deferred<Response>();
		const second = deferred<Response>();
		const fetchMock = vi
			.fn()
			.mockReturnValueOnce(first.promise)
			.mockReturnValueOnce(second.promise);
		vi.stubGlobal('fetch', fetchMock);

		const stale = session.refreshActiveRoom();
		const fresh = session.refreshActiveRoom();

		second.resolve(response(200, { room: null }));
		await fresh;
		first.resolve(response(200, { room: activeRoom }));
		await stale;

		expect(session.activeRoom).toBeNull();
	});
});

describe('session.abandonActiveRoom', () => {
	it('clears the banner and the matching room on success', async () => {
		const fetchMock = stubFetch(response(204));
		session.activeRoom = activeRoom;
		session.room = room;

		await expect(session.abandonActiveRoom()).resolves.toBe(true);

		expect(fetchMock).toHaveBeenCalledTimes(1);
		const [path] = fetchMock.mock.calls[0] as [string, RequestInit];
		expect(path).toBe('/api/rooms/ABCDEF/abandon');
		expect(session.activeRoom).toBeNull();
		expect(session.room).toBeNull();
		expect(session.error).toBeNull();
	});

	it('treats a room that is already gone as success', async () => {
		stubFetch(response(404, { error: { code: 'room_not_found', message: 'Unbekannter Raum.' } }));
		session.activeRoom = activeRoom;

		await expect(session.abandonActiveRoom()).resolves.toBe(true);
		expect(session.activeRoom).toBeNull();
	});

	it('keeps the banner and shows the error on a real failure', async () => {
		stubFetch(response(500, { error: { code: 'internal', message: 'Serverfehler.' } }));
		session.activeRoom = activeRoom;

		await expect(session.abandonActiveRoom()).resolves.toBe(false);

		expect(session.activeRoom).toEqual(activeRoom);
		expect(session.error).toBe('Serverfehler.');
	});

	it('ignores a refresh that resolves after a successful abandon', async () => {
		const refresh = deferred<Response>();
		const fetchMock = vi
			.fn()
			.mockReturnValueOnce(refresh.promise)
			.mockResolvedValueOnce(response(204));
		vi.stubGlobal('fetch', fetchMock);
		session.activeRoom = activeRoom;
		session.room = room;

		const pending = session.refreshActiveRoom();
		await expect(session.abandonActiveRoom()).resolves.toBe(true);

		refresh.resolve(response(200, { room: activeRoom }));
		await pending;

		expect(session.activeRoom).toBeNull();
		expect(session.room).toBeNull();
	});

	it('keeps an unrelated room state when the codes differ', async () => {
		stubFetch(response(204));
		session.activeRoom = activeRoom;
		session.room = { ...room, code: 'QWERTY' };

		await expect(session.abandonActiveRoom()).resolves.toBe(true);

		expect(session.room?.code).toBe('QWERTY');
	});

	it('does nothing without an active room', async () => {
		const fetchMock = stubFetch(response(204));

		await expect(session.abandonActiveRoom()).resolves.toBe(true);

		expect(fetchMock).not.toHaveBeenCalled();
	});
});

function deferred<T>(): { promise: Promise<T>; resolve: (value: T) => void } {
	let resolve: (value: T) => void = () => {};
	const promise = new Promise<T>((done) => {
		resolve = done;
	});
	return { promise, resolve };
}
