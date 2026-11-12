// REST client for the non-realtime endpoints (Spec.md §6).

import type { ActiveRoom, RoomSnapshot, User } from './types';

export class ApiError extends Error {
	code: string;
	status: number;

	constructor(status: number, code: string, message: string) {
		super(message);
		this.name = 'ApiError';
		this.status = status;
		this.code = code;
	}
}

interface ErrorEnvelope {
	error?: { message?: string; code?: string };
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
	const response = await fetch(path, {
		...init,
		headers: {
			'Content-Type': 'application/json',
			...init?.headers
		}
	});
	const text = await response.text();
	let body: unknown = null;
	if (text) {
		try {
			body = JSON.parse(text);
		} catch {
			body = null;
		}
	}
	if (!response.ok) {
		const envelope = (body ?? {}) as ErrorEnvelope;
		throw new ApiError(
			response.status,
			envelope.error?.code ?? 'internal',
			envelope.error?.message ?? `Fehler ${response.status}`
		);
	}
	return body as T;
}

export async function fetchMe(): Promise<User> {
	const body = await request<{ user: User }>('/api/me');
	return body.user;
}

/**
 * The room that currently blocks create/join/invitations, or null. Uses the same
 * server-side rule as those endpoints, so the start page never guesses.
 */
export async function fetchMyRoom(): Promise<ActiveRoom | null> {
	const body = await request<{ room: ActiveRoom | null }>('/api/me/room');
	return body.room;
}

export async function createRoom(displayName: string, color: string): Promise<RoomSnapshot> {
	const body = await request<{ room: RoomSnapshot }>('/api/rooms', {
		method: 'POST',
		body: JSON.stringify({ displayName, color })
	});
	return body.room;
}

export async function joinRoom(
	code: string,
	displayName: string,
	color: string
): Promise<RoomSnapshot> {
	const body = await request<{ room: RoomSnapshot }>(`/api/rooms/${encodeURIComponent(code)}/join`, {
		method: 'POST',
		body: JSON.stringify({ displayName, color })
	});
	return body.room;
}

export async function fetchRoom(code: string): Promise<RoomSnapshot> {
	const body = await request<{ room: RoomSnapshot }>(`/api/rooms/${encodeURIComponent(code)}`);
	return body.room;
}

/**
 * Frees the caller's lobby seat. A room that is already gone (404) is treated
 * as success: the seat cannot be occupied any more.
 */
export async function leaveRoom(code: string): Promise<void> {
	try {
		await request<null>(`/api/rooms/${encodeURIComponent(code)}/leave`, { method: 'POST' });
	} catch (err) {
		if (err instanceof ApiError && err.status === 404) return;
		throw err;
	}
}

/**
 * Finally gives up the caller's seat. A room that is already gone (404) counts as
 * success, like leaveRoom: the seat cannot be occupied any more.
 */
export async function abandonRoom(code: string): Promise<void> {
	try {
		await request<null>(`/api/rooms/${encodeURIComponent(code)}/abandon`, {
			method: 'POST'
		});
	} catch (err) {
		if (err instanceof ApiError && err.status === 404) return;
		throw err;
	}
}

export function websocketURL(code: string): string {
	const scheme = window.location.protocol === 'https:' ? 'wss' : 'ws';
	return `${scheme}://${window.location.host}/api/ws?room=${encodeURIComponent(code)}`;
}

/** Presence channel URL options: only the room code, never a display name. */
export interface PresenceURLOptions {
	/** Room code; omitted for a start-page connection. */
	room?: string;
}

export function presenceURL({ room }: PresenceURLOptions = {}): string {
	const scheme = window.location.protocol === 'https:' ? 'wss' : 'ws';
	const suffix = room ? `?room=${encodeURIComponent(room)}` : '';
	return `${scheme}://${window.location.host}/api/ws/presence${suffix}`;
}
