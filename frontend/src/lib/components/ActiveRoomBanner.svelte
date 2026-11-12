<script lang="ts">
	// Start-page banner for the caller's blocking room. Purely presentational:
	// the page owns the fetch, the abandon request and the busy guard; this
	// component owns the phase-aware copy and the confirmation dialog.
	import type { ActiveRoom, RoomPhase } from '#lib/types';

	let {
		room,
		leaving = false,
		onRejoin,
		onAbandon
	}: {
		room: ActiveRoom;
		leaving?: boolean;
		onRejoin: () => void;
		onAbandon: () => void;
	} = $props();

	const PHASE_LABELS: Record<RoomPhase, string> = {
		lobby: 'Lobby',
		playing: 'Spiel läuft',
		finished: 'Spiel beendet'
	};

	const HINTS: Record<RoomPhase, string> = {
		lobby: 'Dein Sitz in der Lobby ist noch belegt. Solange du den Raum nicht endgültig verlässt, kannst du keinen neuen Raum eröffnen, keinem anderen Raum beitreten und nicht eingeladen werden.',
		playing:
			'Du bist noch in einem laufenden Spiel. Solange du den Raum nicht endgültig verlässt, kannst du keinen neuen Raum eröffnen, keinem anderen Raum beitreten und nicht eingeladen werden.',
		finished:
			'Das Spiel ist beendet, aber dein Sitz ist noch mit einer offenen Raumverbindung verknüpft. Verlasse den Raum, um wieder verfügbar zu sein.'
	};

	const CONSEQUENCES: Record<RoomPhase, string> = {
		lobby:
			'Du gibst deinen Sitz in Raum {code} endgültig auf. Ist es der letzte Sitz, wird der Raum geschlossen; andernfalls bleibt er für die übrigen Spieler offen.',
		playing:
			'Du verlässt das Spiel endgültig. Bist du noch im Spiel, scheidest du aus: Gebäude und Felder gehen an die Bank, offene Handelsangebote verfallen. Bist du bereits ausgeschieden, wird nur dein Sitz entfernt. Ein Wiedereinstieg ins Spiel ist danach nicht mehr möglich.',
		finished:
			'Du entfernst deinen Sitz aus dem beendeten Spiel. Der Raum bleibt für die anderen unverändert.'
	};

	const phaseLabel = $derived(PHASE_LABELS[room.phase]);
	const hint = $derived(HINTS[room.phase]);
	const consequence = $derived(CONSEQUENCES[room.phase].replace('{code}', room.code));

	let confirming = $state(false);
	let dialogEl = $state<HTMLElement>();
	// Captured on open rather than inside the effect: the effect re-runs once the
	// bound dialog element exists, which would otherwise capture the dialog itself.
	let opener: HTMLElement | null = null;

	// While the dialog is open: take focus for keyboard use, lock the page behind
	// it and listen for Escape, like the other dialogs. On close, undo the lock
	// and hand focus back to the trigger.
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
		onAbandon();
	}

	function backdrop(event: MouseEvent) {
		if (event.target === event.currentTarget) close();
	}
</script>

<section class="active-room" aria-labelledby="active-room-heading">
	<div class="head">
		<h2 id="active-room-heading">Du bist noch in Raum {room.code}</h2>
		<span class="badge">{phaseLabel}</span>
	</div>
	<p class="hint">{hint}</p>
	<div class="actions">
		<button class="primary" disabled={leaving} onclick={onRejoin}>Wieder beitreten</button>
		<button class="plain danger" disabled={leaving} onclick={open}>
			{leaving ? 'Verlassen …' : 'Raum verlassen'}
		</button>
	</div>
</section>

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
			aria-label="Raum verlassen bestätigen"
			aria-describedby="abandon-consequence"
			tabindex="-1"
		>
			<h2>Raum verlassen</h2>
			<p id="abandon-consequence">{consequence}</p>
			<div class="dialog-actions">
				<button class="primary danger" onclick={confirm}>Ja, Raum verlassen</button>
				<button class="plain" onclick={close}>Abbrechen</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.active-room {
		background: var(--cd-info-container);
		color: var(--cd-on-info-container);
		border-radius: var(--md-sys-shape-corner-medium);
		padding: 1rem;
	}

	.head {
		display: flex;
		align-items: baseline;
		gap: 0.6rem;
		flex-wrap: wrap;
	}

	.head h2 {
		margin: 0;
	}

	.hint {
		margin: 0.35rem 0 0;
		font-size: 0.88rem;
		line-height: 1.4;
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		margin-top: 0.75rem;
	}

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
