<script lang="ts">
	// Shows every newly drawn Ereignis- or Gemeinschaftskarte as a notice docked
	// to the bottom-left corner, so all players see what happened without
	// opening the event log (Spec.md §4.9). No modal, no focus theft.
	import type { GameEvent, GameState, Player } from '#lib/types';
	import CardToast from './CardToast.svelte';

	let { game, me = null }: { game: GameState; me?: Player | null } = $props();

	interface CardDraw {
		event: GameEvent;
		/** Events that describe the card's outcome (payment, move, jail, ...). */
		results: GameEvent[];
	}

	/** Events already present on the first snapshot never pop up (page reload). */
	let baselineDone = $state(false);
	let seen = $state<Set<number>>(new Set());
	let dismissed = $state<Set<number>>(new Set());
	let draws = $state<CardDraw[]>([]);

	const suppress = $derived(
		me?.autopilot?.active === true && me?.autopilot?.config?.hideCards === true
	);
	const open = $derived(
		suppress ? [] : draws.filter((draw) => !dismissed.has(draw.event.seq))
	);

	// A card draw is followed by its effect events until the turn moves on or the
	// next card in a chain is drawn. Those lines belong to the same notice.
	function resultsOf(events: GameEvent[], card: GameEvent): GameEvent[] {
		const index = events.findIndex((event) => event.seq === card.seq);
		if (index < 0) return [];
		const results: GameEvent[] = [];
		for (let i = index + 1; i < events.length; i++) {
			const event = events[i];
			if (event.type === 'card' || event.type === 'turn') break;
			if (event.type === 'move' || event.type === 'move_back' || event.type === 'roll') continue;
			results.push(event);
		}
		return results;
	}

	$effect(() => {
		const events = game.events;
		if (!baselineDone) {
			seen = new Set(events.map((event) => event.seq));
			baselineDone = true;
			return;
		}
		if (suppress && draws.length > 0) draws = [];
		const fresh = events.filter((event) => event.type === 'card' && !seen.has(event.seq));
		if (fresh.length === 0) return;
		seen = new Set([...seen, ...fresh.map((event) => event.seq)]);
		if (suppress) return;
		draws = [...draws, ...fresh.map((event) => ({ event, results: resultsOf(events, event) }))];
	});

	function dismiss(seq: number) {
		dismissed = new Set(dismissed).add(seq);
	}
</script>

{#if open.length > 0}
	<div class="notice-stack">
		{#each open as draw (draw.event.seq)}
			<CardToast {draw} {me} drawer={game.players[draw.event.player]} onDismiss={dismiss} />
		{/each}
	</div>
{/if}

<style>
	.notice-stack {
		position: fixed;
		bottom: 1rem;
		left: 1rem;
		z-index: 55;
		display: flex;
		flex-direction: column;
		gap: 0.6rem;
		width: min(26rem, calc(100vw - 2rem));
		max-height: calc(100vh - 2rem);
		overflow-y: auto;
		/* Empty gaps between notices must not block clicks on the board. */
		pointer-events: none;
	}

	.notice-stack > :global(*) {
		pointer-events: auto;
	}
</style>
