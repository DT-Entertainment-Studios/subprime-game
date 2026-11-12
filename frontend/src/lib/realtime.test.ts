import { afterEach, describe, expect, it, vi } from 'vitest';
import { Realtime } from './realtime';
import type { RoomSnapshot } from './types';

class FakeSocket {
	static OPEN = 1;
	static CLOSED = 3;
	static instances: FakeSocket[] = [];

	readyState = FakeSocket.OPEN;
	sent: string[] = [];
	onopen: (() => void) | null = null;
	onmessage: ((event: { data: unknown }) => void) | null = null;
	onclose: (() => void) | null = null;
	onerror: (() => void) | null = null;

	constructor(public url: string) {
		FakeSocket.instances.push(this);
	}

	send(data: string): void {
		this.sent.push(data);
	}

	close(): void {
		this.readyState = FakeSocket.CLOSED;
		this.onclose?.();
	}

	emit(payload: unknown): void {
		this.onmessage?.({ data: JSON.stringify(payload) });
	}
}

const room = { code: 'ABCDEF', phase: 'lobby' } as RoomSnapshot;

describe('Realtime', () => {
	afterEach(() => {
		FakeSocket.instances = [];
		vi.unstubAllGlobals();
	});

	it('delivers state and error messages and forwards intents', () => {
		vi.stubGlobal('WebSocket', FakeSocket);
		const states: RoomSnapshot[] = [];
		const errors: [string, string][] = [];
		const statuses: boolean[] = [];
		const realtime = new Realtime('ws://example/api/ws?room=ABCDEF', {
			onState: (value) => states.push(value),
			onError: (code, message) => errors.push([code, message]),
			onStatus: (connected) => statuses.push(connected)
		});

		realtime.connect();
		const socket = FakeSocket.instances[0];
		expect(socket.url).toContain('room=ABCDEF');
		socket.onopen?.();
		socket.emit({ type: 'state', room });
		socket.emit({ type: 'error', error: { code: 'not_your_turn', message: 'Nein' } });

		expect(statuses).toEqual([true]);
		expect(states[0].code).toBe('ABCDEF');
		expect(errors[0]).toEqual(['not_your_turn', 'Nein']);

		expect(realtime.send({ type: 'roll' })).toBe(true);
		expect(JSON.parse(socket.sent[0])).toEqual({ type: 'roll' });

		realtime.close();
		expect(statuses.at(-1)).toBe(false);
	});

	it('reports a send failure while disconnected', () => {
		vi.stubGlobal('WebSocket', FakeSocket);
		const realtime = new Realtime('ws://example/api/ws', {
			onState: () => {},
			onError: () => {},
			onStatus: () => {}
		});
		expect(realtime.send({ type: 'roll' })).toBe(false);
		realtime.close();
	});
});
