import { describe, expect, it } from 'vitest';
import {
	BANKRUPTCY_CAUSE_LABELS,
	bankruptcyCauseLabel,
	bankruptcyFor,
	eliminationInfo
} from './bankruptcy';
import type { GameEvent } from './types';

function bankruptcyEvent(overrides: Partial<GameEvent> = {}): GameEvent {
	return {
		seq: 1,
		type: 'bankruptcy',
		player: 0,
		cause: 'miete',
		debt: 420,
		realizable: 180,
		shortfall: 240,
		text: 'Anton ist pleite. Miete an Berta: 420 GE. Realisierbar: 180 GE. Fehlbetrag: 240 GE.',
		at: '2026-01-01T00:00:00Z',
		...overrides
	};
}

function event(type: string, seat: number, text: string, seq = 1): GameEvent {
	return { seq, type, player: seat, text, at: '2026-01-01T00:00:00Z' };
}

describe('bankruptcy', () => {
	it('maps every bankruptcy cause code to a German label', () => {
		expect(BANKRUPTCY_CAUSE_LABELS).toEqual({
			miete: 'Miete',
			nutzungsanteil: 'Nutzungsanteil',
			auktion: 'Auktionskauf',
			steuer: 'Steuer',
			reparaturkosten: 'Reparaturkosten',
			kartenreparatur: 'Reparatur laut Karte',
			kartenzahlung: 'Kartenzahlung',
			zahlung_an_alle: 'Zahlung an alle',
			beitreibung: 'Beitreibung',
			zahlung: 'Zahlung'
		});
		for (const [code, label] of Object.entries(BANKRUPTCY_CAUSE_LABELS)) {
			expect(bankruptcyCauseLabel(code)).toBe(label);
		}
		expect(bankruptcyCauseLabel(undefined)).toBe('Zahlung');
		expect(bankruptcyCauseLabel('unbekannt')).toBe('Zahlung');
	});

	it('finds only the structured bankruptcy event for a seat', () => {
		const events: GameEvent[] = [
			{
				seq: 1,
				type: 'bankruptcy',
				player: 0,
				text: 'Anton scheidet aus.',
				at: '2026-01-01T00:00:00Z'
			},
			bankruptcyEvent({ seq: 2, player: 1 }),
			{ seq: 3, type: 'move', player: 0, text: 'Anton zieht.', at: '2026-01-01T00:00:01Z' }
		];

		expect(bankruptcyFor(events, 0)).toBeUndefined();
		expect(bankruptcyFor(events, 1)?.seq).toBe(2);
		expect(bankruptcyFor(events, 2)).toBeUndefined();
	});
});

describe('eliminationInfo', () => {
	it('labels give_up as Aufgegeben with the event text as tooltip', () => {
		const info = eliminationInfo([event('give_up', 1, 'Ben gibt auf und scheidet aus.')], 1);
		expect(info).toEqual({ label: 'Aufgegeben', tooltip: 'Ben gibt auf und scheidet aus.' });
	});

	it('labels leave as Verlassen with the event text as tooltip', () => {
		const info = eliminationInfo([event('leave', 2, 'Clara verlässt das Spiel.')], 2);
		expect(info).toEqual({ label: 'Verlassen', tooltip: 'Clara verlässt das Spiel.' });
	});

	it('uses the structured cause label, tooltip and fallback for bankruptcies', () => {
		const cause = eliminationInfo([bankruptcyEvent({ player: 1 })], 1);
		expect(cause.label).toBe('Miete');
		expect(cause.tooltip).toContain('240 GE');

		const generic = eliminationInfo(
			[event('bankruptcy', 1, 'Ben ist pleite.', 2)],
			1
		);
		expect(generic).toEqual({ label: 'Zahlung', tooltip: '' });
	});

	it('prefers give_up over leave and a structured bankruptcy cause', () => {
		const info = eliminationInfo(
			[
				bankruptcyEvent({ player: 1 }),
				event('leave', 1, 'Ben verlässt das Spiel.', 2),
				event('give_up', 1, 'Ben gibt auf und scheidet aus.', 3)
			],
			1
		);
		expect(info).toEqual({ label: 'Aufgegeben', tooltip: 'Ben gibt auf und scheidet aus.' });
	});

	it('returns the Zahlung fallback for a seat with no events', () => {
		expect(eliminationInfo([], 3)).toEqual({ label: 'Zahlung', tooltip: '' });
	});
});
