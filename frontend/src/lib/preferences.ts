// User preferences live in local storage only and never reach the server
// (Spec.md §2).

export interface Preferences {
	theme: 'light' | 'dark';
	zoom: number;
	animations: boolean;
	color: string;
}

const STORAGE_KEY = 'subprime.preferences';

const DEFAULTS: Preferences = {
	theme: 'light',
	zoom: 1,
	animations: true,
	color: 'rot'
};

export function loadPreferences(): Preferences {
	if (typeof localStorage === 'undefined') return { ...DEFAULTS };
	try {
		const raw = localStorage.getItem(STORAGE_KEY);
		if (!raw) return { ...DEFAULTS };
		const parsed = JSON.parse(raw) as Partial<Preferences>;
		return {
			theme: parsed.theme === 'dark' ? 'dark' : 'light',
			zoom: typeof parsed.zoom === 'number' && parsed.zoom > 0.5 && parsed.zoom < 2 ? parsed.zoom : 1,
			animations: parsed.animations !== false,
			color: typeof parsed.color === 'string' ? parsed.color : DEFAULTS.color
		};
	} catch {
		return { ...DEFAULTS };
	}
}

export function savePreferences(prefs: Preferences): void {
	if (typeof localStorage === 'undefined') return;
	try {
		localStorage.setItem(STORAGE_KEY, JSON.stringify(prefs));
	} catch {
		// Ignore storage failures (private mode, quota).
	}
}

export function applyTheme(theme: 'light' | 'dark'): void {
	if (typeof document === 'undefined') return;
	document.documentElement.dataset.theme = theme;
	document.documentElement.style.colorScheme = theme;
}
