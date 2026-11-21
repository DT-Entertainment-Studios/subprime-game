<script lang="ts">
	import type { GameEvent } from '#lib/types';
	import { formatMoney, formatTime } from '#lib/format';

	let { events }: { events: GameEvent[] } = $props();

	// Newest first: the log renders the reverse of the arrival order.
	const ordered = $derived([...events].reverse());
	const newestSeq = $derived(events.length > 0 ? events[events.length - 1].seq : 0);

	let log: HTMLDivElement | null = null;
	// Follow the newest entry only while the reader sits at the top. Once they
	// scrolled down, incoming entries must not move their viewport.
	let atTop = true;
	let seenSeq = 0;
	let heightBefore = 0;
	let topBefore = 0;

	function handleScroll() {
		if (!log) return;
		atTop = log.scrollTop <= 2;
	}

	// Snapshot the scroll geometry before the new rows are in the DOM.
	$effect.pre(() => {
		if (newestSeq === seenSeq || !log) return;
		heightBefore = log.scrollHeight;
		topBefore = log.scrollTop;
	});

	// Effects run after Svelte updated the DOM, so the snapshot above and the
	// scrollHeight read here belong to the same insertion.
	$effect(() => {
		if (newestSeq === seenSeq) return;
		seenSeq = newestSeq;
		if (!log) return;
		if (atTop) {
			log.scrollTop = 0;
			return;
		}
		const grewBy = log.scrollHeight - heightBefore;
		if (grewBy > 0) log.scrollTop = topBefore + grewBy;
	});
</script>

<div class="log" aria-label="Ereignisprotokoll" bind:this={log} onscroll={handleScroll}>
	{#if events.length === 0}
		<p class="muted">Noch keine Ereignisse.</p>
	{/if}
	{#each ordered as event (event.seq)}
		<div class="entry" class:bankrupt={event.type === 'bankruptcy'}>
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
		overflow-anchor: none;                          /* scroll position is managed in the script */
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

	/* Bankruptcy lines override the zebra tint; declared after :nth-child so the
	   equal-specificity rule wins (Spec.md §4.3). */
	.entry.bankrupt {
		background: var(--md-sys-color-error-container);
		color: var(--md-sys-color-on-error-container);
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
