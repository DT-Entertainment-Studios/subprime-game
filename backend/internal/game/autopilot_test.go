package game

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"
)

// enableAutopilot enables the seat's autopilot with a complete test config.
func enableAutopilot(t *testing.T, g *Game, player int, settings AutopilotSettings) {
	t.Helper()
	settings.Enabled = true
	if err := g.SetAutopilot(player, settings); err != nil {
		t.Fatalf("SetAutopilot(%d) error = %v", player, err)
	}
}

func countEvents(g *Game, kind string) int {
	n := 0
	for _, e := range g.Events {
		if e.Type == kind {
			n++
		}
	}
	return n
}

func lastEvent(g *Game, kind string) *Event {
	for i := len(g.Events) - 1; i >= 0; i-- {
		if g.Events[i].Type == kind {
			return &g.Events[i]
		}
	}
	return nil
}

func TestAutopilotDefaults(t *testing.T) {
	g, _ := newTestGame(t, 3)
	for i, p := range g.Players {
		if p.Autopilot.Enabled {
			t.Errorf("player %d: Enabled = true, want false", i)
		}
		if !p.Autopilot.BuyAlways {
			t.Errorf("player %d: BuyAlways = false, want true", i)
		}
		if p.Autopilot.Jail != AutopilotJailNone {
			t.Errorf("player %d: Jail = %q, want %q", i, p.Autopilot.Jail, AutopilotJailNone)
		}
		if p.Autopilot.BuildGroups == nil {
			t.Errorf("player %d: BuildGroups = nil, want non-nil empty slice", i)
		}
		if len(p.Autopilot.BuildGroups) != 0 {
			t.Errorf("player %d: BuildGroups = %v, want empty", i, p.Autopilot.BuildGroups)
		}
		if p.Autopilot.Liquidity != 0 {
			t.Errorf("player %d: Liquidity = %d, want 0", i, p.Autopilot.Liquidity)
		}
		if p.Autopilot.HideCards {
			t.Errorf("player %d: HideCards = true, want false", i)
		}
		if !p.autopilotNextAt.IsZero() {
			t.Errorf("player %d: autopilotNextAt = %v, want zero", i, p.autopilotNextAt)
		}
	}
}

func TestSetAutopilotValidation(t *testing.T) {
	tests := []struct {
		name       string
		player     int
		setup      func(*Game)
		settings   AutopilotSettings
		wantCode   string
		wantJail   string
		wantGroups []string
	}{
		{
			name:     "negative liquidity",
			settings: AutopilotSettings{Liquidity: -1},
			wantCode: "invalid_autopilot",
		},
		{
			name:     "liquidity too high",
			settings: AutopilotSettings{Liquidity: MaxAutopilotLiquidity + 1},
			wantCode: "invalid_autopilot",
		},
		{
			name:     "unknown jail option",
			settings: AutopilotSettings{Jail: "teleport"},
			wantCode: "invalid_autopilot",
		},
		{
			name:     "unknown group",
			settings: AutopilotSettings{BuildGroups: []string{"lila"}},
			wantCode: "invalid_autopilot",
		},
		{
			name:     "non-street group",
			settings: AutopilotSettings{BuildGroups: []string{"tankstelle"}},
			wantCode: "invalid_autopilot",
		},
		{
			name:     "finished game",
			setup:    func(g *Game) { g.Phase = PhaseFinished },
			settings: AutopilotSettings{Enabled: true},
			wantCode: "game_over",
		},
		{
			name:     "bankrupt seat",
			player:   1,
			setup:    func(g *Game) { g.Players[1].Bankrupt = true },
			settings: AutopilotSettings{Enabled: true},
			wantCode: "not_permitted",
		},
		{
			name:     "unknown seat",
			player:   9,
			settings: AutopilotSettings{Enabled: true},
			wantCode: "not_permitted",
		},
		{
			name:       "empty jail becomes none",
			settings:   AutopilotSettings{},
			wantJail:   AutopilotJailNone,
			wantGroups: []string{},
		},
		{
			name:       "groups sorted and deduplicated",
			settings:   AutopilotSettings{BuildGroups: []string{"hellblau", "braun", "hellblau"}},
			wantJail:   AutopilotJailNone,
			wantGroups: []string{"braun", "hellblau"},
		},
		{
			name:       "jail and liquidity accepted",
			settings:   AutopilotSettings{Jail: AutopilotJailPay, Liquidity: MaxAutopilotLiquidity, BuildGroups: []string{"gruen"}},
			wantJail:   AutopilotJailPay,
			wantGroups: []string{"gruen"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, _ := newTestGame(t, 3)
			if tc.setup != nil {
				tc.setup(g)
			}
			err := g.SetAutopilot(tc.player, tc.settings)
			if tc.wantCode != "" {
				re, ok := err.(*RuleError)
				if !ok {
					t.Fatalf("SetAutopilot() error = %v (%T), want *RuleError", err, err)
				}
				if re.Code != tc.wantCode {
					t.Errorf("code = %q, want %q", re.Code, tc.wantCode)
				}
				if tc.player >= 0 && tc.player < len(g.Players) {
					p := g.Players[tc.player]
					if p.Autopilot.Enabled || len(p.Autopilot.BuildGroups) != 0 || p.Autopilot.Liquidity != 0 {
						t.Errorf("settings changed after a rejected update: %+v", p.Autopilot)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("SetAutopilot() error = %v", err)
			}
			p := g.Players[tc.player]
			if p.Autopilot.Jail != tc.wantJail {
				t.Errorf("jail = %q, want %q", p.Autopilot.Jail, tc.wantJail)
			}
			if p.Autopilot.BuildGroups == nil {
				t.Errorf("buildGroups = nil, want non-nil")
			}
			if !slices.Equal(p.Autopilot.BuildGroups, tc.wantGroups) {
				t.Errorf("buildGroups = %v, want %v", p.Autopilot.BuildGroups, tc.wantGroups)
			}
		})
	}
}

func TestSetAutopilotEvents(t *testing.T) {
	g, clock := newTestGame(t, 2)
	settings := AutopilotSettings{
		Enabled:     true,
		BuyAlways:   true,
		Jail:        AutopilotJailNone,
		BuildGroups: []string{"braun"},
		Liquidity:   0,
	}
	if err := g.SetAutopilot(0, settings); err != nil {
		t.Fatalf("SetAutopilot() error = %v", err)
	}
	p := g.Players[0]
	if !p.Autopilot.Enabled {
		t.Fatal("autopilot not enabled")
	}
	wantNext := clock.Now().Add(autopilotDelay)
	if !p.autopilotNextAt.Equal(wantNext) {
		t.Errorf("nextAt = %v, want %v", p.autopilotNextAt, wantNext)
	}
	on := lastEvent(g, "autopilot_on")
	if on == nil || on.Player != 0 || on.Reason != "" {
		t.Fatalf("autopilot_on event = %+v", on)
	}

	// Repeating the same state logs nothing and keeps the pacing.
	events := len(g.Events)
	if err := g.SetAutopilot(0, settings); err != nil {
		t.Fatalf("SetAutopilot() repeat error = %v", err)
	}
	if len(g.Events) != events {
		t.Errorf("repeat enable added %d events, want 0", len(g.Events)-events)
	}
	if !p.autopilotNextAt.Equal(wantNext) {
		t.Errorf("nextAt changed while still enabled: %v, want %v", p.autopilotNextAt, wantNext)
	}

	// Changing other settings while enabled does not reset the pacing.
	settings.Liquidity = 500
	if err := g.SetAutopilot(0, settings); err != nil {
		t.Fatalf("SetAutopilot() change error = %v", err)
	}
	if len(g.Events) != events {
		t.Errorf("settings change added %d events, want 0", len(g.Events)-events)
	}
	if !p.autopilotNextAt.Equal(wantNext) {
		t.Errorf("nextAt reset by a settings change: %v, want %v", p.autopilotNextAt, wantNext)
	}

	// Disabling logs the manual stop exactly once.
	settings.Enabled = false
	if err := g.SetAutopilot(0, settings); err != nil {
		t.Fatalf("SetAutopilot() disable error = %v", err)
	}
	if p.Autopilot.Enabled {
		t.Error("autopilot still enabled")
	}
	if !p.autopilotNextAt.IsZero() {
		t.Errorf("nextAt = %v after disabling, want zero", p.autopilotNextAt)
	}
	off := lastEvent(g, "autopilot_off")
	if off == nil || off.Player != 0 || off.Reason != AutopilotStopManual {
		t.Fatalf("autopilot_off event = %+v", off)
	}
	events = len(g.Events)
	if err := g.SetAutopilot(0, settings); err != nil {
		t.Fatalf("SetAutopilot() repeat disable error = %v", err)
	}
	if len(g.Events) != events {
		t.Errorf("repeat disable added %d events, want 0", len(g.Events)-events)
	}
}

func TestAutopilotPacing(t *testing.T) {
	g, clock := newTestGame(t, 2)
	setDice(g, 1, 2)
	enableAutopilot(t, g, 0, AutopilotSettings{BuyAlways: true})

	clock.Advance(autopilotDelay - time.Millisecond)
	if g.Tick(clock.Now()) {
		t.Error("Tick() 1 ms before the delay = true, want false")
	}
	if g.TurnPhase != TurnAwaitingRoll {
		t.Fatalf("turn phase = %q, want %q", g.TurnPhase, TurnAwaitingRoll)
	}
	if n := countEvents(g, "roll"); n != 0 {
		t.Errorf("rolls = %d before the delay, want 0", n)
	}

	clock.Advance(time.Millisecond)
	if !g.Tick(clock.Now()) {
		t.Fatal("Tick() at the delay = false, want true")
	}
	if got := countEvents(g, "roll"); got != 1 {
		t.Errorf("rolls = %d, want exactly 1", got)
	}
	if g.TurnPhase != TurnAwaitingPurchase {
		t.Errorf("turn phase = %q, want %q after the roll", g.TurnPhase, TurnAwaitingPurchase)
	}

	// A second tick in the same instant must not perform another action.
	if g.Tick(clock.Now()) {
		t.Error("second immediate Tick() = true, want false")
	}
	if got := countEvents(g, "roll"); got != 1 {
		t.Errorf("rolls after the second tick = %d, want 1", got)
	}
	if g.Tiles[3].Owner != Bank {
		t.Error("purchase happened in the same tick as the roll")
	}
}

func TestAutopilotDisabledDoesNothing(t *testing.T) {
	g, clock := newTestGame(t, 2)
	setDice(g, 1, 2)
	tiles := slices.Clone(g.Tiles)
	events := len(g.Events)

	// The default is opt-in: no SetAutopilot call and no arming.
	clock.Advance(10 * autopilotDelay)
	if g.Tick(clock.Now()) {
		t.Error("Tick() with the autopilot off = true, want false")
	}
	if g.Current != 0 || g.TurnPhase != TurnAwaitingRoll {
		t.Errorf("current = %d phase = %q, want 0/awaiting_roll", g.Current, g.TurnPhase)
	}
	if len(g.Events) != events {
		t.Errorf("events = %d, want %d (no action)", len(g.Events), events)
	}
	for i := range tiles {
		if g.Tiles[i] != tiles[i] {
			t.Errorf("tile %d changed: %+v, want %+v", i, g.Tiles[i], tiles[i])
		}
	}
}

func TestAutopilotNewTurnWaits(t *testing.T) {
	g, clock := newTestGame(t, 2)
	setDice(g, 4, 6) // the second seat lands on the workshop, no decision
	enableAutopilot(t, g, 0, AutopilotSettings{BuyAlways: true})
	enableAutopilot(t, g, 1, AutopilotSettings{BuyAlways: true})
	g.offerPurchase(0, 1)

	clock.Advance(autopilotDelay)
	if !g.Tick(clock.Now()) {
		t.Fatal("Tick() for the purchase = false, want true")
	}
	if g.Tiles[1].Owner != 0 {
		t.Fatalf("tile 1 owner = %d, want 0 after the purchase", g.Tiles[1].Owner)
	}
	if g.Current != 1 || g.TurnPhase != TurnAwaitingRoll {
		t.Fatalf("current = %d phase = %q, want 1/awaiting_roll", g.Current, g.TurnPhase)
	}
	rolls := countEvents(g, "roll")

	clock.Advance(autopilotDelay - time.Millisecond)
	if g.Tick(clock.Now()) {
		t.Error("Tick() 1 ms before the delay = true, want false")
	}
	if got := countEvents(g, "roll"); got != rolls {
		t.Errorf("rolls = %d, want %d before the delay", got, rolls)
	}
	if g.Current != 1 || g.TurnPhase != TurnAwaitingRoll {
		t.Errorf("current = %d phase = %q before the delay, want 1/awaiting_roll", g.Current, g.TurnPhase)
	}

	clock.Advance(time.Millisecond)
	if !g.Tick(clock.Now()) {
		t.Fatal("Tick() at the delay = false, want true")
	}
	if got := countEvents(g, "roll"); got != rolls+1 {
		t.Errorf("rolls = %d, want %d after the delay", got, rolls+1)
	}
}

func TestAutopilotRollAndDoubles(t *testing.T) {
	t.Run("non double advances the turn", func(t *testing.T) {
		g, clock := newTestGame(t, 2)
		setDice(g, 4, 6) // lands on the workshop, no decision
		enableAutopilot(t, g, 0, AutopilotSettings{BuyAlways: true})
		clock.Advance(autopilotDelay)
		if !g.Tick(clock.Now()) {
			t.Fatal("Tick() = false, want true")
		}
		if g.Current != 1 || g.TurnPhase != TurnAwaitingRoll {
			t.Errorf("current = %d phase = %q, want 1/awaiting_roll", g.Current, g.TurnPhase)
		}
	})

	t.Run("double grants an extra roll after another delay", func(t *testing.T) {
		g, clock := newTestGame(t, 2)
		setDice(g, 2, 2) // tax tile, no decision, double
		enableAutopilot(t, g, 0, AutopilotSettings{BuyAlways: true})
		clock.Advance(autopilotDelay)
		if !g.Tick(clock.Now()) {
			t.Fatal("Tick() = false, want true")
		}
		if g.Current != 0 || g.TurnPhase != TurnAwaitingRoll {
			t.Fatalf("current = %d phase = %q, want 0/awaiting_roll after the double", g.Current, g.TurnPhase)
		}
		if got := countEvents(g, "roll"); got != 1 {
			t.Fatalf("rolls = %d, want 1", got)
		}
		if g.Tick(clock.Now()) {
			t.Error("extra roll happened without the delay")
		}
		clock.Advance(autopilotDelay)
		if !g.Tick(clock.Now()) {
			t.Fatal("extra roll after the delay = false, want true")
		}
		if got := countEvents(g, "roll"); got != 2 {
			t.Errorf("rolls = %d, want 2", got)
		}
	})
}

func TestAutopilotPurchase(t *testing.T) {
	tests := []struct {
		name        string
		buyAlways   bool
		preSetup    func(*Game)
		postSetup   func(*Game)
		wantOwner   int
		wantCash    int
		wantEnabled bool
		wantReason  string
	}{
		{
			name:        "normal price",
			buyAlways:   true,
			wantOwner:   0,
			wantCash:    1440,
			wantEnabled: true,
		},
		{
			name:      "doubled price",
			buyAlways: true,
			preSetup: func(g *Game) {
				g.Tiles[3].Owner = 0 // completes the group with tile 1
			},
			wantOwner:   0,
			wantCash:    1380,
			wantEnabled: true,
		},
		{
			name:        "buy always off stops",
			buyAlways:   false,
			wantOwner:   Bank,
			wantCash:    1500,
			wantEnabled: false,
			wantReason:  AutopilotStopPurchase,
		},
		{
			name:      "insufficient cash stops",
			buyAlways: true,
			preSetup: func(g *Game) {
				g.Players[0].Cash = 10
			},
			wantOwner:   Bank,
			wantCash:    10,
			wantEnabled: false,
			wantReason:  AutopilotStopPurchase,
		},
		{
			name:      "missing pending stops defensively",
			buyAlways: true,
			postSetup: func(g *Game) {
				g.Pending = nil
			},
			wantOwner:   Bank,
			wantCash:    1500,
			wantEnabled: false,
			wantReason:  AutopilotStopPurchase,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, clock := newTestGame(t, 2)
			if tc.preSetup != nil {
				tc.preSetup(g)
			}
			g.offerPurchase(0, 1)
			if tc.postSetup != nil {
				tc.postSetup(g)
			}
			enableAutopilot(t, g, 0, AutopilotSettings{BuyAlways: tc.buyAlways})
			clock.Advance(autopilotDelay)
			g.Tick(clock.Now())

			if got := g.Tiles[1].Owner; got != tc.wantOwner {
				t.Errorf("owner = %d, want %d", got, tc.wantOwner)
			}
			if got := g.Players[0].Cash; got != tc.wantCash {
				t.Errorf("cash = %d, want %d", got, tc.wantCash)
			}
			if got := g.Players[0].Autopilot.Enabled; got != tc.wantEnabled {
				t.Errorf("enabled = %v, want %v", got, tc.wantEnabled)
			}
			if tc.wantReason != "" {
				off := lastEvent(g, "autopilot_off")
				if off == nil || off.Reason != tc.wantReason {
					t.Errorf("autopilot_off = %+v, want reason %q", off, tc.wantReason)
				}
			}
			if g.Auction != nil {
				t.Error("autopilot started an auction")
			}
		})
	}
}

func TestAutopilotJail(t *testing.T) {
	tests := []struct {
		name        string
		jail        string
		cash        int
		cards       []string
		dice        [2]int
		wantEnabled bool
		wantInJail  bool
		wantCash    int
		wantCards   int
		wantCurrent int
		wantReason  string
	}{
		{
			name:        "none stops",
			jail:        AutopilotJailNone,
			cash:        1500,
			wantEnabled: false,
			wantInJail:  true,
			wantCash:    1500,
			wantCurrent: 0,
			wantReason:  AutopilotStopJail,
		},
		{
			name:        "pay uses a held card",
			jail:        AutopilotJailPay,
			cash:        1500,
			cards:       []string{"chest-jail-free"},
			dice:        [2]int{1, 2},
			wantEnabled: true,
			wantInJail:  false,
			wantCash:    1500,
			wantCards:   0,
			wantCurrent: 0,
		},
		{
			name:        "pay without a card pays the fine",
			jail:        AutopilotJailPay,
			cash:        1500,
			dice:        [2]int{1, 2},
			wantEnabled: true,
			wantInJail:  false,
			wantCash:    1450,
			wantCards:   0,
			wantCurrent: 0,
		},
		{
			name:        "pay without enough cash stops",
			jail:        AutopilotJailPay,
			cash:        10,
			wantEnabled: false,
			wantInJail:  true,
			wantCash:    10,
			wantCurrent: 0,
			wantReason:  AutopilotStopJail,
		},
		{
			name:        "roll failure passes the turn",
			jail:        AutopilotJailRoll,
			cash:        1500,
			dice:        [2]int{1, 2},
			wantEnabled: true,
			wantInJail:  true,
			wantCash:    1500,
			wantCurrent: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, clock := newTestGame(t, 2)
			g.Players[0].InJail = true
			g.Players[0].Position = 10
			g.Players[0].Cash = tc.cash
			g.Players[0].JailCards = append([]string(nil), tc.cards...)
			g.TurnPhase = TurnAwaitingJail
			if tc.dice != [2]int{} {
				setDice(g, tc.dice[0], tc.dice[1])
			}
			enableAutopilot(t, g, 0, AutopilotSettings{Jail: tc.jail})
			clock.Advance(autopilotDelay)
			g.Tick(clock.Now())

			if got := g.Players[0].Autopilot.Enabled; got != tc.wantEnabled {
				t.Errorf("enabled = %v, want %v", got, tc.wantEnabled)
			}
			if got := g.Players[0].InJail; got != tc.wantInJail {
				t.Errorf("inJail = %v, want %v", got, tc.wantInJail)
			}
			if got := g.Players[0].Cash; got != tc.wantCash {
				t.Errorf("cash = %d, want %d", got, tc.wantCash)
			}
			if got := len(g.Players[0].JailCards); got != tc.wantCards {
				t.Errorf("jail cards = %d, want %d", got, tc.wantCards)
			}
			if got := g.Current; got != tc.wantCurrent {
				t.Errorf("current = %d, want %d", got, tc.wantCurrent)
			}
			if tc.wantReason != "" {
				off := lastEvent(g, "autopilot_off")
				if off == nil || off.Reason != tc.wantReason {
					t.Errorf("autopilot_off = %+v, want reason %q", off, tc.wantReason)
				}
			}
		})
	}
}

func TestAutopilotBuildCandidate(t *testing.T) {
	own := func(g *Game, tiles ...int) {
		for _, idx := range tiles {
			g.Tiles[idx].Owner = 0
		}
	}
	tests := []struct {
		name      string
		groups    []string
		cash      int
		liquidity int
		setup     func(*Game)
		want      int
		wantOK    bool
	}{
		{
			name:   "global lowest group wins over lower index",
			groups: []string{"braun", "hellblau"},
			setup: func(g *Game) {
				own(g, 1, 3)
				own(g, 6, 8, 9)
				g.Tiles[1].Houses = 1
				g.Tiles[3].Houses = 1
			},
			want:   6,
			wantOK: true,
		},
		{
			name:   "tie picks the lowest tile index",
			groups: []string{"braun", "hellblau"},
			setup: func(g *Game) {
				own(g, 1, 3)
				own(g, 6, 8, 9)
			},
			want:   1,
			wantOK: true,
		},
		{
			name:   "even build restricts to the group minimum",
			groups: []string{"braun"},
			setup: func(g *Game) {
				own(g, 1, 3)
				g.Tiles[1].Houses = 1
			},
			want:   3,
			wantOK: true,
		},
		{
			name:   "hotel counts as five and is not chosen again",
			groups: []string{"braun"},
			setup: func(g *Game) {
				own(g, 1, 3)
				g.Tiles[1].Houses = 5
				g.Tiles[3].Houses = 4
			},
			want:   3,
			wantOK: true,
		},
		{
			name:   "mortgaged group is skipped",
			groups: []string{"braun"},
			setup: func(g *Game) {
				own(g, 1, 3)
				g.Tiles[3].Mortgaged = true
			},
			wantOK: false,
		},
		{
			name:   "incomplete group is skipped",
			groups: []string{"braun"},
			setup: func(g *Game) {
				own(g, 1)
			},
			wantOK: false,
		},
		{
			name:   "all hotels are skipped",
			groups: []string{"braun"},
			setup: func(g *Game) {
				own(g, 1, 3)
				g.Tiles[1].Houses = 5
				g.Tiles[3].Houses = 5
			},
			wantOK: false,
		},
		{
			name:   "liquidity boundary is inclusive",
			groups: []string{"braun"},
			cash:   100,
			setup: func(g *Game) {
				own(g, 1, 3)
			},
			liquidity: 50,
			want:      1,
			wantOK:    true,
		},
		{
			name:   "liquidity above the boundary blocks",
			groups: []string{"braun"},
			cash:   100,
			setup: func(g *Game) {
				own(g, 1, 3)
			},
			liquidity: 51,
			wantOK:    false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, _ := newTestGame(t, 2)
			g.Players[0].Autopilot.BuildGroups = tc.groups
			g.Players[0].Autopilot.Liquidity = tc.liquidity
			if tc.cash != 0 {
				g.Players[0].Cash = tc.cash
			}
			if tc.setup != nil {
				tc.setup(g)
			}
			idx, ok := g.autopilotBuildCandidate(0)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (idx = %d)", ok, tc.wantOK, idx)
			}
			if tc.wantOK && idx != tc.want {
				t.Errorf("candidate = %d, want %d", idx, tc.want)
			}
			if !tc.wantOK && idx != -1 {
				t.Errorf("candidate = %d, want -1", idx)
			}
		})
	}
}

func TestAutopilotBuild(t *testing.T) {
	t.Run("one house per action respects liquidity", func(t *testing.T) {
		g, clock := newTestGame(t, 2)
		g.Tiles[1].Owner = 0
		g.Tiles[3].Owner = 0
		g.Players[0].Cash = 120
		setDice(g, 4, 6)
		enableAutopilot(t, g, 0, AutopilotSettings{BuildGroups: []string{"braun"}, Liquidity: 20})

		clock.Advance(autopilotDelay)
		if !g.Tick(clock.Now()) {
			t.Fatal("first Tick() = false, want true")
		}
		if got := g.Tiles[1].Houses + g.Tiles[3].Houses; got != 1 {
			t.Errorf("houses after one action = %d, want 1", got)
		}
		if g.Tiles[1].Houses != 1 {
			t.Errorf("tile 1 houses = %d, want 1 (lowest index)", g.Tiles[1].Houses)
		}
		if got := g.Players[0].Cash; got != 70 {
			t.Errorf("cash = %d, want 70", got)
		}

		clock.Advance(autopilotDelay)
		if !g.Tick(clock.Now()) {
			t.Fatal("second Tick() = false, want true")
		}
		if got := g.Tiles[1].Houses + g.Tiles[3].Houses; got != 2 {
			t.Errorf("houses after two actions = %d, want 2", got)
		}
		if got := g.Players[0].Cash; got != 20 {
			t.Errorf("cash = %d, want 20", got)
		}

		// No candidate fits the reserve any more, so the next action rolls.
		clock.Advance(autopilotDelay)
		if !g.Tick(clock.Now()) {
			t.Fatal("third Tick() = false, want true")
		}
		if got := g.Tiles[1].Houses + g.Tiles[3].Houses; got != 2 {
			t.Errorf("houses after the fallback = %d, want 2", got)
		}
		if g.Current != 1 {
			t.Errorf("current = %d, want 1 after the fallback roll", g.Current)
		}
	})

	t.Run("hotel is level five and falls back to roll", func(t *testing.T) {
		g, clock := newTestGame(t, 2)
		g.Tiles[1].Owner = 0
		g.Tiles[3].Owner = 0
		g.Tiles[1].Houses = 5
		g.Tiles[3].Houses = 4
		g.Players[0].Cash = 1000
		setDice(g, 4, 6)
		enableAutopilot(t, g, 0, AutopilotSettings{BuildGroups: []string{"braun"}})

		clock.Advance(autopilotDelay)
		if !g.Tick(clock.Now()) {
			t.Fatal("Tick() = false, want true")
		}
		if g.Tiles[3].Houses != 5 {
			t.Errorf("tile 3 houses = %d, want a hotel", g.Tiles[3].Houses)
		}
		if got := g.Players[0].Cash; got != 950 {
			t.Errorf("cash = %d, want 950", got)
		}

		clock.Advance(autopilotDelay)
		if !g.Tick(clock.Now()) {
			t.Fatal("second Tick() = false, want true")
		}
		if g.Tiles[1].Houses != 5 || g.Tiles[3].Houses != 5 {
			t.Errorf("hotels changed: %d/%d", g.Tiles[1].Houses, g.Tiles[3].Houses)
		}
		if g.Current != 1 {
			t.Errorf("current = %d, want 1 after the fallback roll", g.Current)
		}
	})
}

func TestAutopilotAuctionPause(t *testing.T) {
	g, clock := newTestGame(t, 2)
	setDice(g, 4, 6) // the second seat lands on the workshop, no decision
	enableAutopilot(t, g, 0, AutopilotSettings{BuyAlways: true})
	enableAutopilot(t, g, 1, AutopilotSettings{BuyAlways: true})
	g.offerPurchase(0, 1)
	if err := g.Decline(0); err != nil {
		t.Fatalf("Decline() error = %v", err)
	}
	if g.Auction == nil || g.TurnPhase != TurnAuction {
		t.Fatalf("auction = %+v phase = %q, want a running auction", g.Auction, g.TurnPhase)
	}
	if !g.Players[0].Autopilot.Enabled {
		t.Fatal("declining disabled the autopilot")
	}
	declines := countEvents(g, "decline")
	before := len(g.Events)

	clock.Advance(autopilotDelay)
	if g.Tick(clock.Now()) {
		t.Error("Tick() during the auction = true, want false")
	}
	if g.Auction == nil {
		t.Fatal("auction resolved before its deadline")
	}
	if len(g.Events) != before {
		t.Errorf("events during the auction = %d, want 0", len(g.Events)-before)
	}
	if g.Auction.Bidder != Bank {
		t.Errorf("auction bidder = %d, want the bank (no auto bids)", g.Auction.Bidder)
	}

	clock.Advance(16 * time.Second)
	if !g.Tick(clock.Now()) {
		t.Fatal("Tick() after the auction = false, want true")
	}
	if g.Auction != nil {
		t.Errorf("auction = %+v after expiry, want nil", g.Auction)
	}
	if !g.Players[0].Autopilot.Enabled {
		t.Error("autopilot was disabled by the auction")
	}
	if got := countEvents(g, "bid"); got != 0 {
		t.Errorf("bids = %d, want 0", got)
	}
	if got := countEvents(g, "decline"); got != declines {
		t.Errorf("declines = %d, want %d", got, declines)
	}

	// The paused autopilot resumes and the next seat acts after another delay.
	if g.Current != 1 {
		t.Fatalf("current = %d, want 1 after the auction", g.Current)
	}
	rolls := countEvents(g, "roll")
	clock.Advance(autopilotDelay)
	if !g.Tick(clock.Now()) {
		t.Fatal("Tick() for the next seat = false, want true")
	}
	if got := countEvents(g, "roll"); got != rolls+1 {
		t.Errorf("rolls = %d, want %d after the resumed delay", got, rolls+1)
	}
}

func TestAutopilotIncomingTradeStops(t *testing.T) {
	g, _ := newTestGame(t, 2)
	enableAutopilot(t, g, 0, AutopilotSettings{BuyAlways: true})
	enableAutopilot(t, g, 1, AutopilotSettings{BuyAlways: true})

	if _, err := g.ProposeTrade(1, TradeInput{To: 0, OfferCash: 10}); err != nil {
		t.Fatalf("ProposeTrade() error = %v", err)
	}
	if g.Players[0].Autopilot.Enabled {
		t.Error("recipient autopilot still enabled after an incoming offer")
	}
	if !g.Players[1].Autopilot.Enabled {
		t.Error("proposer autopilot was disabled by its own offer")
	}
	off := lastEvent(g, "autopilot_off")
	if off == nil || off.Player != 0 || off.Reason != AutopilotStopTradeOffer {
		t.Fatalf("autopilot_off = %+v, want player 0 with reason %q", off, AutopilotStopTradeOffer)
	}

	// Re-enabling while the offer is still open is allowed.
	if err := g.SetAutopilot(0, AutopilotSettings{Enabled: true, BuyAlways: true, Jail: AutopilotJailNone}); err != nil {
		t.Fatalf("SetAutopilot() while a trade is open error = %v", err)
	}
	if !g.Players[0].Autopilot.Enabled {
		t.Error("autopilot not enabled after re-enabling with an open trade")
	}
}

func TestAutopilotDebtStops(t *testing.T) {
	g, clock := newTestGame(t, 2)
	enableAutopilot(t, g, 0, AutopilotSettings{BuyAlways: true})
	g.Players[0].Cash = -10
	g.TurnPhase = TurnAwaitingDebt
	tiles := slices.Clone(g.Tiles)

	if !g.Tick(clock.Now()) {
		t.Fatal("Tick() during debt = false, want true")
	}
	if g.Players[0].Autopilot.Enabled {
		t.Error("autopilot still enabled during debt")
	}
	off := lastEvent(g, "autopilot_off")
	if off == nil || off.Player != 0 || off.Reason != AutopilotStopDebt {
		t.Fatalf("autopilot_off = %+v, want reason %q", off, AutopilotStopDebt)
	}
	if got := g.Players[0].Cash; got != -10 {
		t.Errorf("cash = %d, want -10 (no mortgage or sale)", got)
	}
	for i := range tiles {
		if g.Tiles[i] != tiles[i] {
			t.Errorf("tile %d changed: %+v, want %+v", i, g.Tiles[i], tiles[i])
		}
	}
}

func TestAutopilotSilentGameEnd(t *testing.T) {
	t.Run("ForceEnd disables silently", func(t *testing.T) {
		g, _ := newTestGame(t, 3)
		enableAutopilot(t, g, 0, AutopilotSettings{BuyAlways: true})
		enableAutopilot(t, g, 1, AutopilotSettings{BuyAlways: true})
		g.ForceEnd()
		for i, p := range g.Players {
			if p.Autopilot.Enabled {
				t.Errorf("player %d: autopilot still enabled", i)
			}
			if !p.autopilotNextAt.IsZero() {
				t.Errorf("player %d: nextAt not cleared", i)
			}
		}
		if got := countEvents(g, "autopilot_off"); got != 0 {
			t.Errorf("autopilot_off events = %d, want 0", got)
		}
	})

	t.Run("checkGameOver disables silently", func(t *testing.T) {
		g, _ := newTestGame(t, 2)
		enableAutopilot(t, g, 0, AutopilotSettings{BuyAlways: true})
		enableAutopilot(t, g, 1, AutopilotSettings{BuyAlways: true})
		if err := g.Leave(0); err != nil {
			t.Fatalf("Leave() error = %v", err)
		}
		if g.Phase != PhaseFinished || g.Winner != 1 {
			t.Fatalf("phase = %q winner = %d, want finished/1", g.Phase, g.Winner)
		}
		for i, p := range g.Players {
			if p.Autopilot.Enabled {
				t.Errorf("player %d: autopilot still enabled", i)
			}
			if !p.autopilotNextAt.IsZero() {
				t.Errorf("player %d: nextAt not cleared", i)
			}
		}
		if got := countEvents(g, "autopilot_off"); got != 0 {
			t.Errorf("autopilot_off events = %d, want 0", got)
		}
	})
}

func TestAutopilotStateVisibility(t *testing.T) {
	g, _ := newTestGame(t, 3)
	if err := g.SetAutopilot(0, AutopilotSettings{
		Enabled:     true,
		BuyAlways:   true,
		Jail:        AutopilotJailPay,
		BuildGroups: []string{"hellblau", "braun"},
		Liquidity:   200,
		HideCards:   true,
	}); err != nil {
		t.Fatalf("SetAutopilot() error = %v", err)
	}

	owner := g.StateFor(0)
	if owner.Players[0].Autopilot.Config == nil {
		t.Fatal("owner config = nil, want the full configuration")
	}
	if !owner.Players[0].Autopilot.Active {
		t.Error("owner active = false, want true")
	}
	if owner.Players[1].Autopilot.Config != nil {
		t.Error("other player config is visible in the owner snapshot")
	}
	if !slices.Equal(owner.Players[0].Autopilot.Config.BuildGroups, []string{"braun", "hellblau"}) {
		t.Errorf("config groups = %v, want sorted [braun hellblau]", owner.Players[0].Autopilot.Config.BuildGroups)
	}
	if !owner.Players[0].Autopilot.Config.HideCards {
		t.Error("owner config hideCards = false, want true")
	}

	other := g.StateFor(1)
	if other.Players[0].Autopilot.Config != nil {
		t.Error("opponent config is visible")
	}
	if !other.Players[0].Autopilot.Active {
		t.Error("opponent active = false, want true")
	}
	if other.Players[1].Autopilot.Config == nil {
		t.Error("own seat config = nil in StateFor(1)")
	}

	spectator := g.StateFor(-1)
	for i, p := range spectator.Players {
		if p.Autopilot.Config != nil {
			t.Errorf("player %d: spectator sees the config", i)
		}
	}
	if !spectator.Players[0].Autopilot.Active {
		t.Error("spectator active = false for player 0")
	}
	if spectator.Players[1].Autopilot.Active {
		t.Error("spectator active = true for an inactive player")
	}

	// State() is the spectator view.
	if g.State().Players[0].Autopilot.Config != nil {
		t.Error("State() exposes a config")
	}

	ownerJSON, err := json.Marshal(owner)
	if err != nil {
		t.Fatalf("marshal owner: %v", err)
	}
	if !strings.Contains(string(ownerJSON), `"buildGroups":["braun","hellblau"]`) {
		t.Errorf("owner JSON missing sorted groups: %s", ownerJSON)
	}
	if strings.Contains(string(ownerJSON), `"buildGroups":null`) {
		t.Errorf("owner JSON contains null groups: %s", ownerJSON)
	}

	otherJSON, err := json.Marshal(other)
	if err != nil {
		t.Fatalf("marshal other: %v", err)
	}
	if !strings.Contains(string(otherJSON), `"buildGroups":[]`) {
		t.Errorf("default config must serialize as an empty array: %s", otherJSON)
	}
	if strings.Contains(string(otherJSON), `"buildGroups":null`) {
		t.Errorf("other JSON contains null groups: %s", otherJSON)
	}

	spectatorJSON, err := json.Marshal(spectator)
	if err != nil {
		t.Fatalf("marshal spectator: %v", err)
	}
	if strings.Contains(string(spectatorJSON), `"config"`) {
		t.Errorf("spectator JSON contains a config: %s", spectatorJSON)
	}
}

func TestAutopilotDisconnectedSeat(t *testing.T) {
	g, clock := newTestGame(t, 2)
	g.Players[0].Connected = false
	setDice(g, 4, 6)
	enableAutopilot(t, g, 0, AutopilotSettings{BuyAlways: true})
	clock.Advance(autopilotDelay)
	if !g.Tick(clock.Now()) {
		t.Fatal("Tick() = false, want true")
	}
	if g.Current != 1 {
		t.Errorf("current = %d, want 1 (the disconnected seat must still act)", g.Current)
	}
	if got := countEvents(g, "roll"); got != 1 {
		t.Errorf("rolls = %d, want 1", got)
	}
}
