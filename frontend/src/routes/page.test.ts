import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import type { ActiveRoom } from '#lib/types';
import Page from './+page.svelte';

// The page is a thin wiring layer: the session, presence and router are mocked
// so the test asserts the banner visibility and the disabled create/join cards
// without opening sockets or hitting the REST API.
const mocks = vi.hoisted(() => ({
	goto: vi.fn(),
	session: {
		user: null as { name: string } | null,
		room: null,
		activeRoom: null as ActiveRoom | null,
		connected: false,
		error: null as string | null,
		loading: false,
		init: vi.fn().mockResolvedValue(undefined),
		refreshActiveRoom: vi.fn().mockResolvedValue(undefined),
		abandonActiveRoom: vi.fn().mockResolvedValue(true),
		clearError: vi.fn(),
		create: vi.fn(),
		join: vi.fn(),
		leave: vi.fn()
	},
	presence: {
		invites: [],
		notice: null,
		error: null as string | null,
		publishedName: '',
		connectStart: vi.fn(),
		disconnectStart: vi.fn(),
		accept: vi.fn(),
		decline: vi.fn(),
		dismissNotice: vi.fn(),
		clearError: vi.fn()
	}
}));

vi.mock('$app/navigation', () => ({ goto: mocks.goto }));
vi.mock('#lib/session.svelte', () => ({ session: mocks.session }));
vi.mock('#lib/presence.svelte', () => ({
	presence: mocks.presence,
	isNamePending: () => false
}));

const ACTIVE_ROOM: ActiveRoom = { code: 'ABCDEF', phase: 'playing' };

beforeEach(() => {
	mocks.goto.mockReset();
	mocks.session.user = null;
	mocks.session.room = null;
	mocks.session.activeRoom = null;
	mocks.session.error = null;
	mocks.session.loading = false;
	mocks.session.init.mockClear();
	mocks.session.refreshActiveRoom.mockClear();
	mocks.session.abandonActiveRoom.mockClear();
	mocks.presence.connectStart.mockClear();
	mocks.presence.disconnectStart.mockClear();
});

describe('start page', () => {
	it('shows the active-room banner and disables creating and joining', () => {
		mocks.session.activeRoom = ACTIVE_ROOM;
		render(Page);

		expect(
			screen.getByRole('heading', { name: 'Du bist noch in Raum ABCDEF' })
		).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Raum erstellen' })).toBeDisabled();
		expect(screen.getByRole('button', { name: 'Beitreten' })).toBeDisabled();
	});

	it('keeps joining disabled while a code is typed', async () => {
		mocks.session.activeRoom = ACTIVE_ROOM;
		render(Page);

		await fireEvent.input(screen.getByLabelText('Raumcode'), { target: { value: 'QWERTY' } });

		expect(screen.getByRole('button', { name: 'Beitreten' })).toBeDisabled();
	});

	it('rejoins the active room', async () => {
		mocks.session.activeRoom = ACTIVE_ROOM;
		render(Page);

		await fireEvent.click(screen.getByRole('button', { name: 'Wieder beitreten' }));

		expect(mocks.goto).toHaveBeenCalledWith('/room/ABCDEF');
	});

	it('hides the banner and enables creating without an active room', () => {
		render(Page);

		expect(screen.queryByRole('heading', { name: /Du bist noch in Raum/ })).not.toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Raum erstellen' })).toBeEnabled();
		// Joining still needs a code.
		expect(screen.getByRole('button', { name: 'Beitreten' })).toBeDisabled();
	});

	it('enables joining once a code is entered and no banner is shown', async () => {
		render(Page);

		await fireEvent.input(screen.getByLabelText('Raumcode'), { target: { value: 'QWERTY' } });

		expect(screen.getByRole('button', { name: 'Beitreten' })).toBeEnabled();
	});

	it('shows the existing error banner for a session error', () => {
		mocks.session.error = 'Serverfehler.';
		const { container } = render(Page);

		const banner = container.querySelector('.error-banner');
		expect(banner).not.toBeNull();
		expect(banner).toHaveTextContent('Serverfehler.');
	});

	it('refreshes the active room on mount', async () => {
		render(Page);

		await waitFor(() => expect(mocks.session.refreshActiveRoom).toHaveBeenCalled());
	});

	it('refreshes the active room when the window regains focus', async () => {
		render(Page);
		await waitFor(() => expect(mocks.session.refreshActiveRoom).toHaveBeenCalledTimes(1));

		window.dispatchEvent(new Event('focus'));

		expect(mocks.session.refreshActiveRoom).toHaveBeenCalledTimes(2);
	});

	it('removes the focus listener on unmount', async () => {
		const { unmount } = render(Page);
		await waitFor(() => expect(mocks.session.refreshActiveRoom).toHaveBeenCalledTimes(1));

		unmount();
		window.dispatchEvent(new Event('focus'));

		expect(mocks.session.refreshActiveRoom).toHaveBeenCalledTimes(1);
	});
});
