import { describe, expect, it } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import AutopilotToast from './AutopilotToast.svelte';
import type { GameEvent, GameState, Player } from '#lib/types';

function player(seat: number, name: string): Player {
	return {
		id: `user-${seat}`,
		name,
		color: 'blau',
		seat,
		cash: 1500,
		liquidationValue: 1500,
		position: 0,
		inJail: false,
		jailAttempts: 0,
		jailCards: 0,
		bankrupt: false,
		connected: true,
		netWorth: 1500,
		stats: {
			rentCollected: 0,
			rentPaid: 0,
			propertiesBought: 0,
			propertiesSold: 0,
			housesBuilt: 0,
			trades: 0
		}
	};
}

function game(events: GameEvent[]): GameState {
	return {
		phase: 'playing',
		rules: {
			minPlayers: 2,
			maxPlayers: 8,
			startingCash: 1500,
			losSalary: 200,
			jailFine: 50,
			auctionSeconds: 15
		},
		board: [],
		players: [player(0, 'Anton'), player(1, 'Berta')],
		tiles: [],
		current: 0,
		debtor: -1,
		turnPhase: 'awaiting_roll',
		dice: [1, 2],
		doubles: 0,
		trades: [],
		events,
		winner: -1
	};
}

function stop(seq: number, seat: number, reason?: string): GameEvent {
	return {
		seq,
		type: 'autopilot_off',
		player: seat,
		text: 'Autopilot aus.',
		reason,
		at: '2026-01-01T00:00:00Z'
	};
}

const anton = player(0, 'Anton');

describe('AutopilotToast', () => {
	it('does not replay stops that were already in the first snapshot', () => {
		render(AutopilotToast, {
			game: game([stop(1, 0, 'purchase')]),
			me: anton
		});

		expect(screen.queryByRole('status')).not.toBeInTheDocument();
	});

	it('shows a new stop for the viewer and dismisses it', async () => {
		const { rerender } = render(AutopilotToast, { game: game([]), me: anton });

		await rerender({ game: game([stop(1, 0, 'purchase')]) });

		const toast = await screen.findByRole('status');
		expect(toast.textContent).toContain('Autopilot beendet');
		expect(toast.textContent).toContain('Kaufentscheidung nicht automatisiert');

		await fireEvent.click(screen.getByRole('button', { name: 'Schließen' }));

		await waitFor(() => expect(screen.queryByRole('status')).not.toBeInTheDocument());
	});

	it('ignores stops of another seat', async () => {
		const { rerender } = render(AutopilotToast, { game: game([]), me: anton });

		await rerender({ game: game([stop(1, 1, 'purchase')]) });

		expect(screen.queryByRole('status')).not.toBeInTheDocument();
	});

	it('ignores a manual stop', async () => {
		const { rerender } = render(AutopilotToast, { game: game([]), me: anton });

		await rerender({ game: game([stop(1, 0, 'manual')]) });

		expect(screen.queryByRole('status')).not.toBeInTheDocument();
	});

	it('falls back to a generic reason when it is missing', async () => {
		const { rerender } = render(AutopilotToast, { game: game([]), me: anton });

		await rerender({ game: game([stop(1, 0)]) });

		const toast = await screen.findByRole('status');
		expect(toast.textContent).toContain('Unbekannter Grund');
	});

	it('queues several stops and reveals the next one after dismissing', async () => {
		const { rerender } = render(AutopilotToast, { game: game([]), me: anton });

		await rerender({ game: game([stop(1, 0, 'purchase'), stop(2, 0, 'debt')]) });

		const toast = await screen.findByRole('status');
		expect(screen.getAllByRole('status')).toHaveLength(1);
		expect(toast.textContent).toContain('Kaufentscheidung nicht automatisiert');

		await fireEvent.click(screen.getByRole('button', { name: 'Schließen' }));

		await waitFor(() =>
			expect(screen.getByRole('status').textContent).toContain(
				'Schulden müssen beglichen werden'
			)
		);
		expect(screen.getAllByRole('status')).toHaveLength(1);
	});
});
