// Reactive user preferences (local storage only, Spec.md §2).

import { applyTheme, loadPreferences, savePreferences, type Preferences } from './preferences';

class PreferencesStore {
	current = $state<Preferences>(loadPreferences());

	update(patch: Partial<Preferences>): void {
		this.current = { ...this.current, ...patch };
		savePreferences(this.current);
		applyTheme(this.current.theme);
	}

	toggleTheme(): void {
		this.update({ theme: this.current.theme === 'dark' ? 'light' : 'dark' });
	}

	apply(): void {
		applyTheme(this.current.theme);
	}
}

export const preferences = new PreferencesStore();
