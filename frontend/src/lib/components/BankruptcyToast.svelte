<script lang="ts">
	// One automatic bankruptcy, shown as a dismissible notice in the shared
	// bottom-left dock (Spec.md §4.10). No modal, no focus theft, no timer and
	// no auto-dismiss: the reader closes it explicitly with the button.
	import type { GameEvent, Player } from '#lib/types';
	import { tokenColor } from '#lib/format';
	import IconX from '@tabler/icons-svelte/icons/x';

	let {
		event,
		player = undefined,
		onDismiss
	}: {
		event: GameEvent;
		player?: Player;
		onDismiss: (seq: number) => void;
	} = $props();
</script>

<aside class="notice" aria-label={`Pleite von ${player?.name ?? 'einem Spieler'}`}>
	<header>
		<div class="names">
			<h2>Pleite</h2>
			{#if player}
				<span class="who">
					<span class="dot" style:background={tokenColor(player.color)}></span>
					{player.name}
				</span>
			{:else}
				<span class="who">Ein Spieler ist ausgeschieden</span>
			{/if}
		</div>
		<button class="plain close" aria-label="Hinweis schließen" onclick={() => onDismiss(event.seq)}>
			<IconX stroke={2} />
		</button>
	</header>

	<p class="bankruptcy-text">{event.text}</p>
</aside>

<style>
	.notice {
		padding: 0.9rem 1rem 0.8rem;
		background: var(--md-sys-color-error-container);
		color: var(--md-sys-color-on-error-container);
		border: 1px solid var(--md-sys-color-error);
		border-left: 4px solid var(--md-sys-color-error);
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

	.names {
		flex: 1;
		min-width: 0;
	}

	h2 {
		margin: 0;
		font: var(--md-sys-typescale-title-large);
	}

	.who {
		display: inline-flex;
		align-items: center;
		gap: 0.35rem;
		font: var(--md-sys-typescale-body-small);
	}

	.dot {
		width: 0.7rem;
		height: 0.7rem;
		border-radius: 50%;
		border: 1px solid currentColor;
		flex: 0 0 auto;
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

	.bankruptcy-text {
		margin: 0;
		padding: 0.8rem 0.9rem;
		border: 1px solid currentColor;
		border-radius: var(--md-sys-shape-corner-medium);
		font: var(--md-sys-typescale-body-large);
	}
</style>
