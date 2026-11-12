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
				<li>
					<span class="name">{user.name}</span>
					{#if sent.some((invite) => invite.userId === user.userId)}
						<span class="badge">eingeladen</span>
					{:else}
						<button
							class="plain"
							aria-label={`${user.name} einladen`}
							onclick={() => onInvite(user.userId)}>Einladen</button
						>
					{/if}
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
					{#if invite.fromId !== room.you.id}
						<span class="muted">von {invite.fromName}</span>
					{/if}
					{#if invite.fromId === room.you.id}
						<button
							class="plain"
							aria-label={`Einladung an ${invite.name} abbrechen`}
							onclick={() => onCancel(invite.id)}>Abbrechen</button
						>
					{/if}
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
</style>
