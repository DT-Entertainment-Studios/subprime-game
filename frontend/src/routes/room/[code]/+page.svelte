<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { session } from '#lib/session.svelte';
	import { presence } from '#lib/presence.svelte';
	import { preferences } from '#lib/prefs.svelte';
	import type { Intent } from '#lib/types';
	import { formatMoney } from '#lib/format';
	import { createScreenWakeLock } from '#lib/wake-lock';
	import Board from '#lib/components/Board.svelte';
	import ForfeitButton from '#lib/components/ForfeitButton.svelte';
	import Lobby from '#lib/components/Lobby.svelte';
	import PlayersList from '#lib/components/PlayersList.svelte';
	import EventLog from '#lib/components/EventLog.svelte';
	import EstatePanel from '#lib/components/EstatePanel.svelte';
	import AutopilotPanel from '#lib/components/AutopilotPanel.svelte';
	import TradePanel from '#lib/components/TradePanel.svelte';
	import TradePopups from '#lib/components/TradePopups.svelte';
	import CardPopups from '#lib/components/CardPopups.svelte';
	import AutopilotToast from '#lib/components/AutopilotToast.svelte';
	import GameOver from '#lib/components/GameOver.svelte';
	import ThemeToggle from '#lib/components/ThemeToggle.svelte';
	import { tradeUI } from '#lib/trade-ui.svelte';

	const code = $derived((page.params.code ?? '').toUpperCase());
	let joining = $state(false);
	let leaving = $state(false);

	const wakeLock = createScreenWakeLock();

	onMount(() => {
		void session.init().then(async () => {
			await session.load(code);
			session.connect(code);
		});
		return () => {
			session.disconnect();
			presence.disconnectRoom(code);
			wakeLock.dispose();
		};
	});

	const room = $derived(session.room);
	const game = $derived(room?.game ?? null);
	const me = $derived(session.me);
	const isMyTurn = $derived(session.isMyTurn);
	// The seat settling a debt can differ from the active player.
	const isDebtor = $derived(session.isDebtor);
	const debtor = $derived(game && game.debtor >= 0 ? (game.players[game.debtor] ?? null) : null);

	// The estate card keeps its own state; only the user toggles it.
	let estateOpen = $state(preferences.current.estatePanelOpen);

	$effect(() => {
		const open = estateOpen;
		// Persist without tracking preferences.current, otherwise update()
		// would re-trigger this effect in a loop.
		untrack(() => {
			if (preferences.current.estatePanelOpen !== open) {
				preferences.update({ estatePanelOpen: open });
			}
		});
	});

	// Only a lobby needs presence; leaving the lobby or the page closes this scope.
	$effect(() => {
		if (room?.phase === 'lobby') presence.connectRoom(code);
		else presence.disconnectRoom(code);
	});

	// Keep the screen on for the whole running game, release it otherwise.
	$effect(() => wakeLock.setActive(room?.phase === 'playing'));

	function send(intent: Intent) {
		session.send(intent);
	}

	async function handleJoin(name: string, color: string) {
		joining = true;
		await session.join(code, name, color);
		joining = false;
	}

	// A lobby leave must free the seat through REST before the view is left: a
	// failed request keeps the seat, so navigating anyway would silently lose it.
	// In a running or finished game there is no REST call; closing the room
	// socket pauses the seat server-side and a rejoin continues it.
	async function leave() {
		if (leaving) return;
		leaving = true;
		try {
			if (room?.phase === 'lobby' && !(await session.leave(code))) return;
			session.disconnect();
			presence.disconnectRoom(code);
			await goto('/');
		} finally {
			// Teardown or navigation can fail; the guard must never stay set, or
			// the control would be disabled forever. The page may already be
			// navigating, which makes this write irrelevant.
			leaving = false;
		}
	}

	// Clicking the brand is a second way out of the room, so an open lobby must
	// send the same REST leave: SvelteKit's client-side navigation would
	// otherwise keep the seat occupied without any request.
	function handleBrandClick(event: MouseEvent) {
		// Auxclick, middle click and modified clicks open a new tab/window; keep
		// the native link behaviour and do not consume them.
		if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) {
			return;
		}
		// In a running or finished game leaving is only a view change, so the
		// plain link stays and needs no request.
		if (room?.phase !== 'lobby') return;
		event.preventDefault();
		void leave();
	}

	function adjustZoom(delta: number) {
		const next = Math.min(1.5, Math.max(0.6, Math.round((preferences.current.zoom + delta) * 10) / 10));
		preferences.update({ zoom: next });
	}
</script>

<div class="room">
	<header class="topbar">
		<div class="row">
			<a
				class="brand"
				href="/"
				aria-label="Subprime – zur Startseite"
				onclick={handleBrandClick}>Subprime</a
			>
			<code class="code">{code}</code>
			<span class="badge" class:online={session.connected} class:offline={!session.connected}>
				{session.connected ? 'verbunden' : 'getrennt'}
			</span>
			{#if room}<span class="badge">{room.phase === 'lobby' ? 'Lobby' : room.phase === 'playing' ? 'Spiel läuft' : 'Beendet'}</span>{/if}
		</div>
		<div class="row">
			<button class="plain" aria-label="Verkleinern" onclick={() => adjustZoom(-0.1)}>−</button>
			<span class="badge">{Math.round(preferences.current.zoom * 100)}%</span>
			<button class="plain" aria-label="Vergrößern" onclick={() => adjustZoom(0.1)}>+</button>
			<ThemeToggle />
			<!-- Mounted in every state so the trigger reserves the slot and "Verlassen"
			     never jumps. The Lobby hides and disables it while its slot stays
			     reserved. Fallback props keep it disabled while there is no game. -->
			<ForfeitButton
				phase={game?.phase ?? 'finished'}
				turnPhase={game?.turnPhase ?? 'finished'}
				{isMyTurn}
				{me}
				hidden={room?.phase === 'lobby'}
				onIntent={send}
			/>
			{#if room}
				<button class="plain" onclick={leave} disabled={leaving} aria-busy={leaving}>
					{leaving ? 'Verlassen …' : 'Verlassen'}
				</button>
			{/if}
		</div>
	</header>

	{#if session.error}
		<div class="error-banner">
			<span>{session.error}</span>
			<button class="plain" onclick={() => session.clearError()}>Schließen</button>
		</div>
	{/if}

	{#if !room}
		<p class="muted pad">Raum wird geladen …</p>
	{:else if room.phase === 'lobby'}
		<div class="pad">
			<Lobby
				{room}
				onIntent={send}
				onJoin={handleJoin}
				{joining}
				online={presence.online}
				sent={presence.sent}
				error={presence.error}
				onInvite={(userId) => presence.invite(userId)}
				onCancel={(inviteId) => presence.cancel(inviteId)}
				onDismissError={() => presence.clearError()}
			/>
		</div>
	{:else if game}
		<div class="game" style="zoom: {preferences.current.zoom};">
			<main class="board-area">
				<Board {game} {isMyTurn} {isDebtor} {debtor} {me} onIntent={send} />
			</main>
			<aside class="side">
				<section class="card">
					<div class="row between">
						<h3>Spieler</h3>
						{#if me}<span class="badge">{formatMoney(me.cash)}</span>{/if}
					</div>
					<PlayersList
						{game}
						{me}
						kickVotes={room.kickVotes}
						onVotekick={(userId) => send({ type: 'votekick', targetId: userId })}
					/>
				</section>

				{#if me && game.phase === 'playing'}
					<details class="card" bind:open={estateOpen}>
						<summary>Bauen und Verwalten</summary>
						<EstatePanel {game} {me} {isMyTurn} {isDebtor} onIntent={send} />
					</details>

					{#if !me.bankrupt}
						<details class="card">
							<summary>Autopilot{#if me.autopilot?.active}<span class="badge">aktiv</span>{/if}</summary>
							<AutopilotPanel {game} {me} onIntent={send} />
						</details>
					{/if}

					<details class="card" bind:open={tradeUI.panelOpen}>
						<summary>Handel</summary>
						<TradePanel {game} {me} onIntent={send} />
					</details>
				{/if}

				<section class="card">
					<h3>Ereignisprotokoll</h3>
					<EventLog events={game.events} />
				</section>
			</aside>
		</div>
		{#if game.phase === 'finished'}
			<GameOver {game} />
		{/if}
		<CardPopups {game} {me} />
		{#if game.phase === 'playing'}
			<TradePopups {game} {me} onIntent={send} />
		{/if}
		{#if me && game.phase === 'playing'}<AutopilotToast {game} {me} />{/if}
	{/if}
</div>

<style>
	.room {
		min-height: 100vh;
		display: flex;
		flex-direction: column;
	}

	.topbar {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 0.75rem;
		padding: 0.6rem 1rem;
		border-bottom: 1px solid var(--md-sys-color-outline-variant);
		background: var(--md-sys-color-surface-container);
		flex-wrap: wrap;
	}

	.brand {
		font-weight: 700;
		font-size: 16px;                                /* title-medium, tokens.css:169 */
		line-height: 24px;
		text-decoration: none;
		color: var(--md-sys-color-on-surface);
	}

	.code {
		font-size: 22px;                                /* title-large, tokens.css:168 */
		line-height: 28px;
		font-weight: 700;
		letter-spacing: 0.15em;
	}

	.badge.online {
		background: var(--cd-success-container);
		color: var(--cd-on-success-container);
	}

	.badge.offline {
		background: var(--md-sys-color-error-container);
		color: var(--md-sys-color-on-error-container);
	}

	.pad {
		padding: 1rem;
	}

	.game {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(300px, 380px);
		gap: 0.75rem;
		padding: 0.75rem 1rem 1rem;
		align-items: start;
		/* Keep the board and the side panel next to each other instead of
		   leaving a gap between them on wide screens. */
		max-width: calc(min(calc(100vh - 5.5rem), 1320px) + 380px + 0.75rem + 2rem);
		width: 100%;
		margin: 0 auto;
	}

	.board-area {
		min-width: 0;
		overflow: auto;
	}

	.side {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}

	.side .card {
		margin-top: 0;
	}

	.between {
		justify-content: space-between;
	}

	details summary {
		cursor: pointer;
		font-weight: 600;
		font-size: 16px;                                /* title-medium metrics, tokens.css:169 */
		line-height: 24px;
		color: var(--md-sys-color-on-surface);
	}

	details summary .badge {
		margin-left: 0.5rem;
		vertical-align: middle;
	}

	@media (max-width: 1100px) {
		.game {
			grid-template-columns: minmax(0, 1fr);
			max-width: calc(min(calc(100vh - 5.5rem), 1320px) + 2rem);
		}
	}
</style>
