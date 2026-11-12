<script lang="ts">
	import type { GameEvent } from '#lib/types';
	import { formatMoney, formatTime } from '#lib/format';

	let { events }: { events: GameEvent[] } = $props();
</script>

<div class="log" aria-label="Ereignisprotokoll">
	{#if events.length === 0}
		<p class="muted">Noch keine Ereignisse.</p>
	{/if}
	{#each events as event (event.seq)}
		<div class="entry">
			<span class="time">{formatTime(event.at)}</span>
			<span class="text">{event.text}</span>
			{#if event.amount}
				<span class="amount">{formatMoney(event.amount)}</span>
			{/if}
		</div>
	{/each}
</div>

<style>
	.log {
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
		max-height: 22rem;
		overflow-y: auto;
		font-size: 12px;                                /* body-small, tokens.css:173 */
		line-height: 16px;
	}

	.entry {
		display: grid;
		grid-template-columns: auto 1fr auto;
		gap: 0.4rem;
		align-items: baseline;
		padding: 0.15rem 0.25rem;
		border-radius: var(--md-sys-shape-corner-extra-small);
	}

	.entry:nth-child(odd) {
		background: var(--md-sys-color-surface-container-low);
	}

	.time {
		color: var(--md-sys-color-on-surface-variant);
		font-variant-numeric: tabular-nums;
	}

	.amount {
		font-variant-numeric: tabular-nums;
		font-weight: 600;
	}
</style>
