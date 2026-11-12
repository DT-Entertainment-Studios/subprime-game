import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/svelte';
import { afterEach } from 'vitest';

afterEach(() => {
	cleanup();
});

// jsdom knows no ResizeObserver; RoadCars uses one to measure the road ring.
if (typeof globalThis.ResizeObserver === 'undefined') {
	class ResizeObserverStub {
		observe(): void {}
		unobserve(): void {}
		disconnect(): void {}
	}
	// Assigned through unknown: in jsdom the real ResizeObserver never existed.
	globalThis.ResizeObserver = ResizeObserverStub as unknown as typeof ResizeObserver;
}
