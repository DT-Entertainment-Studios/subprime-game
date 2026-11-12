<script lang="ts">
	import type { AutopilotJail, AutopilotSettings, GameState, Intent, Player } from '#lib/types';
	import { autopilotStopLabel, groupColor, groupLabel, STREET_GROUPS } from '#lib/format';

	const DEFAULTS: AutopilotSettings = {
		enabled: false,
		buyAlways: true,
		jail: 'none',
		buildGroups: [],
		liquidity: 0
	};

	const MAX_LIQUIDITY = 100000;

	let {
		game,
		me,
		onIntent
	}: {
		game: GameState;
		me: Player;
		onIntent: (intent: Intent) => void;
	} = $props();

	// Always derive from the server snapshot; no local copy that could drift.
	const config = $derived(me.autopilot?.config ?? DEFAULTS);

	/** First board index of a group; the shared order of every group list. */
	function boardIndex(group: string): number {
		const tile = game.board.find((entry) => entry.group === group);
		return tile ? tile.index : Number.MAX_SAFE_INTEGER;
	}

	/** Deduplicates groups and keeps them in board order. */
	function orderedGroups(groups: Iterable<string>): string[] {
		return [...new Set(groups)].sort((a, b) => boardIndex(a) - boardIndex(b));
	}

	function update(patch: Partial<AutopilotSettings>): void {
		onIntent({
			type: 'set_autopilot',
			autopilot: {
				...config,
				...patch,
				buildGroups: orderedGroups(patch.buildGroups ?? config.buildGroups)
			}
		});
	}

	/** The main switch paints from the server snapshot, so the native checkbox
	 *  state is reset to it right after the intent is sent. Otherwise a rejected
	 *  or dropped intent would leave the input checked while the switch shows
	 *  "Aus". */
	function setEnabled(input: HTMLInputElement, next: boolean): void {
		// The native state is reset before the side effect, so it can never stay
		// checked when the intent is rejected, dropped or the send throws.
		input.checked = config.enabled;
		update({ enabled: next });
	}

	/** Street groups whose every tile belongs to this seat, none is mortgaged and
	 *  at least one tile can still take a house (a hotel counts as five). */
	const buildable = $derived<string[]>(
		STREET_GROUPS.filter((group) => {
			const tiles = game.board.filter((tile) => tile.group === group);
			if (tiles.length === 0) return false;
			const states = tiles.map((tile) => game.tiles[tile.index]);
			const owned = states.every((state) => state?.owner === me.seat && !state.mortgaged);
			const hasRoom = states.some((state) => state !== undefined && state.houses < 5);
			return owned && hasRoom;
		})
	);

	// Configured groups stay visible even when they are no longer buildable, so
	// they can be switched off again.
	const groups = $derived.by(() =>
		orderedGroups([...buildable, ...config.buildGroups]).map((group) => ({
			group,
			buildable: buildable.includes(group)
		}))
	);

	/** Last automatic or manual stop recorded for this seat. */
	const stopEvent = $derived.by(() => {
		for (let index = game.events.length - 1; index >= 0; index -= 1) {
			const event = game.events[index];
			if (event.type === 'autopilot_off' && event.player === me.seat) return event;
		}
		return null;
	});

	function setJail(jail: AutopilotJail): void {
		update({ jail });
	}

	function toggleGroup(group: string, on: boolean): void {
		const next = new Set(config.buildGroups);
		if (on) next.add(group);
		else next.delete(group);
		update({ buildGroups: [...next] });
	}

	function commitLiquidity(input: HTMLInputElement): void {
		const parsed = Number(input.value.replace(',', '.'));
		const value = Number.isFinite(parsed) ? Math.trunc(parsed) : 0;
		const clamped = Math.min(MAX_LIQUIDITY, Math.max(0, value));
		// Reflect the committed value immediately; the server confirms it.
		input.value = String(clamped);
		update({ liquidity: clamped });
	}
</script>

<div class="autopilot">
	<div class="power" class:is-on={config.enabled} class:is-disabled={me.bankrupt}>
		<label class="power-switch" for="autopilot-enabled">
			<input
				id="autopilot-enabled"
				type="checkbox"
				role="switch"
				checked={config.enabled}
				disabled={me.bankrupt}
				onchange={(event) => setEnabled(event.currentTarget, event.currentTarget.checked)}
			/>
			<span class="track" aria-hidden="true"><span class="thumb"></span></span>
			<span class="power-title">Autopilot aktiv</span>
		</label>
		<span class="power-state" aria-hidden="true">{config.enabled ? 'Aktiv' : 'Aus'}</span>
	</div>

	<label class="check" for="autopilot-buy">
		<input
			id="autopilot-buy"
			type="checkbox"
			checked={config.buyAlways}
			disabled={me.bankrupt}
			onchange={(event) => update({ buyAlways: event.currentTarget.checked })}
		/>
		<span>Immer kaufen</span>
	</label>

	<fieldset>
		<legend>Werkstatt</legend>
		<div class="choices">
			<label class="check" for="autopilot-jail-none">
				<input
					id="autopilot-jail-none"
					type="radio"
					name="autopilot-jail"
					value="none"
					checked={config.jail === 'none'}
					disabled={me.bankrupt}
					onchange={() => setJail('none')}
				/>
				<span>Aus</span>
			</label>
			<label class="check" for="autopilot-jail-pay">
				<input
					id="autopilot-jail-pay"
					type="radio"
					name="autopilot-jail"
					value="pay"
					checked={config.jail === 'pay'}
					disabled={me.bankrupt}
					onchange={() => setJail('pay')}
				/>
				<span>Freikaufen</span>
			</label>
			<label class="check" for="autopilot-jail-roll">
				<input
					id="autopilot-jail-roll"
					type="radio"
					name="autopilot-jail"
					value="roll"
					checked={config.jail === 'roll'}
					disabled={me.bankrupt}
					onchange={() => setJail('roll')}
				/>
				<span>Pasch versuchen</span>
			</label>
		</div>
	</fieldset>

	<fieldset>
		<legend>Automatisch bauen</legend>
		<label for="autopilot-liquidity">Liquidität (Mindestrest in GE)</label>
		<input
			id="autopilot-liquidity"
			type="number"
			min="0"
			max={MAX_LIQUIDITY}
			step="10"
			value={config.liquidity}
			disabled={me.bankrupt}
			aria-describedby="autopilot-liquidity-hint"
			onchange={(event) => commitLiquidity(event.currentTarget)}
		/>
		<p id="autopilot-liquidity-hint" class="muted hint">
			Gebaut wird nur, solange mindestens dieser Betrag übrig bleibt.
		</p>
		{#if groups.length === 0}
			<p class="muted hint">Du besitzt noch keine vollständige Farbgruppe.</p>
		{:else}
			<div class="choices">
				{#each groups as entry (entry.group)}
					<label class="check" for="autopilot-group-{entry.group}">
						<input
							id="autopilot-group-{entry.group}"
							type="checkbox"
							checked={config.buildGroups.includes(entry.group)}
							disabled={me.bankrupt}
							onchange={(event) => toggleGroup(entry.group, event.currentTarget.checked)}
						/>
						<span class="swatch" style="background: {groupColor(entry.group)}"></span>
						<span>{groupLabel(entry.group)}</span>
						{#if !entry.buildable}<span class="unbuildable">nicht baubar</span>{/if}
					</label>
				{/each}
			</div>
		{/if}
	</fieldset>

	<p class="status">
		{#if config.enabled}
			Autopilot aktiv.
		{:else if stopEvent}
			Autopilot aus – {autopilotStopLabel(stopEvent.reason)}.
		{:else}
			Autopilot aus.
		{/if}
	</p>
</div>

<style>
	.autopilot {
		display: flex;
		flex-direction: column;
		gap: 0.55rem;
	}

	.check {
		display: flex;
		align-items: center;
		gap: 0.35rem;
		margin: 0;
		font-size: 12px; /* label-medium, tokens.css:175 */
		line-height: 16px;
		font-weight: 500;
		color: var(--md-sys-color-on-surface);
	}

	.check input {
		width: auto;
		margin: 0;
		flex: 0 0 auto;
	}

	/* The main switch is the panel's primary control, so it gets its own
	   emphasised row instead of a plain 12px checkbox label. */
	.power {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		padding: 0.5rem 0.6rem;
		background: var(--md-sys-color-surface-container-high);
		border: 1px solid var(--md-sys-color-outline-variant);
		border-radius: var(--md-sys-shape-corner-small);
	}

	.power-switch {
		position: relative;
		display: flex;
		align-items: center;
		gap: 0.6rem;
		margin: 0;
		cursor: pointer;
	}

	/* Visually hidden but still focusable: keeps the native checkbox keyboard
	   behaviour, checked state and accessible name; the track is the paint. */
	.power-switch input {
		position: absolute;
		inline-size: 1px;
		block-size: 1px;
		opacity: 0;
	}

	.track {
		position: relative;
		display: block;
		flex: 0 0 auto;
		inline-size: 2.75rem;
		block-size: 1.6rem;
		border: 2px solid var(--md-sys-color-outline);
		border-radius: var(--md-sys-shape-corner-full);
		background: var(--md-sys-color-surface-variant);
		transition:
			background-color 120ms ease,
			border-color 120ms ease;
	}

	/* Off: 1rem thumb, 0.25rem inset inside the 2px border. */
	.thumb {
		position: absolute;
		inset-block-start: 50%;
		inset-inline-start: 0.25rem;
		inline-size: 1rem;
		block-size: 1rem;
		border-radius: var(--md-sys-shape-corner-full);
		background: var(--md-sys-color-outline);
		transform: translateY(-50%);
		transition:
			inset-inline-start 120ms ease,
			inline-size 120ms ease,
			block-size 120ms ease,
			background-color 120ms ease;
	}

	/* Track, thumb and pill all follow config.enabled, so the painted switch can
	   never contradict the state pill while an intent is in flight. */
	.is-on .track {
		background: var(--md-sys-color-primary);
		border-color: var(--md-sys-color-primary);
	}

	/* On: 1.2rem thumb, still 0.25rem from both inner edges (100% is the
	   padding box of the 2.75rem track, i.e. 2.75rem - 2 * 2px border). */
	.is-on .track .thumb {
		inset-inline-start: calc(100% - 1.2rem - 0.25rem);
		inline-size: 1.2rem;
		block-size: 1.2rem;
		background: var(--md-sys-color-on-primary);
	}

	/* Primary rather than --cd-focus-ring-color: the switch is the panel's
	   primary control and needs the stronger ring on its grey row. */
	input:focus-visible + .track {
		outline: var(--cd-focus-ring-width) solid var(--md-sys-color-primary);
		outline-offset: var(--cd-focus-ring-offset);
	}

	.power-title {
		font: var(--md-sys-typescale-title-medium);
		color: var(--md-sys-color-on-surface);
	}

	/* Outside the label so it never joins the switch's accessible name. */
	.power-state {
		margin-left: auto;
		padding: 0.05rem 0.5rem;
		border-radius: var(--md-sys-shape-corner-full);
		font: var(--md-sys-typescale-label-medium);
		background: var(--md-sys-color-surface-variant);
		color: var(--md-sys-color-on-surface-variant);
	}

	.is-on .power-state {
		background: var(--cd-success-container);
		color: var(--cd-on-success-container);
	}

	.is-disabled .track,
	.is-disabled .power-title,
	.is-disabled .power-state {
		opacity: 0.5;
	}

	.is-disabled .power-switch {
		cursor: default;
	}

	@media (prefers-reduced-motion: reduce) {
		.track,
		.thumb {
			transition: none;
		}
	}

	fieldset {
		margin: 0;
		padding: 0.45rem 0.55rem 0.55rem;
		border: 1px solid var(--md-sys-color-outline-variant);
		border-radius: var(--md-sys-shape-corner-small);
	}

	legend {
		padding: 0 0.25rem;
		font-size: 12px; /* label-medium, tokens.css:175 */
		line-height: 16px;
		font-weight: 500;
		color: var(--md-sys-color-on-surface-variant);
	}

	.choices {
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
	}

	fieldset > label {
		margin-bottom: 0.25rem;
	}

	fieldset input[type='number'] {
		max-width: 12rem;
	}

	.hint {
		margin: 0.25rem 0 0;
		font-size: 0.78rem;
	}

	.swatch {
		width: 0.85rem;
		height: 0.85rem;
		border-radius: var(--md-sys-shape-corner-extra-small);
		background: var(--md-sys-color-surface-variant);
		border: 1px solid color-mix(in srgb, var(--md-sys-color-shadow) 28%, transparent);
		flex: 0 0 auto;
	}

	.unbuildable {
		font-size: 0.68rem;
		padding: 0.02rem 0.35rem;
		border-radius: var(--md-sys-shape-corner-full);
		background: var(--cd-warning-container);
		color: var(--cd-on-warning-container);
		font-weight: 600;
	}

	.status {
		margin: 0;
		font-size: 0.78rem;
		color: var(--md-sys-color-on-surface-variant);
	}
</style>
