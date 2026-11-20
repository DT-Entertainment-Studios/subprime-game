// Package game implements the authoritative Subprime rules engine (Spec.md §4).
// All state is in memory; the engine is single-threaded and expects the caller
// (a room) to hold a lock while invoking its methods.
package game

import (
	"fmt"
	"time"

	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/board"
	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/rng"
)

// Phase is the coarse lifecycle of a game.
type Phase string

const (
	PhaseLobby    Phase = "lobby"
	PhasePlaying  Phase = "playing"
	PhaseFinished Phase = "finished"
)

// TurnPhase is the current step of the active turn.
type TurnPhase string

const (
	TurnAwaitingRoll     TurnPhase = "awaiting_roll"
	TurnAwaitingPurchase TurnPhase = "awaiting_purchase"
	TurnAwaitingJail     TurnPhase = "awaiting_jail"
	TurnAwaitingDebt     TurnPhase = "awaiting_debt"
	TurnAuction          TurnPhase = "auction"
	TurnFinished         TurnPhase = "finished"
)

// Bank is the pseudo player index of the neutral bank.
const Bank = -1

// Rules are the fixed v1.0 rule parameters (Spec.md §4.12).
type Rules struct {
	MinPlayers     int `json:"minPlayers"`
	MaxPlayers     int `json:"maxPlayers"`
	StartingCash   int `json:"startingCash"`
	LosSalary      int `json:"losSalary"`
	JailFine       int `json:"jailFine"`
	AuctionSeconds int `json:"auctionSeconds"`
}

// DefaultRules is the fixed v1.0 rule set.
var DefaultRules = Rules{
	MinPlayers:     2,
	MaxPlayers:     8,
	StartingCash:   1500,
	LosSalary:      200,
	JailFine:       50,
	AuctionSeconds: 15,
}

// Stats are per-game player statistics shown on the final screen.
type Stats struct {
	RentCollected    int `json:"rentCollected"`
	RentPaid         int `json:"rentPaid"`
	PropertiesBought int `json:"propertiesBought"`
	PropertiesSold   int `json:"propertiesSold"`
	HousesBuilt      int `json:"housesBuilt"`
	Trades           int `json:"trades"`
}

// Player is one participant. JailCards holds the ids of held
// get-out-of-jail cards.
type Player struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Color        string   `json:"color"`
	Seat         int      `json:"seat"`
	Cash         int      `json:"cash"`
	Position     int      `json:"position"`
	InJail       bool     `json:"inJail"`
	JailAttempts int      `json:"jailAttempts"`
	JailCards    []string `json:"-"`
	Bankrupt     bool     `json:"bankrupt"`
	Connected    bool     `json:"connected"`
	Stats        Stats    `json:"stats"`

	Autopilot       AutopilotSettings `json:"-"`
	autopilotNextAt time.Time
}

// TileState is the mutable state of one board tile.
type TileState struct {
	Owner     int  `json:"owner"`
	Houses    int  `json:"houses"`
	Mortgaged bool `json:"mortgaged"`
}

// PendingPurchase is a purchase offer the active player must answer.
type PendingPurchase struct {
	Tile    int  `json:"tile"`
	Price   int  `json:"price"`
	Doubled bool `json:"doubled"`
}

// Bid is one auction bid.
type Bid struct {
	Player int       `json:"player"`
	Amount int       `json:"amount"`
	At     time.Time `json:"at"`
}

// Auction is a running tile auction.
type Auction struct {
	Tile    int       `json:"tile"`
	Bid     int       `json:"bid"`
	Bidder  int       `json:"bidder"`
	MinBid  int       `json:"minBid"`
	EndsAt  time.Time `json:"endsAt"`
	History []Bid     `json:"history"`
}

// TradeStatus is the lifecycle of a trade offer.
type TradeStatus string

const (
	TradeOpen      TradeStatus = "open"
	TradeAccepted  TradeStatus = "accepted"
	TradeDeclined  TradeStatus = "declined"
	TradeLapsed    TradeStatus = "lapsed"
	TradeCountered TradeStatus = "countered"
)

// TradeOffer is a proposal between two players. JailCards holds the number of
// get-out-of-jail cards offered.
type TradeOffer struct {
	ID           string      `json:"id"`
	From         int         `json:"from"`
	To           int         `json:"to"`
	OfferTiles   []int       `json:"offerTiles"`
	RequestTiles []int       `json:"requestTiles"`
	OfferCash    int         `json:"offerCash"`
	RequestCash  int         `json:"requestCash"`
	OfferCards   int         `json:"offerCards"`
	RequestCards int         `json:"requestCards"`
	Status       TradeStatus `json:"status"`
	CreatedAt    time.Time   `json:"createdAt"`
}

// Event is one entry of the append-only event log.
type Event struct {
	Seq    int        `json:"seq"`
	Type   string     `json:"type"`
	Player int        `json:"player"`
	Reason string     `json:"reason,omitempty"`
	Amount int        `json:"amount,omitempty"`
	Text   string     `json:"text"`
	Deck   board.Deck `json:"deck,omitempty"`
	At     time.Time  `json:"at"`
}

// DeckState is the draw pile of one card deck, top first.
type DeckState struct {
	Draw []string
}

// Options configures a game; zero values use production defaults. Production
// randomness comes from crypto/rand.
type Options struct {
	Rand  rng.Random
	Now   func() time.Time
	Rules Rules
}

// Game is the complete authoritative state of one match.
type Game struct {
	Catalog *board.Catalog
	Rules   Rules
	Phase   Phase

	Players []*Player
	Tiles   []TileState

	Current   int
	TurnPhase TurnPhase
	Dice      [2]int
	Doubles   int

	Pending *PendingPurchase
	Auction *Auction
	Trades  []*TradeOffer
	Decks   map[board.Deck]*DeckState
	Events  []Event
	Winner  int

	extraTurn bool
	turnOver  bool
	eventSeq  int
	now       time.Time

	rand            rng.Random
	clock           func() time.Time
	dice            func() (int, int)
	auctionDuration time.Duration
}

// New creates a game for the given players in the given seat order.
func New(cat *board.Catalog, players []*Player, opts Options) (*Game, error) {
	if cat == nil {
		return nil, fmt.Errorf("nil catalog")
	}
	rules := opts.Rules
	if rules == (Rules{}) {
		rules = DefaultRules
	}
	if len(players) < rules.MinPlayers || len(players) > rules.MaxPlayers {
		return nil, fmt.Errorf("need %d-%d players, got %d", rules.MinPlayers, rules.MaxPlayers, len(players))
	}
	random := opts.Rand
	if random == nil {
		random = rng.New()
	}
	clock := opts.Now
	if clock == nil {
		clock = time.Now
	}
	g := &Game{
		Catalog:         cat,
		Rules:           rules,
		Phase:           PhasePlaying,
		Players:         players,
		Tiles:           make([]TileState, len(cat.Tiles)),
		Current:         0,
		TurnPhase:       TurnAwaitingRoll,
		Winner:          -1,
		Decks:           map[board.Deck]*DeckState{},
		rand:            random,
		clock:           clock,
		auctionDuration: time.Duration(rules.AuctionSeconds) * time.Second,
	}
	seen := map[string]bool{}
	for i, p := range players {
		if p == nil || p.ID == "" || p.Name == "" {
			return nil, fmt.Errorf("player %d is incomplete", i)
		}
		if seen[p.ID] {
			return nil, fmt.Errorf("duplicate player id %q", p.ID)
		}
		seen[p.ID] = true
		p.Seat = i
		p.Cash = rules.StartingCash
		p.Position = 0
		p.InJail = false
		p.JailAttempts = 0
		p.JailCards = nil
		p.Bankrupt = false
		p.Connected = true
		p.Stats = Stats{}
		p.Autopilot = AutopilotSettings{
			Enabled:     false,
			BuyAlways:   true,
			Jail:        AutopilotJailNone,
			BuildGroups: []string{},
			Liquidity:   0,
		}
		p.autopilotNextAt = time.Time{}
	}
	for i := range g.Tiles {
		g.Tiles[i].Owner = Bank
	}
	g.initDecks()
	g.logf("start", Bank, 0, "Das Spiel beginnt. %s ist am Zug.", players[0].Name)
	return g, nil
}

func (g *Game) initDecks() {
	for _, deck := range []board.Deck{board.DeckChance, board.DeckChest} {
		ids := make([]string, 0, 16)
		for _, card := range g.Catalog.Cards {
			if card.Deck == deck {
				ids = append(ids, card.ID)
			}
		}
		g.rand.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
		g.Decks[deck] = &DeckState{Draw: ids}
	}
}

// Now returns the engine's current time.
func (g *Game) Now() time.Time { return g.clock() }

func (g *Game) tile(i int) board.Tile { return g.Catalog.Tile(i) }

func (g *Game) player(i int) *Player { return g.Players[i] }

func (g *Game) active(i int) bool {
	return i >= 0 && i < len(g.Players) && !g.Players[i].Bankrupt
}

func (g *Game) activeCount() int {
	n := 0
	for _, p := range g.Players {
		if !p.Bankrupt {
			n++
		}
	}
	return n
}

func (g *Game) logf(kind string, player, amount int, format string, args ...any) {
	g.eventSeq++
	g.Events = append(g.Events, Event{
		Seq:    g.eventSeq,
		Type:   kind,
		Player: player,
		Amount: amount,
		Text:   fmt.Sprintf(format, args...),
		At:     g.clock(),
	})
}

func (g *Game) ownsGroup(player int, group board.Group) bool {
	if group == "" {
		return false
	}
	for _, idx := range g.Catalog.GroupTiles(group) {
		if g.Tiles[idx].Owner != player {
			return false
		}
	}
	return true
}

func (g *Game) groupTiles(group board.Group) []int {
	return g.Catalog.GroupTiles(group)
}

func (g *Game) ownsAllStreets(player int, group board.Group) bool {
	if group == "" || group == board.GroupTankstelle || group == board.GroupVersorger {
		return false
	}
	return g.ownsGroup(player, group)
}

func (g *Game) groupHasBuildings(group board.Group) bool {
	for _, idx := range g.groupTiles(group) {
		if g.Tiles[idx].Houses > 0 {
			return true
		}
	}
	return false
}

func (g *Game) countOwned(player int, group board.Group) int {
	n := 0
	for _, idx := range g.groupTiles(group) {
		if g.Tiles[idx].Owner == player {
			n++
		}
	}
	return n
}

// wouldCompleteGroup reports whether buying tile idx would complete a color
// group for the given player (the double-price rule, Spec.md §4.4).
func (g *Game) wouldCompleteGroup(player, idx int) bool {
	t := g.tile(idx)
	if !t.IsStreet() {
		return false
	}
	for _, other := range g.groupTiles(t.Group) {
		if other == idx {
			continue
		}
		if g.Tiles[other].Owner != player {
			return false
		}
	}
	return true
}

// priceFor returns the price the player would pay when buying from the bank,
// including the double-price rule for a completing street.
func (g *Game) priceFor(player, idx int) (price int, doubled bool) {
	t := g.tile(idx)
	price = t.Price
	if g.wouldCompleteGroup(player, idx) {
		price *= 2
		doubled = true
	}
	return price, doubled
}

func (g *Game) fail(code, format string, args ...any) error {
	return &RuleError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// RuleError is a rejected intent with a machine-readable code.
type RuleError struct {
	Code    string
	Message string
}

func (e *RuleError) Error() string { return e.Message }

// IsRuleError reports whether err is a RuleError.
func IsRuleError(err error) bool {
	_, ok := err.(*RuleError)
	return ok
}

func (g *Game) requireTurn(player int) error {
	if g.Phase != PhasePlaying {
		return g.fail("game_over", "Das Spiel ist beendet.")
	}
	if player != g.Current || !g.active(player) {
		return g.fail("not_your_turn", "Du bist nicht am Zug.")
	}
	return nil
}

func (g *Game) requireTurnPhase(phase TurnPhase) error {
	if g.TurnPhase != phase {
		return g.fail("wrong_phase", "Diese Aktion ist jetzt nicht möglich.")
	}
	return nil
}

// PlayerIndex returns the index for a user id, or -1.
func (g *Game) PlayerIndex(id string) int {
	for i, p := range g.Players {
		if p.ID == id {
			return i
		}
	}
	return -1
}
