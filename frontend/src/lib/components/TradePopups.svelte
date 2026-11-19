<script lang="ts">
	import type { GameState, Intent, Player, TradeOffer } from '#lib/types';
	import { tradeUI } from '#lib/trade-ui.svelte';
	import TradeDetails from './TradeDetails.svelte';
	import IconArrowsExchange from '@tabler/icons-svelte/icons/arrows-exchange';
	import IconX from '@tabler/icons-svelte/icons/x';

	let {
		game,
		me,
		onIntent
	}: {
		game: GameState;
		me: Player | null;
		onIntent: (intent: Intent) => void;
	} = $props();

	/** Offers that were open on the very first snapshot do not pop up. */
	let baselineDone = $state(false);
	let seen = $state<Set<string>>(new Set());
	let dismissed = $state<Set<string>>(new Set());

	const openTrades = $derived(game.trades.filter((t) => t.status === 'open'));
	const popups = $derived(
		openTrades.filter((t) => baselineDone && !seen.has(t.id) && !dismissed.has(t.id))
	);

	// Baseline the offers of the first snapshot so a page reload does not replay
	// old offers as popups.
	$effect(() => {
		if (baselineDone) return;
		seen = new Set(openTrades.map((t) => t.id));
		baselineDone = true;
	});

	function dismiss(id: string): void {
		dismissed = new Set(dismissed).add(id);
	}

	/** One short line stating whether the viewer takes part in the trade. */
	function participation(trade: TradeOffer): string {
		const fromName = game.players[trade.from]?.name ?? '?';
		const toName = game.players[trade.to]?.name ?? '?';
		if (!me || (trade.from !== me.seat && trade.to !== me.seat)) {
			return `Tausch zwischen ${fromName} und ${toName}.`;
		}
		if (trade.to === me.seat) return 'Gerichtet an dich.';
		return 'Dein Angebot.';
	}
</script>

{#if popups.length > 0}
	<div class="popups">
		{#each popups as trade (trade.id)}
			<aside class="popup card" aria-label={`Tauschangebot ${game.players[trade.from]?.name} an ${game.players[trade.to]?.name}`}>
				<header class="row">
					<span class="aura"><IconArrowsExchange stroke={2} /></span>
					<strong>Tauschangebot</strong>
					{#if me && trade.to === me.seat}<span class="pill mine">für dich</span>{/if}
					<button class="plain close" aria-label="Popup schließen" onclick={() => dismiss(trade.id)}>
						<IconX stroke={2} />
					</button>
				</header>
				<p class="parties">
					<strong>{game.players[trade.from]?.name}</strong>
					<span class="muted">→</span>
					<strong>{game.players[trade.to]?.name}</strong>
				</p>
				<p class="muted context">{participation(trade)}</p>
				<TradeDetails {game} {trade} />
				{#if me && trade.to === me.seat}
					<div class="row actions">
						<button
							class="plain"
							onclick={() => onIntent({ type: 'respond_trade', tradeId: trade.id, accept: false })}
						>
							Ablehnen
						</button>
						<button class="plain" onclick={() => { tradeUI.requestCounter(trade.id); dismiss(trade.id); }}>
							Gegenangebot
						</button>
						<button
							class="primary"
							onclick={() => onIntent({ type: 'respond_trade', tradeId: trade.id, accept: true })}
						>
							Annehmen
						</button>
					</div>
				{:else if me && trade.from === me.seat}
					<p class="muted waiting">Wartet auf die Antwort von {game.players[trade.to]?.name}.</p>
				{:else}
					<p class="muted waiting">Details wie gehabt im Handelsbereich.</p>
				{/if}
			</aside>
		{/each}
	</div>
{/if}

<style>
	.popups {
		position: fixed;
		top: 4.4rem;
		right: 1rem;
		z-index: 50;
		display: flex;
		flex-direction: column;
		gap: 0.6rem;
		max-width: min(24rem, calc(100vw - 2rem));
		max-height: calc(100vh - 6rem);
		overflow-y: auto;
		padding: 0;
	}

	.popup {
		background: var(--md-sys-color-surface-container-high);
		border: 1px solid var(--md-sys-color-outline-variant);
		border-radius: var(--md-sys-shape-corner-medium);
		box-shadow: 0 var(--md-sys-elevation-2) calc(var(--md-sys-elevation-5))
			color-mix(in srgb, var(--md-sys-color-shadow) 22%, transparent);
		padding: 0.6rem 0.7rem;
	}

	header {
		gap: 0.4rem;
		margin-bottom: 0.15rem;
	}

	.aura {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 1.5rem;
		height: 1.5rem;
		border-radius: 50%;
		background: var(--md-sys-color-primary);
		color: var(--md-sys-color-on-primary);
		flex: 0 0 auto;
	}

	.aura :global(svg) {
		width: 0.95rem;
		height: 0.95rem;
	}

	.close {
		margin-left: auto;
		width: 1.6rem;
		height: 1.6rem;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 0;
	}

	.close :global(svg) {
		width: 0.85rem;
		height: 0.85rem;
	}

	.mine {
		background: var(--md-sys-color-primary-container);
		color: var(--md-sys-color-on-primary-container);
		border-radius: var(--md-sys-shape-corner-full);
		font-size: 0.68rem;
		font-weight: 700;
		padding: 0.1rem 0.45rem;
	}

	.parties {
		margin: 0.1rem 0 0;
		font-size: 0.92rem;
	}

	.context {
		margin: 0 0 0.2rem;
		font-size: 0.76rem;
	}

	.actions {
		margin-top: 0.35rem;
	}

	.actions button {
		font-size: 0.8rem;
		padding: 0.32rem 0.75rem;
	}

	.waiting {
		margin: 0.25rem 0 0;
		font-size: 0.76rem;
	}
</style>
