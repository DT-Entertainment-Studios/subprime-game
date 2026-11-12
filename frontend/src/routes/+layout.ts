// Static SPA: no SSR, no prerendering (Spec.md §6); the Go backend serves the
// built assets and the client-side router owns every path.
export const ssr = false;
export const prerender = false;
