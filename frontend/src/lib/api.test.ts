import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiError, abandonRoom, fetchMyRoom, leaveRoom } from './api';

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

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('leaveRoom', () => {
	it('sends a bodyless POST to the leave endpoint', async () => {
		const fetchMock = stubFetch(response(204));

		await leaveRoom('ABCDEF');

		expect(fetchMock).toHaveBeenCalledTimes(1);
		const [path, init] = fetchMock.mock.calls[0] as [string, RequestInit];
		expect(path).toBe('/api/rooms/ABCDEF/leave');
		expect(init.method).toBe('POST');
		expect(init.body).toBeUndefined();
	});

	it('resolves for an empty 204 response', async () => {
		stubFetch(response(204));

		await expect(leaveRoom('ABCDEF')).resolves.toBeUndefined();
	});

	it('resolves when the room is already gone', async () => {
		stubFetch(
			response(404, { error: { code: 'room_not_found', message: 'Unbekannter Raum.' } })
		);

		await expect(leaveRoom('ABCDEF')).resolves.toBeUndefined();
	});

	it('rejects other errors with the API code and message', async () => {
		stubFetch(response(500, { error: { code: 'internal', message: 'Serverfehler.' } }));

		const error = await leaveRoom('ABCDEF').catch((err: unknown) => err);
		expect(error).toBeInstanceOf(ApiError);
		expect(error).toMatchObject({
			status: 500,
			code: 'internal',
			message: 'Serverfehler.'
		});
	});
});

describe('fetchMyRoom', () => {
	it('asks the me-room endpoint with GET', async () => {
		const fetchMock = stubFetch(response(200, { room: null }));

		await fetchMyRoom();

		expect(fetchMock).toHaveBeenCalledTimes(1);
		const [path, init] = fetchMock.mock.calls[0] as [string, RequestInit];
		expect(path).toBe('/api/me/room');
		expect(init.method).toBeUndefined();
	});

	it('maps a missing room to null', async () => {
		stubFetch(response(200, { room: null }));

		await expect(fetchMyRoom()).resolves.toBeNull();
	});

	it('maps the blocking room with its phase', async () => {
		stubFetch(response(200, { room: { code: 'ABCDEF', phase: 'playing' } }));

		await expect(fetchMyRoom()).resolves.toEqual({ code: 'ABCDEF', phase: 'playing' });
	});

	it('propagates an API error', async () => {
		stubFetch(response(401, { error: { code: 'unauthorized', message: 'Nicht angemeldet.' } }));

		const error = await fetchMyRoom().catch((err: unknown) => err);
		expect(error).toBeInstanceOf(ApiError);
		expect(error).toMatchObject({
			status: 401,
			code: 'unauthorized',
			message: 'Nicht angemeldet.'
		});
	});
});

describe('abandonRoom', () => {
	it('sends a bodyless POST to the abandon endpoint', async () => {
		const fetchMock = stubFetch(response(204));

		await abandonRoom('ABCDEF');

		expect(fetchMock).toHaveBeenCalledTimes(1);
		const [path, init] = fetchMock.mock.calls[0] as [string, RequestInit];
		expect(path).toBe('/api/rooms/ABCDEF/abandon');
		expect(init.method).toBe('POST');
		expect(init.body).toBeUndefined();
	});

	it('resolves for an empty 204 response', async () => {
		stubFetch(response(204));

		await expect(abandonRoom('ABCDEF')).resolves.toBeUndefined();
	});

	it('resolves when the room is already gone', async () => {
		stubFetch(response(404, { error: { code: 'room_not_found', message: 'Unbekannter Raum.' } }));

		await expect(abandonRoom('ABCDEF')).resolves.toBeUndefined();
	});

	it('rejects other errors with the API code and message', async () => {
		stubFetch(response(500, { error: { code: 'internal', message: 'Serverfehler.' } }));

		const error = await abandonRoom('ABCDEF').catch((err: unknown) => err);
		expect(error).toBeInstanceOf(ApiError);
		expect(error).toMatchObject({
			status: 500,
			code: 'internal',
			message: 'Serverfehler.'
		});
	});
});
