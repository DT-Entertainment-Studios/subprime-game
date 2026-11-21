package game

import (
	"fmt"
	"sort"
	"time"

	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/board"
)

// Stop reasons stored in Event.Reason for autopilot_off.
const (
	AutopilotStopManual     = "manual"
	AutopilotStopTradeOffer = "trade_offer"
	AutopilotStopPurchase   = "purchase"
	AutopilotStopJail       = "jail"
	AutopilotStopDebt       = "debt"
)

// Jail choices in AutopilotSettings.Jail.
const (
	AutopilotJailNone = "none"
	AutopilotJailPay  = "pay"
	AutopilotJailRoll = "roll"
)

// MaxAutopilotLiquidity is the upper bound accepted for the reserve.
const MaxAutopilotLiquidity = 100000

// autopilotDelay is the pause between two autopilot actions.
const autopilotDelay = 800 * time.Millisecond

// AutopilotSettings is the complete owner-visible configuration.
type AutopilotSettings struct {
	Enabled     bool     `json:"enabled"`
	BuyAlways   bool     `json:"buyAlways"`
	Jail        string   `json:"jail"`
	BuildGroups []string `json:"buildGroups"` // always non-nil after validation
	Liquidity   int      `json:"liquidity"`
}

// AutopilotView is the per-seat view. Config is set only for the owner.
type AutopilotView struct {
	Active bool               `json:"active"`
	Config *AutopilotSettings `json:"config,omitempty"`
}

// autopilotStreetGroups lists the color groups the autopilot may build in.
var autopilotStreetGroups = []board.Group{
	board.GroupBraun,
	board.GroupHellblau,
	board.GroupRosa,
	board.GroupOrange,
	board.GroupRot,
	board.GroupGelb,
	board.GroupGruen,
	board.GroupDunkelblau,
}

// SetAutopilot replaces the seat's complete configuration.
// Rejections are *RuleError codes: game_over, not_permitted, invalid_autopilot.
func (g *Game) SetAutopilot(player int, settings AutopilotSettings) error {
	if g.Phase != PhasePlaying {
		return g.fail("game_over", "Das Spiel ist beendet.")
	}
	if player < 0 || player >= len(g.Players) || g.Players[player].Bankrupt {
		return g.fail("not_permitted", "Dieser Sitz kann den Autopilot nicht steuern.")
	}
	if settings.Liquidity < 0 {
		return g.fail("invalid_autopilot", "Die Liquidität muss mindestens 0 betragen.")
	}
	if settings.Liquidity > MaxAutopilotLiquidity {
		return g.fail("invalid_autopilot", "Die Liquidität ist zu hoch (maximal 100000 GE).")
	}
	jail := settings.Jail
	if jail == "" {
		jail = AutopilotJailNone
	}
	switch jail {
	case AutopilotJailNone, AutopilotJailPay, AutopilotJailRoll:
	default:
		return g.fail("invalid_autopilot", "Unbekannte Werkstatt-Option für den Autopilot.")
	}

	groups := make([]string, 0, len(settings.BuildGroups))
	seen := map[string]bool{}
	for _, name := range settings.BuildGroups {
		group := board.Group(name)
		if !isAutopilotStreetGroup(group) {
			return g.fail("invalid_autopilot", "Unbekannte Farbgruppe im Autopilot.")
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		groups = append(groups, name)
	}
	sort.Slice(groups, func(i, j int) bool {
		return g.groupTiles(board.Group(groups[i]))[0] < g.groupTiles(board.Group(groups[j]))[0]
	})

	p := g.Players[player]
	wasEnabled := p.Autopilot.Enabled
	settings.Jail = jail
	settings.BuildGroups = groups
	p.Autopilot = settings

	switch {
	case !wasEnabled && settings.Enabled:
		g.logAutopilot("autopilot_on", player, "", fmt.Sprintf("%s aktiviert den Autopilot.", p.Name))
		g.armAutopilot(player)
	case wasEnabled && !settings.Enabled:
		p.autopilotNextAt = time.Time{}
		text := fmt.Sprintf("%s schaltet den Autopilot aus.", p.Name)
		g.logAutopilot("autopilot_off", player, AutopilotStopManual, text)
	}
	return nil
}

// isAutopilotStreetGroup reports whether the group is one of the eight color
// groups the autopilot may build in.
func isAutopilotStreetGroup(group board.Group) bool {
	for _, allowed := range autopilotStreetGroups {
		if group == allowed {
			return true
		}
	}
	return false
}

// logAutopilot appends an event with an optional stop reason.
func (g *Game) logAutopilot(kind string, player int, reason, text string) {
	g.eventSeq++
	g.Events = append(g.Events, Event{
		Seq:    g.eventSeq,
		Type:   kind,
		Player: player,
		Reason: reason,
		Text:   text,
		At:     g.clock(),
	})
}

// armAutopilot schedules the seat's next action one delay from now. A disabled
// autopilot has no pending action.
func (g *Game) armAutopilot(player int) {
	if player < 0 || player >= len(g.Players) {
		return
	}
	p := g.Players[player]
	if !p.Autopilot.Enabled {
		p.autopilotNextAt = time.Time{}
		return
	}
	p.autopilotNextAt = g.clock().Add(autopilotDelay)
}

// stopAutopilot disables the seat with a visible event. It reports whether the
// autopilot was on before, so callers can decide whether a broadcast is needed.
func (g *Game) stopAutopilot(player int, reason, text string) bool {
	if player < 0 || player >= len(g.Players) {
		return false
	}
	p := g.Players[player]
	if !p.Autopilot.Enabled {
		return false
	}
	p.Autopilot.Enabled = false
	p.autopilotNextAt = time.Time{}
	g.logAutopilot("autopilot_off", player, reason, text)
	return true
}

// disableAutopilotSilently switches the seat off without an event (bankruptcy,
// game end, forced shutdown).
func (g *Game) disableAutopilotSilently(player int) {
	if player < 0 || player >= len(g.Players) {
		return
	}
	g.Players[player].Autopilot.Enabled = false
	g.Players[player].autopilotNextAt = time.Time{}
}

// tickAutopilot resolves expired auctions and runs at most one autopilot
// action. It reports whether a broadcast is needed. While a debt is open the
// whole game is blocked: no auction is resolved and no action is taken, so the
// invariant that no turn completes with an open debt cannot be broken here.
func (g *Game) tickAutopilot(now time.Time) bool {
	if g.Debtor >= 0 {
		return false
	}
	if g.Auction != nil {
		if now.Before(g.Auction.EndsAt) {
			return false
		}
		g.resolveAuction()
		return true
	}
	p := g.Players[g.Current]
	if !p.Autopilot.Enabled {
		return false
	}
	if !p.autopilotNextAt.IsZero() && now.Before(p.autopilotNextAt) {
		return false
	}
	p.autopilotNextAt = now.Add(autopilotDelay)

	switch g.TurnPhase {
	case TurnAwaitingRoll:
		if idx, ok := g.autopilotBuildCandidate(g.Current); ok {
			if err := g.Build(g.Current, idx); err == nil {
				return true
			}
		}
		_ = g.Roll(g.Current)
		return true
	case TurnAwaitingPurchase:
		return g.autopilotPurchase()
	case TurnAwaitingJail:
		return g.autopilotJail()
	}
	return false
}

// autopilotPurchase accepts an affordable purchase, or stops when the decision
// is not automated. It never declines or starts an auction.
func (g *Game) autopilotPurchase() bool {
	p := g.Players[g.Current]
	off := func(text string) bool {
		return g.stopAutopilot(g.Current, AutopilotStopPurchase, text)
	}
	if !p.Autopilot.BuyAlways {
		if g.Pending != nil {
			text := fmt.Sprintf(
				"%s schaltet den Autopilot aus: Immer kaufen ist aus (Kauf von %s).",
				p.Name, g.tile(g.Pending.Tile).Name,
			)
			return off(text)
		}
		return off(fmt.Sprintf(
			"%s schaltet den Autopilot aus: Kaufentscheidung nicht automatisiert.",
			p.Name,
		))
	}
	if g.Pending == nil {
		return off(fmt.Sprintf(
			"%s schaltet den Autopilot aus: Kaufentscheidung nicht automatisiert.",
			p.Name,
		))
	}
	if p.Cash < g.Pending.Price {
		text := fmt.Sprintf(
			"%s schaltet den Autopilot aus: Geld für %s reicht nicht.",
			p.Name, g.tile(g.Pending.Tile).Name,
		)
		return off(text)
	}
	if err := g.Buy(g.Current); err != nil {
		return off(fmt.Sprintf(
			"%s schaltet den Autopilot aus: Kaufentscheidung nicht automatisiert.",
			p.Name,
		))
	}
	return true
}

// autopilotJail executes the configured workshop option, or stops when none is
// executable. A held card is only used for "pay" (D9).
func (g *Game) autopilotJail() bool {
	p := g.Players[g.Current]
	off := func() bool {
		text := fmt.Sprintf("%s schaltet den Autopilot aus: keine ausführbare Werkstatt-Option.", p.Name)
		return g.stopAutopilot(g.Current, AutopilotStopJail, text)
	}
	switch p.Autopilot.Jail {
	case AutopilotJailPay:
		switch {
		case len(p.JailCards) > 0:
			if err := g.JailOption(g.Current, JailOptionCard); err != nil {
				return off()
			}
		case p.Cash >= g.Rules.JailFine:
			if err := g.JailOption(g.Current, JailOptionPay); err != nil {
				return off()
			}
		default:
			return off()
		}
		return true
	case AutopilotJailRoll:
		if err := g.JailOption(g.Current, JailOptionRoll); err != nil {
			return off()
		}
		return true
	default:
		return off()
	}
}

// autopilotBuildCandidate returns the street that should receive the next
// house: the globally lowest level among configured, complete, unmortgaged
// groups, restricted to the group minimum (even build) and affordable with the
// configured liquidity reserve. Hotels count as level 5.
func (g *Game) autopilotBuildCandidate(player int) (int, bool) {
	p := g.Players[player]
	best := -1
	bestLevel := 6
	for _, name := range p.Autopilot.BuildGroups {
		group := board.Group(name)
		if !g.ownsAllStreets(player, group) {
			continue
		}
		tiles := g.groupTiles(group)
		if len(tiles) == 0 {
			continue
		}
		minLevel := 5
		mortgaged := false
		for _, idx := range tiles {
			if g.Tiles[idx].Mortgaged {
				mortgaged = true
				break
			}
			if g.Tiles[idx].Houses < minLevel {
				minLevel = g.Tiles[idx].Houses
			}
		}
		if mortgaged || minLevel >= 5 {
			continue
		}
		for _, idx := range tiles {
			if g.Tiles[idx].Houses != minLevel {
				continue
			}
			if p.Cash-g.tile(idx).Build < p.Autopilot.Liquidity {
				continue
			}
			houses := g.Tiles[idx].Houses
			if houses < bestLevel || (houses == bestLevel && (best < 0 || idx < best)) {
				best = idx
				bestLevel = houses
			}
		}
	}
	return best, best >= 0
}
