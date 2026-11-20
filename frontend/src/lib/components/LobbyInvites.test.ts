import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, within } from '@testing-library/svelte';
import LobbyInvites from './LobbyInvites.svelte';
import type { OnlineUser, RoomSnapshot, SentInvite } from '#lib/types';

function room(): RoomSnapshot {
	return {
		code: 'ABCDEF',
		phase: 'lobby',
		hostId: 'host',
		createdAt: '2026-01-01T00:00:00Z',
		seats: [
			{ userId: 'host', name: 'Entwickler', color: 'blau', host: true, connected: true },
			{ userId: 'anna', name: 'Anna', color: 'rot', host: false, connected: true }
		],
		kickVotes: [],
		you: { id: 'host', name: 'Entwickler', role: 'player', seat: 0, isHost: true },
		canStart: false
	};
}

function online(...users: OnlineUser[]): OnlineUser[] {
	return users;
}

function sent(...invites: SentInvite[]): SentInvite[] {
	return invites;
}

function invite(
	id: string,
	userId: string,
	name: string,
	fromId: string,
	fromName = 'Ben'
): SentInvite {
	return { id, userId, name, fromId, fromName, createdAt: '2026-01-01T00:00:00Z' };
}

const handlers = {
	onInvite: () => {},
	onCancel: () => {},
	onDismissError: () => {}
};

function card(heading: string): HTMLElement {
	return screen.getByRole('heading', { name: heading }).closest('section')!;
}

describe('LobbyInvites', () => {
	it('renders the names of the users who are online', () => {
		render(LobbyInvites, {
			room: room(),
			online: online(
				{ userId: 'ben', name: 'Ben' },
				{ userId: 'cara', name: 'Cara' }
			),
			sent: sent(),
			error: null,
			...handlers
		});

		expect(screen.getByRole('heading', { name: 'In der Lobby online' })).toBeInTheDocument();
		expect(screen.getByText('Ben')).toBeInTheDocument();
		expect(screen.getByText('Cara')).toBeInTheDocument();
		// No sent invitations, so the second card stays hidden.
		expect(screen.queryByRole('heading', { name: 'Gesendete Einladungen' })).toBeNull();
	});

	it('shows a muted empty state when nobody else is online', () => {
		render(LobbyInvites, {
			room: room(),
			online: online(),
			sent: sent(),
			error: null,
			...handlers
		});

		expect(screen.getByText('Gerade ist niemand sonst auf der Startseite.')).toBeInTheDocument();
	});

	it('marks invited users and only invites the others', async () => {
		const onInvite = vi.fn();
		render(LobbyInvites, {
			room: room(),
			online: online(
				{ userId: 'ben', name: 'Ben' },
				{ userId: 'cara', name: 'Cara' }
			),
			sent: sent(invite('inv-1', 'cara', 'Cara', 'host')),
			error: null,
			...handlers,
			onInvite
		});

		const onlineCard = within(card('In der Lobby online'));
		const cara = onlineCard.getByText('Cara').closest('li')!;
		expect(within(cara).getByText('eingeladen')).toBeInTheDocument();
		expect(within(cara).queryByRole('button', { name: 'Cara einladen' })).toBeNull();

		const ben = onlineCard.getByText('Ben').closest('li')!;
		await fireEvent.click(within(ben).getByRole('button', { name: 'Ben einladen' }));
		expect(onInvite).toHaveBeenCalledWith('ben');
	});

	it('shows the stored inviter name even after the inviter left the room', () => {
		render(LobbyInvites, {
			room: room(),
			online: online(),
			// 'zoe' is not in room.seats, so the name must come from the invite itself.
			sent: sent(invite('inv-1', 'ben', 'Ben', 'zoe', 'Zoe')),
			error: null,
			...handlers
		});

		expect(screen.getByText('von Zoe')).toBeInTheDocument();
		expect(screen.queryByText('einem anderen Spieler')).toBeNull();
	});

	it('keeps the invite button and badge in one stable slot', () => {
		render(LobbyInvites, {
			room: room(),
			online: online({ userId: 'ben', name: 'Ben' }),
			sent: sent(invite('inv-1', 'ben', 'Ben', 'host')),
			error: null,
			...handlers
		});

		const ben = within(card('In der Lobby online')).getByText('Ben').closest('li') as HTMLElement;
		const button = within(ben).getByText('Einladen');
		const badge = within(ben).getByText('eingeladen');
		expect(button).toHaveAttribute('aria-hidden', 'true');
		expect(button).toBeDisabled();
		expect(badge).not.toHaveAttribute('aria-hidden');
		expect(button.parentElement).toBe(badge.parentElement);
		expect(button.parentElement).toHaveClass('invite-slot');
	});

	it('keeps the sender text and cancel button in one stable slot', () => {
		render(LobbyInvites, {
			room: room(),
			online: online(),
			sent: sent(invite('inv-1', 'ben', 'Ben', 'host')),
			error: null,
			...handlers
		});

		const ben = screen.getByText('Ben').closest('li') as HTMLElement;
		const sender = within(ben).getByText(/^von /);
		const cancel = within(ben).getByText('Abbrechen');
		expect(sender).toHaveAttribute('aria-hidden', 'true');
		expect(cancel).not.toHaveAttribute('aria-hidden');
		expect(sender.parentElement).toBe(cancel.parentElement);
	});

	it('gives repeated buttons contextual accessible names', () => {
		render(LobbyInvites, {
			room: room(),
			online: online(
				{ userId: 'ben', name: 'Ben' },
				{ userId: 'cara', name: 'Cara' }
			),
			sent: sent(invite('inv-1', 'dora', 'Dora', 'host', 'Entwickler')),
			error: null,
			...handlers
		});

		expect(screen.getByRole('button', { name: 'Ben einladen' })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Cara einladen' })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Einladung an Dora abbrechen' })).toBeInTheDocument();
	});

	it('shows the sent list with the inviter and cancels only own invitations', async () => {
		const onCancel = vi.fn();
		render(LobbyInvites, {
			room: room(),
			online: online(),
			sent: sent(
				invite('inv-1', 'ben', 'Ben', 'host', 'Entwickler'),
				invite('inv-2', 'cara', 'Cara', 'anna', 'Anna')
			),
			error: null,
			...handlers,
			onCancel
		});

		expect(screen.getByRole('heading', { name: 'Gesendete Einladungen' })).toBeInTheDocument();

		const own = screen.getByText('Ben').closest('li')!;
		// The sender text stays mounted in its slot but is hidden from assistive tech.
		expect(within(own).getByText(/^von /)).toHaveAttribute('aria-hidden', 'true');
		await fireEvent.click(within(own).getByRole('button', { name: 'Einladung an Ben abbrechen' }));
		expect(onCancel).toHaveBeenCalledWith('inv-1');

		const foreign = screen.getByText('Cara').closest('li')!;
		expect(within(foreign).getByText('von Anna')).toBeInTheDocument();
		expect(within(foreign).queryByRole('button', { name: 'Einladung an Cara abbrechen' })).toBeNull();
	});

	it('shows a dismissible error banner', async () => {
		const onDismissError = vi.fn();
		render(LobbyInvites, {
			room: room(),
			online: online(),
			sent: sent(),
			error: 'Ben ist nicht mehr online.',
			...handlers,
			onDismissError
		});

		const banner = screen.getByText('Ben ist nicht mehr online.').closest('.error-banner')!;
		await fireEvent.click(within(banner as HTMLElement).getByRole('button', { name: 'Schließen' }));
		expect(onDismissError).toHaveBeenCalledTimes(1);
	});

	it('renders no error banner without an error', () => {
		const { container } = render(LobbyInvites, {
			room: room(),
			online: online(),
			sent: sent(),
			error: null,
			...handlers
		});

		expect(container.querySelector('.error-banner')).toBeNull();
	});
});
