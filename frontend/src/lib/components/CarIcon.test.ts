import { describe, expect, it } from 'vitest';
import { render } from '@testing-library/svelte';
import CarIcon from './CarIcon.svelte';

function headlights(container: HTMLElement): SVGRectElement[] {
	return Array.from(container.querySelectorAll('rect')).filter((rect) => {
		const fill = rect.getAttribute('fill') ?? '';
		return fill === 'var(--car-lamp-on)' || fill === 'var(--car-lamp-off)';
	});
}

describe('CarIcon lamps', () => {
	it('renders dark headlights by default', () => {
		const { container } = render(CarIcon);

		const svg = container.querySelector('svg');
		expect(svg).not.toBeNull();
		expect(svg?.dataset.lit).toBe('false');

		const lamps = headlights(container);
		expect(lamps).toHaveLength(2);
		for (const lamp of lamps) {
			expect(lamp.getAttribute('fill')).toBe('var(--car-lamp-off)');
			expect(lamp.hasAttribute('filter')).toBe(false);
		}

		expect(container.querySelector('[data-beam]')).toBeNull();
		expect(container.querySelector('defs filter')).toBeNull();
	});

	it('lights up the active car', () => {
		const { container } = render(CarIcon, { active: true });

		const svg = container.querySelector('svg');
		expect(svg?.dataset.lit).toBe('true');
		expect(container.querySelectorAll('[data-beam]')).toHaveLength(1);
		expect(container.querySelectorAll('[data-pool]')).toHaveLength(0);

		const filter = container.querySelector('defs filter');
		expect(filter).not.toBeNull();
		const filterId = filter?.getAttribute('id');

		const lamps = headlights(container);
		expect(lamps).toHaveLength(2);
		for (const lamp of lamps) {
			expect(lamp.getAttribute('fill')).toBe('var(--car-lamp-on)');
			expect(lamp.getAttribute('filter')).toBe(`url(#${filterId})`);
		}
	});

	it('uses a unique filter id per instance', () => {
		const first = render(CarIcon, { active: true });
		const second = render(CarIcon, { active: true });

		const firstId = first.container.querySelector('defs filter')?.getAttribute('id');
		const secondId = second.container.querySelector('defs filter')?.getAttribute('id');

		expect(firstId).toBeTruthy();
		expect(secondId).toBeTruthy();
		expect(firstId).not.toBe(secondId);
	});

	it('keeps the initial letter', () => {
		const { container } = render(CarIcon, { initial: 'A' });

		expect(container.querySelector('text')?.textContent).toBe('A');
	});

	it('removes the light state when active turns off and reuses its id', async () => {
		const { container, rerender } = render(CarIcon, { active: true });
		const filterId = container.querySelector('defs filter')?.getAttribute('id');
		expect(filterId).toBeTruthy();

		await rerender({ active: false });

		expect(container.querySelector('svg')?.dataset.lit).toBe('false');
		expect(container.querySelector('defs')).toBeNull();
		expect(container.querySelector('[data-beam]')).toBeNull();
		for (const lamp of headlights(container)) {
			expect(lamp.hasAttribute('filter')).toBe(false);
		}

		await rerender({ active: true });

		expect(container.querySelector('defs filter')?.getAttribute('id')).toBe(filterId);
	});
});
