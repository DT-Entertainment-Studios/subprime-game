import { describe, expect, it } from 'vitest';
import {
	autopilotStopLabel,
	buildingLabel,
	colorLabel,
	formatMoney,
	gridPosition,
	groupColor,
	mortgageValue,
	groupLabel,
	secondsUntil,
	STREET_GROUPS,
	tokenColor,
	turnPhaseLabel,
	unmortgageCost
} from './format';

describe('formatMoney', () => {
	it('formats German thousands and the GE unit', () => {
		expect(formatMoney(1500)).toBe('1.500 GE');
		expect(formatMoney(0)).toBe('0 GE');
		expect(formatMoney(undefined)).toBe('–');
	});
});

describe('secondsUntil', () => {
	it('rounds up and never goes below zero', () => {
		const now = Date.parse('2026-01-01T12:00:00Z');
		expect(secondsUntil('2026-01-01T12:00:15Z', now)).toBe(15);
		expect(secondsUntil('2026-01-01T12:00:00.400Z', now)).toBe(1);
		expect(secondsUntil('2026-01-01T11:59:00Z', now)).toBe(0);
	});
});

describe('gridPosition', () => {
	it('places the corners and the four sides', () => {
		expect(gridPosition(0)).toEqual({ row: 11, col: 11 });
		expect(gridPosition(10)).toEqual({ row: 11, col: 1 });
		expect(gridPosition(20)).toEqual({ row: 1, col: 1 });
		expect(gridPosition(30)).toEqual({ row: 1, col: 11 });
		expect(gridPosition(5)).toEqual({ row: 11, col: 6 });
		expect(gridPosition(15)).toEqual({ row: 6, col: 1 });
		expect(gridPosition(25)).toEqual({ row: 1, col: 6 });
		expect(gridPosition(35)).toEqual({ row: 6, col: 11 });
	});
});

describe('mortgage arithmetic', () => {
	it('mirrors the backend integer math (`price / 2` and `price * 60 / 100`)', () => {
		expect(mortgageValue(220)).toBe(110);
		expect(unmortgageCost(220)).toBe(132);
		expect(unmortgageCost(150)).toBe(90);
		expect(unmortgageCost(350)).toBe(210);
		// The float path `price * 0.6` can round the same prices differently.
		expect(unmortgageCost(60)).toBe(36);
		expect(mortgageValue(undefined)).toBe(0);
		expect(unmortgageCost(undefined)).toBe(0);
		expect(unmortgageCost(0)).toBe(0);
	});
});

describe('autopilotStopLabel', () => {
	it.each([
		['trade_offer', 'Eingegangenes Tauschangebot'],
		['purchase', 'Kaufentscheidung nicht automatisiert'],
		['jail', 'Keine ausführbare Werkstatt-Option'],
		['debt', 'Schulden müssen beglichen werden'],
		['manual', 'Manuell ausgeschaltet'],
		[undefined, 'Unbekannter Grund'],
		['irgendwas', 'Unbekannter Grund']
	])('maps %s to its German label', (reason, label) => {
		expect(autopilotStopLabel(reason)).toBe(label);
	});
});

describe('labels and colors', () => {
	it('maps groups and token colors', () => {
		expect(groupColor('rosa')).toBe('#d93a96');
		expect(groupColor('tankstelle')).toBe('#455a64');
		expect(groupColor('versorger')).toBe('#00796b');
		expect(groupColor(undefined)).toBe('transparent');
		expect(groupLabel('gruen')).toBe('Grün');
		expect(groupLabel('tankstelle')).toBe('Tankstellen');
		expect(groupLabel(undefined)).toBe('');
		expect(STREET_GROUPS).toHaveLength(8);
		expect(tokenColor('blau')).toBe('#283593');
		expect(colorLabel('gruen')).toBe('Grün');
		expect(turnPhaseLabel('awaiting_roll')).toBe('Vor dem Würfeln');
		expect(turnPhaseLabel('awaiting_debt')).toBe('Schulden begleichen');
		expect(buildingLabel(0)).toBe('');
		expect(buildingLabel(3)).toBe('🏠🏠🏠');
		expect(buildingLabel(5)).toBe('🏨');
	});

	it('keeps player colours distinct from the board groups', () => {
		const pairs: [string, string][] = [
			['rot', 'rot'],
			['blau', 'dunkelblau'],
			['gruen', 'gruen'],
			['gelb', 'gelb'],
			['rosa', 'rosa'],
			['orange', 'orange']
		];
		for (const [token, group] of pairs) {
			expect(tokenColor(token)).not.toBe(groupColor(group));
		}
	});
});
