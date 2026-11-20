<script lang="ts">
	import type { OnlineUser, RoomSnapshot, SentInvite } from '#lib/types';

	let {
		room,
		online,
		sent,
		error,
		onInvite,
		onCancel,
		onDismissError
	}: {
		room: RoomSnapshot;
		online: OnlineUser[];
		sent: SentInvite[];
		error: string | null;
		onInvite: (userId: string) => void;
		onCancel: (inviteId: string) => void;
		onDismissError: () => void;
	} = $props();
</script>

{#if error}
	<div class="error-banner">
		<span>{error}</span>
		<button class="plain" onclick={onDismissError}>Schließen</button>
	</div>
{/if}

<section class="card">
	<h3>In der Lobby online</h3>
	{#if online.length === 0}
		<p class="muted">Gerade ist niemand sonst auf der Startseite.</p>
	{:else}
		<ul class="people">
			{#each online as user (user.userId)}
				{@const invited = sent.some((entry) => entry.userId === user.userId)}
				<li>
					<span class="name">{user.name}</span>
					<!-- Button and badge share one cell; the wider state fixes the width. -->
					<span class="invite-slot">
						<span
							class="badge"
							class:slot-hidden={!invited}
							aria-hidden={invited ? undefined : 'true'}
						>eingeladen</span
						>
						<button
							class="plain"
							class:slot-hidden={invited}
							disabled={invited}
							aria-hidden={invited ? 'true' : undefined}
							tabindex={invited ? -1 : undefined}
							aria-label={`${user.name} einladen`}
							onclick={() => onInvite(user.userId)}>Einladen</button
						>
					</span>
				</li>
			{/each}
		</ul>
	{/if}
</section>

{#if sent.length > 0}
	<section class="card">
		<h3>Gesendete Einladungen</h3>
		<ul class="people">
			{#each sent as invite (invite.id)}
				<li>
					<span class="name">{invite.name}</span>
					<!-- Sender text and cancel button share one cell so no sibling moves. -->
					<span class="invite-slot">
						<span
							class="muted"
							class:slot-hidden={invite.fromId === room.you.id}
							aria-hidden={invite.fromId === room.you.id ? 'true' : undefined}
						>von {invite.fromName}</span
						>
						<button
							class="plain"
							class:slot-hidden={invite.fromId !== room.you.id}
							disabled={invite.fromId !== room.you.id}
							aria-hidden={invite.fromId !== room.you.id ? 'true' : undefined}
							tabindex={invite.fromId !== room.you.id ? -1 : undefined}
							aria-label={`Einladung an ${invite.name} abbrechen`}
							onclick={() => onCancel(invite.id)}>Abbrechen</button
						>
					</span>
				</li>
			{/each}
		</ul>
	</section>
{/if}

<style>
	.people {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0.35rem;
	}

	.people li {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		flex-wrap: wrap;
		padding: 0.3rem 0.4rem;
		border-radius: var(--md-sys-shape-corner-small);
		background: var(--md-sys-color-surface);
		border: 1px solid var(--md-sys-color-outline-variant);
	}

	.name {
		flex: 1 1 auto;
	}

	/* The two states of each action share one cell so swaps never move siblings. */
	.invite-slot {
		display: inline-grid;
		align-items: center;
	}

	.invite-slot > * {
		grid-area: 1 / 1;
	}

	.slot-hidden {
		visibility: hidden;
	}
</style>
