import { describe, expect, it } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import CardPopups from './CardPopups.svelte';
import type { GameEvent, GameState, Player } from '#lib/types';

function player(seat: number, name: string): Player {
	return {
		id: `user-${seat}`,
		name,
		color: 'blau',
		seat,
		cash: 1500,
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
			auctionSeconds: 15,
		},
		board: [],
		players: [player(0, 'Anton'), player(1, 'Berta')],
		tiles: [],
		current: 0,
		turnPhase: 'awaiting_roll',
		dice: [1, 2],
		doubles: 0,
		trades: [],
		events,
		winner: -1
	};
}

function card(seq: number, deck: GameEvent['deck'], text: string): GameEvent {
	return {
		seq,
		type: 'card',
		player: 0,
		text: `Anton zieht: ${text}`,
		deck,
		at: '2026-01-01T00:00:00Z'
	};
}

describe('CardPopups', () => {
	it('does not replay cards that were already in the first snapshot', () => {
		render(CardPopups, {
			game: game([card(1, 'ereignisfeld', 'Die Bank zahlt dir eine Dividende von 50 GE.')]),
			me: null
		});

		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
	});

	it('shows a newly drawn card with deck, player and effect', async () => {
		const { rerender } = render(CardPopups, { game: game([]), me: player(0, 'Anton') });

		await rerender({
			game: game([
				card(1, 'gemeinschaftsfeld', 'Du erbst 100 GE.'),
				{ seq: 2, type: 'payout', player: 0, amount: 100, text: 'Die Bank zahlt Anton 100 GE.', at: '2026-01-01T00:00:01Z' },
				{ seq: 3, type: 'turn', player: 1, text: 'Berta ist am Zug.', at: '2026-01-01T00:00:02Z' }
			])
		});

		const dialog = await screen.findByRole('dialog', { name: 'Gemeinschaftskarte von Anton' });
		expect(dialog.textContent).toContain('Du erbst 100 GE.');
		expect(dialog.textContent).toContain('Anton zieht');
		expect(dialog.textContent).toContain('für dich');
		expect(dialog.textContent).toContain('Die Bank zahlt Anton 100 GE.');
		// The next turn is not part of the card's outcome.
		expect(dialog.textContent).not.toContain('Berta ist am Zug.');
	});

	it('closes the popup and keeps the event log untouched', async () => {
		const { rerender } = render(CardPopups, { game: game([]), me: null });

		await rerender({ game: game([card(1, 'ereignisfeld', 'Gehe drei Felder zurück.')]) });

		await screen.findByRole('dialog', { name: 'Ereigniskarte von Anton' });
		await fireEvent.click(screen.getByRole('button', { name: 'Weiter' }));

		await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
	});
});
