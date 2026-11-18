package room

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/game"
)

// autopilotTestManager installs a mutable clock on the manager before any room
// is created. A room copies m.now at creation, so mutating the returned setter
// keeps the room tick and the game clock in agreement.
func autopilotTestManager(t *testing.T) (*Manager, func() time.Time, func(time.Time)) {
	t.Helper()
	m := newTestManager(t)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }
	return m, func() time.Time { return now }, func(next time.Time) { now = next }
}

// withGame mutates the room's game under the room lock. It is only used for
// states that no intent can reach deterministically (fixed dice, forced
// shutdown); ordinary flows go through HandleIntent.
func withGame(t *testing.T, r *Room, mutate func(*game.Game)) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.game == nil {
		t.Fatal("room has no game")
	}
	mutate(r.game)
}

// playerViewOf returns the public view of playerID inside the snapshot built
// for viewerID. It matters for autopilot: the viewer's own seat always carries
// the private config, so only another viewer proves what the owner keeps hidden.
func playerViewOf(t *testing.T, r *Room, viewerID, playerID string) game.PlayerView {
	t.Helper()
	snap := r.Snapshot(viewerID)
	if snap.Game == nil {
		t.Fatalf("no game state for viewer %q", viewerID)
	}
	for _, p := range snap.Game.Players {
		if p.ID == playerID {
			return p
		}
	}
	t.Fatalf("player %q not visible to %q", playerID, viewerID)
	return game.PlayerView{}
}

// nonCurrentID returns a seated user that does not hold the active seat.
func nonCurrentID(t *testing.T, r *Room, ids ...string) string {
	t.Helper()
	snap := r.Snapshot(ids[0])
	if snap.Game == nil {
		t.Fatal("no game state")
	}
	currentID := snap.Game.Players[snap.Game.Current].ID
	for _, id := range ids {
		if id != currentID {
			return id
		}
	}
	t.Fatalf("no non-current seat among %v", ids)
	return ""
}

// drainClientSends empties a test client's send queue.
func drainClientSends(c *Client) {
	for {
		select {
		case <-c.send:
		default:
			return
		}
	}
}

// wantRuleError asserts the code of an engine rejection.
func wantRuleError(t *testing.T, err error, code string) {
	t.Helper()
	re, ok := err.(*game.RuleError)
	if !ok {
		t.Fatalf("error = %v (%T), want *game.RuleError", err, err)
	}
	if re.Code != code {
		t.Errorf("code = %q, want %q", re.Code, code)
	}
}

func TestSetAutopilotIntent(t *testing.T) {
	m, _, _ := autopilotTestManager(t)
	r, _ := startGameWithClients(t, m, "host", "guest", "third")

	settings := game.AutopilotSettings{
		Enabled:     true,
		BuyAlways:   true,
		Jail:        game.AutopilotJailPay,
		BuildGroups: []string{"hellblau", "braun"},
		Liquidity:   200,
		HideCards:   true,
	}
	if err := r.HandleIntent("host", Intent{Type: "set_autopilot", Autopilot: &settings}); err != nil {
		t.Fatalf("set_autopilot error = %v", err)
	}

	owner := playerView(t, r, "host")
	if !owner.Autopilot.Active {
		t.Error("owner active = false, want true")
	}
	cfg := owner.Autopilot.Config
	if cfg == nil {
		t.Fatal("owner config = nil, want the full configuration")
	}
	if !cfg.Enabled || !cfg.BuyAlways {
		t.Errorf("owner config enabled/buyAlways = %v/%v, want true/true", cfg.Enabled, cfg.BuyAlways)
	}
	if cfg.Jail != game.AutopilotJailPay {
		t.Errorf("owner config jail = %q, want %q", cfg.Jail, game.AutopilotJailPay)
	}
	if cfg.Liquidity != 200 {
		t.Errorf("owner config liquidity = %d, want 200", cfg.Liquidity)
	}
	if !cfg.HideCards {
		t.Error("owner config hideCards = false, want true")
	}
	if want := []string{"braun", "hellblau"}; !slices.Equal(cfg.BuildGroups, want) {
		t.Errorf("owner config groups = %v, want %v", cfg.BuildGroups, want)
	}

	// An opponent sees the activity but never the configuration.
	opponent := playerViewOf(t, r, "guest", "host")
	if !opponent.Autopilot.Active {
		t.Error("opponent active = false, want true")
	}
	if opponent.Autopilot.Config != nil {
		t.Errorf("opponent config = %+v, want nil", opponent.Autopilot.Config)
	}

	// A user without a seat gets the spectator view.
	spectator := r.Snapshot("spectator-id")
	if spectator.You.Role != "spectator" || spectator.You.Seat != -1 {
		t.Fatalf("spectator you = %+v, want spectator without a seat", spectator.You)
	}
	if spectator.Game == nil {
		t.Fatal("no game state for the spectator")
	}
	seen := false
	for _, p := range spectator.Game.Players {
		if p.ID != "host" {
			continue
		}
		seen = true
		if !p.Autopilot.Active {
			t.Error("spectator active = false for the owner, want true")
		}
		if p.Autopilot.Config != nil {
			t.Errorf("spectator config = %+v, want nil", p.Autopilot.Config)
		}
	}
	if !seen {
		t.Fatal("owner not visible to the spectator")
	}

	// A missing payload is rejected without touching the stored config.
	wantRoomError(t, r.HandleIntent("host", Intent{Type: "set_autopilot"}), 400, "invalid_autopilot")
	if !playerView(t, r, "host").Autopilot.Active {
		t.Error("owner active = false after a rejected payload, want it unchanged")
	}

	// Switching off keeps the config private to the owner and says so.
	if err := r.HandleIntent("host", Intent{
		Type:      "set_autopilot",
		Autopilot: &game.AutopilotSettings{Enabled: false, BuyAlways: true},
	}); err != nil {
		t.Fatalf("set_autopilot off error = %v", err)
	}
	ownerAfter := playerView(t, r, "host")
	if ownerAfter.Autopilot.Active {
		t.Error("owner active = true after switching off")
	}
	if playerViewOf(t, r, "guest", "host").Autopilot.Active {
		t.Error("opponent active = true after switching off")
	}
	if cfg := ownerAfter.Autopilot.Config; cfg == nil || cfg.Enabled {
		t.Errorf("owner config after switching off = %+v, want a disabled config", cfg)
	}
}

func TestSetAutopilotIntentOutsideTurn(t *testing.T) {
	settings := game.AutopilotSettings{
		Enabled:     true,
		BuyAlways:   true,
		Jail:        game.AutopilotJailNone,
		BuildGroups: []string{"braun"},
	}
	ids := []string{"host", "guest", "third"}

	tests := []struct {
		name  string
		setup func(t *testing.T, r *Room) string // returns the sending user
	}{
		{
			name: "non-current seat",
			setup: func(t *testing.T, r *Room) string {
				return nonCurrentID(t, r, ids...)
			},
		},
		{
			name: "awaiting purchase",
			setup: func(t *testing.T, r *Room) string {
				withGame(t, r, func(g *game.Game) {
					g.Pending = &game.PendingPurchase{Tile: 1, Price: 60}
					g.TurnPhase = game.TurnAwaitingPurchase
				})
				return nonCurrentID(t, r, ids...)
			},
		},
		{
			name: "awaiting jail",
			setup: func(t *testing.T, r *Room) string {
				withGame(t, r, func(g *game.Game) {
					g.Players[g.Current].InJail = true
					g.TurnPhase = game.TurnAwaitingJail
				})
				return nonCurrentID(t, r, ids...)
			},
		},
		{
			name: "auction",
			setup: func(t *testing.T, r *Room) string {
				withGame(t, r, func(g *game.Game) {
					g.Auction = &game.Auction{
						Tile:   1,
						Bidder: game.Bank,
						MinBid: 20,
						EndsAt: g.Now().Add(15 * time.Second),
					}
					g.TurnPhase = game.TurnAuction
				})
				return nonCurrentID(t, r, ids...)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, _, _ := autopilotTestManager(t)
			r, _ := startGameWithClients(t, m, ids...)
			sender := tc.setup(t, r)
			if err := r.HandleIntent(sender, Intent{Type: "set_autopilot", Autopilot: &settings}); err != nil {
				t.Fatalf("set_autopilot by %q during %s error = %v, want nil", sender, tc.name, err)
			}
			if !playerView(t, r, sender).Autopilot.Active {
				t.Errorf("autopilot active = false for %q after enabling out of turn", sender)
			}
		})
	}

	t.Run("spectator", func(t *testing.T) {
		m, _, _ := autopilotTestManager(t)
		r, _ := startGameWithClients(t, m, ids...)
		wantRoomError(t,
			r.HandleIntent("watcher", Intent{Type: "set_autopilot", Autopilot: &settings}),
			403, "spectator")
	})

	t.Run("finished game", func(t *testing.T) {
		m, _, _ := autopilotTestManager(t)
		r, _ := startGameWithClients(t, m, ids...)
		withGame(t, r, func(g *game.Game) { g.ForceEnd() })
		wantRuleError(t,
			r.HandleIntent("host", Intent{Type: "set_autopilot", Autopilot: &settings}),
			"game_over")
	})
}

func TestAutopilotTickActsAndBroadcasts(t *testing.T) {
	m, now, setNow := autopilotTestManager(t)
	r, clients := startGameWithClients(t, m, "host", "guest", "third")

	before := r.Snapshot("host")
	if before.Game == nil {
		t.Fatal("no game state")
	}
	currentID := before.Game.Players[before.Game.Current].ID
	beforeSeq := 0
	if n := len(before.Game.Events); n > 0 {
		beforeSeq = before.Game.Events[n-1].Seq
	}

	settings := game.AutopilotSettings{Enabled: true, BuyAlways: true, Jail: game.AutopilotJailNone}
	if err := r.HandleIntent(currentID, Intent{Type: "set_autopilot", Autopilot: &settings}); err != nil {
		t.Fatalf("set_autopilot error = %v", err)
	}

	// The enabling push (and every earlier push) must not satisfy the assertion.
	client := clients[currentID]
	drainClientSends(client)

	// SetAutopilot arms the first action one delay after the current clock.
	setNow(now().Add(800 * time.Millisecond))
	r.tick(now())

	after := nextRoomState(t, client, func(s Snapshot) bool {
		if s.Game == nil {
			return false
		}
		for _, e := range s.Game.Events {
			if e.Seq > beforeSeq {
				return true
			}
		}
		return false
	})
	if len(after.Game.Events) <= len(before.Game.Events) {
		t.Fatalf("events = %d after the tick, want more than %d",
			len(after.Game.Events), len(before.Game.Events))
	}
	rolled := false
	for _, e := range after.Game.Events {
		if e.Seq > beforeSeq && e.Type == "roll" && e.Player == before.Game.Current {
			rolled = true
		}
	}
	if !rolled {
		t.Errorf("no roll event for seat %d after the autopilot tick: %+v",
			before.Game.Current, after.Game.Events)
	}
}

func TestAutopilotSnapshotJSON(t *testing.T) {
	m, _, _ := autopilotTestManager(t)
	r, _ := startGameWithClients(t, m, "host", "guest", "third")

	settings := game.AutopilotSettings{
		Enabled:     true,
		BuyAlways:   true,
		Jail:        game.AutopilotJailPay,
		BuildGroups: []string{"hellblau", "braun"},
		Liquidity:   200,
		HideCards:   true,
	}
	if err := r.HandleIntent("host", Intent{Type: "set_autopilot", Autopilot: &settings}); err != nil {
		t.Fatalf("set_autopilot error = %v", err)
	}

	ownerSnap := r.Snapshot("host")
	ownerJSON := marshalSnapshot(t, ownerSnap)
	if ownerSnap.You.Seat < 0 {
		t.Fatalf("owner seat = %d, want a seated player", ownerSnap.You.Seat)
	}

	// Owner: the full configuration is part of the autopilot object.
	ownerAP := autopilotJSON(t, ownerJSON, ownerSnap.You.Seat)
	for _, want := range []string{
		`"active":true`,
		`"config"`,
		`"enabled":true`,
		`"buyAlways":true`,
		`"jail":"pay"`,
		`"buildGroups":["braun","hellblau"]`,
		`"liquidity":200`,
		`"hideCards":true`,
	} {
		if !strings.Contains(ownerAP, want) {
			t.Errorf("owner autopilot JSON = %s, want %s", ownerAP, want)
		}
	}

	// Opponent: the owner's seat entry carries the activity flag only. The
	// opponent's own seat keeps its own config, so the check targets the owner
	// seat instead of the whole document.
	opponentSnap := r.Snapshot("guest")
	opponentJSON := marshalSnapshot(t, opponentSnap)
	if opponentSnap.You.Seat < 0 || opponentSnap.You.Seat == ownerSnap.You.Seat {
		t.Fatalf("opponent seat = %d, want a different seated player", opponentSnap.You.Seat)
	}
	opponentAP := autopilotJSON(t, opponentJSON, ownerSnap.You.Seat)
	if !strings.Contains(opponentAP, `"active":true`) {
		t.Errorf("opponent autopilot JSON = %s, want \"active\":true", opponentAP)
	}
	if strings.Contains(opponentAP, `"config"`) {
		t.Errorf("opponent autopilot JSON = %s, want no config", opponentAP)
	}

	// Spectators never receive a config at all.
	spectatorSnap := r.Snapshot("spectator-id")
	spectatorJSON := marshalSnapshot(t, spectatorSnap)
	if strings.Contains(string(spectatorJSON), `"config"`) {
		t.Errorf("spectator snapshot contains a config: %s", spectatorJSON)
	}
	if !strings.Contains(autopilotJSON(t, spectatorJSON, ownerSnap.You.Seat), `"active":true`) {
		t.Error("spectator autopilot JSON does not report the owner as active")
	}

	// Both documents keep the standard snapshot contract.
	var decoded Snapshot
	if err := json.Unmarshal(ownerJSON, &decoded); err != nil {
		t.Fatalf("owner snapshot JSON unmarshal error = %v", err)
	}
	if decoded.Code != ownerSnap.Code || decoded.Phase != "playing" || decoded.Game == nil {
		t.Errorf("decoded owner snapshot = %+v, want code/phase/playing state", decoded)
	}
	if decoded.You.Seat != ownerSnap.You.Seat || decoded.You.Role != "player" {
		t.Errorf("decoded owner you = %+v, want seat %d as player", decoded.You, ownerSnap.You.Seat)
	}
	if len(decoded.Game.Players) != 3 {
		t.Errorf("decoded players = %d, want 3", len(decoded.Game.Players))
	}
}

// marshalSnapshot serialises a snapshot and asserts it is valid JSON.
func marshalSnapshot(t *testing.T, snap Snapshot) []byte {
	t.Helper()
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("json.Marshal(snapshot) error = %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("snapshot is not valid JSON: %s", data)
	}
	return data
}

// autopilotJSON returns the serialized autopilot object of one seat inside a
// snapshot document.
func autopilotJSON(t *testing.T, data []byte, seat int) string {
	t.Helper()
	var body struct {
		Game *struct {
			Players []struct {
				Autopilot json.RawMessage `json:"autopilot"`
			} `json:"players"`
		} `json:"game"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("snapshot JSON unmarshal error = %v", err)
	}
	if body.Game == nil || seat < 0 || seat >= len(body.Game.Players) {
		t.Fatalf("seat %d not present in the snapshot", seat)
	}
	return string(body.Game.Players[seat].Autopilot)
}
