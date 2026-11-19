<script lang="ts">
	import type { Invitation } from '#lib/types';

	let {
		invites,
		notice,
		onAccept,
		onDecline,
		onDismissNotice
	}: {
		invites: Invitation[];
		notice: Invitation | null;
		onAccept: (invite: Invitation) => void;
		onDecline: (invite: Invitation) => void;
		onDismissNotice: () => void;
	} = $props();
</script>

{#if notice || invites.length > 0}
	{#if notice}
		<div class="notice" role="status">
			<span>Neue Einladung von {notice.fromName} in Raum {notice.room}.</span>
			<button class="plain" onclick={onDismissNotice}>Schließen</button>
		</div>
	{/if}

	{#if invites.length > 0}
		<section class="card">
			<h2>Einladungen</h2>
			<ul class="invites">
				{#each invites as invite (invite.id)}
					<li>
						<div class="head">
							<strong>Raum {invite.room}</strong>
							<span class="muted">von {invite.fromName}</span>
						</div>
						<div class="row">
							<button
								class="plain"
								aria-label={`Einladung nach ${invite.room} ablehnen`}
								onclick={() => onDecline(invite)}>Ablehnen</button
							>
							<button
								class="primary"
								aria-label={`Einladung nach ${invite.room} annehmen`}
								onclick={() => onAccept(invite)}>Annehmen</button
							>
						</div>
					</li>
				{/each}
			</ul>
		</section>
	{/if}
{/if}

<style>
	.notice {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.75rem;
		padding: 0.5rem 0.75rem;
		border-radius: var(--md-sys-shape-corner-small);
		background: var(--cd-info-container);
		color: var(--cd-on-info-container);
	}

	.invites {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0.35rem;
	}

	.invites li {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.75rem;
		flex-wrap: wrap;
		padding: 0.3rem 0.4rem;
		border-radius: var(--md-sys-shape-corner-small);
		background: var(--md-sys-color-surface);
		border: 1px solid var(--md-sys-color-outline-variant);
	}

	.head {
		display: flex;
		align-items: baseline;
		gap: 0.5rem;
		flex-wrap: wrap;
	}
</style>
