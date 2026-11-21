<script lang="ts">
	// Voluntary forfeit ("Aufgeben") during the player's own turn, next to
	// "Verlassen" in the room topbar. The two-step confirmation keeps the intent
	// from firing on a misclick; the engine decides whether it is allowed.
	// This is not the debt exit: an open debt blocks the whole game immediately,
	// and the debtor settles it (or gives up) from the debt panel, even out of
	// turn. A debt beyond full liquidation ends in automatic bankruptcy, so no
	// manual giving-up is needed there.
	import type { Intent, Phase, Player, TurnPhase } from '#lib/types';

	let {
		phase,
		turnPhase,
		isMyTurn,
		me,
		onIntent
	}: {
		phase: Phase;
		turnPhase: TurnPhase;
		isMyTurn: boolean;
		me: Player | null;
		onIntent: (intent: Intent) => void;
	} = $props();

	// Mirrors the whitelist of the engine's Game.Forfeit (Spec.md §4.10): a running
	// game, own turn only, and never in the global debt phase (that keeps its own
	// debt panel) or during an auction. Keep both lists in sync.
	const FORFEIT_PHASES: TurnPhase[] = ['awaiting_roll', 'awaiting_purchase', 'awaiting_jail'];
	const canForfeit = $derived(
		phase === 'playing' &&
			isMyTurn &&
			me !== null &&
			!me.bankrupt &&
			FORFEIT_PHASES.includes(turnPhase)
	);

	let confirming = $state(false);
	let dialogEl = $state<HTMLElement>();
	// Captured on open rather than inside the effect: the effect re-runs once the
	// bound dialog element exists, which would otherwise capture the dialog itself.
	let opener: HTMLElement | null = null;

	// A turn change or a resolved phase closes the dialog before it goes stale.
	$effect(() => {
		if (!canForfeit) confirming = false;
	});

	// While the dialog is open: take focus for keyboard use, lock the page behind
	// it and listen for Escape, like the other dialogs (EstatePanel, TileCard). On
	// close, undo the lock and hand focus back to the trigger.
	$effect(() => {
		if (!confirming) return;
		const onKeydown = (event: KeyboardEvent) => {
			if (event.key === 'Escape') close();
		};
		dialogEl?.focus();
		document.body.style.overflow = 'hidden';
		window.addEventListener('keydown', onKeydown);
		return () => {
			window.removeEventListener('keydown', onKeydown);
			document.body.style.overflow = '';
			if (opener?.isConnected) opener.focus();
		};
	});

	function open() {
		opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
		confirming = true;
	}

	function close() {
		confirming = false;
	}

	function confirm() {
		confirming = false;
		onIntent({ type: 'forfeit' });
	}

	function backdrop(event: MouseEvent) {
		if (event.target === event.currentTarget) close();
	}
</script>

{#if canForfeit}
	<button class="plain danger" onclick={open}>Aufgeben</button>
{/if}

{#if confirming}
	<!-- The backdrop only closes on click; keyboard users close with Escape or
	     the cancel button, so the pointer-only interaction is intentional here. -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="backdrop" onclick={backdrop}>
		<div
			class="dialog"
			bind:this={dialogEl}
			role="dialog"
			aria-modal="true"
			aria-label="Aufgeben bestätigen"
			aria-describedby="forfeit-consequence"
			tabindex="-1"
		>
			<h2>Aufgeben</h2>
			<p id="forfeit-consequence">
				Wirklich aufgeben? Du scheidest endgültig aus dem Spiel aus und verlierst. Gebäude und
				Felder gehen an die Bank; offene Handelsangebote verfallen.
			</p>
			<p class="muted">
				Bei Schulden nutzt der Schuldner das Schuldenpanel – auch wenn eine andere Spielerin am
				Zug ist. Reicht der komplette Liquidationswert nicht, scheidet er automatisch aus.
			</p>
			{#if turnPhase === 'awaiting_purchase'}
				<p class="muted">Das offene Kaufangebot verfällt.</p>
			{/if}
			<div class="dialog-actions">
				<button class="primary danger" onclick={confirm}>Ja, aufgeben</button>
				<button class="plain" onclick={close}>Abbrechen</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.danger {
		color: var(--md-sys-color-error);
		border-color: var(--md-sys-color-error);
	}

	.primary.danger {
		background: var(--md-sys-color-error);
		color: var(--md-sys-color-on-error);
	}

	.backdrop {
		position: fixed;
		inset: 0;
		z-index: 70;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 1rem;
		background: color-mix(in srgb, var(--md-sys-color-scrim) 52%, transparent);
	}

	.dialog {
		width: min(24rem, 94vw);
		outline: none;
		padding: 0.9rem 1rem 1rem;
		background: var(--md-sys-color-surface-container);
		border: 1px solid var(--md-sys-color-outline-variant);
		border-radius: var(--md-sys-shape-corner-large);
		box-shadow: 0 var(--md-sys-elevation-4) calc(var(--md-sys-elevation-5) * 3)
			color-mix(in srgb, var(--md-sys-color-shadow) 32%, transparent);
	}

	.dialog h2 {
		margin: 0 0 0.5rem;
		font-size: 1.05rem;
		line-height: 1.2;
	}

	.dialog p {
		margin: 0 0 0.45rem;
		font-size: 0.88rem;
		line-height: 1.4;
	}

	.dialog-actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
		margin-top: 0.8rem;
	}
</style>
