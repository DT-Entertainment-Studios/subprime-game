import { beforeEach, describe, expect, it } from 'vitest';
import { loadPreferences, savePreferences } from './preferences';

describe('preferences', () => {
	beforeEach(() => {
		localStorage.clear();
	});

	it('returns defaults without stored values', () => {
		const prefs = loadPreferences();
		expect(prefs.theme).toBe('light');
		expect(prefs.zoom).toBe(1);
		expect(prefs.animations).toBe(true);
		expect(prefs.color).toBe('rot');
		expect(prefs.hideCards).toBe(false);
	});

	it('round-trips saved values', () => {
		savePreferences({ theme: 'dark', zoom: 1.2, animations: false, color: 'lila', hideCards: true });
		const prefs = loadPreferences();
		expect(prefs).toEqual({
			theme: 'dark',
			zoom: 1.2,
			animations: false,
			color: 'lila',
			hideCards: true
		});
	});

	it('falls back on invalid stored values', () => {
		localStorage.setItem(
			'subprime.preferences',
			JSON.stringify({ theme: 'neon', zoom: 99, animations: 'yes', color: 5, hideCards: 'yes' })
		);
		const prefs = loadPreferences();
		expect(prefs.theme).toBe('light');
		expect(prefs.zoom).toBe(1);
		expect(prefs.color).toBe('rot');
		expect(prefs.hideCards).toBe(false);
	});

	it('loads a stored hideCards = true value', () => {
		localStorage.setItem('subprime.preferences', JSON.stringify({ hideCards: true }));
		expect(loadPreferences().hideCards).toBe(true);
	});
});
