import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { session } from './session.svelte';
import type { ActiveRoom, RoomSnapshot } from './types';

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
