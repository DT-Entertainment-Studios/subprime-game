<script lang="ts">
	import type { GameState, KickVote, Player } from '#lib/types';
	import { formatMoney, tokenColor } from '#lib/format';
	import { bankruptcyFor, bankruptcyTooltip } from '#lib/bankruptcy';
	import CarIcon from './CarIcon.svelte';
	import IconTool from '@tabler/icons-svelte/icons/tool';
	import IconTicket from '@tabler/icons-svelte/icons/ticket';
	import IconWifiOff from '@tabler/icons-svelte/icons/wifi-off';

	let {
		game,
		me = null,
		kickVotes = [],
		onVotekick,
		selectedSeat = null,
		onSelect
	}: {
		game: GameState;
		me?: Player | null;
		kickVotes?: KickVote[];
		onVotekick?: (userId: string) => void;
		selectedSeat?: number | null;
		onSelect?: (seat: number) => void;
	} = $props();

	// The name button is the keyboard control; the row click is the larger
	// pointer target around it. Stopping propagation in the button keeps a click
	// on the name from toggling twice.
	function handleRowClick(event: MouseEvent, seat: number) {
		if (event.target instanceof Element && event.target.closest('.vote-slot')) return;
		onSelect?.(seat);
	}
</script>

<ul class="players">
	{#each game.players as player (player.id)}
		{@const vote = kickVotes.find((entry) => entry.userId === player.id)}
		{@const bust = player.bankrupt ? bankruptcyFor(game.events, player.seat) : undefined}
		{@const needed = game.players.filter(
			(p) => p.id !== player.id && p.connected && !p.bankrupt
		).length}
		{@const ownedCount = game.tiles.filter((tile) => tile.owner === player.seat).length}
		{@const canVote =
			game.phase === 'playing' &&
			me !== null &&
			!me.bankrupt &&
			!player.bankrupt &&
			!player.connected &&
			player.id !== me.id}
		<!-- The <li> click is a pointer convenience; Enter/Space stay on the real button. -->
		<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions -->
		<li
			class="player"
			class:current={game.current === player.seat && game.phase === 'playing'}
			class:bankrupt={player.bankrupt}
			class:selected={selectedSeat === player.seat}
			onclick={(event) => handleRowClick(event, player.seat)}
		>
			<span class="car" class:active={game.current === player.seat && game.phase === 'playing'}>
				<CarIcon color={tokenColor(player.color)} />
			</span>
			<span class="name-cell">
				<button
					type="button"
					class="name"
					aria-pressed={selectedSeat === player.seat}
					aria-describedby={`player-tip-${player.id}`}
					onclick={(event) => {
						event.stopPropagation();
						onSelect?.(player.seat);
					}}
				>{player.name}</button>
				<span class="tip" role="tooltip" id={`player-tip-${player.id}`}>
					<span>NAV: {formatMoney(player.netWorth)}</span>
					<span>{ownedCount} {ownedCount === 1 ? 'Feld' : 'Felder'}</span>
				</span>
				{#if me?.id === player.id}<span class="badge">Du</span>{/if}
				{#if player.autopilot?.active}<span class="badge" title="Autopilot aktiv">Autopilot</span>{/if}
			</span>
			<span class="figures">
				{#if player.inJail}<span title="In der Werkstatt"><IconTool size={14} /></span>{/if}
				{#if player.jailCards > 0}<span title="Pannenhilfen"><IconTicket size={14} />{player.jailCards}</span>{/if}
				{#if !player.connected}<span title="Getrennt"><IconWifiOff size={14} /></span>{/if}
				<!-- Only rows that can ever vote reserve the slot; the two states
				     overlay each other so a cast vote never changes its width. -->
				{#if canVote}
					<span class="vote-slot">
						<span
							class="badge"
							class:slot-hidden={!vote?.voted}
							aria-hidden={vote?.voted ? undefined : 'true'}
							title="Du hast für einen Votekick gestimmt"
						>Votekick {vote?.votes ?? 0}/{needed} · abgestimmt</span
						>
						<button
							class="plain vote"
							type="button"
							class:slot-hidden={!!vote?.voted}
							disabled={!!vote?.voted}
							aria-hidden={vote?.voted ? 'true' : undefined}
							tabindex={vote?.voted ? -1 : undefined}
							aria-label="Votekick gegen {player.name}"
							title="Votekick gegen {player.name} starten"
							onclick={() => onVotekick?.(player.id)}>Votekick {vote?.votes ?? 0}/{needed}</button
						>
					</span>
				{/if}
				{#if player.bankrupt}<span class="badge" title={bust ? bankruptcyTooltip(bust) : 'Aus dem Spiel ausgeschieden'}>pleite</span>{/if}
			</span>
			<span class="cash" class:negative={player.cash < 0}>{formatMoney(player.cash)}</span>
		</li>
	{/each}
</ul>

<style>
	.players {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	.player {
		display: grid;
		grid-template-columns: auto 1fr auto auto;
		align-items: center;
		gap: 0.5rem;
		padding: 0.35rem 0.5rem;
		border-radius: var(--md-sys-shape-corner-small);
		background: var(--md-sys-color-surface);
		border: 1px solid transparent;
		cursor: pointer;
	}

	.player.current {
		border-color: var(--md-sys-color-tertiary-container);
		background: var(--md-sys-color-tertiary-container);
		color: var(--md-sys-color-on-tertiary-container);
	}

	/* The tint alone is near-invisible on the white row in light mode; the inset
	   bar marks the active seat without taking width. Dark mode already reads
	   well, so the accent stays light-only (clarify D3). */
	:global(:root[data-theme='light']) .player.current {
		box-shadow: inset 3px 0 0 0 var(--md-sys-color-tertiary);
	}

	/* Inside outline so the row keeps its size; the current-turn tint and inset
	   bar stay visible underneath it. */
	.player.selected {
		outline: 2px solid var(--md-sys-color-primary);
		outline-offset: -2px;
	}

	.player.bankrupt {
		opacity: 0.6;
		text-decoration: line-through;
	}

	.car {
		display: block;
		width: 1.1rem;
		flex: 0 0 auto;
	}

	.car.active {
		filter: drop-shadow(0 0 0.2rem color-mix(in srgb, var(--md-sys-color-on-surface) 55%, transparent));
	}

	.name-cell {
		position: relative;
		display: flex;
		align-items: center;
		gap: 0.25rem;
		min-width: 0;
		overflow: visible;
	}

	/* The name is a plain-looking toggle, not a separate CTA. */
	.name {
		display: block;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		background: none;
		border: 0;
		padding: 0;
		color: inherit;
		font: inherit;
		text-align: left;
		cursor: pointer;
	}

	.name-cell .badge {
		flex: 0 0 auto;
	}

	.tip {
		display: none;
		position: absolute;
		top: calc(100% + 0.25rem);
		left: 0;
		z-index: 5;
		min-width: max-content;
		padding: 0.5rem 0.75rem;
		border-radius: var(--md-sys-shape-corner-small);
		background: var(--md-sys-color-inverse-surface);
		color: var(--md-sys-color-inverse-on-surface);
		font: var(--md-sys-typescale-label-small);
		white-space: nowrap;
		line-height: 1.4;
	}

	.tip span {
		display: block;
	}

	/* jsdom cannot evaluate media queries; the source-level gate is tested in
	   PlayersList.test.ts and the behaviour needs a real fine-pointer check. */
	@media (hover: hover) and (pointer: fine) {
		.player:hover .tip,
		.name:focus-visible + .tip {
			display: block;
		}
	}

	.figures {
		display: flex;
		align-items: center;
		gap: 0.25rem;
		font-size: 12px;                                /* body-small, tokens.css:173 */
		line-height: 16px;
	}

	.figures span {
		display: inline-flex;
		align-items: center;
		gap: 0.1rem;
	}

	.figures .vote {
		padding: 0.05rem 0.45rem;
		font-size: 11px;                                /* label-small, tokens.css:176 */
		line-height: 16px;
		white-space: nowrap;
	}

	/* Both vote states share one grid cell; the widest one fixes the slot width. */
	.figures .vote-slot {
		display: inline-grid;
		align-items: center;
		justify-items: start;
	}

	.figures .vote-slot > * {
		grid-area: 1 / 1;
	}

	.slot-hidden {
		visibility: hidden;
	}

	.cash {
		font-variant-numeric: tabular-nums;
		font-weight: 600;
	}

	.negative {
		color: var(--md-sys-color-error);
	}
</style>
