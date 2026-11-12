import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vitest/config';

// Separate config so the unit tests do not pull in the production adapter.
// The sveltekit() plugin provides #lib aliasing and Svelte compilation.
export default defineConfig({
	// Options object: production config (adapter etc.) lives in vite.config.ts
	// and is irrelevant for tests, but passing options silences the
	// "No Svelte config file found" warning from the default config lookup.
	// The runes forcing from vite.config.ts applies here too — without it Svelte
	// compiles components in legacy mode and a prop named `state` collides with
	// the legacy `$state` store subscription.
	plugins: [
		sveltekit({
			compilerOptions: {
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			}
		})
	],
	// Compile Svelte components for the client (not SSR) so they can be mounted
	// in component tests — see https://svelte.dev/docs/kit/testing.
	resolve: process.env.VITEST ? { conditions: ['browser'] } : undefined,
	test: {
		environment: 'jsdom',
		include: ['src/**/*.test.ts'],
		setupFiles: ['src/lib/testing/setup.ts']
	}
});
