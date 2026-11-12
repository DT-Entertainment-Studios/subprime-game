import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, within } from '@testing-library/svelte';
import Invitations from './Invitations.svelte';
import type { Invitation } from '#lib/types';

function invitation(over: Partial<Invitation> = {}): Invitation {
	return {
		id: 'inv-1',
		room: 'ABCDEF',
		fromId: 'anna',
		fromName: 'Anna',
		createdAt: '2026-01-01T00:00:00Z',
		...over
	};
}

const handlers = {
	onAccept: () => {},
	onDecline: () => {},
	onDismissNotice: () => {}
};

describe('Invitations', () => {
	it('lists every invitation with room code and inviter', () => {
		render(Invitations, {
			invites: [
				invitation(),
				invitation({ id: 'inv-2', room: 'QWERTY', fromId: 'ben', fromName: 'Ben' })
			],
			notice: null,
			...handlers
		});

		expect(screen.getByRole('heading', { name: 'Einladungen' })).toBeInTheDocument();
		expect(screen.getByText('Raum ABCDEF')).toBeInTheDocument();
		expect(screen.getByText('von Anna')).toBeInTheDocument();
		expect(screen.getByText('Raum QWERTY')).toBeInTheDocument();
		expect(screen.getByText('von Ben')).toBeInTheDocument();
	});

	it('accepts and declines the matching invitation', async () => {
		const onAccept = vi.fn();
		const onDecline = vi.fn();
		const first = invitation();
		const second = invitation({ id: 'inv-2', room: 'QWERTY', fromId: 'ben', fromName: 'Ben' });

		render(Invitations, {
			invites: [first, second],
			notice: null,
			...handlers,
			onAccept,
			onDecline
		});

		const firstItem = screen.getByText('Raum ABCDEF').closest('li')!;
		await fireEvent.click(
			within(firstItem).getByRole('button', { name: 'Einladung nach ABCDEF annehmen' })
		);
		expect(onAccept).toHaveBeenCalledWith(first);

		await fireEvent.click(
			within(firstItem).getByRole('button', { name: 'Einladung nach ABCDEF ablehnen' })
		);
		expect(onDecline).toHaveBeenCalledWith(first);
	});

	it('names each action with its room so repeated buttons stay distinguishable', () => {
		render(Invitations, {
			invites: [
				invitation(),
				invitation({ id: 'inv-2', room: 'QWERTY', fromId: 'ben', fromName: 'Ben' })
			],
			notice: null,
			...handlers
		});

		expect(
			screen.getByRole('button', { name: 'Einladung nach ABCDEF annehmen' })
		).toBeInTheDocument();
		expect(
			screen.getByRole('button', { name: 'Einladung nach ABCDEF ablehnen' })
		).toBeInTheDocument();
		expect(
			screen.getByRole('button', { name: 'Einladung nach QWERTY annehmen' })
		).toBeInTheDocument();
		expect(
			screen.getByRole('button', { name: 'Einladung nach QWERTY ablehnen' })
		).toBeInTheDocument();
	});

	it('shows a dismissible notice for a new invitation', async () => {
		const onDismissNotice = vi.fn();
		render(Invitations, {
			invites: [],
			notice: invitation(),
			...handlers,
			onDismissNotice
		});

		expect(screen.getByText('Neue Einladung von Anna in Raum ABCDEF.')).toBeInTheDocument();
		await fireEvent.click(screen.getByRole('button', { name: 'Schließen' }));
		expect(onDismissNotice).toHaveBeenCalledTimes(1);
	});

	it('renders nothing without invitations and notice', () => {
		const { container } = render(Invitations, {
			invites: [],
			notice: null,
			...handlers
		});

		expect(container.querySelector('*')).toBeNull();
		expect(container.textContent?.trim()).toBe('');
	});
});
