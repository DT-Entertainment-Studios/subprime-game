import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, within } from '@testing-library/svelte';
import AutopilotPanel from './AutopilotPanel.svelte';
import type { AutopilotSettings, GameEvent, GameState, Player } from '#lib/types';

const board = [
	{ index: 0, name: 'Los', type: 'go' },
	{
		index: 1,
		name: 'Zollgasse',
		type: 'street',
		group: 'braun',
		price: 60,
		rents: [2, 10, 30, 90, 160, 250],
		build: 50,
		sell: 42
	},
	{
		index: 2,
		name: 'Münzgasse',
		type: 'street',
		group: 'braun',
		price: 60,
		rents: [4, 20, 60, 180, 320, 450],
		build: 50,
		sell: 42
	},
	{
		index: 3,
		name: 'Seestraße',
		type: 'street',
		group: 'hellblau',
		price: 100,
		rents: [6, 30, 90, 270, 400, 550],
		build: 50,
		sell: 50
	},
	{
		index: 4,
		name: 'Hafenstraße',
		type: 'street',
		group: 'hellblau',
		price: 100,
		rents: [6, 30, 90, 270, 400, 550],
		build: 50,
		sell: 50
	}
] as GameState['board'];

function player(seat: number, overrides: Partial<Player> = {}): Player {
	return {
		id: `user-${seat}`,
		name: seat === 0 ? 'Anton' : 'Berta',
		color: seat === 0 ? 'blau' : 'rot',
		seat,
		cash: 1500,
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

function config(overrides: Partial<AutopilotSettings> = {}): AutopilotSettings {
	return {
		enabled: false,
		buyAlways: true,
		jail: 'none',
		buildGroups: [],
		liquidity: 0,
		hideCards: false,
		...overrides
	};
}

function me(overrides: Partial<Player> = {}, settings: Partial<AutopilotSettings> = {}): Player {
	return player(0, { autopilot: { active: false, config: config(settings) }, ...overrides });
}

function game(overrides: Partial<GameState> = {}): GameState {
	return {
		phase: 'playing',
		rules: {
			minPlayers: 2,
			maxPlayers: 8,
			startingCash: 1500,
			losSalary: 200,
			jailFine: 50,
			auctionSeconds: 15
		},
		board,
		players: [player(0), player(1)],
		tiles: [
			{ owner: -1, houses: 0, mortgaged: false },
			{ owner: 0, houses: 0, mortgaged: false },
			{ owner: 0, houses: 0, mortgaged: false },
			{ owner: 0, houses: 0, mortgaged: false },
			{ owner: 1, houses: 0, mortgaged: false }
		],
		current: 0,
		turnPhase: 'awaiting_roll',
		dice: [3, 4],
		doubles: 0,
		trades: [],
		events: [],
		winner: -1,
		...overrides
	};
}

function stopEvent(seq: number, playerSeat: number, reason: string): GameEvent {
	return {
		seq,
		type: 'autopilot_off',
		player: playerSeat,
		reason,
		text: 'Autopilot beendet.',
		at: '2026-01-01T12:00:00Z'
	};
}

/** The emphasised main-switch row; the state pill is its trailing sibling. */
function powerRow(): HTMLElement {
	return screen.getByRole('switch', { name: 'Autopilot aktiv' }).closest('.power') as HTMLElement;
}

describe('AutopilotPanel controls', () => {
	it('renders the switch, buy, jail, build and liquidity controls', () => {
		render(AutopilotPanel, { game: game(), me: me(), onIntent: vi.fn() });

		expect(screen.getByLabelText('Autopilot aktiv')).toBeInTheDocument();
		expect(screen.getByRole('switch', { name: 'Autopilot aktiv' })).toBeInTheDocument();
		expect(within(powerRow()).getByText('Aus')).toBeInTheDocument();
		expect(within(powerRow()).getByText('Aus')).toHaveAttribute('aria-hidden', 'true');
		expect(screen.getByLabelText('Immer kaufen')).toBeInTheDocument();
		expect(screen.getByLabelText('Keine Karten anzeigen')).toBeInTheDocument();
		expect(
			screen.getByText(
				'Nur Ereignis- und Gemeinschaftskarten, und nur solange der Autopilot aktiv ist.'
			)
		).toBeInTheDocument();
		expect(screen.getByRole('radio', { name: 'Aus' })).toBeInTheDocument();
		expect(screen.getByRole('radio', { name: 'Freikaufen' })).toBeInTheDocument();
		expect(screen.getByRole('radio', { name: 'Pasch versuchen' })).toBeInTheDocument();
		expect(screen.getByLabelText('Liquidität (Mindestrest in GE)')).toBeInTheDocument();
		expect(screen.getByRole('group', { name: 'Automatisch bauen' })).toBeInTheDocument();
	});

	it('reflects the enabled state in the switch and its state pill', () => {
		const enabled = render(AutopilotPanel, {
			game: game(),
			me: me({}, { enabled: true }),
			onIntent: vi.fn()
		});

		expect(screen.getByRole('switch', { name: 'Autopilot aktiv' })).toBeChecked();
		expect(within(powerRow()).getByText('Aktiv')).toBeInTheDocument();

		enabled.unmount();
		render(AutopilotPanel, { game: game(), me: me(), onIntent: vi.fn() });

		expect(screen.getByRole('switch', { name: 'Autopilot aktiv' })).not.toBeChecked();
		expect(within(powerRow()).getByText('Aus')).toBeInTheDocument();
	});

	it('keeps the switch in step with the server when an intent is not confirmed', async () => {
		const onIntent = vi.fn();
		render(AutopilotPanel, { game: game(), me: me(), onIntent });

		const toggle = screen.getByRole('switch', { name: 'Autopilot aktiv' });
		await fireEvent.click(toggle);

		expect(onIntent).toHaveBeenCalledTimes(1);
		// The snapshot still says "off": the native state must not stay checked.
		expect(toggle).not.toBeChecked();
		expect(within(powerRow()).getByText('Aus')).toBeInTheDocument();
	});

	it('resets the native state before the intent is sent', async () => {
		let checkedDuringIntent: boolean | null = null;
		const onIntent = vi.fn(() => {
			checkedDuringIntent = (screen.getByRole('switch', { name: 'Autopilot aktiv' }) as HTMLInputElement)
				.checked;
		});
		render(AutopilotPanel, { game: game(), me: me(), onIntent });

		await fireEvent.click(screen.getByRole('switch', { name: 'Autopilot aktiv' }));

		// The reset happens before the side effect, so the intent can never be
		// sent with a stale native state.
		expect(checkedDuringIntent).toBe(false);
	});

	it('turns the switch on when the server confirms the new config', async () => {
		const onIntent = vi.fn();
		const { rerender } = render(AutopilotPanel, { game: game(), me: me(), onIntent });

		await fireEvent.click(screen.getByRole('switch', { name: 'Autopilot aktiv' }));
		expect(screen.getByRole('switch', { name: 'Autopilot aktiv' })).not.toBeChecked();

		await rerender({ game: game(), me: me({}, { enabled: true }), onIntent });

		expect(screen.getByRole('switch', { name: 'Autopilot aktiv' })).toBeChecked();
		expect(within(powerRow()).getByText('Aktiv')).toBeInTheDocument();
	});

	it('offers a checkbox only for a fully owned group', () => {
		render(AutopilotPanel, { game: game(), me: me(), onIntent: vi.fn() });

		expect(screen.getByLabelText('Braun')).toBeInTheDocument();
		expect(screen.queryByLabelText('Hellblau')).not.toBeInTheDocument();
	});

	it('shows a hint when there is no owned or configured group', () => {
		const state = game();
		state.tiles[1].owner = 1;
		state.tiles[2].owner = 1;

		render(AutopilotPanel, { game: state, me: me(), onIntent: vi.fn() });

		expect(screen.getByText('Du besitzt noch keine vollständige Farbgruppe.')).toBeInTheDocument();
	});
});

describe('AutopilotPanel intents', () => {
	it('sends the full config when the main switch is enabled', async () => {
		const onIntent = vi.fn();
		render(AutopilotPanel, {
			game: game(),
			me: me({}, { buildGroups: ['braun'], liquidity: 200 }),
			onIntent
		});

		await fireEvent.click(screen.getByLabelText('Autopilot aktiv'));

		expect(onIntent).toHaveBeenCalledTimes(1);
		expect(onIntent).toHaveBeenCalledWith({
			type: 'set_autopilot',
			autopilot: {
				hideCards: false,
				enabled: true,
				buyAlways: true,
				jail: 'none',
				buildGroups: ['braun'],
				liquidity: 200
			}
		});
	});

	it('sends the full config when the buy switch changes', async () => {
		const onIntent = vi.fn();
		render(AutopilotPanel, { game: game(), me: me({}, { enabled: true }), onIntent });

		await fireEvent.click(screen.getByLabelText('Immer kaufen'));

		expect(onIntent).toHaveBeenCalledWith({
			type: 'set_autopilot',
			autopilot: {
				hideCards: false,
				enabled: true,
				buyAlways: false,
				jail: 'none',
				buildGroups: [],
				liquidity: 0
			}
		});
	});

	it('sends hideCards in the full config when the checkbox changes', async () => {
		const onIntent = vi.fn();
		render(AutopilotPanel, { game: game(), me: me({}, { enabled: true }), onIntent });

		await fireEvent.click(screen.getByLabelText('Keine Karten anzeigen'));

		expect(onIntent).toHaveBeenCalledWith({
			type: 'set_autopilot',
			autopilot: {
				enabled: true,
				buyAlways: true,
				jail: 'none',
				buildGroups: [],
				liquidity: 0,
				hideCards: true
			}
		});
	});

	it.each([
		['Freikaufen', 'pay'],
		['Pasch versuchen', 'roll']
	])('sends the full config when %s is selected', async (label, jail) => {
		const onIntent = vi.fn();
		render(AutopilotPanel, { game: game(), me: me({}, { enabled: true }), onIntent });

		await fireEvent.click(screen.getByRole('radio', { name: label }));

		expect(onIntent).toHaveBeenCalledWith({
			type: 'set_autopilot',
			autopilot: {
				hideCards: false,
				enabled: true,
				buyAlways: true,
				jail,
				buildGroups: [],
				liquidity: 0
			}
		});
	});

	it('sends the full config when the jail option is switched off', async () => {
		const onIntent = vi.fn();
		render(AutopilotPanel, { game: game(), me: me({}, { enabled: true, jail: 'pay' }), onIntent });

		await fireEvent.click(screen.getByRole('radio', { name: 'Aus' }));

		expect(onIntent).toHaveBeenCalledWith({
			type: 'set_autopilot',
			autopilot: {
				hideCards: false,
				enabled: true,
				buyAlways: true,
				jail: 'none',
				buildGroups: [],
				liquidity: 0
			}
		});
	});

	it('adds a group in board order and deduplicated', async () => {
		const onIntent = vi.fn();
		render(AutopilotPanel, {
			game: game(),
			me: me({}, { buildGroups: ['hellblau'] }),
			onIntent
		});

		await fireEvent.click(screen.getByLabelText('Braun'));

		expect(onIntent).toHaveBeenCalledWith({
			type: 'set_autopilot',
			autopilot: {
				hideCards: false,
				enabled: false,
				buyAlways: true,
				jail: 'none',
				buildGroups: ['braun', 'hellblau'],
				liquidity: 0
			}
		});
	});

	it.each([
		['1500', 1500],
		['150000', 100000],
		['-20', 0],
		['', 0],
		['abc', 0]
	])('clamps the liquidity input %j to %i and sends the full config', async (raw, expected) => {
		const onIntent = vi.fn();
		render(AutopilotPanel, { game: game(), me: me({}, { enabled: true }), onIntent });

		await fireEvent.change(screen.getByLabelText('Liquidität (Mindestrest in GE)'), {
			target: { value: raw }
		});

		expect(onIntent).toHaveBeenCalledWith({
			type: 'set_autopilot',
			autopilot: {
				hideCards: false,
				enabled: true,
				buyAlways: true,
				jail: 'none',
				buildGroups: [],
				liquidity: expected
			}
		});
	});
});

describe('AutopilotPanel configured groups', () => {
	it('marks a complete group with a mortgaged tile as unbuildable', () => {
		const state = game();
		state.tiles[2].mortgaged = true;

		render(AutopilotPanel, {
			game: state,
			me: me({}, { buildGroups: ['braun'] }),
			onIntent: vi.fn()
		});

		expect(screen.getByLabelText(/Braun/)).toBeChecked();
		expect(screen.getByText('nicht baubar')).toBeInTheDocument();
	});

	it('marks a fully built group as unbuildable', () => {
		const state = game();
		state.tiles[1].houses = 5;
		state.tiles[2].houses = 5;

		render(AutopilotPanel, {
			game: state,
			me: me({}, { buildGroups: ['braun'] }),
			onIntent: vi.fn()
		});

		expect(screen.getByLabelText(/Braun/)).toBeChecked();
		expect(screen.getByText('nicht baubar')).toBeInTheDocument();
	});

	it('keeps a configured but unbuildable group checked and switchable', async () => {
		const onIntent = vi.fn();
		render(AutopilotPanel, {
			game: game(),
			me: me({}, { buildGroups: ['hellblau'] }),
			onIntent
		});

		const checkbox = screen.getByLabelText(/Hellblau/);
		expect(checkbox).toBeChecked();
		expect(screen.getByText('nicht baubar')).toBeInTheDocument();

		await fireEvent.click(checkbox);

		expect(onIntent).toHaveBeenCalledWith({
			type: 'set_autopilot',
			autopilot: {
				hideCards: false,
				enabled: false,
				buyAlways: true,
				jail: 'none',
				buildGroups: [],
				liquidity: 0
			}
		});
	});
});

describe('AutopilotPanel status', () => {
	it('reports an active autopilot', () => {
		render(AutopilotPanel, { game: game(), me: me({}, { enabled: true }), onIntent: vi.fn() });

		expect(screen.getByText('Autopilot aktiv.')).toBeInTheDocument();
	});

	it('reports the last stop reason for this seat and ignores another seat', () => {
		const state = game({
			events: [
				stopEvent(1, 0, 'trade_offer'),
				stopEvent(2, 1, 'purchase'),
				stopEvent(3, 0, 'debt'),
				stopEvent(4, 1, 'jail')
			]
		});

		render(AutopilotPanel, { game: state, me: me(), onIntent: vi.fn() });

		expect(
			screen.getByText('Autopilot aus – Schulden müssen beglichen werden.')
		).toBeInTheDocument();
	});

	it('falls back to a plain off status without a stop event', () => {
		render(AutopilotPanel, { game: game(), me: me(), onIntent: vi.fn() });

		expect(screen.getByText('Autopilot aus.')).toBeInTheDocument();
	});
});

describe('AutopilotPanel bankrupt seat', () => {
	it('disables every control', () => {
		render(AutopilotPanel, {
			game: game(),
			me: me({ bankrupt: true }),
			onIntent: vi.fn()
		});

		expect(screen.getByLabelText('Autopilot aktiv')).toBeDisabled();
		expect(screen.getByRole('switch', { name: 'Autopilot aktiv' })).toBeDisabled();
		expect(screen.getByLabelText('Immer kaufen')).toBeDisabled();
		expect(screen.getByLabelText('Keine Karten anzeigen')).toBeDisabled();
		expect(screen.getByRole('radio', { name: 'Aus' })).toBeDisabled();
		expect(screen.getByRole('radio', { name: 'Freikaufen' })).toBeDisabled();
		expect(screen.getByRole('radio', { name: 'Pasch versuchen' })).toBeDisabled();
		expect(screen.getByLabelText('Braun')).toBeDisabled();
		expect(screen.getByLabelText('Liquidität (Mindestrest in GE)')).toBeDisabled();
	});
});
