import { describe, expect, it } from 'vitest';
import { fireEvent, render } from '@testing-library/svelte';
import EventLog from './EventLog.svelte';
import type { GameEvent } from '#lib/types';

function event(seq: number, text = `Ereignis ${seq}`): GameEvent {
	return { seq, type: 'move', player: 0, text, at: '2026-01-01T12:00:00Z' };
}

/** jsdom has no layout, so the scroll height is provided by the test. */
function scrollHeight(element: HTMLElement, value: () => number) {
	Object.defineProperty(element, 'scrollHeight', { configurable: true, get: value });
}

function logOf(container: HTMLElement): HTMLElement {
	return container.querySelector('.log') as HTMLElement;
}

describe('EventLog', () => {
	it('renders the newest event first', () => {
		const { container } = render(EventLog, { props: { events: [event(1), event(2), event(3)] } });
		const texts = [...container.querySelectorAll('.text')].map((node) => node.textContent);
		expect(texts).toEqual(['Ereignis 3', 'Ereignis 2', 'Ereignis 1']);
	});

	it('shows dice totals with a sigma and without GE, money events keep GE', () => {
		const roll: GameEvent = { ...event(1), type: 'roll', amount: 7 };
		const money: GameEvent = { ...event(2), type: 'buy', amount: 1500 };
		const { container } = render(EventLog, { props: { events: [roll, money] } });
		const amounts = [...container.querySelectorAll('.amount')].map((node) => node.textContent ?? '');

		const rollAmount = amounts.find((value) => value.includes('Σ'));
		expect(rollAmount).toBe('Σ 7');
		expect(rollAmount).not.toContain('GE');
		expect(amounts.some((value) => value.includes('GE'))).toBe(true);
	});

	it('keeps a reader at the top on the newest entry', async () => {
		const events = [event(1), event(2)];
		const { container, rerender } = render(EventLog, { props: { events } });
		const log = logOf(container);
		scrollHeight(log, () => 400);

		// Two pixels of jitter still count as "at the top".
		log.scrollTop = 2;
		await fireEvent.scroll(log);

		await rerender({ events: [...events, event(3)] });

		expect(log.scrollTop).toBe(0);
	});

	it('does not move a reader who scrolled down', async () => {
		const events = [event(1), event(2)];
		const { container, rerender } = render(EventLog, { props: { events } });
		const log = logOf(container);
		// The content grows with the two inserted rows, like a real layout would.
		scrollHeight(log, () => (log.querySelectorAll('.entry').length > 2 ? 460 : 400));

		log.scrollTop = 120;
		await fireEvent.scroll(log);

		// Two new entries grow the content above the reader's position.
		await rerender({ events: [...events, event(3), event(4)] });

		expect(log.scrollTop).toBe(180);
	});

	it('highlights bankruptcy entries', () => {
		const bust = { ...event(2, 'Anton ist pleite.'), type: 'bankruptcy' };
		const { container } = render(EventLog, { props: { events: [event(1), bust] } });

		const bustEntry = [...container.querySelectorAll('.entry')].find((node) =>
			node.textContent?.includes('Anton ist pleite.')
		);
		expect(bustEntry).toHaveClass('bankrupt');
		// Only bankruptcy entries carry the marker, not normal rows.
		const normalEntry = [...container.querySelectorAll('.entry')].find((node) =>
			node.textContent?.includes('Ereignis 1')
		);
		expect(normalEntry).not.toHaveClass('bankrupt');
	});

	it('keeps scroll position with a bankruptcy entry', async () => {
		const events = [event(1), event(2)];
		const { container, rerender } = render(EventLog, { props: { events } });
		const log = logOf(container);
		scrollHeight(log, () => (log.querySelectorAll('.entry').length > 2 ? 460 : 400));

		log.scrollTop = 120;
		await fireEvent.scroll(log);

		const bust = { ...event(4, 'Anton ist pleite.'), type: 'bankruptcy' };
		await rerender({ events: [...events, event(3), bust] });

		expect(log.scrollTop).toBe(180);
	});
});
