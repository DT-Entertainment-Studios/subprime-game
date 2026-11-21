// Structured details for automatic bankruptcy events (Spec.md §4.10).
//
// The backend sends one `bankruptcy` event per automatic insolvency with a
// stable `cause` code and the three amounts. Legacy/generic bankruptcy events
// have no `cause`; those are deliberately not surfaced as toasts/badges.

import type { GameEvent } from './types';
import { formatMoney } from './format';

/** Every bankruptcy cause code sent by the backend and its German label. */
const CAUSE_LABELS = new Map<string, string>([
	['miete', 'Miete'],
	['nutzungsanteil', 'Nutzungsanteil'],
	['auktion', 'Auktionskauf'],
	['steuer', 'Steuer'],
	['reparaturkosten', 'Reparaturkosten'],
	['kartenreparatur', 'Reparatur laut Karte'],
	['kartenzahlung', 'Kartenzahlung'],
	['zahlung_an_alle', 'Zahlung an alle'],
	['beitreibung', 'Beitreibung'],
	['zahlung', 'Zahlung']
]);

/** German display labels, exposed as a plain record to keep the public API stable. */
export const BANKRUPTCY_CAUSE_LABELS: Record<string, string> = Object.fromEntries(CAUSE_LABELS);

const FALLBACK_LABEL = 'Zahlung';

/** Human label for a cause code; unknown and missing codes fall back to "Zahlung". */
export function bankruptcyCauseLabel(cause: string | undefined): string {
	if (!cause) return FALLBACK_LABEL;
	return CAUSE_LABELS.get(cause) ?? FALLBACK_LABEL;
}

/**
 * The single structured bankruptcy event for a seat, if any.
 * Events without a `cause` (generic/legacy/voluntary) are ignored.
 */
export function bankruptcyFor(events: GameEvent[], seat: number): GameEvent | undefined {
	return events.find(
		(event) => event.type === 'bankruptcy' && event.cause !== undefined && event.player === seat
	);
}

/**
 * Tooltip text for a bankruptcy badge / end screen. Prefers the backend text
 * (which carries the optional creditor name); builds a fallback from the
 * structured fields when no text is present.
 */
export function bankruptcyTooltip(event: GameEvent): string {
	if (event.text) return event.text;
	return `${bankruptcyCauseLabel(event.cause)}: Schuld ${formatMoney(event.debt)}, ` +
		`Realisierbar ${formatMoney(event.realizable)}, ` +
		`Fehlbetrag ${formatMoney(event.shortfall)}.`;
}

/** Display label and native tooltip for the `Ausscheiden` column of one seat. */
export interface EliminationInfo {
	label: string;
	tooltip: string;
}

/**
 * Label for a seat's elimination. Voluntary eliminations take precedence over a
 * structured bankruptcy cause (the backend logs no cause for `give_up`/`leave`).
 */
export function eliminationInfo(events: GameEvent[], seat: number): EliminationInfo {
	const giveUp = events.find((event) => event.type === 'give_up' && event.player === seat);
	if (giveUp) return { label: 'Aufgegeben', tooltip: giveUp.text };

	const leave = events.find((event) => event.type === 'leave' && event.player === seat);
	if (leave) return { label: 'Verlassen', tooltip: leave.text };

	const bust = bankruptcyFor(events, seat);
	return {
		label: bankruptcyCauseLabel(bust?.cause),
		tooltip: bust ? bankruptcyTooltip(bust) : ''
	};
}
