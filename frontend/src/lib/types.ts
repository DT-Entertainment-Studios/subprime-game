// Mirrored TypeScript types of the Go message contract (Spec.md §6). The Go
// structs are the single source of truth; these types must follow them.

export type TileType =
	| 'go'
	| 'street'
	| 'tankstelle'
	| 'utility'
	| 'tax'
	| 'chance'
	| 'chest'
	| 'jail'
	| 'gotojail'
	| 'parking';

export interface BoardTile {
	index: number;
	name: string;
	type: TileType;
	group?: string;
	price?: number;
	rents?: number[];
	build?: number;
	sell?: number;
	tax?: number;
	taxPercent?: number;
}

export interface Rules {
	minPlayers: number;
	maxPlayers: number;
	startingCash: number;
	losSalary: number;
	jailFine: number;
	auctionSeconds: number;
}

export interface PlayerStats {
	rentCollected: number;
	rentPaid: number;
	propertiesBought: number;
	propertiesSold: number;
	housesBuilt: number;
	trades: number;
}

export type AutopilotJail = 'none' | 'pay' | 'roll';

export interface AutopilotSettings {
	enabled: boolean;
	buyAlways: boolean;
	jail: AutopilotJail;
	buildGroups: string[];
	liquidity: number;
}

export interface AutopilotView {
	active: boolean;
	/** Present only for the viewer's own seat (D14). */
	config?: AutopilotSettings;
}

export interface Player {
	id: string;
	name: string;
	color: string;
	seat: number;
	cash: number;
	/** Liquidation value: maximum cash raisable by liquidating all assets (cash + buildings + sale values). */
	liquidationValue: number;
	position: number;
	inJail: boolean;
	jailAttempts: number;
	jailCards: number;
	bankrupt: boolean;
	connected: boolean;
	netWorth: number;
	stats: PlayerStats;
	/** Optional so existing fixtures that predate the autopilot keep compiling. */
	autopilot?: AutopilotView;
}

export interface TileState {
	owner: number;
	houses: number;
	mortgaged: boolean;
}

export interface PendingPurchase {
	tile: number;
	price: number;
	doubled: boolean;
}

export interface Bid {
	player: number;
	amount: number;
	at: string;
}

export interface Auction {
	tile: number;
	bid: number;
	bidder: number;
	minBid: number;
	endsAt: string;
	history: Bid[];
}

export type TradeStatus = 'open' | 'accepted' | 'declined' | 'lapsed' | 'countered';

export interface TradeOffer {
	id: string;
	from: number;
	to: number;
	offerTiles: number[];
	requestTiles: number[];
	offerCash: number;
	requestCash: number;
	offerCards: number;
	requestCards: number;
	status: TradeStatus;
	createdAt: string;
}

export interface GameEvent {
	seq: number;
	type: string;
	player: number;
	amount?: number;
	text: string;
	/** Only set on card events: the deck the drawn card came from. */
	deck?: 'ereignisfeld' | 'gemeinschaftsfeld';
	/** Set on autopilot_off events: why the autopilot stopped. */
	reason?: string;
	/** Set on structured bankruptcy events: stable cause code, see BANKRUPTCY_CAUSE_LABELS. */
	cause?: string;
	/** Debt at the moment of bankruptcy (= -cash). */
	debt?: number;
	/** Realizable part of the liquidation value before liquidation (= liquidationValue - cash). */
	realizable?: number;
	/** Shortfall after full liquidation (= -liquidationValue). */
	shortfall?: number;
	at: string;
}

export type Phase = 'playing' | 'finished';

export type TurnPhase =
	| 'awaiting_roll'
	| 'awaiting_purchase'
	| 'awaiting_jail'
	| 'awaiting_debt'
	| 'auction'
	| 'finished';

export interface GameState {
	phase: Phase;
	rules: Rules;
	board: BoardTile[];
	players: Player[];
	tiles: TileState[];
	current: number;
	/** Seat index currently settling a debt, -1 if none. */
	debtor: number;
	turnPhase: TurnPhase;
	dice: [number, number];
	doubles: number;
	pending?: PendingPurchase;
	auction?: Auction;
	trades: TradeOffer[];
	events: GameEvent[];
	winner: number;
}

export interface Seat {
	userId: string;
	name: string;
	color: string;
	host: boolean;
	connected: boolean;
}

/** Votekick tally for one paused seat as seen by the current viewer. */
export interface KickVote {
	userId: string;
	votes: number;
	voted: boolean;
}

export interface Viewer {
	id: string;
	name: string;
	role: 'player' | 'spectator';
	seat: number;
	isHost: boolean;
}

export type RoomPhase = 'lobby' | 'playing' | 'finished';

/** The caller's blocking room as reported to the start page (Spec.md §6). */
export interface ActiveRoom {
	code: string;
	phase: RoomPhase;
}

export interface RoomSnapshot {
	code: string;
	phase: RoomPhase;
	hostId: string;
	createdAt: string;
	seats: Seat[];
	/** Always present; entries only while the game is playing. */
	kickVotes: KickVote[];
	you: Viewer;
	canStart: boolean;
	game?: GameState;
}

export interface User {
	id: string;
	name: string;
	email: string;
	groups: string[];
}

export interface TradeInput {
	to: number;
	offerTiles: number[];
	requestTiles: number[];
	offerCash: number;
	requestCash: number;
	offerCards: number;
	requestCards: number;
	/** Optional id of the open offer this proposal replaces (counter-offer). */
	counterTo?: string;
}

export interface Intent {
	type: string;
	amount?: number;
	tile?: number;
	option?: string;
	trade?: TradeInput;
	tradeId?: string;
	accept?: boolean;
	name?: string;
	color?: string;
	/** Votekick target. */
	targetId?: string;
	/** Complete autopilot configuration for `set_autopilot`. */
	autopilot?: AutopilotSettings;
}

// Presence and invitations travel over their own WebSocket (Spec.md §6);
// mirrors backend/internal/presence/types.go.

export interface OnlineUser {
	userId: string;
	name: string;
}

export interface Invitation {
	id: string;
	room: string;
	fromId: string;
	fromName: string;
	createdAt: string;
}

export interface SentInvite {
	id: string;
	userId: string;
	name: string;
	fromId: string;
	fromName: string;
	createdAt: string;
}

/** Push on a room-scoped presence connection. */
export interface LobbyState {
	type: 'lobby';
	online: OnlineUser[];
	sent: SentInvite[];
}

/** Push on a start-page presence connection. */
export interface InboxState {
	type: 'inbox';
	invites: Invitation[];
}

/** Reply to a successful `accept_invite`. */
export interface AcceptedMessage {
	type: 'accepted';
	room: RoomSnapshot;
}

export interface PresenceIntent {
	type: 'invite' | 'cancel_invite' | 'accept_invite' | 'decline_invite' | 'set_name';
	userId?: string;
	inviteId?: string;
	name?: string;
	color?: string;
}
