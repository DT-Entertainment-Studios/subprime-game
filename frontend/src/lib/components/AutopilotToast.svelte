<script lang="ts">
	// Notifies the owner when the autopilot stopped on its own (Spec.md §4.13,
	// clarify D10). One toast at a time; several stops queue and are dismissed
	// one after another.
	import type { GameEvent, GameState, Player } from '#lib/types';
	import { autopilotStopLabel } from '#lib/format';

	let { game, me }: { game: GameState; me: Player } = $props();

	/** Events already present on the first snapshot never pop up (page reload). */
	let baselineDone = $state(false);
	let seen = $state<Set<number>>(new Set());
	let dismissed = $state<Set<number>>(new Set());
	let stops = $state<GameEvent[]>([]);

	const open = $derived(stops.filter((event) => !dismissed.has(event.seq)));
	const current = $derived(open[0]);

	$effect(() => {
		const events = game.events;
		if (!baselineDone) {
			seen = new Set(events.map((event) => event.seq));
			baselineDone = true;
			return;
		}
		const fresh = events.filter(
			(event) =>
				!seen.has(event.seq) &&
				event.type === 'autopilot_off' &&
				event.player === me.seat &&
				event.reason !== 'manual'
		);
		if (fresh.length === 0) return;
		seen = new Set([...seen, ...fresh.map((event) => event.seq)]);
		stops = [...stops, ...fresh];
	});

	function dismiss(seq: number): void {
		dismissed = new Set(dismissed).add(seq);
	}
</script>

{#if current}
	<div class="toast" role="status" aria-live="polite">
		<div class="body">
			<strong class="heading">Autopilot beendet</strong>
			<p class="reason">{autopilotStopLabel(current.reason)}</p>
		</div>
		<button class="plain close" type="button" onclick={() => dismiss(current.seq)}>Schließen</button>
	</div>
{/if}

<style>
	.toast {
		position: fixed;
		bottom: 1rem;
		right: 1rem;
		z-index: 45; /* below the trade popups (50) and dialogs */
		display: flex;
		align-items: flex-start;
		gap: 0.6rem;
		max-width: min(22rem, calc(100vw - 2rem));
		padding: 0.65rem 0.75rem;
		background: var(--md-sys-color-surface-container-high);
		color: var(--md-sys-color-on-surface);
		border: 1px solid var(--md-sys-color-outline-variant);
		border-radius: var(--md-sys-shape-corner-medium);
		box-shadow: 0 var(--md-sys-elevation-2) var(--md-sys-elevation-5)
			color-mix(in srgb, var(--md-sys-color-shadow) 22%, transparent);
	}

	.body {
		flex: 1;
		min-width: 0;
	}

	.heading {
		display: block;
		font: var(--md-sys-typescale-title-small);
	}

	.reason {
		margin: 0.1rem 0 0;
		font: var(--md-sys-typescale-body-small);
		color: var(--md-sys-color-on-surface-variant);
	}

	.close {
		flex: 0 0 auto;
		font-size: 12px; /* label-small, tokens.css:179 */
		line-height: 16px;
		padding: 0.2rem 0.55rem;
	}
</style>
