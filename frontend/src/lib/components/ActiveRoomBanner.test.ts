import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/svelte';
import ActiveRoomBanner from './ActiveRoomBanner.svelte';
import type { ActiveRoom, RoomPhase } from '#lib/types';

const ROOM: ActiveRoom = { code: 'ABCDEF', phase: 'playing' };

const handlers = { onRejoin: () => {}, onAbandon: () => {} };

describe('ActiveRoomBanner', () => {
	it.each([
		['lobby', 'Lobby', 'Dein Sitz in der Lobby ist noch belegt'],
		['playing', 'Spiel läuft', 'Du bist noch in einem laufenden Spiel'],
		['finished', 'Spiel beendet', 'Das Spiel ist beendet, aber dein Sitz']
	] as [RoomPhase, string, string][])(
		'names the code, phase and hint for %s',
		(phase, label, hint) => {
			render(ActiveRoomBanner, { room: { code: 'ABCDEF', phase }, ...handlers });

			const banner = screen.getByRole('region', { name: 'Du bist noch in Raum ABCDEF' });
			expect(banner).toHaveTextContent(hint);
			expect(screen.getByText(label)).toBeInTheDocument();
		}
	);

	it('rejoins without a confirmation', async () => {
		const onRejoin = vi.fn();
		render(ActiveRoomBanner, { room: ROOM, ...handlers, onRejoin });

		await fireEvent.click(screen.getByRole('button', { name: 'Wieder beitreten' }));

		expect(onRejoin).toHaveBeenCalledTimes(1);
	});

	it('asks for confirmation before abandoning', async () => {
		const onAbandon = vi.fn();
		render(ActiveRoomBanner, { room: ROOM, ...handlers, onAbandon });

		await fireEvent.click(screen.getByRole('button', { name: 'Raum verlassen' }));

		expect(screen.getByRole('dialog', { name: 'Raum verlassen bestätigen' })).toBeInTheDocument();
		expect(onAbandon).not.toHaveBeenCalled();
	});

	it.each([
		['lobby', 'letzte Sitz'],
		['playing', 'Bist du noch im Spiel, scheidest du aus'],
		['finished', 'beendeten Spiel']
	] as [RoomPhase, string][])('explains the %s consequence in the dialog', async (phase, copy) => {
		render(ActiveRoomBanner, { room: { code: 'ABCDEF', phase }, ...handlers });

		await fireEvent.click(screen.getByRole('button', { name: 'Raum verlassen' }));

		expect(screen.getByRole('dialog').textContent).toContain(copy);
	});

	it('warns that the assets go to the bank when abandoning a running game', async () => {
		render(ActiveRoomBanner, { room: ROOM, ...handlers });

		await fireEvent.click(screen.getByRole('button', { name: 'Raum verlassen' }));

		const dialog = screen.getByRole('dialog');
		expect(dialog.textContent).toMatch(/Gebäude und Felder gehen an die Bank/);
		expect(dialog.textContent).toContain(
			'Bist du bereits ausgeschieden, wird nur dein Sitz entfernt.'
		);
		expect(dialog.textContent).toContain(
			'Ein Wiedereinstieg ins Spiel ist danach nicht mehr möglich.'
		);
	});

	it('describes the dialog with the consequence paragraph', async () => {
		render(ActiveRoomBanner, { room: ROOM, ...handlers });

		await fireEvent.click(screen.getByRole('button', { name: 'Raum verlassen' }));

		const dialog = screen.getByRole('dialog');
		expect(dialog).toHaveAttribute('aria-modal', 'true');
		const describedBy = dialog.getAttribute('aria-describedby');
		expect(describedBy).toBeTruthy();
		expect(document.getElementById(describedBy as string)).toHaveTextContent(
			'Bist du noch im Spiel, scheidest du aus'
		);
	});

	it('abandons exactly once and closes after the confirmation', async () => {
		const onAbandon = vi.fn();
		render(ActiveRoomBanner, { room: ROOM, ...handlers, onAbandon });

		await fireEvent.click(screen.getByRole('button', { name: 'Raum verlassen' }));
		await fireEvent.click(screen.getByRole('button', { name: 'Ja, Raum verlassen' }));

		expect(onAbandon).toHaveBeenCalledTimes(1);
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
	});

	it('closes without abandoning when cancelled', async () => {
		const onAbandon = vi.fn();
		render(ActiveRoomBanner, { room: ROOM, ...handlers, onAbandon });

		await fireEvent.click(screen.getByRole('button', { name: 'Raum verlassen' }));
		await fireEvent.click(screen.getByRole('button', { name: 'Abbrechen' }));

		expect(onAbandon).not.toHaveBeenCalled();
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
	});

	it('closes without abandoning on Escape', async () => {
		const onAbandon = vi.fn();
		render(ActiveRoomBanner, { room: ROOM, ...handlers, onAbandon });

		await fireEvent.click(screen.getByRole('button', { name: 'Raum verlassen' }));
		await fireEvent.keyDown(window, { key: 'Escape' });

		expect(onAbandon).not.toHaveBeenCalled();
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
	});

	it('closes without abandoning on a backdrop click', async () => {
		const onAbandon = vi.fn();
		const { container } = render(ActiveRoomBanner, { room: ROOM, ...handlers, onAbandon });

		await fireEvent.click(screen.getByRole('button', { name: 'Raum verlassen' }));
		const backdrop = container.querySelector<HTMLElement>('.backdrop');
		expect(backdrop).not.toBeNull();
		await fireEvent.click(backdrop as HTMLElement);

		expect(onAbandon).not.toHaveBeenCalled();
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
	});

	it('disables both actions and marks the pending abandon while leaving', () => {
		render(ActiveRoomBanner, { room: ROOM, leaving: true, ...handlers });

		expect(screen.getByRole('button', { name: 'Wieder beitreten' })).toBeDisabled();
		const abandonTrigger = screen.getByRole('button', { name: 'Verlassen …' });
		expect(abandonTrigger).toBeDisabled();
	});

	it('moves focus into the dialog and back to the trigger on close', async () => {
		render(ActiveRoomBanner, { room: ROOM, ...handlers });

		const trigger = screen.getByRole('button', { name: 'Raum verlassen' });
		// jsdom does not move focus on click the way a browser does.
		trigger.focus();
		await fireEvent.click(trigger);
		expect(screen.getByRole('dialog')).toHaveFocus();

		await fireEvent.click(screen.getByRole('button', { name: 'Abbrechen' }));
		expect(trigger).toHaveFocus();
	});
});
