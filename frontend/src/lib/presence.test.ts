import { afterEach, describe, expect, it, vi } from 'vitest';
import { PresenceSocket } from './presence';
import { Presence, isNamePending } from './presence.svelte';
import type { Invitation, OnlineUser, RoomSnapshot, SentInvite } from './types';

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

	send(data: string): boolean {
		if (this.readyState !== FakeSocket.OPEN) return false;
		this.sent.push(data);
		return true;
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

const onlineUser = (userId: string, name: string): OnlineUser => ({ userId, name });

const sentInvite = (
	id: string,
	userId: string,
	name: string,
	overrides: Partial<SentInvite> = {}
): SentInvite => ({
	id,
	userId,
	name,
	fromId: 'host',
	fromName: 'Ben',
	createdAt: '2026-01-01T10:00:00Z',
	...overrides
});

const invitation = (id: string, overrides: Partial<Invitation> = {}): Invitation => ({
	id,
	room: 'ABCDEF',
	fromId: 'host',
	fromName: 'Ben',
	createdAt: '2026-01-01T10:00:00Z',
	...overrides
});

function install(): void {
	vi.stubGlobal('WebSocket', FakeSocket);
}

afterEach(() => {
	FakeSocket.instances = [];
	vi.useRealTimers();
	vi.unstubAllGlobals();
});

describe('PresenceSocket', () => {
	it('routes lobby, inbox, accepted and error messages to the handlers', () => {
		install();
		const lobbies: [OnlineUser[], SentInvite[]][] = [];
		const inboxes: Invitation[][] = [];
		const accepted: RoomSnapshot[] = [];
		const errors: [string, string][] = [];
		const statuses: boolean[] = [];
		const socket = new PresenceSocket('ws://example/api/ws/presence', {
			onLobby: (online, sent) => lobbies.push([online, sent]),
			onInbox: (invites) => inboxes.push(invites),
			onAccepted: (value) => accepted.push(value),
			onError: (code, message) => errors.push([code, message]),
			onStatus: (connected) => statuses.push(connected)
		});

		socket.connect();
		const fake = FakeSocket.instances[0];
		expect(fake.url).toBe('ws://example/api/ws/presence');
		fake.onopen?.();
		fake.emit({
			type: 'lobby',
			online: [onlineUser('u1', 'Ben')],
			sent: [sentInvite('i1', 'u2', 'Cara')]
		});
		fake.emit({ type: 'inbox', invites: [invitation('i1')] });
		fake.emit({ type: 'accepted', room });
		fake.emit({ type: 'error', error: { code: 'not_online', message: 'Nicht online.' } });

		expect(statuses).toEqual([true]);
		expect(lobbies[0][0]).toEqual([onlineUser('u1', 'Ben')]);
		expect(lobbies[0][1]).toEqual([sentInvite('i1', 'u2', 'Cara')]);
		expect(inboxes[0]).toEqual([invitation('i1')]);
		expect(accepted[0].code).toBe('ABCDEF');
		expect(errors[0]).toEqual(['not_online', 'Nicht online.']);

		expect(socket.send({ type: 'invite', userId: 'u2' })).toBe(true);
		expect(JSON.parse(fake.sent[0])).toEqual({ type: 'invite', userId: 'u2' });

		socket.close();
		// An intentional close reports no status and schedules no reconnect.
		expect(statuses).toEqual([true]);
	});

	it('reports a send failure while disconnected', () => {
		install();
		const socket = new PresenceSocket('ws://example/api/ws/presence', {
			onLobby: () => {},
			onInbox: () => {},
			onAccepted: () => {},
			onError: () => {},
			onStatus: () => {}
		});
		expect(socket.send({ type: 'invite', userId: 'u2' })).toBe(false);
		socket.close();
	});
});

describe('Presence store', () => {
	it('connects once per scope', () => {
		install();
		const presence = new Presence();

		presence.connectStart('Anna');
		presence.connectStart('Anna');
		expect(FakeSocket.instances).toHaveLength(1);
		const startURL = new URL(FakeSocket.instances[0].url);
		expect(startURL.pathname).toBe('/api/ws/presence');
		expect(startURL.search).toBe('');
		expect(startURL.searchParams.has('name')).toBe(false);

		presence.connectRoom('AAAAAA');
		presence.connectRoom('AAAAAA');
		expect(FakeSocket.instances).toHaveLength(2);
		expect(FakeSocket.instances[1].url).toContain('/api/ws/presence?room=AAAAAA');

		presence.disconnect();
	});

	it('switches scopes, closes the old socket and clears the old scope state', () => {
		install();
		const presence = new Presence();

		presence.connectStart('Anna');
		const start = FakeSocket.instances[0];
		start.onopen?.();
		start.emit({ type: 'inbox', invites: [invitation('i1')] });
		expect(presence.invites).toHaveLength(1);

		presence.connectRoom('AAAAAA');
		expect(start.readyState).toBe(FakeSocket.CLOSED);
		expect(FakeSocket.instances).toHaveLength(2);
		expect(presence.invites).toEqual([]);
		expect(presence.notice).toBeNull();

		const roomA = FakeSocket.instances[1];
		roomA.onopen?.();
		roomA.emit({
			type: 'lobby',
			online: [onlineUser('u1', 'Ben')],
			sent: [sentInvite('i1', 'u2', 'Cara')]
		});
		expect(presence.online).toHaveLength(1);
		expect(presence.sent).toHaveLength(1);

		presence.connectRoom('BBBBBB');
		expect(roomA.readyState).toBe(FakeSocket.CLOSED);
		expect(FakeSocket.instances).toHaveLength(3);
		expect(presence.online).toEqual([]);
		expect(presence.sent).toEqual([]);

		const roomB = FakeSocket.instances[2];
		roomB.onopen?.();
		roomB.emit({ type: 'lobby', online: [onlineUser('u9', 'Dora')], sent: [] });

		presence.connectStart('Anna');
		expect(roomB.readyState).toBe(FakeSocket.CLOSED);
		expect(FakeSocket.instances).toHaveLength(4);
		expect(presence.online).toEqual([]);
		expect(presence.sent).toEqual([]);
		const reopenedURL = new URL(FakeSocket.instances[3].url);
		expect(reopenedURL.search).toBe('');
		expect(reopenedURL.searchParams.has('name')).toBe(false);

		presence.disconnect();
	});

	it('publishes the trimmed value only on commit and ignores repeats', () => {
		vi.useFakeTimers();
		install();
		const presence = new Presence();

		presence.connectStart('Anna');
		const socket = FakeSocket.instances[0];
		socket.onopen?.();

		// Typing never reaches the store, so nothing can be pending after 1 s.
		vi.advanceTimersByTime(1000);
		expect(socket.sent).toHaveLength(0);

		presence.commitName('Anna Bär');
		expect(socket.sent).toHaveLength(1);
		expect(JSON.parse(socket.sent[0])).toEqual({ type: 'set_name', name: 'Anna Bär' });

		// A repeated commit and a trailing-space variant are no-ops.
		presence.commitName('Anna Bär');
		presence.commitName('Anna Bär ');
		expect(socket.sent).toHaveLength(1);

		presence.disconnect();
	});

	it('ignores an empty or whitespace-only commit', () => {
		install();
		const presence = new Presence();

		presence.connectStart('Anna');
		const socket = FakeSocket.instances[0];
		socket.onopen?.();

		presence.commitName('');
		presence.commitName('   ');
		expect(socket.sent).toHaveLength(0);
		expect(presence.publishedName).toBe('Anna');

		presence.disconnect();
	});

	it('sends nothing for commitName on a room scope', () => {
		vi.useFakeTimers();
		install();
		const presence = new Presence();

		presence.connectRoom('AAAAAA');
		const socket = FakeSocket.instances[0];
		socket.onopen?.();
		presence.commitName('Anna');
		vi.advanceTimersByTime(1000);
		expect(socket.sent).toHaveLength(0);

		presence.disconnect();
	});

	it('sends no set_name when a room-scope connection reconnects', () => {
		vi.useFakeTimers();
		install();
		const presence = new Presence();

		presence.connectRoom('AAAAAA');
		const socket = FakeSocket.instances[0];
		socket.onopen?.();
		socket.close();
		vi.advanceTimersByTime(1000);

		expect(FakeSocket.instances).toHaveLength(2);
		const reconnected = FakeSocket.instances[1];
		reconnected.onopen?.();
		expect(reconnected.sent).toHaveLength(0);

		presence.disconnect();
	});

	it('sends nothing on the first open with an unchanged login name', () => {
		install();
		const presence = new Presence();

		presence.connectStart('Anna');
		const socket = FakeSocket.instances[0];
		socket.onopen?.();
		expect(socket.sent).toHaveLength(0);

		presence.disconnect();
	});

	it('resends the committed name after a reconnect', () => {
		vi.useFakeTimers();
		install();
		const presence = new Presence();

		presence.connectStart('Anna');
		const socket = FakeSocket.instances[0];
		socket.onopen?.();
		presence.commitName('Anna Bär');
		expect(JSON.parse(socket.sent[0])).toEqual({ type: 'set_name', name: 'Anna Bär' });

		socket.close();
		vi.advanceTimersByTime(1000);
		expect(FakeSocket.instances).toHaveLength(2);
		const reconnected = FakeSocket.instances[1];
		expect(new URL(reconnected.url).searchParams.has('name')).toBe(false);
		expect(reconnected.sent).toHaveLength(0);

		reconnected.onopen?.();
		expect(reconnected.sent).toHaveLength(1);
		expect(JSON.parse(reconnected.sent[0])).toEqual({ type: 'set_name', name: 'Anna Bär' });

		presence.disconnect();
	});

	it('reconnect sends exactly the confirmed name once', () => {
		vi.useFakeTimers();
		install();
		const presence = new Presence();

		presence.connectStart('Anna');
		const socket = FakeSocket.instances[0];
		socket.onopen?.();
		presence.commitName('Anna Bär');
		expect(socket.sent).toHaveLength(1);

		socket.close();
		vi.advanceTimersByTime(1000);
		const reconnected = FakeSocket.instances[1];
		reconnected.onopen?.();

		// After the commit the store holds only the published name; the reconnect
		// re-sends it exactly once and nothing else.
		expect(reconnected.sent.map((raw) => JSON.parse(raw))).toEqual([
			{ type: 'set_name', name: presence.publishedName }
		]);

		presence.disconnect();
	});

	it('keeps an offline commit and flushes it on reconnect', () => {
		vi.useFakeTimers();
		install();
		const presence = new Presence();

		presence.connectStart('Anna');
		const socket = FakeSocket.instances[0];
		socket.onopen?.();
		socket.readyState = FakeSocket.CLOSED;

		presence.commitName('Anna Bär');
		expect(socket.sent).toHaveLength(0);
		expect(presence.publishedName).toBe('Anna Bär');
		expect(presence.error).toBeNull();

		socket.onclose?.();
		vi.advanceTimersByTime(1000);
		const reconnected = FakeSocket.instances[1];
		reconnected.onopen?.();
		expect(reconnected.sent).toHaveLength(1);
		expect(JSON.parse(reconnected.sent[0])).toEqual({ type: 'set_name', name: 'Anna Bär' });

		presence.disconnect();
	});

	it('tracks the confirmed name across commit and teardown', () => {
		install();
		const presence = new Presence();

		presence.connectStart('Anna');
		expect(presence.publishedName).toBe('Anna');

		presence.commitName('Anna Bär');
		expect(presence.publishedName).toBe('Anna Bär');

		presence.disconnectStart();
		expect(presence.publishedName).toBe('');

		presence.connectStart('Anna');
		expect(presence.publishedName).toBe('Anna');

		presence.disconnect();
	});

	it('shows no notice for the first inbox, a notice for a new invitation and clears it', () => {
		vi.useFakeTimers();
		install();
		const presence = new Presence();

		presence.connectStart('Anna');
		const socket = FakeSocket.instances[0];
		socket.onopen?.();

		const first = invitation('i1');
		const second = invitation('i2', {
			room: 'BBBBBB',
			fromName: 'Cara',
			createdAt: '2026-01-01T10:01:00Z'
		});
		const third = invitation('i3', { room: 'CCCCCC', createdAt: '2026-01-01T10:02:00Z' });

		socket.emit({ type: 'inbox', invites: [first] });
		expect(presence.invites).toEqual([first]);
		expect(presence.notice).toBeNull();

		socket.emit({ type: 'inbox', invites: [first, second] });
		expect(presence.notice).toEqual(second);

		presence.dismissNotice();
		expect(presence.notice).toBeNull();

		socket.emit({ type: 'inbox', invites: [first, second] });
		expect(presence.notice).toBeNull();

		socket.emit({ type: 'inbox', invites: [first, second, third] });
		expect(presence.notice).toEqual(third);
		vi.advanceTimersByTime(5999);
		expect(presence.notice).toEqual(third);
		vi.advanceTimersByTime(1);
		expect(presence.notice).toBeNull();

		presence.disconnect();
	});

	it('resolves accept with the room on accepted', async () => {
		install();
		const presence = new Presence();

		presence.connectStart('Anna');
		const socket = FakeSocket.instances[0];
		socket.onopen?.();

		const pending = presence.accept('i1', 'Anna', '#ff0000');
		expect(JSON.parse(socket.sent[0])).toEqual({
			type: 'accept_invite',
			inviteId: 'i1',
			name: 'Anna',
			color: '#ff0000'
		});

		socket.emit({ type: 'accepted', room });
		await expect(pending).resolves.toEqual(room);

		presence.disconnect();
	});

	it('rejects accept on an error reply and shows the error', async () => {
		install();
		const presence = new Presence();

		presence.connectStart('Anna');
		const socket = FakeSocket.instances[0];
		socket.onopen?.();

		const pending = presence.accept('i1', 'Anna', '#ff0000');
		const rejected = expect(pending).rejects.toMatchObject({
			code: 'game_started',
			message: 'Das Spiel läuft bereits.'
		});
		socket.emit({
			type: 'error',
			error: { code: 'game_started', message: 'Das Spiel läuft bereits.' }
		});
		await rejected;
		expect(presence.error).toBe('game_started: Das Spiel läuft bereits.');

		presence.disconnect();
	});

	it('rejects accept when the socket closes before a reply', async () => {
		vi.useFakeTimers();
		install();
		const presence = new Presence();

		presence.connectStart('Anna');
		const socket = FakeSocket.instances[0];
		socket.onopen?.();

		const pending = presence.accept('i1', 'Anna', '#ff0000');
		const rejected = expect(pending).rejects.toThrow('Keine Verbindung zum Server.');
		// Mirror a real socket: after the close event it is no longer writable.
		socket.readyState = FakeSocket.CLOSED;
		socket.onclose?.();
		await rejected;
		expect(presence.error).toBe('Keine Verbindung zum Server.');
		expect(socket.sent).toHaveLength(1);

		// The resolver was cleared; the reconnect carries the next accept.
		vi.advanceTimersByTime(1000);
		const reconnected = FakeSocket.instances[1];
		expect(reconnected).toBeDefined();
		reconnected.onopen?.();
		const next = presence.accept('i2', 'Anna', '#ff0000');
		reconnected.emit({ type: 'accepted', room });
		await expect(next).resolves.toEqual(room);

		presence.disconnect();
	});

	it('shows an error when a dropped connection rejects a pending accept', async () => {
		install();
		const presence = new Presence();

		presence.connectStart('Anna');
		const socket = FakeSocket.instances[0];
		socket.onopen?.();

		const pending = presence.accept('i1', 'Anna', '#ff0000');
		const rejected = expect(pending).rejects.toMatchObject({ code: 'disconnected' });
		socket.onclose?.();
		await rejected;
		expect(presence.error).toBe('Keine Verbindung zum Server.');
		expect(presence.connected).toBe(false);

		// Without a pending accept a disconnect leaves the banner empty.
		presence.disconnect();
		presence.connectStart('Anna');
		const second = FakeSocket.instances[1];
		second.onopen?.();
		expect(presence.error).toBeNull();
		second.onclose?.();
		expect(presence.error).toBeNull();
		expect(presence.connected).toBe(false);

		presence.disconnect();
	});

	it('rejects accept on disconnect', async () => {
		install();
		const presence = new Presence();

		presence.connectStart('Anna');
		const socket = FakeSocket.instances[0];
		socket.onopen?.();

		const pending = presence.accept('i1', 'Anna', '#ff0000');
		const rejected = expect(pending).rejects.toThrow('Verbindung getrennt.');
		presence.disconnect();
		await rejected;
		expect(presence.connected).toBe(false);
	});

	it('rejects accept when the scope switches', async () => {
		install();
		const presence = new Presence();

		presence.connectStart('Anna');
		FakeSocket.instances[0].onopen?.();

		const pending = presence.accept('i1', 'Anna', '#ff0000');
		const rejected = expect(pending).rejects.toThrow('Verbindung getrennt.');
		presence.connectRoom('AAAAAA');
		await rejected;

		presence.disconnect();
	});

	it('rejects the previous accept when another accept starts', async () => {
		install();
		const presence = new Presence();

		presence.connectStart('Anna');
		const socket = FakeSocket.instances[0];
		socket.onopen?.();

		const first = presence.accept('i1', 'Anna', '#ff0000');
		const rejected = expect(first).rejects.toThrow('Abgebrochen.');
		const second = presence.accept('i2', 'Anna', '#ff0000');
		await rejected;
		expect(socket.sent).toHaveLength(2);

		socket.emit({ type: 'accepted', room });
		await expect(second).resolves.toEqual(room);

		presence.disconnect();
	});

	it('keeps the same array instance for unchanged lobby and inbox payloads', () => {
		install();
		const presence = new Presence();

		presence.connectRoom('AAAAAA');
		const socket = FakeSocket.instances[0];
		socket.onopen?.();

		const online = [onlineUser('u1', 'Ben')];
		const sent = [sentInvite('i1', 'u2', 'Cara')];
		socket.emit({ type: 'lobby', online, sent });
		const onlineRef = presence.online;
		const sentRef = presence.sent;

		socket.emit({ type: 'lobby', online: [...online], sent: [...sent] });
		expect(presence.online).toBe(onlineRef);
		expect(presence.sent).toBe(sentRef);

		socket.emit({ type: 'lobby', online: [onlineUser('u1', 'Ben2')], sent: [] });
		expect(presence.online).not.toBe(onlineRef);
		expect(presence.sent).not.toBe(sentRef);

		presence.disconnect();

		presence.connectStart('Anna');
		const inboxSocket = FakeSocket.instances[1];
		inboxSocket.onopen?.();
		inboxSocket.emit({ type: 'inbox', invites: [invitation('i1')] });
		const invitesRef = presence.invites;
		inboxSocket.emit({ type: 'inbox', invites: [invitation('i1')] });
		expect(presence.invites).toBe(invitesRef);

		inboxSocket.emit({ type: 'inbox', invites: [invitation('i1'), invitation('i2')] });
		expect(presence.invites).not.toBe(invitesRef);

		presence.disconnect();
	});

	it('refreshes sent invitations when the inviter name changes', () => {
		install();
		const presence = new Presence();

		presence.connectRoom('AAAAAA');
		const socket = FakeSocket.instances[0];
		socket.onopen?.();

		socket.emit({ type: 'lobby', online: [], sent: [sentInvite('i1', 'u2', 'Cara')] });
		const sentRef = presence.sent;

		socket.emit({
			type: 'lobby',
			online: [],
			sent: [sentInvite('i1', 'u2', 'Cara', { fromName: 'Ben2' })]
		});
		expect(presence.sent).not.toBe(sentRef);
		expect(presence.sent[0].fromName).toBe('Ben2');

		// An unchanged payload keeps the array identity.
		const renamedRef = presence.sent;
		socket.emit({
			type: 'lobby',
			online: [],
			sent: [sentInvite('i1', 'u2', 'Cara', { fromName: 'Ben2' })]
		});
		expect(presence.sent).toBe(renamedRef);

		presence.disconnect();
	});

	it('clears the notice timer on disconnect and on a scope switch', () => {
		vi.useFakeTimers();
		install();
		const presence = new Presence();

		presence.connectStart('Anna');
		const start = FakeSocket.instances[0];
		start.onopen?.();
		start.emit({ type: 'inbox', invites: [invitation('i1')] });
		start.emit({ type: 'inbox', invites: [invitation('i1'), invitation('i2')] });
		expect(presence.notice).toEqual(invitation('i2'));
		// Only the notice timer is pending.
		expect(vi.getTimerCount()).toBe(1);

		presence.connectRoom('AAAAAA');
		expect(vi.getTimerCount()).toBe(0);
		vi.advanceTimersByTime(6000);
		expect(presence.notice).toBeNull();

		// disconnect() drops the timer of the scope it tears down as well.
		presence.connectStart('Anna');
		const secondStart = FakeSocket.instances[2];
		secondStart.onopen?.();
		secondStart.emit({ type: 'inbox', invites: [invitation('i3')] });
		secondStart.emit({ type: 'inbox', invites: [invitation('i3'), invitation('i4')] });
		expect(presence.notice).toEqual(invitation('i4'));
		expect(vi.getTimerCount()).toBe(1);

		presence.disconnect();
		expect(vi.getTimerCount()).toBe(0);
		expect(presence.notice).toBeNull();
		vi.advanceTimersByTime(6000);
		expect(presence.notice).toBeNull();
	});

	it('ignores late events of a socket that was closed on purpose', async () => {
		vi.useFakeTimers();
		install();
		const presence = new Presence();

		presence.connectStart('Anna');
		const staleStart = FakeSocket.instances[0];
		staleStart.onopen?.();

		presence.connectRoom('AAAAAA');
		const roomSocket = FakeSocket.instances[1];
		roomSocket.onopen?.();
		expect(presence.connected).toBe(true);

		// Late callbacks of the intentionally closed start socket.
		staleStart.onclose?.();
		staleStart.onopen?.();
		expect(presence.connected).toBe(true);
		expect(FakeSocket.instances).toHaveLength(2);
		vi.advanceTimersByTime(10000);
		expect(FakeSocket.instances).toHaveLength(2);

		// A pending accept of the current scope survives stale close events.
		presence.connectStart('Anna');
		const current = FakeSocket.instances[2];
		current.onopen?.();
		const pending = presence.accept('i1', 'Anna', '#ff0000');

		staleStart.onclose?.();
		roomSocket.onclose?.();
		expect(presence.connected).toBe(true);

		current.emit({ type: 'accepted', room });
		await expect(pending).resolves.toEqual(room);

		presence.disconnect();
	});

	it('tears down only the scope that is currently active', () => {
		install();
		const presence = new Presence();

		presence.connectStart('Anna');
		const start = FakeSocket.instances[0];
		start.onopen?.();
		start.emit({ type: 'inbox', invites: [invitation('i1')] });

		// A room cleanup while the start page is active is a no-op.
		presence.disconnectRoom('AAAAAA');
		expect(start.readyState).toBe(FakeSocket.OPEN);
		expect(presence.invites).toHaveLength(1);
		expect(presence.connected).toBe(true);

		presence.disconnectStart();
		expect(start.readyState).toBe(FakeSocket.CLOSED);
		expect(presence.invites).toEqual([]);
		expect(presence.connected).toBe(false);

		presence.connectRoom('AAAAAA');
		const roomSocket = FakeSocket.instances[1];
		roomSocket.onopen?.();
		roomSocket.emit({ type: 'lobby', online: [onlineUser('u1', 'Ben')], sent: [] });

		// A start-page cleanup while a room lobby is active is a no-op.
		presence.disconnectStart();
		expect(roomSocket.readyState).toBe(FakeSocket.OPEN);
		expect(presence.online).toHaveLength(1);
		expect(presence.connected).toBe(true);

		presence.disconnectRoom('BBBBBB');
		expect(roomSocket.readyState).toBe(FakeSocket.OPEN);
		expect(presence.connected).toBe(true);

		presence.disconnectRoom('AAAAAA');
		expect(roomSocket.readyState).toBe(FakeSocket.CLOSED);
		expect(presence.online).toEqual([]);
		expect(presence.sent).toEqual([]);
		expect(presence.connected).toBe(false);
	});

	it('sets an error when accept cannot be sent', async () => {
		install();
		const presence = new Presence();

		const offline = presence.accept('i1', 'Anna', '#ff0000');
		await expect(offline).rejects.toThrow('Keine Verbindung zum Server.');
		expect(presence.error).toBe('Keine Verbindung zum Server.');

		presence.connectRoom('AAAAAA');
		FakeSocket.instances[0].onopen?.();
		const roomScope = presence.accept('i2', 'Anna', '#ff0000');
		await expect(roomScope).rejects.toThrow('Keine Verbindung zum Server.');
		expect(presence.error).toBe('Keine Verbindung zum Server.');

		presence.disconnect();
	});
});

describe('isNamePending', () => {
	it('is false for an empty or unchanged value and true for a new name', () => {
		expect(isNamePending('', 'Anna')).toBe(false);
		expect(isNamePending('Anna', 'Anna')).toBe(false);
		expect(isNamePending(' Anna ', 'Anna')).toBe(false);
		expect(isNamePending('Anna Bär', 'Anna')).toBe(true);
	});
});
