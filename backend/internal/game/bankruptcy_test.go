package game

import (
	"strings"
	"testing"

	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/board"
)

// bankruptcyEvent returns the single bankruptcy event, failing when the number
// of bankruptcy events differs from want.
func bankruptcyEvent(t *testing.T, g *Game, want int) Event {
	t.Helper()
	var found []Event
	for _, e := range g.Events {
		if e.Type == "bankruptcy" {
			found = append(found, e)
		}
	}
	if len(found) != want {
		t.Fatalf("bankruptcy events = %d, want %d (events: %+v)", len(found), want, g.Events)
	}
	if want == 0 {
		return Event{}
	}
	return found[0]
}

func TestBankruptcyEventCarriesCauseAndAmounts(t *testing.T) {
	g, _ := newTestGame(t, 3)
	// Anleiheweg has Sell = 182 and is the payer's only realizable asset.
	g.Tiles[26].Owner = 0
	g.Players[0].Cash = 0 // paying 420 rent leaves exactly -420

	g.chargeRent(0, 1, 26, 420)

	e := bankruptcyEvent(t, g, 1)
	if e.Player != 0 {
		t.Errorf("event player = %d, want 0", e.Player)
	}
	if e.Cause != "miete" {
		t.Errorf("cause = %q, want %q", e.Cause, "miete")
	}
	// liquidation value = -420 + 182 = -238, so debt = 420, realizable = 182, shortfall = 238.
	if e.Debt != 420 {
		t.Errorf("debt = %d, want 420", e.Debt)
	}
	if e.Realizable != 182 {
		t.Errorf("realizable = %d, want 182", e.Realizable)
	}
	if e.Shortfall != 238 {
		t.Errorf("shortfall = %d, want 238", e.Shortfall)
	}
	for _, want := range []string{"Miete an B", "420 GE", "182 GE", "238 GE"} {
		if !strings.Contains(e.Text, want) {
			t.Errorf("event text %q does not contain %q", e.Text, want)
		}
	}
	if !g.Players[0].Bankrupt {
		t.Error("payer was not marked bankrupt")
	}
}

func TestBankruptcyCauseForAuction(t *testing.T) {
	g, _ := newTestGame(t, 3)
	g.Players[0].Cash = 10 // no assets, a 1000 bid is far beyond liquidation value
	g.Auction = &Auction{Tile: 1, Bidder: 0, Bid: 1000, MinBid: auctionBidStep}

	g.resolveAuction()

	e := bankruptcyEvent(t, g, 1)
	if e.Player != 0 {
		t.Errorf("event player = %d, want 0", e.Player)
	}
	if e.Cause != "auktion" {
		t.Errorf("cause = %q, want %q", e.Cause, "auktion")
	}
	tileName := g.tile(1).Name
	if want := "Auktionskauf für " + tileName; !strings.Contains(e.Text, want) {
		t.Errorf("event text %q does not contain %q", e.Text, want)
	}
	if !strings.Contains(e.Text, "Fehlbetrag") {
		t.Errorf("event text %q does not mention Fehlbetrag", e.Text)
	}
}

func TestBankruptcyCauseForCardPayments(t *testing.T) {
	tests := []struct {
		name   string
		cause  string
		player int
		setup  func(g *Game)
		act    func(g *Game)
	}{
		{
			name:   "kartenzahlung",
			cause:  "kartenzahlung",
			player: 0,
			setup: func(g *Game) {
				g.Players[0].Cash = -1000
				g.Decks[board.DeckChest].Draw = []string{"chest-hospital"}
			},
			act: func(g *Game) { g.drawAndApply(0, board.DeckChest, 0) },
		},
		{
			name:   "kartenreparatur",
			cause:  "kartenreparatur",
			player: 0,
			setup: func(g *Game) {
				g.Players[0].Cash = -1000
				g.Tiles[1].Owner = 0
				g.Tiles[1].Houses = 1
				g.Decks[board.DeckChest].Draw = []string{"chest-street-repair"}
			},
			act: func(g *Game) { g.drawAndApply(0, board.DeckChest, 0) },
		},
		{
			name:   "zahlung_an_alle",
			cause:  "zahlung_an_alle",
			player: 0,
			setup: func(g *Game) {
				g.Players[0].Cash = -1000
				g.Decks[board.DeckChance].Draw = []string{"chance-vorstand"}
			},
			act: func(g *Game) { g.drawAndApply(0, board.DeckChance, 0) },
		},
		{
			name:   "beitreibung",
			cause:  "beitreibung",
			player: 1,
			setup: func(g *Game) {
				g.Players[1].Cash = -1000
				g.Decks[board.DeckChest].Draw = []string{"chest-birthday"}
			},
			act: func(g *Game) { g.drawAndApply(0, board.DeckChest, 0) },
		},
		{
			name:   "steuer",
			cause:  "steuer",
			player: 0,
			setup: func(g *Game) {
				g.Players[0].Cash = -1000
			},
			act: func(g *Game) { g.resolveLanding(0, 4, 0) }, // Einkommensteuer, 200 GE
		},
		{
			name:   "reparaturkosten",
			cause:  "reparaturkosten",
			player: 0,
			setup: func(g *Game) {
				g.Players[0].Cash = -1000
				g.Players[0].InJail = true
				g.Players[0].JailAttempts = 2
				setDice(g, 1, 2) // third failed attempt, no doubles
			},
			act: func(g *Game) { g.attemptJailRoll(0) },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, _ := newTestGame(t, 3)
			tc.setup(g)
			tc.act(g)

			e := bankruptcyEvent(t, g, 1)
			if e.Player != tc.player {
				t.Errorf("event player = %d, want %d", e.Player, tc.player)
			}
			if e.Cause != tc.cause {
				t.Errorf("cause = %q, want %q", e.Cause, tc.cause)
			}
			if e.Shortfall <= 0 {
				t.Errorf("shortfall = %d, want a positive amount", e.Shortfall)
			}
		})
	}
}

func TestSettleWithoutCauseDefaultsToZahlung(t *testing.T) {
	g, _ := newTestGame(t, 3)
	g.Players[1].Cash = 5 // no assets -> -95 after paying 100

	g.settle(1, map[int]int{0: 100}) // two-argument legacy call

	e := bankruptcyEvent(t, g, 1)
	if e.Player != 1 {
		t.Errorf("event player = %d, want 1", e.Player)
	}
	if e.Cause != "zahlung" {
		t.Errorf("cause = %q, want %q", e.Cause, "zahlung")
	}
}

func TestVoluntaryLeaveKeepsGenericBankruptcyEvent(t *testing.T) {
	g, _ := newTestGame(t, 3)
	if err := g.Leave(1); err != nil {
		t.Fatalf("Leave(1) error = %v", err)
	}

	e := bankruptcyEvent(t, g, 1)
	if e.Cause != "" {
		t.Errorf("cause = %q, want empty for a voluntary leave", e.Cause)
	}
	if e.Debt != 0 || e.Realizable != 0 || e.Shortfall != 0 {
		t.Errorf("legacy event amounts = %d/%d/%d, want 0/0/0", e.Debt, e.Realizable, e.Shortfall)
	}
	want := "scheidet aus; die verbleibenden Werte gehen an die Bank."
	if !strings.Contains(e.Text, want) {
		t.Errorf("event text %q does not contain %q", e.Text, want)
	}
}

func TestNoDuplicateBankruptcyEventOnAutomaticInsolvency(t *testing.T) {
	g, _ := newTestGame(t, 3)
	g.Players[0].Cash = 5 // no assets -> undercoverable

	g.settle(0, map[int]int{1: 100})

	if n := countEvents(g, "bankruptcy"); n != 1 {
		t.Errorf("bankruptcy events = %d, want exactly 1", n)
	}
	if !g.Players[0].Bankrupt {
		t.Error("payer was not marked bankrupt")
	}
}
