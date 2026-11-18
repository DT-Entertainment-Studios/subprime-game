<script lang="ts">
	// One newly drawn Ereignis- or Gemeinschaftskarte as a self-dismissing
	// notice. The timer belongs to this component so every notice in the stack
	// runs independently and can be frozen on its own.
	import type { GameEvent, Player } from '#lib/types';
	import { tokenColor } from '#lib/format';
	import IconGift from '@tabler/icons-svelte/icons/gift';
	import IconHelpCircle from '@tabler/icons-svelte/icons/help-circle';
	import IconX from '@tabler/icons-svelte/icons/x';

	interface CardDraw {
		event: GameEvent;
		/** Events that describe the card's outcome (payment, move, jail, ...). */
		results: GameEvent[];
	}

	let {
		draw,
		me = null,
		drawer,
		onDismiss
	}: {
		draw: CardDraw;
		me?: Player | null;
		drawer?: Player;
		onDismiss: (seq: number) => void;
	} = $props();

	const TOTAL_MS = 12_000;
	const TICK_MS = 100;

	let remaining = $state(TOTAL_MS);
	let hovered = $state(false);
	let focused = $state(false);

	const frozen = $derived(hovered || focused);
	const progress = $derived(remaining / TOTAL_MS);

	// Keep the effect body free of reactive reads: `frozen`, `onDismiss` and
	// `draw` are only touched inside the tick callback. Otherwise every hover
	// or focus change would tear down and recreate the interval.
	$effect(() => {
		const timer = setInterval(() => {
			if (frozen) return;
			remaining -= TICK_MS;
			if (remaining <= 0) {
				remaining = 0;
				clearInterval(timer);
				onDismiss(draw.event.seq);
			}
		}, TICK_MS);
		return () => clearInterval(timer);
	});

	const deck = $derived.by(() => {
		const isChest = draw.event.deck === 'gemeinschaftsfeld';
		return {
			label: isChest ? 'Gemeinschaftskarte' : 'Ereigniskarte',
			icon: isChest ? IconGift : IconHelpCircle,
			accent: isChest ? 'var(--cd-blue-800)' : 'var(--cd-orange-700)'
		};
	});
	const DeckIcon = $derived(deck.icon);
</script>

<aside
	class="notice"
	style:--notice-accent={deck.accent}
	aria-label={`${deck.label} von ${drawer?.name ?? 'einem Spieler'}`}
	onmouseenter={() => (hovered = true)}
	onmouseleave={() => (hovered = false)}
	onfocusin={() => (focused = true)}
	onfocusout={() => (focused = false)}
>
	<header>
		<span class="icon-badge"><DeckIcon stroke={2} /></span>
		<div class="names">
			<h2>{deck.label}</h2>
			<span class="drawer">
				{#if drawer}
					<span class="dot" style:background={tokenColor(drawer.color)}></span>
					{drawer.name} zieht
				{:else}
					Es wird gezogen
				{/if}
			</span>
		</div>
		{#if me && draw.event.player === me.seat}<span class="pill">für dich</span>{/if}
		<button class="plain close" aria-label="Karte schließen" onclick={() => onDismiss(draw.event.seq)}>
			<IconX stroke={2} />
		</button>
	</header>

	<p class="card-text">{draw.event.text.replace(/^.*? zieht: /, '')}</p>

	{#if draw.results.length > 0}
		<h3>Auswirkung</h3>
		<ul class="results">
			{#each draw.results as event (event.seq)}
				<li>{event.text}</li>
			{/each}
		</ul>
	{/if}

	<div
		class="timer"
		role="progressbar"
		aria-label="Verbleibende Anzeigezeit"
		aria-valuemin={0}
		aria-valuemax={12}
		aria-valuenow={Math.round(remaining / 100) / 10}
	>
		<div class="timer-fill" style:width={`${progress * 100}%`}></div>
	</div>
</aside>

<style>
	.notice {
		padding: 0.9rem 1rem 0.8rem;
		background: var(--md-sys-color-surface-container-high);
		color: var(--md-sys-color-on-surface);
		border: 1px solid var(--md-sys-color-outline-variant);
		border-left: 4px solid var(--notice-accent);
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
		width: 1.9rem;
		height: 1.9rem;
		border-radius: 50%;
		background: var(--notice-accent);
		color: var(--cd-ral9016);
		flex: 0 0 auto;
	}

	.icon-badge :global(svg) {
		width: 1.1rem;
		height: 1.1rem;
	}

	.names {
		flex: 1;
		min-width: 0;
	}

	h2 {
		margin: 0;
		font: var(--md-sys-typescale-title-large);
	}

	.drawer {
		display: inline-flex;
		align-items: center;
		gap: 0.35rem;
		font: var(--md-sys-typescale-body-small);
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
		font: var(--md-sys-typescale-label-small);
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
		font: var(--md-sys-typescale-body-large);
	}

	h3 {
		margin: 0.7rem 0 0.35rem;
		font: var(--md-sys-typescale-label-medium);
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
		font: var(--md-sys-typescale-body-large);
	}

	.timer {
		height: 6px;
		margin-top: 0.8rem;
		background: var(--md-sys-color-surface-container-highest);
		border-radius: var(--md-sys-shape-corner-full);
		overflow: hidden;
	}

	.timer-fill {
		height: 100%;
		background: var(--notice-accent);
		transition: width 0.1s linear;
	}
</style>
