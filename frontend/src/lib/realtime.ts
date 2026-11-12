// Small reconnecting WebSocket wrapper: one socket, JSON messages, intents
// and state pushes (Spec.md §6).

import type { Intent, RoomSnapshot } from './types';

export interface RealtimeHandlers {
	onState: (room: RoomSnapshot) => void;
	onError: (code: string, message: string) => void;
	onStatus: (connected: boolean) => void;
}

interface StateMessage {
	type: 'state';
	room: RoomSnapshot;
}

interface ErrorMessage {
	type: 'error';
	error: { code: string; message: string };
}

export class Realtime {
	private socket: WebSocket | null = null;
	private closed = false;
	private attempt = 0;
	private timer: ReturnType<typeof setTimeout> | null = null;

	constructor(
		private url: string,
		private handlers: RealtimeHandlers
	) {}

	connect(): void {
		this.closed = false;
		this.open();
	}

	private open(): void {
		if (this.closed) return;
		let socket: WebSocket;
		try {
			socket = new WebSocket(this.url);
		} catch {
			this.scheduleReconnect();
			return;
		}
		this.socket = socket;
		socket.onopen = () => {
			this.attempt = 0;
			this.handlers.onStatus(true);
		};
		socket.onmessage = (event) => {
			this.handleMessage(event.data);
		};
		socket.onclose = () => {
			this.handlers.onStatus(false);
			this.scheduleReconnect();
		};
		socket.onerror = () => {
			socket.close();
		};
	}

	private handleMessage(data: unknown): void {
		if (typeof data !== 'string') return;
		let message: StateMessage | ErrorMessage;
		try {
			message = JSON.parse(data);
		} catch {
			return;
		}
		if (message.type === 'state' && message.room) {
			this.handlers.onState(message.room);
		} else if (message.type === 'error' && message.error) {
			this.handlers.onError(message.error.code, message.error.message);
		}
	}

	private scheduleReconnect(): void {
		if (this.closed) return;
		this.attempt += 1;
		const delay = Math.min(8000, 500 * 2 ** Math.min(this.attempt, 4));
		if (this.timer) clearTimeout(this.timer);
		this.timer = setTimeout(() => this.open(), delay);
	}

	send(intent: Intent): boolean {
		if (!this.socket || this.socket.readyState !== WebSocket.OPEN) return false;
		this.socket.send(JSON.stringify(intent));
		return true;
	}

	close(): void {
		this.closed = true;
		if (this.timer) clearTimeout(this.timer);
		this.socket?.close();
		this.socket = null;
	}
}
