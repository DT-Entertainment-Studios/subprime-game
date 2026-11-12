// Presence store: the online list and sent invitations of a room lobby, the
// invitations and the transient notice of the start page (Spec.md §6).
//
// Exactly one presence socket is open at a time. Switching scope closes the
// previous socket, drops the state that belonged to it and opens the new one;
// calling connect with the current scope key is a no-op.

import { ApiError, presenceURL } from './api';
import { PresenceSocket, type PresenceHandlers } from './presence';
import type { Invitation, OnlineUser, PresenceIntent, RoomSnapshot, SentInvite } from './types';

const START_SCOPE = 'start';
const ROOM_PREFIX = 'room:';
const NOTICE_MS = 6000;

interface PendingAccept {
	resolve: (room: RoomSnapshot) => void;
	reject: (error: ApiError) => void;
}

export class Presence {
	connected = $state(false);
	online = $state<OnlineUser[]>([]);
	sent = $state<SentInvite[]>([]);
	invites = $state<Invitation[]>([]);
	notice = $state<Invitation | null>(null);
	error = $state<string | null>(null);
	/**
	 * The name currently in effect outwards (the login name until the user confirms
	 * another); drives the commit button state.
	 */
	publishedName = $state('');

	private socket: PresenceSocket | null = null;
	private scope: string | null = null;
	/** Trimmed login name the server starts the scope with; the re-send baseline. */
	private initialName = '';
	private noticeTimer: ReturnType<typeof setTimeout> | null = null;
	/** The first inbox of a connection is the baseline for the notice diff. */
	private inboxSeen = false;
	private pendingAccept: PendingAccept | null = null;

	private handlers: PresenceHandlers = {
		onLobby: (online, sent) => {
			if (!sameOnline(this.online, online)) this.online = online;
			if (!sameSent(this.sent, sent)) this.sent = sent;
		},
		onInbox: (invites) => {
			const previous = this.invites;
			if (!sameInvites(previous, invites)) this.invites = invites;
			if (!this.inboxSeen) {
				this.inboxSeen = true;
				return;
			}
			const known = new Set(previous.map((invite) => invite.id));
			const fresh = invites.filter((invite) => !known.has(invite.id));
			if (fresh.length === 0) return;
			const newest = fresh.reduce((current, invite) =>
				createdAfter(invite, current) ? invite : current
			);
			this.showNotice(newest);
		},
		onAccepted: (room) => {
			const pending = this.pendingAccept;
			this.pendingAccept = null;
			pending?.resolve(room);
		},
		onError: (code, message) => {
			this.error = `${code}: ${message}`;
			const pending = this.pendingAccept;
			if (!pending) return;
			this.pendingAccept = null;
			pending.reject(new ApiError(0, code, message));
		},
		onStatus: (connected) => {
			this.connected = connected;
			if (connected) {
				this.inboxSeen = false;
				this.resendName();
				return;
			}
			// A dropped connection that kills a pending accept must be visible:
			// the page discards the rejection and relies on `error`. A plain
			// disconnect without a pending accept stays silent (the reconnect
			// indicator covers it).
			if (this.pendingAccept) this.error = 'Keine Verbindung zum Server.';
			this.rejectAccept('Keine Verbindung zum Server.', 'disconnected');
		}
	};

	/** Connects the start-page scope; no-op while already on it. */
	connectStart(name: string): void {
		if (this.scope === START_SCOPE) return;
		this.openScope(START_SCOPE, name);
	}

	/** Connects the lobby scope of one room; no-op while already on it. */
	connectRoom(code: string): void {
		const key = `${ROOM_PREFIX}${code}`;
		if (this.scope === key) return;
		this.openScope(key, '');
	}

	/** Closes the socket and clears every scope-owned value. */
	disconnect(): void {
		this.teardown();
	}

	/** Tears down only while the start-page scope is active; otherwise a no-op. */
	disconnectStart(): void {
		if (this.scope !== START_SCOPE) return;
		this.teardown();
	}

	/** Tears down only while the given room lobby is active; otherwise a no-op. */
	disconnectRoom(code: string): void {
		if (this.scope !== `${ROOM_PREFIX}${code}`) return;
		this.teardown();
	}

	/**
	 * Publishes the value the user confirmed as the display name. The store keeps
	 * no draft of its own: the field value only reaches it here, trimmed. A
	 * commit without an open connection deliberately raises no banner: the value
	 * is kept and the next successful connect re-sends it.
	 */
	commitName(value: string): void {
		if (this.scope !== START_SCOPE) return;
		const next = value.trim();
		if (!next || next === this.publishedName) return;
		this.publishedName = next;
		this.socket?.send({ type: 'set_name', name: next });
	}

	invite(userId: string): void {
		this.send({ type: 'invite', userId });
	}

	cancel(inviteId: string): void {
		this.send({ type: 'cancel_invite', inviteId });
	}

	decline(inviteId: string): void {
		this.send({ type: 'decline_invite', inviteId });
	}

	/**
	 * Accepts an invitation and resolves with the room snapshot. Rejects with an
	 * `ApiError` on a server error reply, when the socket closes or disconnects
	 * before the reply, when the scope switches, and when another accept starts.
	 */
	accept(inviteId: string, name: string, color: string): Promise<RoomSnapshot> {
		this.rejectAccept('Abgebrochen.', 'cancelled');
		this.error = null;
		return new Promise<RoomSnapshot>((resolve, reject) => {
			this.pendingAccept = { resolve, reject };
			const sent =
				this.scope === START_SCOPE &&
				this.socket?.send({ type: 'accept_invite', inviteId, name, color });
			if (!sent) {
				this.pendingAccept = null;
				this.error = 'Keine Verbindung zum Server.';
				reject(new ApiError(0, 'disconnected', 'Keine Verbindung zum Server.'));
			}
		});
	}

	dismissNotice(): void {
		if (this.noticeTimer) clearTimeout(this.noticeTimer);
		this.noticeTimer = null;
		this.notice = null;
	}

	clearError(): void {
		this.error = null;
	}

	private openScope(key: string, name: string): void {
		this.teardown();
		this.scope = key;
		if (key === START_SCOPE) {
			this.initialName = name.trim();
			this.publishedName = this.initialName;
			this.socket = new PresenceSocket(() => presenceURL(), this.handlers);
		} else {
			const code = key.slice(ROOM_PREFIX.length);
			this.socket = new PresenceSocket(() => presenceURL({ room: code }), this.handlers);
		}
		this.socket.connect();
	}

	/** Rejects a pending accept, closes the socket and clears all scope state. */
	private teardown(): void {
		this.rejectAccept('Verbindung getrennt.', 'disconnected');
		this.closeSocket();
		this.resetState();
		this.scope = null;
		this.initialName = '';
		this.publishedName = '';
	}

	private closeSocket(): void {
		this.socket?.close();
		this.socket = null;
		this.connected = false;
		if (this.noticeTimer) clearTimeout(this.noticeTimer);
		this.noticeTimer = null;
	}

	private resetState(): void {
		this.online = [];
		this.sent = [];
		this.invites = [];
		this.notice = null;
		this.error = null;
		this.inboxSeen = false;
	}

	private send(intent: PresenceIntent): void {
		if (!this.socket?.send(intent)) {
			this.error = 'Keine Verbindung zum Server.';
		}
	}

	/**
	 * Re-sends the confirmed name after a successful (re)connect while it differs
	 * from the login name the server started with. Repeated sends are idempotent:
	 * the hub normalises and drops unchanged lobby pushes.
	 */
	private resendName(): void {
		if (this.scope !== START_SCOPE) return;
		if (!this.publishedName || this.publishedName === this.initialName) return;
		this.socket?.send({ type: 'set_name', name: this.publishedName });
	}

	private showNotice(invite: Invitation): void {
		if (this.noticeTimer) clearTimeout(this.noticeTimer);
		this.notice = invite;
		this.noticeTimer = setTimeout(() => {
			this.noticeTimer = null;
			this.notice = null;
		}, NOTICE_MS);
	}

	private rejectAccept(message: string, code: string): void {
		const pending = this.pendingAccept;
		if (!pending) return;
		this.pendingAccept = null;
		pending.reject(new ApiError(0, code, message));
	}
}

/**
 * Whether the field value differs from the currently published name, i.e. whether
 * the commit button is actionable. Kept pure so it can be tested without a DOM.
 */
export function isNamePending(value: string, published: string): boolean {
	const trimmed = value.trim();
	return trimmed !== '' && trimmed !== published;
}

function sameOnline(current: OnlineUser[], next: OnlineUser[]): boolean {
	return sameEach(
		current,
		next,
		(user, other) => user.userId === other.userId && user.name === other.name
	);
}

function sameSent(current: SentInvite[], next: SentInvite[]): boolean {
	return sameEach(
		current,
		next,
		(invite, other) =>
			invite.id === other.id &&
			invite.userId === other.userId &&
			invite.name === other.name &&
			invite.fromId === other.fromId &&
			invite.fromName === other.fromName
	);
}

function sameInvites(current: Invitation[], next: Invitation[]): boolean {
	return sameEach(
		current,
		next,
		(invite, other) =>
			invite.id === other.id &&
			invite.room === other.room &&
			invite.fromId === other.fromId &&
			invite.fromName === other.fromName
	);
}

/**
 * Compares two equally long arrays pairwise. Uses `Array.prototype.at` so the
 * comparison stays free of dynamic bracket access (a false positive for the
 * `detect-object-injection` static-analysis rule).
 */
function sameEach<T>(current: T[], next: T[], same: (item: T, other: T) => boolean): boolean {
	if (current.length !== next.length) return false;
	return current.every((item, index) => {
		const other = next.at(index);
		return other !== undefined && same(item, other);
	});
}

function createdAfter(candidate: Invitation, current: Invitation): boolean {
	return Date.parse(candidate.createdAt) >= Date.parse(current.createdAt);
}

export const presence = new Presence();
