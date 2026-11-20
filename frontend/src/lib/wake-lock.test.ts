import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createScreenWakeLock } from './wake-lock';

type ReleaseListener = () => void;

const originalWakeLock = Object.getOwnPropertyDescriptor(navigator, 'wakeLock');
const originalVisibilityState = Object.getOwnPropertyDescriptor(document, 'visibilityState');

function setVisibility(state: 'visible' | 'hidden'): void {
	Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => state });
}

function createMockSentinel() {
	const listeners = new Set<ReleaseListener>();
	const release = vi.fn(async () => {});
	const addEventListener = vi.fn((type: string, listener: ReleaseListener) => {
		if (type === 'release') listeners.add(listener);
	});
	const removeEventListener = vi.fn((type: string, listener: ReleaseListener) => {
		if (type === 'release') listeners.delete(listener);
	});
	return {
		release,
		addEventListener,
		removeEventListener,
		emitRelease: () => listeners.forEach((listener) => listener())
	};
}

type MockSentinel = ReturnType<typeof createMockSentinel>;

let request: ReturnType<typeof vi.fn>;
let sentinels: MockSentinel[];

async function flush(): Promise<void> {
	await Promise.resolve();
	await Promise.resolve();
}

beforeEach(() => {
	sentinels = [];
	request = vi.fn(() => {
		const sentinel = createMockSentinel();
		sentinels.push(sentinel);
		return Promise.resolve(sentinel);
	});
	Object.defineProperty(navigator, 'wakeLock', {
		configurable: true,
		value: { request }
	});
	setVisibility('visible');
});

afterEach(() => {
	if (originalWakeLock) Object.defineProperty(navigator, 'wakeLock', originalWakeLock);
	else Reflect.deleteProperty(navigator, 'wakeLock');
	if (originalVisibilityState) {
		Object.defineProperty(document, 'visibilityState', originalVisibilityState);
	} else {
		Reflect.deleteProperty(document, 'visibilityState');
	}
	vi.clearAllMocks();
});

describe('createScreenWakeLock', () => {
	it('does not request a lock while inactive', () => {
		const controller = createScreenWakeLock();
		controller.setActive(false);
		expect(request).not.toHaveBeenCalled();
		controller.dispose();
	});

	it('requests one screen lock when activated', async () => {
		const controller = createScreenWakeLock();
		controller.setActive(true);
		await flush();
		expect(request).toHaveBeenCalledTimes(1);
		expect(request).toHaveBeenCalledWith('screen');
		expect(sentinels).toHaveLength(1);
		controller.dispose();
	});

	it('is idempotent for repeated activation', () => {
		const controller = createScreenWakeLock();
		controller.setActive(true);
		controller.setActive(true);
		expect(request).toHaveBeenCalledTimes(1);
		controller.dispose();
	});

	it('releases the sentinel when deactivated', async () => {
		const controller = createScreenWakeLock();
		controller.setActive(true);
		await flush();
		controller.setActive(false);
		expect(sentinels[0].release).toHaveBeenCalledTimes(1);
		controller.dispose();
	});

	it('re-requests when the tab becomes visible again', async () => {
		const controller = createScreenWakeLock();
		controller.setActive(true);
		await flush();

		setVisibility('hidden');
		document.dispatchEvent(new Event('visibilitychange'));
		expect(request).toHaveBeenCalledTimes(1);

		setVisibility('visible');
		document.dispatchEvent(new Event('visibilitychange'));
		expect(request).toHaveBeenCalledTimes(2);
		controller.dispose();
	});

	it('allows a later request after the browser releases the sentinel itself', async () => {
		const controller = createScreenWakeLock();
		controller.setActive(true);
		await flush();

		sentinels[0].emitRelease();
		expect(sentinels[0].removeEventListener).toHaveBeenCalled();

		setVisibility('visible');
		document.dispatchEvent(new Event('visibilitychange'));
		expect(request).toHaveBeenCalledTimes(2);
		controller.dispose();
	});

	it('releases on dispose and ignores later activation', async () => {
		const controller = createScreenWakeLock();
		controller.setActive(true);
		await flush();
		controller.dispose();
		expect(sentinels[0].release).toHaveBeenCalledTimes(1);

		controller.setActive(true);
		expect(request).toHaveBeenCalledTimes(1);
	});

	it('is a silent no-op when the API is missing', () => {
		Reflect.deleteProperty(navigator, 'wakeLock');
		const controller = createScreenWakeLock();
		expect(() => {
			controller.setActive(true);
			controller.setActive(false);
			controller.dispose();
		}).not.toThrow();
	});

	it('swallows a rejected request', async () => {
		request.mockRejectedValueOnce(new Error('denied'));
		const controller = createScreenWakeLock();
		expect(() => controller.setActive(true)).not.toThrow();
		await flush();
		expect(request).toHaveBeenCalledTimes(1);
		controller.dispose();
	});
});
