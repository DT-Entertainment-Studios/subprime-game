import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig, loadEnv, type Plugin } from 'vite';

const DEV_PORT = 8081;
const APP_NAME = 'subprime';
const HEALTH_PATH = '/__dev/health';

// Dev-only identity endpoint: a client that is unsure which app answers can
// ask /__dev/health. Listed before sveltekit() so it wins over the SPA
// middleware.
function devIdentity(): Plugin {
	return {
		name: 'subprime:dev-identity',
		configureServer(server) {
			server.middlewares.use((req, res, next) => {
				// @types/node is not a dependency, so the connect request is untyped here.
				const url = (req as { url?: string }).url ?? '/';
				// A malformed request target (e.g. `//[`) makes the parser throw; such a
				// request is not the health endpoint, so it belongs to the next handler.
				let pathname: string;
				try {
					pathname = new URL(url, 'http://localhost').pathname;
				} catch {
					next();
					return;
				}
				if (pathname !== HEALTH_PATH) {
					next();
					return;
				}
				// Report the real listening port, so an explicit --port stays truthful.
				const address = server.httpServer?.address();
				const port = typeof address === 'object' && address ? address.port : DEV_PORT;
				res.statusCode = 200;
				res.setHeader('content-type', 'application/json; charset=utf-8');
				res.setHeader('cache-control', 'no-store');
				res.end(JSON.stringify({ app: APP_NAME, port }));
			});
		}
	};
}

export default defineConfig(({ mode }) => {
	// VITE_API_PROXY lets the hot-reload stack (make dev) point the /api proxy
	// at the caddy service instead of localhost, see the proxy below.
	const env = loadEnv(mode, '.', 'VITE_');

	return {
		plugins: [
			devIdentity(),
			sveltekit({
				compilerOptions: {
					// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
					runes: ({ filename }) =>
						filename.split(/[/\\]/).includes('node_modules') ? undefined : true
				},

				// Static SPA: the Go backend serves the built assets and falls back to
				// index.html for all unknown routes (Spec.md §6), so the client-side
				// router owns every path.
				adapter: adapter({ fallback: 'index.html' })
			})
		],

		server: {
			// Listen on all interfaces: the hot-reload stack (make dev) serves the
			// SPA through the caddy container, which dials the container's network
			// address — a ::1/127.0.0.1 only listener would answer 502 on every
			// SPA request. localhost access stays the same for `npm run dev`.
			host: true,
			// Fail loudly instead of silently moving to the next free port.
			strictPort: true,
			port: DEV_PORT,
			// Poll for file changes: bind mounts between the editor and the dev
			// containers deliver no inotify events, so watching must not rely on
			// them or HMR never triggers.
			watch: {
				usePolling: true,
				interval: 1200
			},
			proxy: {
				// Development entry point: the Caddy mock in docker-compose-dev.yaml
				// listens on :7411, injects the identity headers, and forwards to the
				// Go backend. `ws` proxies the realtime channel at /api/ws.
				//
				// The target is configurable because the hot-reload stack (make dev)
				// also publishes this Vite server on :7410, and a page opened there
				// calls /api on the Vite origin. Inside the frontend container
				// localhost:7411 is empty, so docker-compose-dev-hot.yaml points
				// VITE_API_PROXY at the caddy service — and the site port picked
				// there decides the identity: :7412 is Anna, so :7410 is a second
				// player next to Entwickler on :7411.
				'/api': {
					target: env.VITE_API_PROXY || 'http://localhost:7411',
					ws: true
				}
			}
		},

		// Preview is not part of the dev port block: keep Vite's default port and
		// its old "move to the next free port" behaviour.
		preview: {
			strictPort: false
		}
	};
});
