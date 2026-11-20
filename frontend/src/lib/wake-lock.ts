// Best-effort screen wake lock. Keeps the display on for the whole game and
// silently gives up when the browser does not support it or refuses the lock.

// Minimal local shape so this module does not depend on the DOM's
// WakeLockSentinel type being present in the current TS lib.
interface WakeLockSentinelLike {
	release(): Promise<void>;
	addEventListener(type: 'release', listener: () => void): void;
	removeEventListener(type: 'release', listener: () => void): void;
}

interface WakeLockLike {
	request(type: 'screen'): Promise<WakeLockSentinelLike>;
}

export interface WakeLockController {
	setActive(active: boolean): void;
	dispose(): void;
}

/** No-op controller used on the server and in browsers without the API. */
function noopController(): WakeLockController {
	return { setActive() {}, dispose() {} };
}

/** Drops a sentinel without ever throwing or surfacing a rejection. */
function releaseSentinel(sentinel: WakeLockSentinelLike): void {
	try {
		void Promise.resolve(sentinel.release()).catch(() => {});
	} catch {
		// Swallow synchronous failures too; the lock is only an enhancement.
	}
}

export function createScreenWakeLock(): WakeLockController {
	// SSR / unsupported browser: every method stays a silent no-op.
	if (typeof navigator === 'undefined') return noopController();
	const wakeLock = (navigator as unknown as { wakeLock?: WakeLockLike }).wakeLock;
	if (!wakeLock || typeof wakeLock.request !== 'function') return noopController();

	let active = false;
	let disposed = false;
	let pending = false;
	let sentinel: WakeLockSentinelLike | null = null;
	let sentinelRelease: (() => void) | null = null;

	function forgetSentinel(): void {
		if (sentinel && sentinelRelease) sentinel.removeEventListener('release', sentinelRelease);
		sentinel = null;
		sentinelRelease = null;
	}

	function releaseCurrent(): void {
		const current = sentinel;
		forgetSentinel();
		if (current) releaseSentinel(current);
	}

	function attachSentinel(next: WakeLockSentinelLike): void {
		forgetSentinel();
		const onRelease = () => {
			// The browser released the lock (e.g. the tab went hidden). Clear the
			// cache so a later visibility change can acquire a fresh sentinel.
			if (sentinel === next) {
				sentinel = null;
				sentinelRelease = null;
			}
			next.removeEventListener('release', onRelease);
		};
		sentinel = next;
		sentinelRelease = onRelease;
		next.addEventListener('release', onRelease);
	}

	function request(): void {
		if (pending || disposed || !active || sentinel) return;
		pending = true;
		let attempt: Promise<WakeLockSentinelLike>;
		try {
			attempt = Promise.resolve(wakeLock!.request('screen'));
		} catch {
			pending = false;
			return;
		}
		void attempt
			.then((next) => {
				if (!active || disposed) releaseSentinel(next);
				else attachSentinel(next);
			})
			.catch(() => {
				// Insecure context, battery saver, user denial: stay silent.
			})
			.finally(() => {
				pending = false;
			});
	}

	function onVisibilityChange(): void {
		if (!active || disposed || document.visibilityState !== 'visible') return;
		// The browser drops the lock while hidden, so start from a clean state.
		releaseCurrent();
		request();
	}

	return {
		setActive(next: boolean): void {
			if (disposed || next === active) return;
			active = next;
			if (active) {
				request();
				document.addEventListener('visibilitychange', onVisibilityChange);
			} else {
				document.removeEventListener('visibilitychange', onVisibilityChange);
				releaseCurrent();
			}
		},
		dispose(): void {
			if (disposed) return;
			disposed = true;
			active = false;
			document.removeEventListener('visibilitychange', onVisibilityChange);
			releaseCurrent();
		}
	};
}
