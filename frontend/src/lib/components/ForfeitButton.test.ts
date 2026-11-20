import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/svelte';
import ForfeitButton from './ForfeitButton.svelte';
import type { Player, TurnPhase } from '#lib/types';

function player(overrides: Partial<Player> = {}): Player {
	return {
		id: 'anna',
		name: 'Anna',
		color: 'blau',
		seat: 0,
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

function forfeitTrigger(): HTMLElement | null {
	return screen.queryByRole('button', { name: 'Aufgeben' });
}

const ELIGIBLE: TurnPhase[] = ['awaiting_roll', 'awaiting_purchase', 'awaiting_jail'];
const INELIGIBLE: TurnPhase[] = ['awaiting_debt', 'auction', 'finished'];

describe('ForfeitButton', () => {
	// Regression guard: the debt phase has its own give-up panel and blocks the
	// whole game, so the voluntary topbar forfeit stays visible but disabled there.
	it('never offers a forfeit during the debt phase', () => {
		render(ForfeitButton, {
			phase: 'playing',
			turnPhase: 'awaiting_debt',
			isMyTurn: true,
			me: player({ cash: -100 }),
			onIntent: () => {}
		});

		const trigger = screen.getByRole('button', { name: 'Aufgeben' });
		expect(trigger).toBeDisabled();
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
	});

	it.each(ELIGIBLE)('offers the forfeit in own turn phase %s', (turnPhase) => {
		render(ForfeitButton, {
			phase: 'playing',
			turnPhase,
			isMyTurn: true,
			me: player(),
			onIntent: () => {}
		});

		expect(forfeitTrigger()).not.toBeNull();
	});

	it.each(INELIGIBLE)('stays visible but disabled in phase %s', (turnPhase) => {
		render(ForfeitButton, {
			phase: 'playing',
			turnPhase,
			isMyTurn: true,
			me: player(),
			onIntent: () => {}
		});

		const trigger = screen.getByRole('button', { name: 'Aufgeben' });
		expect(trigger).toBeDisabled();
	});

	it('keeps a visible, disabled trigger in the DOM when ineligible', () => {
		render(ForfeitButton, {
			phase: 'finished',
			turnPhase: 'awaiting_roll',
			isMyTurn: true,
			me: player(),
			onIntent: () => {}
		});

		const trigger = screen.getByRole('button', { name: 'Aufgeben' });
		expect(trigger).toBeDisabled();
		expect(trigger).not.toHaveAttribute('aria-hidden');
		expect(trigger).not.toHaveAttribute('tabindex');
		expect(trigger).not.toHaveClass('slot-hidden');
	});

	it('stays visible but disabled once the game is finished', () => {
		render(ForfeitButton, {
			phase: 'finished',
			turnPhase: 'awaiting_roll',
			isMyTurn: true,
			me: player(),
			onIntent: () => {}
		});

		const trigger = screen.getByRole('button', { name: 'Aufgeben' });
		expect(trigger).toBeDisabled();
	});

	it('stays visible but disabled when it is not the viewer’s turn', () => {
		render(ForfeitButton, {
			phase: 'playing',
			turnPhase: 'awaiting_roll',
			isMyTurn: false,
			me: player(),
			onIntent: () => {}
		});

		const trigger = screen.getByRole('button', { name: 'Aufgeben' });
		expect(trigger).toBeDisabled();
	});

	it('stays visible but disabled for a spectator without a player', () => {
		render(ForfeitButton, {
			phase: 'playing',
			turnPhase: 'awaiting_roll',
			isMyTurn: true,
			me: null,
			onIntent: () => {}
		});

		const trigger = screen.getByRole('button', { name: 'Aufgeben' });
		expect(trigger).toBeDisabled();
	});

	it('stays visible but disabled for an already eliminated player', () => {
		render(ForfeitButton, {
			phase: 'playing',
			turnPhase: 'awaiting_roll',
			isMyTurn: true,
			me: player({ bankrupt: true }),
			onIntent: () => {}
		});

		const trigger = screen.getByRole('button', { name: 'Aufgeben' });
		expect(trigger).toBeDisabled();
	});

	it('asks for confirmation before sending the intent', async () => {
		const onIntent = vi.fn();
		render(ForfeitButton, {
			phase: 'playing',
			turnPhase: 'awaiting_roll',
			isMyTurn: true,
			me: player(),
			onIntent
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Aufgeben' }));

		const dialog = screen.getByRole('dialog', { name: 'Aufgeben bestätigen' });
		expect(dialog.textContent).toContain('Du scheidest endgültig aus dem Spiel aus');
		expect(dialog.textContent).toMatch(/Gebäude und\s+Felder gehen an die Bank/);
		expect(onIntent).not.toHaveBeenCalled();
	});

	it('describes the dialog with the consequence text', async () => {
		render(ForfeitButton, {
			phase: 'playing',
			turnPhase: 'awaiting_roll',
			isMyTurn: true,
			me: player(),
			onIntent: () => {}
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Aufgeben' }));

		const dialog = screen.getByRole('dialog');
		const describedBy = dialog.getAttribute('aria-describedby');
		expect(describedBy).toBeTruthy();
		expect(document.getElementById(describedBy as string)).toHaveTextContent(
			'Du scheidest endgültig aus dem Spiel aus'
		);
	});

	it('sends exactly the forfeit intent after the confirmation', async () => {
		const onIntent = vi.fn();
		render(ForfeitButton, {
			phase: 'playing',
			turnPhase: 'awaiting_roll',
			isMyTurn: true,
			me: player(),
			onIntent
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Aufgeben' }));
		await fireEvent.click(screen.getByRole('button', { name: 'Ja, aufgeben' }));

		expect(onIntent).toHaveBeenCalledTimes(1);
		expect(onIntent).toHaveBeenCalledWith({ type: 'forfeit' });
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
	});

	it('closes without an intent when cancelled', async () => {
		const onIntent = vi.fn();
		render(ForfeitButton, {
			phase: 'playing',
			turnPhase: 'awaiting_roll',
			isMyTurn: true,
			me: player(),
			onIntent
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Aufgeben' }));
		await fireEvent.click(screen.getByRole('button', { name: 'Abbrechen' }));

		expect(onIntent).not.toHaveBeenCalled();
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
	});

	it('closes without an intent on Escape', async () => {
		const onIntent = vi.fn();
		render(ForfeitButton, {
			phase: 'playing',
			turnPhase: 'awaiting_roll',
			isMyTurn: true,
			me: player(),
			onIntent
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Aufgeben' }));
		await fireEvent.keyDown(window, { key: 'Escape' });

		expect(onIntent).not.toHaveBeenCalled();
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
	});

	it('closes without an intent on a backdrop click', async () => {
		const onIntent = vi.fn();
		const { container } = render(ForfeitButton, {
			phase: 'playing',
			turnPhase: 'awaiting_roll',
			isMyTurn: true,
			me: player(),
			onIntent
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Aufgeben' }));
		const backdrop = container.querySelector<HTMLElement>('.backdrop');
		expect(backdrop).not.toBeNull();
		await fireEvent.click(backdrop as HTMLElement);

		expect(onIntent).not.toHaveBeenCalled();
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
	});

	it('closes the dialog when the forfeit stops being allowed', async () => {
		const onIntent = vi.fn();
		const { rerender } = render(ForfeitButton, {
			phase: 'playing',
			turnPhase: 'awaiting_roll',
			isMyTurn: true,
			me: player(),
			onIntent
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Aufgeben' }));
		expect(screen.getByRole('dialog')).toBeInTheDocument();

		await rerender({
			phase: 'playing',
			turnPhase: 'awaiting_debt',
			isMyTurn: true,
			me: player(),
			onIntent
		});

		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
		expect(onIntent).not.toHaveBeenCalled();
	});

	it('moves focus into the dialog and back to the trigger on close', async () => {
		render(ForfeitButton, {
			phase: 'playing',
			turnPhase: 'awaiting_roll',
			isMyTurn: true,
			me: player(),
			onIntent: () => {}
		});

		const trigger = screen.getByRole('button', { name: 'Aufgeben' });
		// jsdom does not move focus on click the way a browser does.
		trigger.focus();
		await fireEvent.click(trigger);
		expect(screen.getByRole('dialog')).toHaveFocus();

		await fireEvent.click(screen.getByRole('button', { name: 'Abbrechen' }));
		expect(trigger).toHaveFocus();
	});

	it('warns that the open purchase offer lapses in the purchase phase', async () => {
		render(ForfeitButton, {
			phase: 'playing',
			turnPhase: 'awaiting_purchase',
			isMyTurn: true,
			me: player(),
			onIntent: () => {}
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Aufgeben' }));

		expect(screen.getByRole('dialog').textContent).toContain('Das offene Kaufangebot verfällt.');
	});

	it('keeps the purchase warning out of the roll phase', async () => {
		render(ForfeitButton, {
			phase: 'playing',
			turnPhase: 'awaiting_roll',
			isMyTurn: true,
			me: player(),
			onIntent: () => {}
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Aufgeben' }));

		expect(screen.getByRole('dialog').textContent).not.toContain('Kaufangebot');
	});

	it('hides the trigger in a reserved slot when hidden', () => {
		const { container } = render(ForfeitButton, {
			phase: 'playing',
			turnPhase: 'awaiting_roll',
			isMyTurn: true,
			me: player(),
			hidden: true,
			onIntent: () => {}
		});

		// Deliberately an otherwise eligible config: this proves `hidden` alone
		// disables the trigger. aria-hidden also removes it from role queries,
		// so query the DOM directly.
		const trigger = container.querySelector('button');
		expect(trigger).not.toBeNull();
		expect(trigger).toHaveClass('slot-hidden');
		expect(trigger).toHaveAttribute('aria-hidden', 'true');
		expect(trigger).toHaveAttribute('tabindex', '-1');
		expect(trigger).toBeDisabled();
		expect(screen.queryByRole('button', { name: 'Aufgeben' })).toBeNull();
	});

	it('keeps an eligible trigger visible and enabled by default', () => {
		render(ForfeitButton, {
			phase: 'playing',
			turnPhase: 'awaiting_roll',
			isMyTurn: true,
			me: player(),
			onIntent: () => {}
		});

		const trigger = screen.getByRole('button', { name: 'Aufgeben' });
		expect(trigger).toBeEnabled();
		expect(trigger).not.toHaveClass('slot-hidden');
		expect(trigger).not.toHaveAttribute('aria-hidden');
		expect(trigger).not.toHaveAttribute('tabindex');
	});
});
