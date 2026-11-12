<script lang="ts">
	// Shows every newly drawn Ereignis- or Gemeinschaftskarte as a card popup, so
	// all players see what happened without opening the event log (Spec.md §4.9).
	import type { GameEvent, GameState, Player } from '#lib/types';
	import { tokenColor } from '#lib/format';
	import IconGift from '@tabler/icons-svelte/icons/gift';
	import IconHelpCircle from '@tabler/icons-svelte/icons/help-circle';
	import IconX from '@tabler/icons-svelte/icons/x';

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
	let panelEl = $state<HTMLElement>();

	const open = $derived(draws.filter((draw) => !dismissed.has(draw.event.seq)));
	const current = $derived(open[0]);

	// A card draw is followed by its effect events until the turn moves on or the
	// next card in a chain is drawn. Those lines belong to the same popup.
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
		const fresh = events.filter((event) => event.type === 'card' && !seen.has(event.seq));
		if (fresh.length === 0) return;
		seen = new Set([...seen, ...fresh.map((event) => event.seq)]);
		draws = [...draws, ...fresh.map((event) => ({ event, results: resultsOf(events, event) }))];
	});

	// Focus the card and lock the page behind it, like the field detail card.
	$effect(() => {
		if (!current) return;
		panelEl?.focus();
		document.body.style.overflow = 'hidden';
		return () => {
			document.body.style.overflow = '';
		};
	});

	const deck = $derived.by(() => {
		const isChest = current?.event.deck === 'gemeinschaftsfeld';
		return {
			label: isChest ? 'Gemeinschaftskarte' : 'Ereigniskarte',
			icon: isChest ? IconGift : IconHelpCircle,
			accent: isChest ? '#3f51b5' : '#c2410c'
		};
	});

	const drawer = $derived(current ? game.players[current.event.player] : undefined);

	function dismiss(seq: number) {
		dismissed = new Set(dismissed).add(seq);
	}

	function closeCurrent() {
		if (current) dismiss(current.event.seq);
	}

	function backdrop(event: MouseEvent) {
		if (event.target === event.currentTarget) closeCurrent();
	}
</script>

<svelte:window onkeydown={(event) => event.key === 'Escape' && closeCurrent()} />

{#if current}
	{@const DeckIcon = deck.icon}
	<!-- The backdrop only closes on click; keyboard users close with Escape or
	     the close button, so the pointer-only interaction is intentional here. -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="backdrop" onclick={backdrop}>
		<div
			class="panel"
			bind:this={panelEl}
			role="dialog"
			aria-modal="true"
			aria-label={`${deck.label} von ${drawer?.name ?? 'einem Spieler'}`}
			tabindex="-1"
		>
			<header>
				<span class="icon-badge" style="background: {deck.accent}">
					<DeckIcon stroke={2} />
				</span>
				<div class="names">
					<h2>{deck.label}</h2>
					<span class="drawer">
						{#if drawer}
							<span class="dot" style="background: {tokenColor(drawer.color)}"></span>
							{drawer.name} zieht
						{:else}
							Es wird gezogen
						{/if}
					</span>
				</div>
				{#if me && current.event.player === me.seat}<span class="pill">für dich</span>{/if}
				<button class="plain close" aria-label="Karte schließen" onclick={closeCurrent}>
					<IconX stroke={2} />
				</button>
			</header>

			<p class="card-text">{current.event.text.replace(/^.*? zieht: /, '')}</p>

			{#if current.results.length > 0}
				<h3>Auswirkung</h3>
				<ul class="results">
					{#each current.results as event (event.seq)}
						<li>{event.text}</li>
					{/each}
				</ul>
			{/if}

			<div class="actions">
				{#if open.length > 1}
					<span class="muted">Noch {open.length - 1} weitere Karte{open.length > 2 ? 'n' : ''}</span>
				{/if}
				<button class="cta" onclick={closeCurrent}>Weiter</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.backdrop {
		position: fixed;
		inset: 0;
		z-index: 65;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 1rem;
		background: color-mix(in srgb, var(--md-sys-color-scrim) 52%, transparent);
	}

	.panel {
		width: min(26rem, 94vw);
		max-height: min(46rem, 92vh);
		overflow-y: auto;
		outline: none;
		padding: 0.9rem 1rem 1rem;
		background: var(--md-sys-color-surface-container);
		border: 1px solid var(--md-sys-color-outline-variant);
		border-radius: var(--md-sys-shape-corner-large);
		box-shadow: 0 var(--md-sys-elevation-4) calc(var(--md-sys-elevation-5) * 3)
			color-mix(in srgb, var(--md-sys-color-shadow) 32%, transparent);
	}

	header {
		display: flex;
		align-items: center;
		gap: 0.55rem;
		margin-bottom: 0.6rem;
	}

	.icon-badge {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 1.7rem;
		height: 1.7rem;
		border-radius: 50%;
		color: var(--cd-ral9016);
		flex: 0 0 auto;
	}

	.icon-badge :global(svg) {
		width: 1rem;
		height: 1rem;
	}

	.names {
		flex: 1;
		min-width: 0;
	}

	h2 {
		margin: 0;
		font-size: 22px; /* title-large, tokens.css:168 */
		line-height: 28px;
		font-weight: 400;
	}

	.drawer {
		display: inline-flex;
		align-items: center;
		gap: 0.35rem;
		font-size: 12px; /* label-medium, tokens.css:175 */
		line-height: 16px;
		font-weight: 500;
		color: var(--md-sys-color-on-surface-variant);
	}

	.dot {
		width: 0.7rem;
		height: 0.7rem;
		border-radius: 50%;
		border: 1px solid var(--md-sys-color-outline-variant);
		flex: 0 0 auto;
	}

	.pill {
		background: var(--md-sys-color-primary-container);
		color: var(--md-sys-color-on-primary-container);
		border-radius: var(--md-sys-shape-corner-full);
		font-size: 0.68rem;
		font-weight: 700;
		padding: 0.1rem 0.45rem;
		white-space: nowrap;
	}

	.close {
		width: 1.9rem;
		height: 1.9rem;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 0;
	}

	.close :global(svg) {
		width: 0.95rem;
		height: 0.95rem;
	}

	.card-text {
		margin: 0;
		padding: 0.8rem 0.9rem;
		border: 1px solid var(--md-sys-color-outline-variant);
		border-radius: var(--md-sys-shape-corner-medium);
		background: var(--md-sys-color-surface);
		font-size: 16px; /* body-large, tokens.css:171 */
		line-height: 24px;
	}

	h3 {
		margin: 0.7rem 0 0.35rem;
		font-size: 12px; /* label-medium, tokens.css:175 */
		line-height: 16px;
		font-weight: 500;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--md-sys-color-on-surface-variant);
	}

	.results {
		margin: 0;
		padding-left: 1.1rem;
		display: flex;
		flex-direction: column;
		gap: 0.15rem;
		font-size: 12px; /* body-small, tokens.css:173 */
		line-height: 16px;
	}

	.actions {
		display: flex;
		align-items: center;
		justify-content: flex-end;
		gap: 0.6rem;
		margin-top: 0.9rem;
	}

	.actions .muted {
		font-size: 0.78rem;
	}
</style>
