// German display formatting and board helpers.

import type { BoardTile, TileState, Player } from './types';

const money = new Intl.NumberFormat('de-DE', { maximumFractionDigits: 0 });

/** Formats an amount of GE. */
export function formatMoney(amount: number | undefined): string {
	if (amount === undefined || amount === null) return '–';
	return `${money.format(amount).replaceAll('.', '\u202F')} GE`;
}

/** Beleihungswert: the bank lends 50 % of the purchase price (a mortgaged
    property yields no rent and cannot be built on, Spec.md §4.7). */
export function mortgageValue(price: number | undefined): number {
	if (!price || price <= 0) return 0;
	return Math.floor(price / 2);
}

/** Hypotheken-Ablöse: freeing a mortgage costs 60 % of the purchase price.
    Mirrors the Go integer math `price * 60 / 100`; the alternative
    `price * 0.6` can drift through floating point rounding. */
export function unmortgageCost(price: number | undefined): number {
	if (!price || price <= 0) return 0;
	return Math.floor((price * 60) / 100);
}

/** Formats an ISO timestamp as German wall time. */
export function formatTime(iso: string): string {
	const date = new Date(iso);
	if (Number.isNaN(date.getTime())) return '';
	return date.toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

/** Remaining seconds until an ISO deadline, never below zero. */
export function secondsUntil(iso: string, now = Date.now()): number {
	const target = new Date(iso).getTime();
	if (Number.isNaN(target)) return 0;
	return Math.max(0, Math.ceil((target - now) / 1000));
}

/** Human label for a turn phase. */
export function turnPhaseLabel(phase: string): string {
	switch (phase) {
		case 'awaiting_roll':
			return 'Vor dem Würfeln';
		case 'awaiting_purchase':
			return 'Kaufentscheidung';
		case 'awaiting_jail':
			return 'In der Werkstatt';
		case 'awaiting_debt':
			return 'Schulden begleichen';
		case 'auction':
			return 'Auktion';
		case 'finished':
			return 'Beendet';
		default:
			return phase;
	}
}

/** German label for the reason an autopilot stopped. */
export function autopilotStopLabel(reason: string | undefined): string {
	switch (reason) {
		case 'trade_offer':
			return 'Eingegangenes Tauschangebot';
		case 'purchase':
			return 'Kaufentscheidung nicht automatisiert';
		case 'jail':
			return 'Keine ausführbare Werkstatt-Option';
		case 'debt':
			return 'Schulden müssen beglichen werden';
		case 'manual':
			return 'Manuell ausgeschaltet';
		default:
			return 'Unbekannter Grund';
	}
}

const GROUP_COLORS: Record<string, string> = {
	braun: '#955436',
	hellblau: '#aae0fa',
	rosa: '#d93a96',
	orange: '#f7941d',
	rot: '#ed1b24',
	gelb: '#fef200',
	gruen: '#1fb25a',
	dunkelblau: '#0072bb',
	tankstelle: '#455a64',
	versorger: '#00796b'
};

/** CSS color of a board group. */
export function groupColor(group: string | undefined): string {
	if (!group) return 'transparent';
	return GROUP_COLORS[group] ?? '#888888';
}

const GROUP_LABELS: Record<string, string> = {
	braun: 'Braun',
	hellblau: 'Hellblau',
	rosa: 'Rosa',
	orange: 'Orange',
	rot: 'Rot',
	gelb: 'Gelb',
	gruen: 'Grün',
	dunkelblau: 'Dunkelblau',
	tankstelle: 'Tankstellen',
	versorger: 'Werke'
};

/** German label of a board group. */
export function groupLabel(group: string | undefined): string {
	if (!group) return '';
	return GROUP_LABELS[group] ?? group;
}

/** Order in which the street colour groups appear on the board. */
export const STREET_GROUPS = [
	'braun',
	'hellblau',
	'rosa',
	'orange',
	'rot',
	'gelb',
	'gruen',
	'dunkelblau'
] as const;

// Player colours are deliberately offset from the board group colours so an
// owned field's edge strip never blends into its colour band.
const TOKEN_COLORS: Record<string, string> = {
	rot: '#b3261e',
	blau: '#283593',
	gruen: '#2e7d32',
	gelb: '#ffd54f',
	rosa: '#ad1457',
	orange: '#e65100',
	lila: '#6a1b9a',
	schwarz: '#37474f'
};

/** CSS color of a token color. */
export function tokenColor(color: string | undefined): string {
	if (!color) return '#888888';
	return TOKEN_COLORS[color] ?? '#888888';
}

/** German label of a token color. */
export function colorLabel(color: string): string {
	const labels: Record<string, string> = {
		rot: 'Rot',
		blau: 'Blau',
		gruen: 'Grün',
		gelb: 'Gelb',
		rosa: 'Rosa',
		orange: 'Orange',
		lila: 'Lila',
		schwarz: 'Schwarz'
	};
	return labels[color] ?? color;
}

/** The token colors a player can pick, matching the backend list. */
export const TOKEN_COLORS_LIST = ['rot', 'blau', 'gruen', 'gelb', 'rosa', 'orange', 'lila', 'schwarz'];

/** Short type label for a tile. */
export function tileTypeLabel(tile: BoardTile): string {
	switch (tile.type) {
		case 'go':
			return 'Los';
		case 'tankstelle':
			return 'Tankstelle';
		case 'utility':
			return 'Versorger';
		case 'tax':
			return 'Steuer';
		case 'chance':
			return 'Ereignis';
		case 'chest':
			return 'Gemeinschaft';
		case 'jail':
			return 'Werkstatt';
		case 'gotojail':
			return 'Unfall';
		case 'parking':
			return 'Frei-Parken';
		default:
			return '';
	}
}

/** The rent ladder label of a tile, for tooltips. */
export function rentLabel(tile: BoardTile): string {
	if (tile.type === 'street' && tile.rents) {
		return `Miete ${tile.rents.join(' / ')}`;
	}
	if (tile.type === 'tankstelle' && tile.rents) {
		return `Miete ${tile.rents.join(' / ')}`;
	}
	if (tile.type === 'utility') {
		return 'Miete 4× Würfel (10× bei beiden)';
	}
	return '';
}

/** Build levels for even-building checks (hotel counts as five). */
export function levelOf(state: TileState | undefined): number {
	if (!state) return 0;
	return state.houses;
}

/** Net worth of a player, as reported by the server. */
export function netWorth(player: Player): number {
	return player.netWorth;
}

/** Board grid position (1-based row/column) of a tile index, clockwise from Los. */
export function gridPosition(index: number): { row: number; col: number } {
	if (index === 0) return { row: 11, col: 11 };
	if (index === 10) return { row: 11, col: 1 };
	if (index === 20) return { row: 1, col: 1 };
	if (index === 30) return { row: 1, col: 11 };
	if (index < 10) return { row: 11, col: 11 - index };
	if (index < 20) return { row: 11 - (index - 10), col: 1 };
	if (index < 30) return { row: 1, col: 1 + (index - 20) };
	return { row: 1 + (index - 30), col: 11 };
}

/** Human readable building label for a tile state. */
export function buildingLabel(houses: number): string {
	if (houses === 5) return '🏨';
	if (houses > 0) return '🏠'.repeat(houses);
	return '';
}
