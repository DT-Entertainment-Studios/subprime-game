// Shared helpers to describe trade objects (tiles, cash, workshop passes)
// with label, colour and icon so both the trade panel and the popups render
// them the same way.

import type { GameState } from './types';
import { formatMoney, groupColor } from './format';

/** Marker colour for workshop passes, matching the Werkstatt field accent. */
export const CARD_COLOR = '#a15c00';
/** Neutral colour for the cash chip, matching the Steuer field accent. */
export const CASH_COLOR = '#546e7a';

export interface TradeItem {
	kind: 'tile' | 'cash' | 'card';
	label: string;
	/** Swatch / badge colour as CSS value. */
	color: string;
	/** Icon key for entries that read better with a glyph than a swatch. */
	icon: '' | 'gas' | 'bolt' | 'droplet' | 'coin' | 'tool';
	title: string;
}

/** Builds the display items of one trade side, tiles in board order. */
export function tradeSideItems(
	game: GameState,
	tiles: number[] | null | undefined,
	cards: number,
	cash: number
): TradeItem[] {
	const items: TradeItem[] = [];
	for (const index of [...(tiles ?? [])].sort((a, b) => a - b)) {
		const tile = game.board[index];
		if (!tile) continue;
		items.push({
			kind: 'tile',
			label: tile.name,
			color: groupColor(tile.group),
			icon: tile.type === 'tankstelle' ? 'gas' : tile.type === 'utility' ? (tile.name.includes('Wasser') ? 'droplet' : 'bolt') : '',
			title: tile.type === 'tankstelle' ? 'Tankstelle' : tile.type === 'utility' ? 'Versorger' : tile.group ? 'Straßenfarbe' : tile.name
		});
	}
	if (cash > 0) {
		items.push({
			kind: 'cash',
			label: formatMoney(cash),
			color: CASH_COLOR,
			icon: 'coin',
			title: 'Bargeld'
		});
	}
	if (cards > 0) {
		items.push({
			kind: 'card',
			label: cards === 1 ? '1 Pannenhilfe' : `${cards} Pannenhilfen`,
			color: CARD_COLOR,
			icon: 'tool',
			title: 'Pannenhilfe — kommst gratis aus der Werkstatt'
		});
	}
	return items;
}
