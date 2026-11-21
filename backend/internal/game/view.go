package game

import "github.com/DT-Entertainment-Studios/subprime-game/backend/internal/board"

// PlayerView is the public view of a player.
type PlayerView struct {
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	Color            string        `json:"color"`
	Seat             int           `json:"seat"`
	Cash             int           `json:"cash"`
	LiquidationValue int           `json:"liquidationValue"`
	Position         int           `json:"position"`
	InJail           bool          `json:"inJail"`
	JailAttempts     int           `json:"jailAttempts"`
	JailCards        int           `json:"jailCards"`
	Bankrupt         bool          `json:"bankrupt"`
	EliminatedAt     int           `json:"eliminatedAt"`
	Connected        bool          `json:"connected"`
	NetWorth         int           `json:"netWorth"`
	Stats            Stats         `json:"stats"`
	Autopilot        AutopilotView `json:"autopilot"`
}

// State is the full game state pushed to clients after every change.
type State struct {
	Phase     Phase            `json:"phase"`
	Rules     Rules            `json:"rules"`
	Board     []board.Tile     `json:"board"`
	Players   []PlayerView     `json:"players"`
	Tiles     []TileState      `json:"tiles"`
	Current   int              `json:"current"`
	Debtor    int              `json:"debtor"`
	TurnPhase TurnPhase        `json:"turnPhase"`
	Dice      [2]int           `json:"dice"`
	Doubles   int              `json:"doubles"`
	Pending   *PendingPurchase `json:"pending,omitempty"`
	Auction   *Auction         `json:"auction,omitempty"`
	Trades    []TradeOffer     `json:"trades"`
	Events    []Event          `json:"events"`
	Winner    int              `json:"winner"`
}

// State builds the public snapshot for a spectator.
func (g *Game) State() State {
	return g.StateFor(-1)
}

// StateFor builds the public snapshot. viewer is the seat index whose private
// autopilot configuration is included, or -1 for a spectator.
func (g *Game) StateFor(viewer int) State {
	players := make([]PlayerView, len(g.Players))
	for i, p := range g.Players {
		autopilot := AutopilotView{Active: p.Autopilot.Enabled && !p.Bankrupt}
		if i == viewer {
			cfg := p.Autopilot
			cfg.BuildGroups = append([]string{}, p.Autopilot.BuildGroups...)
			autopilot.Config = &cfg
		}
		players[i] = PlayerView{
			ID:               p.ID,
			Name:             p.Name,
			Color:            p.Color,
			Seat:             p.Seat,
			Cash:             p.Cash,
			LiquidationValue: g.liquidationValue(i),
			Position:         p.Position,
			InJail:           p.InJail,
			JailAttempts:     p.JailAttempts,
			JailCards:        len(p.JailCards),
			Bankrupt:         p.Bankrupt,
			EliminatedAt:     p.EliminatedAt,
			Connected:        p.Connected,
			NetWorth:         g.netWorth(i),
			Stats:            p.Stats,
			Autopilot:        autopilot,
		}
	}
	tiles := make([]TileState, len(g.Tiles))
	copy(tiles, g.Tiles)
	trades := make([]TradeOffer, len(g.Trades))
	for i, t := range g.Trades {
		trades[i] = *t
		trades[i].OfferTiles = append([]int{}, t.OfferTiles...)
		trades[i].RequestTiles = append([]int{}, t.RequestTiles...)
	}
	events := make([]Event, len(g.Events))
	copy(events, g.Events)
	var auction *Auction
	if g.Auction != nil {
		a := *g.Auction
		a.History = append([]Bid(nil), g.Auction.History...)
		auction = &a
	}
	var pending *PendingPurchase
	if g.Pending != nil {
		p := *g.Pending
		pending = &p
	}
	return State{
		Phase:     g.Phase,
		Rules:     g.Rules,
		Board:     g.Catalog.Tiles,
		Players:   players,
		Tiles:     tiles,
		Current:   g.Current,
		Debtor:    g.Debtor,
		TurnPhase: g.TurnPhase,
		Dice:      g.Dice,
		Doubles:   g.Doubles,
		Pending:   pending,
		Auction:   auction,
		Trades:    trades,
		Events:    events,
		Winner:    g.Winner,
	}
}

// SetConnected updates the connection flag of a player.
func (g *Game) SetConnected(idx int, connected bool) {
	if idx >= 0 && idx < len(g.Players) {
		g.Players[idx].Connected = connected
	}
}

// ForceEnd ends the game, used for administrative shutdowns.
func (g *Game) ForceEnd() {
	if g.Phase == PhasePlaying {
		g.Phase = PhaseFinished
		g.TurnPhase = TurnFinished
		g.Pending = nil
		g.Auction = nil
		for i := range g.Players {
			g.disableAutopilotSilently(i)
		}
	}
}
