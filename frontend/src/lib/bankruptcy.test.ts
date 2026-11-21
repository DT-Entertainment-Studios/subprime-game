import { describe, expect, it } from 'vitest';
import { BANKRUPTCY_CAUSE_LABELS, bankruptcyCauseLabel, bankruptcyFor } from './bankruptcy';
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
