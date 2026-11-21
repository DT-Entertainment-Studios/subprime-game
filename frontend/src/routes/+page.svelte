<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { session } from '#lib/session.svelte';
	import { isNamePending, presence } from '#lib/presence.svelte';
	import { preferences } from '#lib/prefs.svelte';
	import type { Invitation } from '#lib/types';
	import ThemeToggle from '#lib/components/ThemeToggle.svelte';
	import ColorSelect from '#lib/components/ColorSelect.svelte';
	import Invitations from '#lib/components/Invitations.svelte';
	import ActiveRoomBanner from '#lib/components/ActiveRoomBanner.svelte';

	let name = $state('');
	let nameInput = $state<HTMLInputElement | null>(null);
	let color = $state(preferences.current.color);
	let joinCode = $state('');
	let abandoning = $state(false);

	onMount(() => {
		let active = true;
		void session.init().then(() => {
			if (!active) return;
			if (session.user && !name) name = session.user.name;
			// The server always starts with the identity name, so the baseline is
			// the login name and never an unconfirmed draft from the field.
			if (session.user) presence.connectStart(session.user.name);
		});
		// The banner is REST-based (no push), so a tab that regains focus picks up
		// a membership that changed elsewhere.
		void session.refreshActiveRoom();
		const refresh = () => void session.refreshActiveRoom();
		const onVisibility = () => {
			if (document.visibilityState === 'visible') refresh();
		};
		window.addEventListener('focus', refresh);
		document.addEventListener('visibilitychange', onVisibility);
		return () => {
			active = false;
			window.removeEventListener('focus', refresh);
			document.removeEventListener('visibilitychange', onVisibility);
			presence.disconnectStart();
		};
	});

	// Publishing is explicit (button/Enter); typing never reaches the store.
	function commit() {
		presence.commitName(name);
		// The button disables itself after the commit, so it would drop the focus.
		nameInput?.focus();
	}

	async function create() {
		preferences.update({ color });
		await session.create(name, color);
		if (session.room) await goto(`/room/${session.room.code}`);
	}

	async function join() {
		if (!joinCode.trim()) return;
		preferences.update({ color });
		await session.join(joinCode.trim().toUpperCase(), name, color);
		if (session.room) await goto(`/room/${session.room.code}`);
	}

	// "Wieder beitreten" is a plain navigation: the room page's load/reconnect
	// flow restores the seat.
	function rejoin() {
		if (session.activeRoom) void goto(`/room/${session.activeRoom.code}`);
	}

	async function abandon() {
		if (abandoning) return;
		abandoning = true;
		try {
			await session.abandonActiveRoom();
		} finally {
			abandoning = false;
		}
	}

	async function accept(invite: Invitation) {
		try {
			const room = await presence.accept(invite.id, name || session.user?.name || '', color);
			preferences.update({ color });
			session.room = room;
			await goto(`/room/${room.code}`);
		} catch {
			// The store already set `presence.error`; the banner below shows it.
		}
	}
</script>

<div class="home">
	<header class="topbar">
		<strong class="brand">Subprime</strong>
		<ThemeToggle />
	</header>

	<main class="content">
		<section class="hero">
			<h1>Subprime</h1>
			<p class="muted">
				Das selbst gehostete Online-Brettspiel für Immobilienhaie: klassisches 40-Felder-Brett,
				echtes Geld (in GE) und die Subprime-Hausregeln – Nutzungsanteil für die Versorger,
				doppelter Preis für die letzte Straße einer Farbgruppe und keine krummen Deals.
			</p>
			{#if session.user}
				<p class="badge">Angemeldet als {session.user.name}</p>
			{/if}
		</section>

		{#if session.error}
			<div class="error-banner">
				<span>{session.error}</span>
				<button class="plain" onclick={() => session.clearError()}>Schließen</button>
			</div>
		{/if}

		{#if presence.error}
			<div class="error-banner">
				<span>{presence.error}</span>
				<button class="plain" onclick={() => presence.clearError()}>Schließen</button>
			</div>
		{/if}

		<Invitations
			invites={presence.invites}
			notice={presence.notice}
			onAccept={accept}
			onDecline={(invite) => presence.decline(invite.id)}
			onDismissNotice={() => presence.dismissNotice()}
		/>

		<section class="card">
			<h2 id="name-heading">Anzeigename</h2>
			<div class="name-row">
				<input
					id="name"
					type="text"
					maxlength="24"
					aria-labelledby="name-heading"
					bind:this={nameInput}
					bind:value={name}
					placeholder="Dein Name"
					onkeydown={(event) => {
						// During an IME composition (CJK etc.) Enter confirms the
						// composition, so it must not commit the half-formed text.
						if (event.isComposing || event.key !== 'Enter') return;
						event.preventDefault();
						commit();
					}}
				/>
				<button
					type="button"
					class="plain"
					disabled={!isNamePending(name, presence.publishedName)}
					onclick={commit}
				>
					Namen übernehmen
				</button>
			</div>
		</section>

		{#if session.activeRoom}
			<ActiveRoomBanner
				room={session.activeRoom}
				leaving={abandoning}
				onRejoin={rejoin}
				onAbandon={abandon}
			/>
		{/if}

		<div class="columns">
			<section class="card">
				<h2>Neuen Raum eröffnen</h2>
				<div class="stack">
					<div>
						<label for="color">Spielfarbe</label>
						<ColorSelect id="color" bind:value={color} />
					</div>
					<button
						class="cta"
						disabled={session.loading || session.activeRoom !== null}
						onclick={create}
					>
						Raum erstellen
					</button>
				</div>
			</section>

			<section class="card">
				<h2>Raum beitreten</h2>
				<div class="stack">
					<div>
						<label for="code">Raumcode</label>
						<input
							id="code"
							type="text"
							maxlength="6"
							placeholder="ABCDEF"
							style="text-transform: uppercase; letter-spacing: 0.15em;"
							bind:value={joinCode}
						/>
					</div>
					<button
						class="primary"
						disabled={session.loading || session.activeRoom !== null || !joinCode.trim()}
						onclick={join}
					>
						Beitreten
					</button>
				</div>
			</section>
		</div>

		<section class="card">
			<h2>So läuft es</h2>
			<ul class="hints">
				<li>2–8 Spieler pro Raum, Startgeld 1500 GE, zufällige Sitzordnung.</li>
				<li>Vor dem Würfeln bauen und handeln, danach Feld auswerten.</li>
				<li>Abgelehnte Felder werden versteigert (Gebote in 20er-Schritten).</li>
				<li>Karten zahlen an die Bank; nur zwei Klassiker bewegen Geld zwischen Spielern.</li>
				<li>Auktionsgebote dürfen bis zu deinem Liquidationswert gehen (Bargeld plus Verkaufs- und Hypothekenwerte). Gewinnst du über dein Bargeld hinaus, stehst du sofort in der Schuldenphase.</li>
				<li>Schulden blockieren das Spiel für alle, auch wenn der Schuldner nicht am Zug ist. Er liquidiert Felder, bis das Konto wieder stimmt; reicht der Liquidationswert nicht, scheidet er automatisch aus. Aufgeben geht bei Schulden über das Schuldenpanel, sonst im eigenen Zug über die Kopfzeile – jeweils mit Bestätigung.</li>
				<li>Wer ausscheidet, ist raus – wer zuletzt übrig bleibt, gewinnt.</li>
			</ul>
		</section>
	</main>
</div>

<style>
	.topbar {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 0.6rem 1rem;
		border-bottom: 1px solid var(--md-sys-color-outline-variant);
		background: var(--md-sys-color-surface-container);
	}

	.brand {
		font-weight: 700;
		font-size: 16px; /* title-medium metrics, tokens.css:169 */
		line-height: 24px;
	}

	.content {
		max-width: 62rem;
		margin: 0 auto;
		padding: 1.5rem 1rem 3rem;
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}

	/* .content already spaces its cards with `gap`; the global .card + .card
	   margin would double that gap before the name card when invites show. */
	.content > .card {
		margin-top: 0;
	}

	.hero h1 {
		font-size: 36px; /* display-small, tokens.css:164 */
		line-height: 44px;
		font-weight: 400;
	}

	.columns {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
		gap: 1rem;
	}

	/* Input grows, the commit button sits beside it on wide screens and wraps below on narrow. */
	.name-row {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.75rem;
		max-width: 30rem;
	}

	.name-row input {
		flex: 1 1 12rem;
		min-width: 0;
	}

	.name-row button {
		flex: 0 0 auto;
	}

	.hints {
		margin: 0;
		padding-left: 1.2rem;
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
	}
</style>
