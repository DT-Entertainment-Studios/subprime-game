// Central session store: identity, room state and the realtime connection.

import {
	abandonRoom,
	createRoom,
	fetchMe,
	fetchMyRoom,
	fetchRoom,
	joinRoom,
	leaveRoom,
	websocketURL
} from './api';
import { Realtime } from './realtime';
import type { ActiveRoom, Intent, RoomSnapshot, User } from './types';

class Session {
	user = $state<User | null>(null);
	room = $state<RoomSnapshot | null>(null);
	/** The live membership that currently blocks create/join/invitations. */
	activeRoom = $state<ActiveRoom | null>(null);
	connected = $state(false);
	error = $state<string | null>(null);
	loading = $state(false);

	private realtime: Realtime | null = null;
	/** Guards against a stale refresh answer overwriting newer state. */
	private activeRoomRequest = 0;

	get isMyTurn(): boolean {
		const room = this.room;
		if (!room?.game) return false;
		return room.game.players[room.game.current]?.id === room.you.id;
	}

	get me() {
		const room = this.room;
		if (!room?.game || room.you.seat < 0) return null;
		return room.game.players[room.you.seat] ?? null;
	}

	async init(): Promise<void> {
		try {
			this.user = await fetchMe();
		} catch (err) {
			this.error = errorMessage(err);
		}
	}

	async create(displayName: string, color: string): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			this.room = await createRoom(displayName, color);
		} catch (err) {
			this.error = errorMessage(err);
		} finally {
			this.loading = false;
		}
	}

	async join(code: string, displayName: string, color: string): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			this.room = await joinRoom(code, displayName, color);
		} catch (err) {
			this.error = errorMessage(err);
		} finally {
			this.loading = false;
		}
	}

	async load(code: string): Promise<void> {
		try {
			this.room = await fetchRoom(code);
		} catch (err) {
			this.error = errorMessage(err);
		}
	}

	/**
	 * Frees the lobby seat through REST. Returns false on a real failure so the
	 * caller can stay on the page and keep the seat instead of navigating away.
	 * A room that is already gone counts as success.
	 */
	async leave(code: string): Promise<boolean> {
		this.error = null;
		try {
			await leaveRoom(code);
			this.room = null;
			return true;
		} catch (err) {
			this.error = errorMessage(err);
			return false;
		}
	}

	/**
	 * Loads the caller's blocking room for the start-page banner. A transport
	 * error keeps the previous banner (the safer state) and only surfaces the
	 * message through the shared error banner.
	 */
	async refreshActiveRoom(): Promise<void> {
		const request = ++this.activeRoomRequest;
		try {
			const room = await fetchMyRoom();
			if (request === this.activeRoomRequest) this.activeRoom = room;
		} catch (err) {
			if (request === this.activeRoomRequest) this.error = errorMessage(err);
		}
	}

	/**
	 * Finally gives up the seat of the active room. Returns false on a real
	 * failure so the caller can keep the banner and retry; a room that is
	 * already gone counts as success.
	 */
	async abandonActiveRoom(): Promise<boolean> {
		const room = this.activeRoom;
		if (!room) return true;
		this.error = null;
		try {
			await abandonRoom(room.code);
			// Invalidate an in-flight refresh so a stale answer cannot re-show the
			// banner after a successful final leave.
			this.activeRoomRequest++;
			this.activeRoom = null;
			if (this.room?.code === room.code) this.room = null;
			return true;
		} catch (err) {
			this.error = errorMessage(err);
			return false;
		}
	}

	connect(code: string): void {
		if (this.realtime) this.realtime.close();
		this.realtime = new Realtime(websocketURL(code), {
			onState: (room) => {
				this.room = room;
			},
			onError: (code, message) => {
				this.error = `${code}: ${message}`;
			},
			onStatus: (connected) => {
				this.connected = connected;
			}
		});
		this.realtime.connect();
	}

	disconnect(): void {
		this.realtime?.close();
		this.realtime = null;
		this.connected = false;
	}

	send(intent: Intent): void {
		if (!this.realtime?.send(intent)) {
			this.error = 'Keine Verbindung zum Server.';
		}
	}

	clearError(): void {
		this.error = null;
	}
}

function errorMessage(err: unknown): string {
	if (err instanceof Error) return err.message;
	return 'Unbekannter Fehler.';
}

export const session = new Session();
