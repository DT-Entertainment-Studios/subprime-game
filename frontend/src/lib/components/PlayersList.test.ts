import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, within } from '@testing-library/svelte';
import PlayersList from './PlayersList.svelte';
import type { GameState, Player } from '#lib/types';

function player(id: string, name: string, seat: number, overrides: Partial<Player> = {}): Player {
	return {
		id,
		name,
		color: 'blau',
		seat,
		cash: 1500,
		liquidationValue: 1500,
		position: 0,
		inJail: false,
		jailAttempts: 0,
		jailCards: 0,
		bankrupt: false,
		connected: true,
		netWorth: 1500,
		stats: {
			rentCollected: 0,
			rentPaid: 0,
			propertiesBought: 0,
			propertiesSold: 0,
			housesBuilt: 0,
			trades: 0
		},
		...overrides
	};
}

function game(phase: GameState['phase'] = 'playing'): GameState {
	return {
		phase,
		rules: {
			minPlayers: 2,
			maxPlayers: 8,
			startingCash: 1500,
			losSalary: 200,
			jailFine: 50,
			auctionSeconds: 15
		},
		board: [],
		players: [
			player('anna', 'Anna', 0),
			player('ben', 'Ben', 1, { connected: false }),
			player('carla', 'Carla', 2)
		],
		tiles: [],
		current: 0,
		debtor: -1,
		turnPhase: phase === 'finished' ? 'finished' : 'awaiting_roll',
		dice: [3, 4],
		doubles: 0,
		trades: [],
		events: [],
		winner: -1
	};
}

const me = () => player('anna', 'Anna', 0);

function votekickButton(name: string): HTMLElement | null {
	return screen.queryByRole('button', { name: `Votekick gegen ${name}` });
}

describe('PlayersList votekick', () => {
	it('offers a votekick for a paused opponent and reports the target', async () => {
		const onVotekick = vi.fn();
		render(PlayersList, { game: game(), me: me(), kickVotes: [], onVotekick });

		const button = votekickButton('Ben');
		expect(button).not.toBeNull();
		expect(button).toHaveTextContent('Votekick 0/2');
		expect(button).toHaveAttribute('aria-label', 'Votekick gegen Ben');

		await fireEvent.click(button as HTMLElement);
		expect(onVotekick).toHaveBeenCalledWith('ben');
	});

	it('shows the tally from the snapshot while a vote is pending', () => {
		render(PlayersList, {
			game: game(),
			me: me(),
			kickVotes: [{ userId: 'ben', votes: 1, voted: false }],
			onVotekick: () => {}
		});

		expect(votekickButton('Ben')).toHaveTextContent('Votekick 1/2');
	});

	it('shows an already voted badge instead of a second button', () => {
		render(PlayersList, {
			game: game(),
			me: me(),
			kickVotes: [{ userId: 'ben', votes: 1, voted: true }],
			onVotekick: () => {}
		});

		expect(screen.getByText('Votekick 1/2 · abgestimmt')).toBeInTheDocument();
		expect(votekickButton('Ben')).toBeNull();
	});

	it('offers no votekick for a connected player', () => {
		render(PlayersList, { game: game(), me: me(), kickVotes: [], onVotekick: () => {} });

		expect(votekickButton('Carla')).toBeNull();
		expect(votekickButton('Ben')).not.toBeNull();
	});

	it('offers no votekick for a bankrupt player', () => {
		const state = game();
		state.players[1].bankrupt = true;

		render(PlayersList, { game: state, me: me(), kickVotes: [], onVotekick: () => {} });

		expect(votekickButton('Ben')).toBeNull();
		expect(screen.queryByRole('button', { name: /Votekick gegen/ })).toBeNull();
	});

	it('offers no votekick against the viewer', () => {
		const state = game();
		state.players[0].connected = false;
		const viewer = state.players[0];

		render(PlayersList, { game: state, me: viewer, kickVotes: [], onVotekick: () => {} });

		expect(votekickButton('Anna')).toBeNull();
		expect(votekickButton('Ben')).not.toBeNull();
	});

	it('offers no votekick in a finished game', () => {
		render(PlayersList, {
			game: game('finished'),
			me: me(),
			kickVotes: [],
			onVotekick: () => {}
		});

		expect(screen.queryByRole('button', { name: /Votekick gegen/ })).toBeNull();
	});

	it('does not reserve a vote slot for players who cannot vote', () => {
		render(PlayersList, { game: game(), me: me(), kickVotes: [], onVotekick: () => {} });

		const row = (name: string) => screen.getByText(name).closest('li') as HTMLElement;
		expect(row('Anna').querySelector('.vote-slot')).toBeNull();
		expect(row('Carla').querySelector('.vote-slot')).toBeNull();
		expect(row('Ben').querySelector('.vote-slot')).not.toBeNull();
	});

	it('keeps both votekick states mounted in one stable slot', () => {
		render(PlayersList, {
			game: game(),
			me: me(),
			kickVotes: [],
			onVotekick: () => {}
		});

		const ben = screen.getByText('Ben').closest('li') as HTMLElement;
		const button = within(ben).getByRole('button', { name: 'Votekick gegen Ben' });
		const badge = within(ben).getByText(/Votekick 0\/2 · abgestimmt/);
		expect(button).not.toHaveClass('slot-hidden');
		expect(badge).toHaveAttribute('aria-hidden', 'true');
		expect(button.parentElement).toBe(badge.parentElement);
		expect(button.parentElement).toHaveClass('vote-slot');
	});

	it('swaps the votekick button for the voted badge without unmounting the slot', () => {
		render(PlayersList, {
			game: game(),
			me: me(),
			kickVotes: [{ userId: 'ben', votes: 1, voted: true }],
			onVotekick: () => {}
		});

		const ben = screen.getByText('Ben').closest('li') as HTMLElement;
		const button = within(ben).getByText('Votekick 1/2');
		const badge = within(ben).getByText('Votekick 1/2 · abgestimmt');
		expect(button).toHaveClass('slot-hidden');
		expect(button).toBeDisabled();
		expect(button).toHaveAttribute('aria-hidden', 'true');
		expect(badge).not.toHaveAttribute('aria-hidden');
		expect(button.parentElement).toBe(badge.parentElement);
	});
});

describe('PlayersList autopilot badge', () => {
	it('shows the badge for the viewer’s own active seat', () => {
		const state = game();
		state.players[0].autopilot = { active: true };

		render(PlayersList, { game: state, me: me(), kickVotes: [], onVotekick: () => {} });

		expect(screen.getByText('Anna').closest('li')).toContainElement(
			screen.getByText('Autopilot')
		);
		expect(screen.getByTitle('Autopilot aktiv')).toBeInTheDocument();
	});

	it('shows the badge for another active seat', () => {
		const state = game();
		state.players[2].autopilot = { active: true };

		render(PlayersList, { game: state, me: me(), kickVotes: [], onVotekick: () => {} });

		expect(screen.getByText('Carla').closest('li')).toContainElement(
			screen.getByText('Autopilot')
		);
		expect(screen.getByText('Anna').closest('li')).not.toContainElement(
			screen.queryByTitle('Autopilot aktiv')
		);
	});

	it('shows the badge in the spectator view', () => {
		const state = game();
		state.players[1].autopilot = { active: true };

		render(PlayersList, { game: state, me: null, kickVotes: [], onVotekick: () => {} });

		expect(screen.getByText('Ben').closest('li')).toContainElement(
			screen.getByText('Autopilot')
		);
	});

	it('shows no badge for inactive or bankrupt seats', () => {
		const state = game();
		state.players[0].autopilot = { active: false };
		state.players[1].bankrupt = true;

		render(PlayersList, { game: state, me: me(), kickVotes: [], onVotekick: () => {} });

		expect(screen.queryByText('Autopilot')).toBeNull();
		expect(screen.queryByTitle('Autopilot aktiv')).toBeNull();
	});
});

describe('PlayersList current player', () => {
	it('marks only the current seat while playing', () => {
		const state = game();
		state.current = 1;

		render(PlayersList, { game: state, me: me(), kickVotes: [], onVotekick: () => {} });

		expect(screen.getByText('Ben').closest('li')).toHaveClass('current');
		expect(screen.getByText('Anna').closest('li')).not.toHaveClass('current');
	});

	it('marks no seat when the game is finished', () => {
		const state = game('finished');
		state.current = 1;

		render(PlayersList, { game: state, me: me(), kickVotes: [], onVotekick: () => {} });

		expect(document.querySelectorAll('.player.current')).toHaveLength(0);
	});
});

describe('PlayersList bankruptcy badge', () => {
	it('shows a pleite badge with cause and amounts tooltip', () => {
		const state = game();
		state.players[1].bankrupt = true;
		state.events = [
			{
				seq: 1,
				type: 'bankruptcy',
				player: 1,
				cause: 'miete',
				debt: 420,
				realizable: 180,
				shortfall: 240,
				text: 'Ben ist pleite. Miete an Berta: 420 GE. Realisierbar: 180 GE. Fehlbetrag: 240 GE.',
				at: '2026-01-01T00:00:00Z'
			}
		];

		render(PlayersList, { game: state, me: me() });

		const badge = screen.getByText('pleite');
		expect(badge.closest('li')).toContainElement(screen.getByText('Ben'));
		const title = badge.getAttribute('title') ?? '';
		expect(title).toContain('Miete');
		expect(title).toContain('420 GE');
		expect(title).toContain('240 GE');
	});

	it('keeps a generic tooltip for legacy bankruptcies', () => {
		const state = game();
		state.players[1].bankrupt = true;
		state.events = [
			{ seq: 1, type: 'bankruptcy', player: 1, text: 'Ben scheidet aus.', at: '2026-01-01T00:00:00Z' }
		];

		render(PlayersList, { game: state, me: me() });

		expect(screen.getByText('pleite')).toHaveAttribute('title', 'Aus dem Spiel ausgeschieden');
	});
});

describe('PlayersList jail-card chip', () => {
	it('titles the chip with Pannenhilfen and shows the count', () => {
		const state = game();
		state.players[0].jailCards = 2;

		render(PlayersList, { game: state, me: me(), kickVotes: [], onVotekick: () => {} });

		expect(screen.getByTitle('Pannenhilfen')).toHaveTextContent('2');
	});
});
