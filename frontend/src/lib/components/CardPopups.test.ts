import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen } from '@testing-library/svelte';
import CardPopups from './CardPopups.svelte';
import type { GameEvent, GameState, Player } from '#lib/types';
import { preferences } from '#lib/prefs.svelte';

function player(seat: number, name: string, overrides: Partial<Player> = {}): Player {
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
		},
		...overrides
	};
}

function autopilotMe(active: boolean): Player {
	return player(0, 'Anton', {
		autopilot: {
			active,
			config: {
				enabled: active,
				buyAlways: true,
				jail: 'none',
				buildGroups: [],
				liquidity: 0
			}
		}
	});
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
		debtor: -1,
		turnPhase: 'awaiting_roll',
		dice: [1, 2],
		doubles: 0,
		trades: [],
		events,
		winner: -1
	};
}

function card(seq: number, deck: GameEvent['deck'], text: string, seat = 0): GameEvent {
	const names = ['Anton', 'Berta', 'Clara', 'Dieter'];
	return {
		seq,
		type: 'card',
		player: seat,
		text: `${names[seat] ?? `Spieler ${seat}`} zieht: ${text}`,
		deck,
		at: '2026-01-01T00:00:00Z'
	};
}

function bankruptcy(overrides: Partial<GameEvent> = {}): GameEvent {
	return {
		seq: 10,
		type: 'bankruptcy',
		player: 0,
		cause: 'miete',
		debt: 420,
		realizable: 180,
		shortfall: 240,
		text: 'Anton ist pleite. Miete an Berta: 420 GE. Realisierbar: 180 GE. Fehlbetrag: 240 GE. Seine Felder gehen an die Bank.',
		at: '2026-01-01T00:00:00Z',
		...overrides
	};
}

/** The fill of the single visible notice, used by the timer/freeze tests. */
function singleFill(): HTMLElement {
	const fill = screen.getByRole('progressbar').querySelector('.timer-fill');
	if (!(fill instanceof HTMLElement)) throw new Error('timer fill not found');
	return fill;
}

async function drawCard(events: GameEvent[], me: Player | null = null) {
	const result = render(CardPopups, { game: game([]), me });
	await act(async () => {
		await result.rerender({ game: game(events), me });
	});
	return result;
}

beforeEach(() => {
	preferences.update({ hideCards: false });
});

afterEach(() => {
	preferences.update({ hideCards: false });
	vi.useRealTimers();
});

describe('CardPopups', () => {
	it('does not replay cards that were already in the first snapshot', () => {
		render(CardPopups, {
			game: game([card(1, 'ereignisfeld', 'Die Bank zahlt dir eine Dividende von 50 GE.')]),
			me: null
		});

		expect(screen.queryByLabelText('Ereigniskarte von Anton')).not.toBeInTheDocument();
		expect(screen.queryByText(/Dividende/)).not.toBeInTheDocument();
		expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
	});

	it('shows a newly drawn card with deck, player and effect, without a modal', async () => {
		await drawCard(
			[
				card(1, 'gemeinschaftsfeld', 'Du erbst 100 GE.'),
				{ seq: 2, type: 'payout', player: 0, amount: 100, text: 'Die Bank zahlt Anton 100 GE.', at: '2026-01-01T00:00:01Z' },
				{ seq: 3, type: 'turn', player: 1, text: 'Berta ist am Zug.', at: '2026-01-01T00:00:02Z' }
			],
			player(0, 'Anton')
		);

		const notice = screen.getByLabelText('Gemeinschaftskarte von Anton');
		expect(notice.textContent).toContain('Du erbst 100 GE.');
		expect(notice.textContent).toContain('Anton zieht');
		expect(notice.textContent).toContain('für dich');
		expect(notice.textContent).toContain('Die Bank zahlt Anton 100 GE.');
		// The next turn is not part of the card's outcome.
		expect(notice.textContent).not.toContain('Berta ist am Zug.');

		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
		expect(document.body.style.overflow).toBe('');
		expect(document.activeElement).toBe(document.body);
	});

	it('dismisses the notice automatically after 12 seconds', async () => {
		vi.useFakeTimers();
		await drawCard([card(1, 'ereignisfeld', 'Gehe drei Felder zurück.')]);

		expect(screen.getByLabelText('Ereigniskarte von Anton')).toBeInTheDocument();
		await act(async () => vi.advanceTimersByTime(11_900));
		expect(screen.getByLabelText('Ereigniskarte von Anton')).toBeInTheDocument();

		await act(async () => vi.advanceTimersByTime(100));
		expect(screen.queryByLabelText('Ereigniskarte von Anton')).not.toBeInTheDocument();
		expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
	});

	it('shows the remaining time on a progress bar', async () => {
		vi.useFakeTimers();
		await drawCard([card(1, 'ereignisfeld', 'Zahle 50 GE.')]);

		await act(async () => vi.advanceTimersByTime(6_000));
		const bar = screen.getByRole('progressbar', { name: 'Verbleibende Anzeigezeit' });
		expect(bar).toHaveAttribute('aria-valuemin', '0');
		expect(bar).toHaveAttribute('aria-valuemax', '12');
		expect(bar).toHaveAttribute('aria-valuenow', '6');
		expect(singleFill().style.width).toBe('50%');

		await act(async () => vi.advanceTimersByTime(6_000));
		expect(screen.queryByLabelText('Ereigniskarte von Anton')).not.toBeInTheDocument();
	});

	it('freezes countdown and bar while the pointer hovers the notice', async () => {
		vi.useFakeTimers();
		await drawCard([card(1, 'ereignisfeld', 'Zahle 50 GE.')]);

		await act(async () => vi.advanceTimersByTime(4_000));
		const notice = screen.getByLabelText('Ereigniskarte von Anton');
		const frozenWidth = singleFill().style.width;
		await act(async () => {
			await fireEvent.mouseEnter(notice);
		});

		await act(async () => vi.advanceTimersByTime(5_000));
		expect(singleFill().style.width).toBe(frozenWidth);
		expect(screen.getByLabelText('Ereigniskarte von Anton')).toBeInTheDocument();

		await act(async () => {
			await fireEvent.mouseLeave(notice);
		});
		await act(async () => vi.advanceTimersByTime(8_000));
		expect(screen.queryByLabelText('Ereigniskarte von Anton')).not.toBeInTheDocument();
	});

	it('freezes countdown and bar while the notice has keyboard focus', async () => {
		vi.useFakeTimers();
		await drawCard([card(1, 'ereignisfeld', 'Zahle 50 GE.')]);

		await act(async () => vi.advanceTimersByTime(4_000));
		const notice = screen.getByLabelText('Ereigniskarte von Anton');
		const frozenWidth = singleFill().style.width;
		await act(async () => {
			await fireEvent.focusIn(notice);
		});

		await act(async () => vi.advanceTimersByTime(5_000));
		expect(singleFill().style.width).toBe(frozenWidth);
		expect(screen.getByLabelText('Ereigniskarte von Anton')).toBeInTheDocument();

		await act(async () => {
			await fireEvent.focusOut(notice);
		});
		await act(async () => vi.advanceTimersByTime(8_000));
		expect(screen.queryByLabelText('Ereigniskarte von Anton')).not.toBeInTheDocument();
	});

	it('shows several cards at once with independent timers', async () => {
		vi.useFakeTimers();
		await drawCard([
			card(1, 'ereignisfeld', 'Zahle 50 GE.', 0),
			card(2, 'gemeinschaftsfeld', 'Du erbst 100 GE.', 1)
		]);

		expect(screen.getAllByRole('progressbar')).toHaveLength(2);
		const closeButtons = screen.getAllByRole('button', { name: 'Karte schließen' });
		expect(closeButtons).toHaveLength(2);

		await act(async () => {
			await fireEvent.click(closeButtons[0]);
		});
		expect(screen.getAllByRole('progressbar')).toHaveLength(1);
		expect(screen.getByLabelText('Gemeinschaftskarte von Berta')).toBeInTheDocument();

		await act(async () => vi.advanceTimersByTime(12_000));
		expect(screen.queryAllByRole('progressbar')).toHaveLength(0);
	});

	it('closes a notice immediately with the close button', async () => {
		await drawCard([card(1, 'ereignisfeld', 'Gehe drei Felder zurück.')]);

		await fireEvent.click(screen.getByRole('button', { name: 'Karte schließen' }));
		expect(screen.queryByLabelText('Ereigniskarte von Anton')).not.toBeInTheDocument();
		expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
	});

	it('suppresses notices when the autopilot is active and hideCards is set', async () => {
		await act(async () => {
			preferences.update({ hideCards: true });
		});
		await drawCard([card(1, 'ereignisfeld', 'Zahle 50 GE.')], autopilotMe(true));

		expect(screen.queryByLabelText('Ereigniskarte von Anton')).not.toBeInTheDocument();
		expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
	});

	it('still shows notices when hideCards is set but the autopilot is inactive', async () => {
		await act(async () => {
			preferences.update({ hideCards: true });
		});
		await drawCard([card(1, 'ereignisfeld', 'Zahle 50 GE.')], autopilotMe(false));

		expect(screen.getByLabelText('Ereigniskarte von Anton')).toBeInTheDocument();
		expect(screen.getByRole('progressbar')).toBeInTheDocument();
	});

	it('still shows notices with an active autopilot and hideCards unset', async () => {
		await drawCard([card(1, 'ereignisfeld', 'Zahle 50 GE.')], autopilotMe(true));

		expect(screen.getByLabelText('Ereigniskarte von Anton')).toBeInTheDocument();
		expect(screen.getByRole('progressbar')).toBeInTheDocument();
	});

	it('removes a visible notice as soon as suppression becomes active', async () => {
		const events = [card(1, 'ereignisfeld', 'Zahle 50 GE.')];
		const { rerender } = await drawCard(events, autopilotMe(true));

		expect(screen.getByLabelText('Ereigniskarte von Anton')).toBeInTheDocument();

		await act(async () => {
			preferences.update({ hideCards: true });
			await rerender({ game: game(events), me: autopilotMe(true) });
		});

		expect(screen.queryByLabelText('Ereigniskarte von Anton')).not.toBeInTheDocument();
		expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
	});

	it('does not show cards drawn during suppression after it ends', async () => {
		const events = [card(1, 'ereignisfeld', 'Zahle 50 GE.')];
		await act(async () => {
			preferences.update({ hideCards: true });
		});
		const { rerender } = await drawCard(events, autopilotMe(true));

		expect(screen.queryByLabelText('Ereigniskarte von Anton')).not.toBeInTheDocument();

		await act(async () => {
			await rerender({ game: game(events), me: autopilotMe(false) });
		});
		expect(screen.queryByLabelText('Ereigniskarte von Anton')).not.toBeInTheDocument();

		await act(async () => {
			preferences.update({ hideCards: false });
			await rerender({ game: game(events), me: autopilotMe(true) });
		});
		expect(screen.queryByLabelText('Ereigniskarte von Anton')).not.toBeInTheDocument();
		expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
	});

	it('shows a card drawn after suppression ends', async () => {
		const events = [card(1, 'ereignisfeld', 'Zahle 50 GE.')];
		await act(async () => {
			preferences.update({ hideCards: true });
		});
		const { rerender } = await drawCard(events, autopilotMe(true));

		await act(async () => {
			preferences.update({ hideCards: false });
			await rerender({ game: game(events), me: autopilotMe(true) });
		});
		expect(screen.queryByLabelText('Ereigniskarte von Anton')).not.toBeInTheDocument();

		const next = [...events, card(2, 'gemeinschaftsfeld', 'Du erbst 100 GE.')];
		await act(async () => {
			await rerender({ game: game(next), me: autopilotMe(true) });
		});

		expect(screen.queryByLabelText('Ereigniskarte von Anton')).not.toBeInTheDocument();
		expect(screen.getByLabelText('Gemeinschaftskarte von Anton')).toBeInTheDocument();
	});

	it('does not replay a bankruptcy event from the first snapshot', () => {
		render(CardPopups, { game: game([bankruptcy()]), me: null });

		expect(screen.queryByLabelText('Pleite von Anton')).not.toBeInTheDocument();
		expect(document.activeElement).toBe(document.body);
	});

	it('shows a dismissible bankruptcy notice for all viewers', async () => {
		await drawCard([bankruptcy()], null);

		const notice = screen.getByLabelText('Pleite von Anton');
		expect(notice.textContent).toContain('Miete');
		expect(notice.textContent).toContain('420 GE');
		expect(notice.textContent).toContain('240 GE');
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
		expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
		expect(document.activeElement).toBe(document.body);

		await fireEvent.click(screen.getByRole('button', { name: 'Hinweis schließen' }));
		expect(screen.queryByLabelText('Pleite von Anton')).not.toBeInTheDocument();
	});

	it('shows exactly one notice per bankrupt player', async () => {
		await drawCard([bankruptcy({ seq: 10 }), bankruptcy({ seq: 11 })], null);

		expect(screen.getAllByLabelText('Pleite von Anton')).toHaveLength(1);
	});

	it('keeps card notices working next to a bankruptcy notice', async () => {
		await drawCard([bankruptcy(), card(11, 'ereignisfeld', 'Zahle 50 GE.')], null);

		expect(screen.getByLabelText('Pleite von Anton')).toBeInTheDocument();
		expect(screen.getByLabelText('Ereigniskarte von Anton')).toBeInTheDocument();
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
	});
});
