// Reconnecting WebSocket wrapper for the presence channel (Spec.md §6): one
// socket per scope (start page or one room lobby), JSON intents and
// lobby/inbox/accepted/error pushes.

import type {
	AcceptedMessage,
	InboxState,
	Invitation,
	LobbyState,
	OnlineUser,
	PresenceIntent,
	RoomSnapshot,
	SentInvite
} from './types';

export interface PresenceHandlers {
	onLobby: (online: OnlineUser[], sent: SentInvite[]) => void;
	onInbox: (invites: Invitation[]) => void;
	onAccepted: (room: RoomSnapshot) => void;
	onError: (code: string, message: string) => void;
	onStatus: (connected: boolean) => void;
}

interface ErrorMessage {
	type: 'error';
	error: { code: string; message: string };
}

type PresenceMessage = LobbyState | InboxState | AcceptedMessage | ErrorMessage;

export class PresenceSocket {
	private socket: WebSocket | null = null;
	private closed = false;
	private attempt = 0;
	private timer: ReturnType<typeof setTimeout> | null = null;

	/**
	 * @param url fixed URL, or a resolver evaluated on every (re)connect. The
	 * presence URL carries no changeable state: the store re-sends the confirmed
	 * display name once the connection reports open.
	 */
	constructor(
		private url: string | (() => string),
		private handlers: PresenceHandlers
	) {}

	connect(): void {
		this.closed = false;
		this.open();
	}

	private open(): void {
		if (this.closed) return;
		let socket: WebSocket;
		try {
			socket = new WebSocket(this.resolveURL());
		} catch {
			this.scheduleReconnect();
			return;
		}
		this.socket = socket;
		socket.onopen = () => {
			if (this.closed || this.socket !== socket) return;
			this.attempt = 0;
			this.handlers.onStatus(true);
		};
		socket.onmessage = (event) => {
			if (this.closed || this.socket !== socket) return;
			this.handleMessage(event.data);
		};
		socket.onclose = () => {
			// A socket that was closed on purpose (scope switch, disconnect) must
			// never report status or schedule a reconnect; the same holds for any
			// callback of a socket that a newer one already replaced.
			if (this.closed || this.socket !== socket) return;
			this.handlers.onStatus(false);
			this.scheduleReconnect();
		};
		socket.onerror = () => {
			if (this.closed || this.socket !== socket) return;
			socket.close();
		};
	}

	private resolveURL(): string {
		return typeof this.url === 'function' ? this.url() : this.url;
	}

	private handleMessage(data: unknown): void {
		if (typeof data !== 'string') return;
		let message: PresenceMessage;
		try {
			message = JSON.parse(data);
		} catch {
			return;
		}
		switch (message.type) {
			case 'lobby':
				this.handlers.onLobby(message.online ?? [], message.sent ?? []);
				break;
			case 'inbox':
				this.handlers.onInbox(message.invites ?? []);
				break;
			case 'accepted':
				if (message.room) this.handlers.onAccepted(message.room);
				break;
			case 'error':
				if (message.error) this.handlers.onError(message.error.code, message.error.message);
				break;
		}
	}

	private scheduleReconnect(): void {
		if (this.closed) return;
		this.attempt += 1;
		const delay = Math.min(8000, 500 * 2 ** Math.min(this.attempt, 4));
		if (this.timer) clearTimeout(this.timer);
		this.timer = setTimeout(() => this.open(), delay);
	}

	send(intent: PresenceIntent): boolean {
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
