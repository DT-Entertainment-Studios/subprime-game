// Placement for the game-end overview.
//
// Survivors all share the top place; bankrupt players are ordered by the
// backend turn-order stamp (`eliminatedAt`) with later elimination ranking
// better. Players eliminated at the same time share a place (dense ranking).

import type { Player } from './types';

export type RankedPlayer = { player: Player; place: number };

/**
 * Orders players for the end screen: survivors first (by net worth for
 * display), then bankrupt players by later elimination first. Equal
 * elimination times share a place; the next group gets the next consecutive
 * place (dense ranking).
 */
export function rankedStandings(players: Player[]): RankedPlayer[] {
	const survivors = players
		.filter((p) => !p.bankrupt)
		.sort((a, b) => b.netWorth - a.netWorth);
	const bankrupt = players
		.filter((p) => p.bankrupt)
		.sort((a, b) => (b.eliminatedAt ?? 0) - (a.eliminatedAt ?? 0));

	const result: RankedPlayer[] = survivors.map((player) => ({ player, place: 1 }));
	let place = survivors.length > 0 ? 1 : 0;
	let previous: number | undefined;
	for (const player of bankrupt) {
		const at = player.eliminatedAt ?? 0;
		if (previous === undefined || at !== previous) place++;
		previous = at;
		result.push({ player, place });
	}
	return result;
}
