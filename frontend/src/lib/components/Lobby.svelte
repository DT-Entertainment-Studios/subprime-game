<script lang="ts">
	import { untrack } from 'svelte';
	import type { Intent, OnlineUser, RoomSnapshot, SentInvite } from '#lib/types';
	import { tokenColor } from '#lib/format';
	import ColorSelect from '#lib/components/ColorSelect.svelte';
	import LobbyInvites from '#lib/components/LobbyInvites.svelte';

	let {
		room,
		onIntent,
		onJoin,
		joining = false,
		online,
		sent,
		error,
		onInvite,
		onCancel,
		onDismissError
	}: {
		room: RoomSnapshot;
		onIntent: (intent: Intent) => void;
		onJoin: (name: string, color: string) => void;
		joining?: boolean;
		online: OnlineUser[];
		sent: SentInvite[];
		error: string | null;
		onInvite: (userId: string) => void;
		onCancel: (inviteId: string) => void;
		onDismissError: () => void;
	} = $props();

	let name = $state(untrack(() => room.you.name ?? ''));
	let color = $state(untrack(() => room.seats.find((s) => s.userId === room.you.id)?.color ?? 'rot'));
	let joinName = $state('');
	let joinColor = $state('blau');

	const shareLink = $derived(
		typeof window === 'undefined' ? `/room/${room.code}` : `${window.location.origin}/room/${room.code}`
	);
	const isMember = $derived(room.you.role === 'player');

	async function copyLink() {
		try {
			await navigator.clipboard.writeText(shareLink);
		} catch {
			// Clipboard is optional.
		}
	}

	const rules: [string, string][] = [
		['Spieler', '2–8 pro Raum'],
		['Startgeld', '1500 GE'],
		['Sitzordnung', 'zufällig vom Server'],
		['Los-Gehalt', '200 GE, kein Doppel beim Landen'],
		['Steuern und Kartenzahlungen', 'an die Bank (außer den zwei Klassikern)'],
		['Karten', '16 Ereignis- und 16 Gemeinschaftskarten'],
		['Auktion', 'abgelehntes Feld wird versteigert, Gebote in 20er-Schritten bis zum Liquidationswert (Verkauf und Hypotheken zählen mit)'],
		['Farbgruppe komplett', 'doppelter Preis beim Kauf, 120 % des Gebots in der Auktion'],
		['Miete in der Werkstatt', 'Besitzer zieht keine Miete, Mieter zahlt nichts'],
		['Unbebaute Miete', 'bei kompletter Farbgruppe verdoppelt'],
		['Versorger-Miete', '4× Würfel, 10× bei beiden Werken'],
		['Nutzungsanteil', '10 % der rosa/orange Miete an das Elektrizitätswerk, 10 % der rot/gelb Miete an das Wasserwerk'],
		['Hypothek', '50 % Darlehen, 60 % Ablösung, nur bei unbebauter Gruppe'],
		['Verkauf an die Bank', '70 % des Kaufpreises; hypothekiert 0 GE'],
		['Bauen', 'Stufen einer Gruppe höchstens 1 auseinander'],
		['Handel', 'vor dem Würfeln, nur unbebaute Felder, ohne Bewertung'],
		['Schulden', 'Zahlungen dürfen ins Minus und blockieren sofort alle; der Schuldner liquidiert, auch wenn er nicht am Zug ist – danach läuft der Zug weiter'],
		['Pleite', 'deckt der komplette Liquidationswert die Schuld nicht, scheidet der Spieler automatisch aus'],
		['Aufgeben', 'im eigenen Zug oder bei Schulden über das Schuldenpanel (auch fremd), mit Bestätigung – Felder zurück an die Bank'],
		['Hotel', 'zählt als fünf Häuser'],
		['Kartenketten', 'unbegrenzt'],
		['Krumme Deals', 'verboten, aber nicht geprüft']
	];
</script>

<div class="lobby">
	<section class="card">
		<h2>Lobby {room.code}</h2>
		<p class="muted">Teile den Code oder den Link, damit weitere Spieler beitreten können.</p>
		<div class="row">
			<code class="code">{room.code}</code>
			<button class="plain" onclick={copyLink}>Link kopieren</button>
		</div>
	</section>

	<section class="card">
		<h3>Sitzplätze ({room.seats.length}/8)</h3>
		<ul class="seats">
			{#each room.seats as seat (seat.userId)}
				<li>
					<span class="dot" style="background: {tokenColor(seat.color)}"></span>
					<span>{seat.name}</span>
					{#if seat.host}<span class="badge">Gastgeber</span>{/if}
					{#if seat.userId === room.you.id}<span class="badge">Du</span>{/if}
					{#if !seat.connected}<span class="badge">offline</span>{/if}
				</li>
			{/each}
		</ul>
	</section>

	{#if isMember}
		<section class="card">
			<h3>Dein Platz</h3>
			<div class="row">
				<div class="field">
					<label for="lobby-name">Name</label>
					<input id="lobby-name" type="text" maxlength="24" bind:value={name} />
				</div>
				<div class="field">
					<label for="lobby-color">Farbe</label>
					<ColorSelect id="lobby-color" bind:value={color} />
				</div>
				<button class="plain" onclick={() => onIntent({ type: 'set_profile', name, color })}>
					Übernehmen
				</button>
			</div>
			{#if room.you.isHost}
				<div class="row start">
					<button class="cta" disabled={!room.canStart} onclick={() => onIntent({ type: 'start_game' })}>
						Spiel starten
					</button>
					{#if !room.canStart}
						<span class="muted">Mindestens zwei Spieler nötig.</span>
					{/if}
				</div>
			{:else}
				<p class="muted">Warte, bis der Gastgeber startet.</p>
			{/if}
		</section>

		<LobbyInvites {room} {online} {sent} {error} {onInvite} {onCancel} {onDismissError} />
	{:else}
		<section class="card">
			<h3>Beitreten</h3>
			<div class="row">
				<div class="field">
					<label for="join-name">Name</label>
					<input id="join-name" type="text" maxlength="24" bind:value={joinName} />
				</div>
				<div class="field">
					<label for="join-color">Farbe</label>
					<ColorSelect id="join-color" bind:value={joinColor} />
				</div>
				<button class="cta" disabled={joining} onclick={() => onJoin(joinName, joinColor)}>
					Platz nehmen
				</button>
			</div>
		</section>
	{/if}

	<section class="card">
		<h3>Regelwerk v1.0</h3>
		<table>
			<tbody>
				{#each rules as [rule, value] (rule)}
					<tr><th>{rule}</th><td>{value}</td></tr>
				{/each}
			</tbody>
		</table>
	</section>
</div>

<style>
	.lobby {
		display: grid;
		gap: 1rem;
		grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
		align-items: start;
	}

	.code {
		font-size: 22px;                                /* title-large, tokens.css:168 */
		line-height: 28px;
		letter-spacing: 0.2em;
		font-weight: 700;
	}

	.seats {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0.35rem;
	}

	.seats li {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.3rem 0.4rem;
		border-radius: var(--md-sys-shape-corner-small);
		background: var(--md-sys-color-surface);
		border: 1px solid var(--md-sys-color-outline-variant);
	}

	.dot {
		width: 0.9rem;
		height: 0.9rem;
		border-radius: 50%;
		border: 1px solid var(--cd-ral9016);
	}

	.field {
		min-width: 10rem;
	}

	.start {
		margin-top: 0.75rem;
	}

	table {
		width: 100%;
		border-collapse: collapse;
		font-size: 12px;                                /* body-small, tokens.css:173 */
		line-height: 16px;
	}

	th,
	td {
		text-align: left;
		padding: 0.25rem 0.4rem;
		border-bottom: 1px solid var(--md-sys-color-outline-variant);
		vertical-align: top;
	}

	th {
		white-space: nowrap;
		color: var(--md-sys-color-on-surface-variant);
		font-weight: 500;
	}
</style>
