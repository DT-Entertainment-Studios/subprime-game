package room

import (
	"encoding/json"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/board"
	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/game"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	cat, err := board.New()
	if err != nil {
		t.Fatalf("board.New() error = %v", err)
	}
	m := NewManager(cat)
	m.now = func() time.Time { return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC) }
	m.seed = func() *rand.Rand { return rand.New(rand.NewSource(1)) }
	return m
}

func TestCreateRoomCode(t *testing.T) {
	m := newTestManager(t)
	r, err := m.Create("host", "Anna", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(r.Code()) != 6 {
		t.Errorf("code = %q, want 6 letters", r.Code())
	}
	for _, c := range r.Code() {
		if strings.ContainsRune("O0I1", c) {
			t.Errorf("code %q contains ambiguous character %q", r.Code(), c)
		}
	}
	snap := r.Snapshot("host")
	if snap.You.Role != "player" || !snap.You.IsHost {
		t.Errorf("you = %+v, want host player", snap.You)
	}
	if snap.CanStart {
		t.Error("CanStart = true for a single-player lobby, want false")
	}
	if _, err := r.Join("guest", "Ben", ""); err != nil {
		t.Fatalf("Join() error = %v", err)
	}
	if !r.Snapshot("host").CanStart {
		t.Error("CanStart = false with two players, want true")
	}
}

func TestCreateRejectsSecondLiveRoom(t *testing.T) {
	m := newTestManager(t)
	first, err := m.Create("host", "Anna", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	_, err = m.Create("host", "Anna", "")
	wantRoomError(t, err, 403, "already_in_room")
	if !strings.Contains(err.Error(), first.Code()) {
		t.Errorf("error = %q, want the existing room code %q", err, first.Code())
	}
}

func TestJoinRejectsDifferentLiveRoom(t *testing.T) {
	m := newTestManager(t)
	first, err := m.Create("host", "Anna", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	second, err := m.Create("guest", "Ben", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, _, err = m.Join(second.Code(), "host", "Anna", "")
	wantRoomError(t, err, 403, "already_in_room")
	if !strings.Contains(err.Error(), first.Code()) {
		t.Errorf("error = %q, want the existing room code %q", err, first.Code())
	}

	if _, snap, err := m.Join(strings.ToLower(first.Code()), "host", "Anna", ""); err != nil {
		t.Errorf("rejoining the same room error = %v, want nil", err)
	} else if snap.You.Role != "player" {
		t.Errorf("role = %q, want player", snap.You.Role)
	}
}

func TestJoinLobbyRejectsDifferentLiveRoom(t *testing.T) {
	m := newTestManager(t)
	first, err := m.Create("host", "Anna", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	second, err := m.Create("guest", "Ben", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, _, err = m.JoinLobby(second.Code(), "host", "Anna", "")
	wantRoomError(t, err, 403, "already_in_room")
	if !strings.Contains(err.Error(), first.Code()) {
		t.Errorf("error = %q, want the existing room code %q", err, first.Code())
	}

	if _, snap, err := m.JoinLobby(strings.ToLower(first.Code()), "host", "Anna", ""); err != nil {
		t.Errorf("rejoining the same lobby error = %v, want nil", err)
	} else if snap.You.Role != "player" {
		t.Errorf("role = %q, want player", snap.You.Role)
	}
}

func TestLiveRoomSeatBlocksUntilLeftOrFinished(t *testing.T) {
	m := newTestManager(t)
	lobby, err := m.Create("host", "Anna", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := lobby.LeaveLobby("host"); err != nil {
		t.Fatalf("LeaveLobby() error = %v", err)
	}
	if _, err := m.Create("host", "Anna", ""); err != nil {
		t.Errorf("Create() after leaving the lobby error = %v, want nil", err)
	}

	gameRoom, err := m.Create("alice", "Alice", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := gameRoom.Join("bob", "Bob", ""); err != nil {
		t.Fatalf("Join() error = %v", err)
	}
	if err := gameRoom.HandleIntent("alice", Intent{Type: "start_game"}); err != nil {
		t.Fatalf("start_game error = %v", err)
	}
	if _, err := m.Create("alice", "Alice", ""); err == nil {
		t.Error("Create() during a running game = nil, want already_in_room")
	} else {
		wantRoomError(t, err, 403, "already_in_room")
	}
	if err := gameRoom.HandleIntent("bob", Intent{Type: "leave_room"}); err != nil {
		t.Fatalf("leave_room error = %v", err)
	}
	if got := gameRoom.Phase(); got != "finished" {
		t.Fatalf("phase = %q, want finished", got)
	}

	other, err := m.Create("carol", "Cara", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := m.Create("alice", "Alice", ""); err != nil {
		t.Errorf("Create() after the game finished error = %v, want nil", err)
	}
	if _, _, err := m.Join(other.Code(), "bob", "Bob", ""); err != nil {
		t.Errorf("Join() after the game finished error = %v, want nil", err)
	}
}

func TestJoinAndStart(t *testing.T) {
	m := newTestManager(t)
	r, err := m.Create("host", "Anna", "rot")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := r.Join("guest", "Ben", "rot"); err != nil {
		t.Fatalf("Join() error = %v", err)
	}
	snap := r.Snapshot("guest")
	if len(snap.Seats) != 2 {
		t.Fatalf("seats = %d, want 2", len(snap.Seats))
	}
	if snap.Seats[1].Color == "rot" {
		t.Errorf("colors not unique: %+v", snap.Seats)
	}
	if err := r.HandleIntent("guest", Intent{Type: "start_game"}); err == nil {
		t.Error("guest could start the game")
	}
	if err := r.HandleIntent("host", Intent{Type: "start_game"}); err != nil {
		t.Fatalf("start error = %v", err)
	}
	if r.Phase() != "playing" {
		t.Errorf("phase = %q, want playing", r.Phase())
	}
	snap = r.Snapshot("guest")
	if snap.Game == nil || len(snap.Game.Players) != 2 {
		t.Fatalf("game state = %+v", snap.Game)
	}
}

func TestJoinAfterStartIsSpectator(t *testing.T) {
	m := newTestManager(t)
	r, _ := m.Create("host", "Anna", "")
	_, _ = r.Join("guest", "Ben", "")
	_ = r.HandleIntent("host", Intent{Type: "start_game"})
	if _, err := r.Join("watcher", "Cara", ""); err != nil {
		t.Fatalf("Join() error = %v", err)
	}
	snap := r.Snapshot("watcher")
	if snap.You.Role != "spectator" {
		t.Errorf("role = %q, want spectator", snap.You.Role)
	}
	if snap.You.Seat != -1 {
		t.Errorf("seat = %d, want -1", snap.You.Seat)
	}
}

func TestLobbyFull(t *testing.T) {
	m := newTestManager(t)
	r, _ := m.Create("host", "Host", "")
	for i := 0; i < 7; i++ {
		if _, err := r.Join(string(rune('a'+i)), "", ""); err != nil {
			t.Fatalf("Join(%d) error = %v", i, err)
		}
	}
	_, err := r.Join("overflow", "", "")
	if err == nil {
		t.Fatal("Join() = nil, want room full error")
	}
	if e, ok := err.(*Error); !ok || e.Status != 403 {
		t.Errorf("error = %+v, want 403", err)
	}
}

func TestHostPromotionOnLeave(t *testing.T) {
	m := newTestManager(t)
	r, _ := m.Create("host", "Anna", "")
	_, _ = r.Join("guest", "Ben", "")
	if err := r.HandleIntent("host", Intent{Type: "leave_room"}); err != nil {
		t.Fatalf("leave error = %v", err)
	}
	snap := r.Snapshot("guest")
	if snap.HostID != "guest" || !snap.You.IsHost {
		t.Errorf("host not promoted: hostId = %q you = %+v", snap.HostID, snap.You)
	}
	if len(snap.Seats) != 1 {
		t.Errorf("seats = %d, want 1", len(snap.Seats))
	}
}

func TestLeaveRunningGameBankrupts(t *testing.T) {
	m := newTestManager(t)
	r, _ := m.Create("host", "Anna", "")
	_, _ = r.Join("guest", "Ben", "")
	_ = r.HandleIntent("host", Intent{Type: "start_game"})
	if err := r.HandleIntent("guest", Intent{Type: "leave_room"}); err != nil {
		t.Fatalf("leave error = %v", err)
	}
	snap := r.Snapshot("host")
	if snap.Game == nil {
		t.Fatal("no game state")
	}
	found := false
	for _, p := range snap.Game.Players {
		if p.ID == "guest" {
			found = true
			if !p.Bankrupt {
				t.Error("leaving player was not bankrupted")
			}
		}
	}
	if !found {
		t.Error("guest player not found in game state")
	}
}

func TestForfeitIntent(t *testing.T) {
	m := newTestManager(t)
	r, _ := startGameWithClients(t, m, "host", "guest", "third")
	before := r.Snapshot("host")
	if before.Game == nil {
		t.Fatal("no game state")
	}
	activeID := before.Game.Players[before.Game.Current].ID
	if err := r.HandleIntent(activeID, Intent{Type: "forfeit"}); err != nil {
		t.Fatalf("forfeit error = %v", err)
	}
	if !playerView(t, r, activeID).Bankrupt {
		t.Error("forfeiting player was not bankrupted")
	}
	after := r.Snapshot("host")
	if after.Game == nil {
		t.Fatal("no game state after forfeiting")
	}
	if after.Game.Current == before.Game.Current {
		t.Errorf("current = %d after forfeiting, want a new seat", after.Game.Current)
	}
}

func TestForfeitIntentRejectsSpectator(t *testing.T) {
	m := newTestManager(t)
	r, _ := startGameWithClients(t, m, "host", "guest", "third")
	wantRoomError(t, r.HandleIntent("watcher", Intent{Type: "forfeit"}), 403, "spectator")
}

func TestForfeitIntentRejectsNonCurrent(t *testing.T) {
	m := newTestManager(t)
	r, _ := startGameWithClients(t, m, "host", "guest", "third")
	snap := r.Snapshot("host")
	if snap.Game == nil {
		t.Fatal("no game state")
	}
	activeID := snap.Game.Players[snap.Game.Current].ID
	otherID := ""
	for _, id := range []string{"host", "guest", "third"} {
		if id != activeID {
			otherID = id
			break
		}
	}
	wantRoomError(t, r.HandleIntent(otherID, Intent{Type: "forfeit"}), 403, "not_your_turn")
}

func TestForfeitIntentRejectsDebtPhase(t *testing.T) {
	m := newTestManager(t)
	r, _ := startGameWithClients(t, m, "host", "guest", "third")
	snap := r.Snapshot("host")
	if snap.Game == nil {
		t.Fatal("no game state")
	}
	activeID := snap.Game.Players[snap.Game.Current].ID
	r.mu.Lock()
	r.game.TurnPhase = game.TurnAwaitingDebt
	r.game.Players[r.game.Current].Cash = -5
	r.mu.Unlock()

	err := r.HandleIntent(activeID, Intent{Type: "forfeit"})
	re, ok := err.(*game.RuleError)
	if !ok {
		t.Fatalf("error = %v (%T), want *game.RuleError", err, err)
	}
	if re.Code != "wrong_phase" {
		t.Errorf("code = %q, want wrong_phase", re.Code)
	}
}

func TestCounterTradeOutOfTurn(t *testing.T) {
	m := newTestManager(t)
	r, _ := m.Create("host", "Anna", "")
	_, _ = r.Join("guest", "Ben", "")
	if err := r.HandleIntent("host", Intent{Type: "start_game"}); err != nil {
		t.Fatalf("start_game error = %v", err)
	}
	snap := r.Snapshot("host")
	if snap.Game == nil {
		t.Fatal("no game state")
	}
	// Seats are shuffled at start; find the active and the other player.
	activeID := snap.Game.Players[snap.Game.Current].ID
	otherID := "host"
	if activeID == "host" {
		otherID = "guest"
	}
	otherSeat := 0
	if snap.Game.Current == 0 {
		otherSeat = 1
	}
	// The active player proposes a cash trade before rolling.
	if err := r.HandleIntent(activeID, Intent{Type: "propose_trade", Trade: &game.TradeInput{To: otherSeat, OfferCash: 20, RequestCash: 10}}); err != nil {
		t.Fatalf("propose_trade error = %v", err)
	}
	snap = r.Snapshot(otherID)
	if snap.Game == nil || len(snap.Game.Trades) != 1 {
		t.Fatalf("trades = %+v", snap.Game)
	}
	id := snap.Game.Trades[0].ID
	// The other player is not active and must still be able to counter.
	counter := &game.TradeInput{To: snap.Game.Current, OfferCash: 10, RequestCash: 20, CounterTo: id}
	if err := r.HandleIntent(otherID, Intent{Type: "propose_trade", Trade: counter}); err != nil {
		t.Fatalf("counter propose_trade out of turn error = %v", err)
	}
	snap = r.Snapshot(otherID)
	open, countered := 0, 0
	for _, tr := range snap.Game.Trades {
		switch tr.Status {
		case game.TradeOpen:
			open++
		case game.TradeCountered:
			countered++
		}
	}
	if open != 1 || countered != 1 {
		t.Errorf("open/countered = %d/%d, want 1/1", open, countered)
	}
}

func TestProfileUpdateInLobby(t *testing.T) {
	m := newTestManager(t)
	r, _ := m.Create("host", "Anna", "rot")
	if err := r.HandleIntent("host", Intent{Type: "set_profile", Name: "Annelie", Color: "blau"}); err != nil {
		t.Fatalf("set_profile error = %v", err)
	}
	snap := r.Snapshot("host")
	if snap.Seats[0].Name != "Annelie" || snap.Seats[0].Color != "blau" {
		t.Errorf("seat = %+v", snap.Seats[0])
	}
}

func TestJoinLobby(t *testing.T) {
	m := newTestManager(t)
	r, err := m.Create("host", "Anna", "rot")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	_, snap, err := m.JoinLobby(r.Code(), "guest", "Ben", "rot")
	if err != nil {
		t.Fatalf("JoinLobby() error = %v", err)
	}
	if len(snap.Seats) != 2 {
		t.Fatalf("seats = %d, want 2", len(snap.Seats))
	}
	if snap.You.Role != "player" || snap.You.Name != "Ben" {
		t.Errorf("you = %+v, want player Ben", snap.You)
	}
	if snap.Seats[1].Name != "Ben" {
		t.Errorf("seat name = %q, want Ben", snap.Seats[1].Name)
	}
	if snap.Seats[1].Color == "rot" {
		t.Errorf("colors not unique: %+v", snap.Seats)
	}
	// Joining again updates the profile instead of adding a seat.
	snap, err = r.JoinLobby("guest", "Benno", "blau")
	if err != nil {
		t.Fatalf("JoinLobby() update error = %v", err)
	}
	if len(snap.Seats) != 2 {
		t.Fatalf("seats = %d after update, want 2", len(snap.Seats))
	}
	if snap.Seats[1].Name != "Benno" || snap.Seats[1].Color != "blau" {
		t.Errorf("seat = %+v, want Benno in blau", snap.Seats[1])
	}
}

func TestJoinLobbyRejectsFullRoom(t *testing.T) {
	m := newTestManager(t)
	r, _ := m.Create("host", "Host", "")
	for i := 0; i < maxPlayers-1; i++ {
		id := string(rune('a' + i))
		if _, err := r.JoinLobby(id, "", ""); err != nil {
			t.Fatalf("JoinLobby(%q) error = %v", id, err)
		}
	}
	_, err := r.JoinLobby("overflow", "", "")
	e, ok := err.(*Error)
	if !ok || e.Status != 403 || e.Code != "room_full" {
		t.Errorf("error = %+v, want 403 room_full", err)
	}
}

func TestJoinLobbyRejectsStartedRoom(t *testing.T) {
	m := newTestManager(t)
	r, _ := m.Create("host", "Anna", "")
	if _, err := r.Join("guest", "Ben", ""); err != nil {
		t.Fatalf("Join() error = %v", err)
	}
	if err := r.HandleIntent("host", Intent{Type: "start_game"}); err != nil {
		t.Fatalf("start_game error = %v", err)
	}
	_, err := r.JoinLobby("watcher", "Cara", "")
	e, ok := err.(*Error)
	if !ok || e.Status != 403 || e.Code != "game_started" {
		t.Errorf("error = %+v, want 403 game_started", err)
	}
	snap := r.Snapshot("watcher")
	if snap.You.Role != "spectator" || snap.You.Seat != -1 {
		t.Errorf("you = %+v, want a spectator without a seat", snap.You)
	}
	if snap.Game == nil || len(snap.Game.Players) != 2 {
		t.Errorf("game state = %+v, want two players", snap.Game)
	}
}

func TestJoinLobbyRejectsClosedRoom(t *testing.T) {
	m := newTestManager(t)
	r, _ := m.Create("host", "Anna", "")
	if _, err := r.Join("guest", "Ben", ""); err != nil {
		t.Fatalf("Join() error = %v", err)
	}
	if err := r.HandleIntent("host", Intent{Type: "leave_room"}); err != nil {
		t.Fatalf("leave host error = %v", err)
	}
	if err := r.HandleIntent("guest", Intent{Type: "leave_room"}); err != nil {
		t.Fatalf("leave guest error = %v", err)
	}
	_, err := r.JoinLobby("late", "Cara", "")
	e, ok := err.(*Error)
	if !ok || e.Status != 404 || e.Code != "room_not_found" {
		t.Errorf("error = %+v, want 404 room_not_found", err)
	}
	if _, _, err := m.JoinLobby(r.Code(), "late", "Cara", ""); err == nil {
		t.Error("Manager.JoinLobby() = nil for a closed room, want error")
	}
}

func TestLobbyInfo(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T, r *Room)
		wantOpen    bool
		wantMembers []string
	}{
		{
			name:        "fresh lobby is open",
			setup:       func(t *testing.T, r *Room) {},
			wantOpen:    true,
			wantMembers: []string{"Anna"},
		},
		{
			name: "full lobby is not open",
			setup: func(t *testing.T, r *Room) {
				for i := 0; i < maxPlayers-1; i++ {
					id := string(rune('a' + i))
					if _, err := r.JoinLobby(id, id, ""); err != nil {
						t.Fatalf("JoinLobby(%q) error = %v", id, err)
					}
				}
			},
			wantOpen:    false,
			wantMembers: []string{"Anna", "a", "b", "c", "d", "e", "f", "g"},
		},
		{
			name: "started room is not open",
			setup: func(t *testing.T, r *Room) {
				if _, err := r.Join("guest", "Ben", ""); err != nil {
					t.Fatalf("Join() error = %v", err)
				}
				if err := r.HandleIntent("host", Intent{Type: "start_game"}); err != nil {
					t.Fatalf("start_game error = %v", err)
				}
			},
			wantOpen:    false,
			wantMembers: []string{"Anna", "Ben"},
		},
		{
			name: "closed room is not open",
			setup: func(t *testing.T, r *Room) {
				if _, err := r.Join("guest", "Ben", ""); err != nil {
					t.Fatalf("Join() error = %v", err)
				}
				if err := r.HandleIntent("host", Intent{Type: "leave_room"}); err != nil {
					t.Fatalf("leave host error = %v", err)
				}
				if err := r.HandleIntent("guest", Intent{Type: "leave_room"}); err != nil {
					t.Fatalf("leave guest error = %v", err)
				}
			},
			wantOpen:    false,
			wantMembers: []string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestManager(t)
			r, err := m.Create("host", "Anna", "")
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			tc.setup(t, r)
			info := r.LobbyInfo()
			if info.Open != tc.wantOpen {
				t.Errorf("Open = %v, want %v", info.Open, tc.wantOpen)
			}
			got := make([]string, 0, len(info.Members))
			for _, member := range info.Members {
				got = append(got, member.Name)
			}
			if strings.Join(got, ",") != strings.Join(tc.wantMembers, ",") {
				t.Errorf("members = %v, want %v", got, tc.wantMembers)
			}
			if r.Phase() != "lobby" {
				return
			}
			seats := r.Snapshot("host").Seats
			if len(seats) != len(info.Members) {
				t.Fatalf("seats = %d, lobby members = %d", len(seats), len(info.Members))
			}
			for i, seat := range seats {
				if info.Members[i].UserID != seat.UserID || info.Members[i].Name != seat.Name {
					t.Errorf("member %d = %+v, want seat %+v", i, info.Members[i], seat)
				}
			}
		})
	}
}

func TestUserRoomAndHasMember(t *testing.T) {
	m := newTestManager(t)
	if code, ok := m.UserRoom("ghost"); ok {
		t.Errorf("UserRoom(ghost) = %q, %v, want no room", code, ok)
	}

	r, err := m.Create("host", "Anna", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !r.HasMember("host") {
		t.Error("HasMember(host) = false for the host seat")
	}
	if r.HasMember("ghost") {
		t.Error("HasMember(ghost) = true, want false")
	}

	// A lobby seat counts as "in a room" even without a room connection: the
	// user still occupies it and has to leave to become available again.
	if !r.InRoom("host") {
		t.Error("InRoom(host) = false for a lobby seat without a connection")
	}
	if code, ok := m.UserRoom("host"); !ok || code != r.Code() {
		t.Errorf("UserRoom(host) = %q, %v, want %q, true", code, ok, r.Code())
	}
	if got := m.InRoomSeats()["host"]; got != r.Code() {
		t.Errorf("InRoomSeats()[host] = %q, want %q", got, r.Code())
	}

	// A connection without a seat is a spectator and never counts as in a room.
	spectator := NewClient(nil, "watcher")
	r.AddClient(spectator)
	if r.InRoom("watcher") {
		t.Error("InRoom(watcher) = true for a spectator")
	}
	if _, ok := m.InRoomSeats()["watcher"]; ok {
		t.Error("InRoomSeats() lists a spectator")
	}

	if _, _, err := m.JoinLobby(r.Code(), "guest", "Ben", ""); err != nil {
		t.Fatalf("JoinLobby(guest) error = %v", err)
	}
	if !r.InRoom("guest") {
		t.Error("InRoom(guest) = false for a lobby seat")
	}
	guestClient := NewClient(nil, "guest")
	r.AddClient(guestClient)
	if !r.InRoom("guest") {
		t.Error("InRoom(guest) = false with a live room connection")
	}

	// A lobby seat keeps counting after the connection is gone.
	r.RemoveClient(guestClient)
	if !r.InRoom("guest") {
		t.Error("InRoom(guest) = false after RemoveClient, want the lobby seat to count")
	}
	if !r.HasMember("guest") {
		t.Error("HasMember(guest) = false after RemoveClient, want the seat kept")
	}

	if err := r.HandleIntent("guest", Intent{Type: "leave_room"}); err != nil {
		t.Fatalf("leave_room error = %v", err)
	}
	if r.HasMember("guest") {
		t.Error("HasMember(guest) = true after leave_room, want false")
	}
	if r.InRoom("guest") {
		t.Error("InRoom(guest) = true after leave_room, want false")
	}
	r.RemoveClient(spectator)
}

func TestInRoomFinishedGame(t *testing.T) {
	m := newTestManager(t)
	r, err := m.Create("host", "Anna", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := r.Join("guest", "Ben", ""); err != nil {
		t.Fatalf("Join() error = %v", err)
	}
	if err := r.HandleIntent("host", Intent{Type: "start_game"}); err != nil {
		t.Fatalf("start_game error = %v", err)
	}
	// A running-game seat counts even without a connection.
	if !r.InRoom("host") {
		t.Error("InRoom(host) = false in a running game without a connection")
	}

	// Leaving bankrupts the guest and finishes the two-player game. The host
	// keeps a seat, but it is stale now.
	if err := r.HandleIntent("guest", Intent{Type: "leave_room"}); err != nil {
		t.Fatalf("leave_room error = %v", err)
	}
	if got := r.Phase(); got != "finished" {
		t.Fatalf("phase = %q, want finished", got)
	}
	if !r.HasMember("host") {
		t.Fatal("HasMember(host) = false, want the seat to survive the game")
	}
	if r.InRoom("host") {
		t.Error("InRoom(host) = true for a finished game without a connection")
	}
	if code, ok := m.UserRoom("host"); ok {
		t.Errorf("UserRoom(host) = %q, %v for a finished game, want no room", code, ok)
	}
	if seats := m.InRoomSeats(); len(seats) != 0 {
		t.Errorf("InRoomSeats() = %v, want none", seats)
	}

	// A live room connection counts again: the user is watching the result.
	client := NewClient(nil, "host")
	r.AddClient(client)
	if !r.InRoom("host") {
		t.Error("InRoom(host) = false with a live room connection")
	}
	if got := m.InRoomSeats()["host"]; got != r.Code() {
		t.Errorf("InRoomSeats()[host] = %q, want %q", got, r.Code())
	}
	r.RemoveClient(client)
	if r.InRoom("host") {
		t.Error("InRoom(host) = true after RemoveClient in a finished game")
	}
}

func TestUserRoomRef(t *testing.T) {
	m := newTestManager(t)
	if ref, ok := m.UserRoomRef("ghost"); ok {
		t.Errorf("UserRoomRef(ghost) = %+v, true, want no room", ref)
	}

	r, err := m.Create("host", "Anna", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	ref, ok := m.UserRoomRef("host")
	if !ok || ref.Code != r.Code() || ref.Phase != "lobby" {
		t.Errorf("UserRoomRef(host) = %+v, %v, want {%s lobby}, true", ref, ok, r.Code())
	}

	// A connection without a seat is a spectator and never counts.
	addTestClient(t, r, "watcher")
	if ref, ok := m.UserRoomRef("watcher"); ok {
		t.Errorf("UserRoomRef(watcher) = %+v, true for a spectator, want no room", ref)
	}

	if _, err := r.Join("guest", "Ben", ""); err != nil {
		t.Fatalf("Join(guest) error = %v", err)
	}
	if err := r.HandleIntent("host", Intent{Type: "start_game"}); err != nil {
		t.Fatalf("start_game error = %v", err)
	}
	ref, ok = m.UserRoomRef("host")
	if !ok || ref.Code != r.Code() || ref.Phase != "playing" {
		t.Errorf("UserRoomRef(host) = %+v, %v while playing, want {%s playing}, true", ref, ok, r.Code())
	}

	// Leaving finishes the two-player game; the remaining seat is stale.
	if err := r.HandleIntent("guest", Intent{Type: "leave_room"}); err != nil {
		t.Fatalf("leave_room error = %v", err)
	}
	if got := r.Phase(); got != "finished" {
		t.Fatalf("phase = %q, want finished", got)
	}
	if ref, ok := m.UserRoomRef("host"); ok {
		t.Errorf("UserRoomRef(host) = %+v, true for a finished game without a connection, want no room",
			ref)
	}

	// A live room connection counts again and reports the finished phase.
	addTestClient(t, r, "host")
	ref, ok = m.UserRoomRef("host")
	if !ok || ref.Code != r.Code() || ref.Phase != "finished" {
		t.Errorf("UserRoomRef(host) = %+v, %v with a live connection, want {%s finished}, true",
			ref, ok, r.Code())
	}
}

func TestClientSendSignalsDropWithoutClosing(t *testing.T) {
	c := NewClient(nil, "host")
	for i := 0; i < cap(c.send); i++ {
		c.send <- outbound{Type: "state"}
	}
	c.Send(outbound{Type: "state"})
	if got := len(c.drop); got != 1 {
		t.Fatalf("drop = %d, want 1", got)
	}
	select {
	case <-c.done:
		t.Fatal("Send closed the connection synchronously")
	default:
	}
	// A second overflow must not block on the full drop channel either.
	c.Send(outbound{Type: "state"})
	if got := len(c.drop); got != 1 {
		t.Errorf("drop = %d after a second overflow, want 1", got)
	}
}

// addTestClient registers a live room connection for one user and removes it at
// the end of the test.
func addTestClient(t *testing.T, r *Room, userID string) *Client {
	t.Helper()
	c := NewClient(nil, userID)
	r.AddClient(c)
	t.Cleanup(func() { r.RemoveClient(c) })
	return c
}

// startGameWithClients creates a lobby with the given users, connects all of
// them and starts the game so every seat is connected.
func startGameWithClients(t *testing.T, m *Manager, ids ...string) (*Room, map[string]*Client) {
	t.Helper()
	if len(ids) < 2 {
		t.Fatalf("startGameWithClients needs at least two users, got %d", len(ids))
	}
	r, err := m.Create(ids[0], ids[0], "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	for _, id := range ids[1:] {
		if _, err := r.Join(id, id, ""); err != nil {
			t.Fatalf("Join(%q) error = %v", id, err)
		}
	}
	clients := make(map[string]*Client, len(ids))
	for _, id := range ids {
		clients[id] = addTestClient(t, r, id)
	}
	if err := r.HandleIntent(ids[0], Intent{Type: "start_game"}); err != nil {
		t.Fatalf("start_game error = %v", err)
	}
	return r, clients
}

// playerView returns the public game state of one player.
func playerView(t *testing.T, r *Room, userID string) game.PlayerView {
	t.Helper()
	snap := r.Snapshot(userID)
	if snap.Game == nil {
		t.Fatalf("no game state for %q", userID)
	}
	for _, p := range snap.Game.Players {
		if p.ID == userID {
			return p
		}
	}
	t.Fatalf("player %q not found in the game state", userID)
	return game.PlayerView{}
}

// findKickVote returns the votekick tally of one target.
func findKickVote(snap Snapshot, userID string) (KickVote, bool) {
	for _, vote := range snap.KickVotes {
		if vote.UserID == userID {
			return vote, true
		}
	}
	return KickVote{}, false
}

// wantRoomError asserts the status and code of a room rejection.
func wantRoomError(t *testing.T, err error, status int, code string) {
	t.Helper()
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("error = %v (%T), want *Error", err, err)
	}
	if e.Status != status || e.Code != code {
		t.Errorf("error = %+v, want %d %s", e, status, code)
	}
}

// assertSeatsJSON asserts the always-array contract of Seats and KickVotes.
func assertSeatsJSON(t *testing.T, snap Snapshot) {
	t.Helper()
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	body := string(data)
	if !strings.Contains(body, `"seats":[]`) {
		t.Errorf("snapshot = %s, want \"seats\":[]", body)
	}
	if strings.Contains(body, `"seats":null`) {
		t.Errorf("snapshot = %s, want no null seats", body)
	}
	if strings.Contains(body, `"kickVotes":null`) {
		t.Errorf("snapshot = %s, want no null kickVotes", body)
	}
}

func TestSnapshotSeatsAlwaysArray(t *testing.T) {
	m := newTestManager(t)
	r, err := m.Create("host", "Anna", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := r.Join("guest", "Ben", ""); err != nil {
		t.Fatalf("Join() error = %v", err)
	}
	if err := r.LeaveLobby("host"); err != nil {
		t.Fatalf("LeaveLobby(host) error = %v", err)
	}
	if err := r.LeaveLobby("guest"); err != nil {
		t.Fatalf("LeaveLobby(guest) error = %v", err)
	}

	snap := r.Snapshot("ghost")
	if snap.Seats == nil {
		t.Error("Seats = nil for an emptied lobby, want an empty slice")
	}
	if len(snap.Seats) != 0 {
		t.Errorf("seats = %d for an emptied lobby, want 0", len(snap.Seats))
	}
	if snap.KickVotes == nil {
		t.Error("KickVotes = nil, want an empty slice")
	}
	assertSeatsJSON(t, snap)

	// The playing and finished phases must serialise the same empty array.
	playing, _ := startGameWithClients(t, m, "a", "b")
	snap = playing.Snapshot("a")
	if snap.Seats == nil {
		t.Error("Seats = nil while playing, want an empty slice")
	}
	assertSeatsJSON(t, snap)

	if err := playing.HandleIntent("b", Intent{Type: "leave_room"}); err != nil {
		t.Fatalf("leave_room error = %v", err)
	}
	if got := playing.Phase(); got != "finished" {
		t.Fatalf("phase = %q, want finished", got)
	}
	snap = playing.Snapshot("a")
	if snap.Seats == nil {
		t.Error("Seats = nil after the game, want an empty slice")
	}
	if snap.KickVotes == nil || len(snap.KickVotes) != 0 {
		t.Errorf("kickVotes = %+v after the game, want an empty slice", snap.KickVotes)
	}
	assertSeatsJSON(t, snap)
}

func TestLeaveLobbyFreesSeatAndPromotesHost(t *testing.T) {
	m := newTestManager(t)
	r, err := m.Create("host", "Anna", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := r.Join("guest", "Ben", ""); err != nil {
		t.Fatalf("Join() error = %v", err)
	}
	if err := r.LeaveLobby("host"); err != nil {
		t.Fatalf("LeaveLobby(host) error = %v", err)
	}
	if r.HasMember("host") {
		t.Error("HasMember(host) = true after LeaveLobby, want the seat freed")
	}
	if r.InRoom("host") {
		t.Error("InRoom(host) = true after LeaveLobby, want the user available again")
	}
	snap := r.Snapshot("guest")
	if snap.HostID != "guest" || !snap.You.IsHost {
		t.Errorf("host not promoted: hostId = %q you = %+v", snap.HostID, snap.You)
	}
	if len(snap.Seats) != 1 || snap.Seats[0].UserID != "guest" {
		t.Errorf("seats = %+v, want only guest", snap.Seats)
	}
	if r.Closed() {
		t.Error("Closed() = true while a member remained")
	}
}

func TestLeaveLobbyLastMemberClosesRoom(t *testing.T) {
	m := newTestManager(t)
	r, err := m.Create("host", "Anna", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := r.Join("guest", "Ben", ""); err != nil {
		t.Fatalf("Join() error = %v", err)
	}
	if err := r.LeaveLobby("host"); err != nil {
		t.Fatalf("LeaveLobby(host) error = %v", err)
	}
	if r.Closed() {
		t.Fatal("Closed() = true after the first of two members left")
	}
	if err := r.LeaveLobby("guest"); err != nil {
		t.Fatalf("LeaveLobby(guest) error = %v", err)
	}
	if !r.Closed() {
		t.Fatal("Closed() = false after the last member left")
	}

	_, err = r.Join("late", "Cara", "")
	wantRoomError(t, err, 404, "room_not_found")
	_, err = r.JoinLobby("late", "Cara", "")
	wantRoomError(t, err, 404, "room_not_found")
	if _, ok := m.UserRoom("guest"); ok {
		t.Error("UserRoom(guest) found the closed room, want no room")
	}
}

func TestLeaveLobbyRejectsGame(t *testing.T) {
	m := newTestManager(t)
	r, err := m.Create("host", "Anna", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := r.Join("guest", "Ben", ""); err != nil {
		t.Fatalf("Join() error = %v", err)
	}
	if err := r.HandleIntent("host", Intent{Type: "start_game"}); err != nil {
		t.Fatalf("start_game error = %v", err)
	}
	wantRoomError(t, r.LeaveLobby("guest"), 403, "game_started")
	if !r.HasMember("guest") {
		t.Fatal("seat was freed by a rejected running-game leave")
	}

	if err := r.HandleIntent("guest", Intent{Type: "leave_room"}); err != nil {
		t.Fatalf("leave_room error = %v", err)
	}
	if got := r.Phase(); got != "finished" {
		t.Fatalf("phase = %q, want finished", got)
	}
	wantRoomError(t, r.LeaveLobby("host"), 400, "game_over")
	if !r.HasMember("host") {
		t.Fatal("seat was freed by a rejected finished-game leave")
	}
}

func TestLeaveLobbyIgnoresNonMember(t *testing.T) {
	m := newTestManager(t)
	r, err := m.Create("host", "Anna", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := r.Join("guest", "Ben", ""); err != nil {
		t.Fatalf("Join() error = %v", err)
	}
	before := r.Snapshot("watcher")
	if err := r.LeaveLobby("watcher"); err != nil {
		t.Fatalf("LeaveLobby(watcher) error = %v", err)
	}
	after := r.Snapshot("watcher")
	if len(after.Seats) != len(before.Seats) {
		t.Errorf("seats = %d after a spectator leave, want %d", len(after.Seats), len(before.Seats))
	}
	if after.HostID != "host" {
		t.Errorf("hostId = %q after a spectator leave, want host", after.HostID)
	}
	if r.Closed() {
		t.Error("Closed() = true after a spectator leave")
	}
}

// nextRoomState returns the next pushed state that satisfies pred. Test clients
// have no writer loop, so every Send is already queued when the call runs.
func nextRoomState(t *testing.T, c *Client, pred func(Snapshot) bool) Snapshot {
	t.Helper()
	for {
		select {
		case msg := <-c.send:
			if msg.Room != nil && pred(*msg.Room) {
				return *msg.Room
			}
		default:
			t.Fatal("no matching room state pushed")
			return Snapshot{}
		}
	}
}

func TestAbandonLobbyFreesSeatAndPromotesHost(t *testing.T) {
	m := newTestManager(t)
	r, err := m.Create("host", "Anna", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := r.Join("guest", "Ben", ""); err != nil {
		t.Fatalf("Join(guest) error = %v", err)
	}
	if err := r.Abandon("host"); err != nil {
		t.Fatalf("Abandon(host) error = %v", err)
	}
	if r.HasMember("host") {
		t.Error("HasMember(host) = true after Abandon, want the seat freed")
	}
	if code, ok := m.UserRoom("host"); ok {
		t.Errorf("UserRoom(host) = %q, true after Abandon, want no room", code)
	}
	snap := r.Snapshot("guest")
	if snap.HostID != "guest" || !snap.You.IsHost {
		t.Errorf("host not promoted: hostId = %q you = %+v", snap.HostID, snap.You)
	}
	if len(snap.Seats) != 1 || snap.Seats[0].UserID != "guest" {
		t.Errorf("seats = %+v, want only guest", snap.Seats)
	}
	if r.Closed() {
		t.Error("Closed() = true while a member remained")
	}
}

func TestAbandonLobbyLastMemberClosesRoom(t *testing.T) {
	m := newTestManager(t)
	r, err := m.Create("host", "Anna", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := r.Join("guest", "Ben", ""); err != nil {
		t.Fatalf("Join(guest) error = %v", err)
	}
	if err := r.Abandon("host"); err != nil {
		t.Fatalf("Abandon(host) error = %v", err)
	}
	if r.Closed() {
		t.Fatal("Closed() = true after the first of two members abandoned")
	}
	if err := r.Abandon("guest"); err != nil {
		t.Fatalf("Abandon(guest) error = %v", err)
	}
	if !r.Closed() {
		t.Fatal("Closed() = false after the last member abandoned")
	}

	_, err = r.JoinLobby("late", "Cara", "")
	wantRoomError(t, err, 404, "room_not_found")
	wantRoomError(t, r.Abandon("guest"), 404, "room_not_found")
}

func TestAbandonRunningGameEliminatesActivePlayer(t *testing.T) {
	m := newTestManager(t)
	r, _ := startGameWithClients(t, m, "host", "guest", "third")
	before := r.Snapshot("host")
	if before.Game == nil {
		t.Fatal("no game state")
	}
	activeID := before.Game.Players[before.Game.Current].ID
	if err := r.Abandon(activeID); err != nil {
		t.Fatalf("Abandon(%q) error = %v", activeID, err)
	}
	if !playerView(t, r, activeID).Bankrupt {
		t.Errorf("%q was not bankrupted by Abandon", activeID)
	}
	if r.HasMember(activeID) {
		t.Errorf("HasMember(%q) = true after Abandon, want the membership removed", activeID)
	}
	after := r.Snapshot("host")
	if after.Game == nil {
		t.Fatal("no game state after Abandon")
	}
	if after.Game.Phase != game.PhasePlaying {
		t.Fatalf("phase = %q, want playing", after.Game.Phase)
	}
	if got := after.Game.Players[after.Game.Current].ID; got == activeID {
		t.Error("turn is still on the abandoned player")
	}
}

func TestAbandonRunningGameKeepsCurrentPlayer(t *testing.T) {
	m := newTestManager(t)
	r, _ := startGameWithClients(t, m, "host", "guest", "third")
	before := r.Snapshot("host")
	if before.Game == nil {
		t.Fatal("no game state")
	}
	activeID := before.Game.Players[before.Game.Current].ID
	otherID := ""
	for _, id := range []string{"host", "guest", "third"} {
		if id != activeID {
			otherID = id
			break
		}
	}
	if err := r.Abandon(otherID); err != nil {
		t.Fatalf("Abandon(%q) error = %v", otherID, err)
	}
	if !playerView(t, r, otherID).Bankrupt {
		t.Errorf("%q was not bankrupted by Abandon", otherID)
	}
	if r.HasMember(otherID) {
		t.Errorf("HasMember(%q) = true after Abandon, want the membership removed", otherID)
	}
	after := r.Snapshot("host")
	if after.Game == nil {
		t.Fatal("no game state after Abandon")
	}
	if after.Game.Current != before.Game.Current {
		t.Errorf("current = %d after abandoning a non-active player, want %d",
			after.Game.Current, before.Game.Current)
	}
	if got := after.Game.Players[after.Game.Current].ID; got != activeID {
		t.Errorf("current player = %q, want %q", got, activeID)
	}
}

// TestAbandonRunningGameRemovesEliminatedMembership proves that abandoning as an
// already-eliminated running-game seat only drops the stale membership: the game
// is not touched a second time.
func TestAbandonRunningGameRemovesEliminatedMembership(t *testing.T) {
	m := newTestManager(t)
	r, _ := startGameWithClients(t, m, "host", "guest", "third")
	before := r.Snapshot("host")
	if before.Game == nil {
		t.Fatal("no game state")
	}
	currentID := before.Game.Players[before.Game.Current].ID
	if err := r.HandleIntent(currentID, Intent{Type: "forfeit"}); err != nil {
		t.Fatalf("HandleIntent(%q, forfeit) error = %v", currentID, err)
	}
	if !r.HasMember(currentID) {
		t.Fatalf("HasMember(%q) = false after forfeit, want the membership kept", currentID)
	}
	forfeited := r.Snapshot("host")
	if forfeited.Game == nil {
		t.Fatal("no game state after forfeit")
	}
	currentAfterForfeit := forfeited.Game.Current

	if err := r.Abandon(currentID); err != nil {
		t.Fatalf("Abandon(%q) error = %v", currentID, err)
	}
	if r.HasMember(currentID) {
		t.Errorf("HasMember(%q) = true after Abandon, want the membership removed", currentID)
	}
	if _, ok := m.UserRoom(currentID); ok {
		t.Errorf("UserRoom(%q) = true after Abandon, want false", currentID)
	}
	if !playerView(t, r, currentID).Bankrupt {
		t.Errorf("%q is no longer bankrupt after Abandon", currentID)
	}
	after := r.Snapshot("host")
	if after.Game == nil {
		t.Fatal("no game state after Abandon")
	}
	if after.Game.Phase != game.PhasePlaying {
		t.Errorf("game phase = %q after Abandon, want %q", after.Game.Phase, game.PhasePlaying)
	}
	if after.Game.Current != currentAfterForfeit {
		t.Errorf("current = %d after Abandon, want unchanged %d",
			after.Game.Current, currentAfterForfeit)
	}
}

func TestAbandonRunningGameFinishesWithOneLeft(t *testing.T) {
	m := newTestManager(t)
	r, _ := startGameWithClients(t, m, "host", "guest")
	if err := r.Abandon("guest"); err != nil {
		t.Fatalf("Abandon(guest) error = %v", err)
	}
	if got := r.Phase(); got != "finished" {
		t.Fatalf("phase = %q, want finished", got)
	}
	snap := r.Snapshot("host")
	if snap.Game == nil {
		t.Fatal("no game state")
	}
	if snap.Game.Phase != game.PhaseFinished {
		t.Errorf("game phase = %q, want finished", snap.Game.Phase)
	}
	if got := snap.Game.Players[snap.Game.Winner].ID; got != "host" {
		t.Errorf("winner = %q, want host", got)
	}
	if snap.Game.TurnPhase != game.TurnFinished {
		t.Errorf("turnPhase = %q, want %q", snap.Game.TurnPhase, game.TurnFinished)
	}
	if r.HasMember("guest") {
		t.Error("HasMember(guest) = true after Abandon, want the membership removed")
	}
}

func TestAbandonFinishedGameRemovesStaleSeat(t *testing.T) {
	m := newTestManager(t)
	r, _ := startGameWithClients(t, m, "host", "guest")
	if err := r.HandleIntent("guest", Intent{Type: "leave_room"}); err != nil {
		t.Fatalf("leave_room error = %v", err)
	}
	if got := r.Phase(); got != "finished" {
		t.Fatalf("phase = %q, want finished", got)
	}
	if !r.HasMember("host") {
		t.Fatal("HasMember(host) = false, want the stale seat to survive the game")
	}

	if err := r.Abandon("host"); err != nil {
		t.Fatalf("Abandon(host) error = %v", err)
	}
	if r.HasMember("host") {
		t.Error("HasMember(host) = true after Abandon, want the stale seat removed")
	}
	if r.InRoom("host") {
		t.Error("InRoom(host) = true after Abandon, want the user available again")
	}
	if code, ok := m.UserRoom("host"); ok {
		t.Errorf("UserRoom(host) = %q, true after Abandon, want no room", code)
	}
	if err := r.Abandon("host"); err != nil {
		t.Errorf("repeated Abandon(host) error = %v, want nil", err)
	}
}

func TestAbandonIgnoresNonMember(t *testing.T) {
	m := newTestManager(t)
	r, err := m.Create("host", "Anna", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := r.Join("guest", "Ben", ""); err != nil {
		t.Fatalf("Join(guest) error = %v", err)
	}
	before := r.Snapshot("watcher")
	if err := r.Abandon("watcher"); err != nil {
		t.Fatalf("Abandon(watcher) error = %v, want nil", err)
	}
	after := r.Snapshot("watcher")
	if len(after.Seats) != len(before.Seats) {
		t.Errorf("seats = %d after a spectator abandon, want %d", len(after.Seats), len(before.Seats))
	}
	if after.HostID != "host" {
		t.Errorf("hostId = %q after a spectator abandon, want host", after.HostID)
	}
	if r.Closed() {
		t.Error("Closed() = true after a spectator abandon")
	}
}

func TestAbandonOnlyTouchesCallerSeat(t *testing.T) {
	m := newTestManager(t)
	r, err := m.Create("host", "Anna", "rot")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, _, err := m.JoinLobby(r.Code(), "guest", "Ben", "blau"); err != nil {
		t.Fatalf("JoinLobby(guest) error = %v", err)
	}
	if err := r.Abandon("host"); err != nil {
		t.Fatalf("Abandon(host) error = %v", err)
	}
	after := r.Snapshot("guest")
	if len(after.Seats) != 1 {
		t.Fatalf("seats = %+v, want only the guest seat", after.Seats)
	}
	seat := after.Seats[0]
	if seat.UserID != "guest" || seat.Name != "Ben" || seat.Color != "blau" || !seat.Host {
		t.Errorf("seat = %+v, want guest/Ben/blau as host", seat)
	}
	if after.HostID != "guest" {
		t.Errorf("hostId = %q, want guest", after.HostID)
	}
}

func TestAbandonBroadcasts(t *testing.T) {
	m := newTestManager(t)
	r, err := m.Create("host", "Anna", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := r.Join("guest", "Ben", ""); err != nil {
		t.Fatalf("Join(guest) error = %v", err)
	}
	guest := addTestClient(t, r, "guest")
	if err := r.Abandon("host"); err != nil {
		t.Fatalf("Abandon(host) error = %v", err)
	}
	snap := nextRoomState(t, guest, func(s Snapshot) bool { return len(s.Seats) == 1 })
	if snap.Seats[0].UserID != "guest" || !snap.Seats[0].Host {
		t.Errorf("seats = %+v, want guest as host", snap.Seats)
	}
	if snap.HostID != "guest" {
		t.Errorf("hostId = %q, want guest", snap.HostID)
	}
}

func TestVotekickRemovesPausedSeat(t *testing.T) {
	m := newTestManager(t)
	r, clients := startGameWithClients(t, m, "host", "guest", "third")
	r.RemoveClient(clients["guest"])
	if playerView(t, r, "guest").Connected {
		t.Fatal("target seat is still connected")
	}

	if err := r.HandleIntent("host", Intent{Type: "votekick", TargetID: "guest"}); err != nil {
		t.Fatalf("first votekick error = %v", err)
	}
	if playerView(t, r, "guest").Bankrupt {
		t.Fatal("target was kicked after one of two votes")
	}
	vote, ok := findKickVote(r.Snapshot("host"), "guest")
	if !ok || vote.Votes != 1 || !vote.Voted {
		t.Errorf("host vote = %+v (found %v), want 1 vote and voted", vote, ok)
	}
	vote, ok = findKickVote(r.Snapshot("third"), "guest")
	if !ok || vote.Votes != 1 || vote.Voted {
		t.Errorf("third vote = %+v (found %v), want 1 vote and not voted", vote, ok)
	}

	if err := r.HandleIntent("third", Intent{Type: "votekick", TargetID: "guest"}); err != nil {
		t.Fatalf("second votekick error = %v", err)
	}
	if !playerView(t, r, "guest").Bankrupt {
		t.Fatal("target was not eliminated after the unanimous vote")
	}
	if !r.HasMember("guest") {
		t.Error("the kicked seat left the lobby member map, want it kept")
	}
	snap := r.Snapshot("host")
	if len(snap.KickVotes) != 0 {
		t.Errorf("kickVotes = %+v after the kick, want empty", snap.KickVotes)
	}
	found := false
	for _, event := range snap.Game.Events {
		if event.Type == "bankruptcy" {
			found = true
		}
	}
	if !found {
		t.Error("no bankruptcy event in the event log")
	}
}

func TestVotekickRequiresEveryConnectedPlayer(t *testing.T) {
	m := newTestManager(t)
	r, clients := startGameWithClients(t, m, "host", "guest", "third", "fourth")
	r.RemoveClient(clients["guest"])
	if err := r.HandleIntent("host", Intent{Type: "votekick", TargetID: "guest"}); err != nil {
		t.Fatalf("votekick error = %v", err)
	}
	if playerView(t, r, "guest").Bankrupt {
		t.Fatal("target was kicked although an eligible voter abstained")
	}
}

func TestVotekickIgnoresDisconnectedVoter(t *testing.T) {
	m := newTestManager(t)
	r, clients := startGameWithClients(t, m, "host", "guest", "third", "fourth")
	r.RemoveClient(clients["guest"])
	r.RemoveClient(clients["fourth"])
	if err := r.HandleIntent("host", Intent{Type: "votekick", TargetID: "guest"}); err != nil {
		t.Fatalf("host votekick error = %v", err)
	}
	if err := r.HandleIntent("third", Intent{Type: "votekick", TargetID: "guest"}); err != nil {
		t.Fatalf("third votekick error = %v", err)
	}
	if !playerView(t, r, "guest").Bankrupt {
		t.Fatal("target was not kicked although every connected voter voted")
	}
}

func TestVotekickRejectsInvalidCalls(t *testing.T) {
	kickGuest := func(t *testing.T, m *Manager) *Room {
		t.Helper()
		r, clients := startGameWithClients(t, m, "host", "guest", "third")
		r.RemoveClient(clients["guest"])
		if err := r.HandleIntent("host", Intent{Type: "votekick", TargetID: "guest"}); err != nil {
			t.Fatalf("first votekick error = %v", err)
		}
		if err := r.HandleIntent("third", Intent{Type: "votekick", TargetID: "guest"}); err != nil {
			t.Fatalf("second votekick error = %v", err)
		}
		if !playerView(t, r, "guest").Bankrupt {
			t.Fatal("target was not eliminated")
		}
		return r
	}
	tests := []struct {
		name   string
		setup  func(t *testing.T, m *Manager) (*Room, string, string)
		status int
		code   string
	}{
		{
			name: "lobby",
			setup: func(t *testing.T, m *Manager) (*Room, string, string) {
				r, err := m.Create("host", "Anna", "")
				if err != nil {
					t.Fatalf("Create() error = %v", err)
				}
				if _, err := r.Join("guest", "Ben", ""); err != nil {
					t.Fatalf("Join() error = %v", err)
				}
				return r, "host", "guest"
			},
			status: 400,
			code:   "not_in_game",
		},
		{
			name: "finished",
			setup: func(t *testing.T, m *Manager) (*Room, string, string) {
				r, _ := startGameWithClients(t, m, "host", "guest")
				if err := r.HandleIntent("guest", Intent{Type: "leave_room"}); err != nil {
					t.Fatalf("leave_room error = %v", err)
				}
				if got := r.Phase(); got != "finished" {
					t.Fatalf("phase = %q, want finished", got)
				}
				return r, "host", "guest"
			},
			status: 400,
			code:   "game_over",
		},
		{
			name: "spectator",
			setup: func(t *testing.T, m *Manager) (*Room, string, string) {
				r, _ := startGameWithClients(t, m, "host", "guest", "third")
				return r, "watcher", "guest"
			},
			status: 403,
			code:   "spectator",
		},
		{
			name: "eliminated voter",
			setup: func(t *testing.T, m *Manager) (*Room, string, string) {
				return kickGuest(t, m), "guest", "third"
			},
			status: 403,
			code:   "eliminated",
		},
		{
			name: "self target",
			setup: func(t *testing.T, m *Manager) (*Room, string, string) {
				r, _ := startGameWithClients(t, m, "host", "guest", "third")
				return r, "host", "host"
			},
			status: 400,
			code:   "invalid_target",
		},
		{
			name: "unknown target",
			setup: func(t *testing.T, m *Manager) (*Room, string, string) {
				r, _ := startGameWithClients(t, m, "host", "guest", "third")
				return r, "host", "ghost"
			},
			status: 400,
			code:   "invalid_target",
		},
		{
			name: "eliminated target",
			setup: func(t *testing.T, m *Manager) (*Room, string, string) {
				return kickGuest(t, m), "host", "guest"
			},
			status: 400,
			code:   "invalid_target",
		},
		{
			name: "connected target",
			setup: func(t *testing.T, m *Manager) (*Room, string, string) {
				r, _ := startGameWithClients(t, m, "host", "guest", "third")
				return r, "host", "third"
			},
			status: 400,
			code:   "target_connected",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestManager(t)
			r, voter, target := tc.setup(t, m)
			wantRoomError(t, r.HandleIntent(voter, Intent{Type: "votekick", TargetID: target}),
				tc.status, tc.code)
		})
	}
}

func TestVotekickClearsVotesOnReconnect(t *testing.T) {
	m := newTestManager(t)
	r, clients := startGameWithClients(t, m, "host", "guest", "third")
	r.RemoveClient(clients["guest"])
	if err := r.HandleIntent("host", Intent{Type: "votekick", TargetID: "guest"}); err != nil {
		t.Fatalf("votekick error = %v", err)
	}
	if votes := r.Snapshot("host").KickVotes; len(votes) != 1 {
		t.Fatalf("kickVotes = %+v after one vote, want one entry", votes)
	}

	if _, err := r.Join("guest", "Guest", ""); err != nil {
		t.Fatalf("Join(guest) error = %v", err)
	}
	if votes := r.Snapshot("host").KickVotes; len(votes) != 0 {
		t.Errorf("kickVotes = %+v after the target reconnected, want empty", votes)
	}
	if !playerView(t, r, "guest").Connected {
		t.Error("Join did not reconnect the target seat")
	}
}

func TestVotekickClearsVotesRequiringReconnect(t *testing.T) {
	m := newTestManager(t)
	r, clients := startGameWithClients(t, m, "host", "guest", "third")
	r.RemoveClient(clients["guest"])
	if err := r.HandleIntent("host", Intent{Type: "votekick", TargetID: "guest"}); err != nil {
		t.Fatalf("votekick error = %v", err)
	}
	if votes := r.Snapshot("host").KickVotes; len(votes) != 1 {
		t.Fatalf("kickVotes = %+v after one vote, want one entry", votes)
	}

	r.AddClient(NewClient(nil, "guest"))
	if votes := r.Snapshot("host").KickVotes; len(votes) != 0 {
		t.Errorf("kickVotes = %+v after the room socket reconnected, want empty", votes)
	}
	if !playerView(t, r, "guest").Connected {
		t.Error("AddClient did not reconnect the target seat")
	}
}

func TestVotekickIsIdempotent(t *testing.T) {
	m := newTestManager(t)
	r, clients := startGameWithClients(t, m, "host", "guest", "third")
	r.RemoveClient(clients["guest"])
	for i := 0; i < 2; i++ {
		if err := r.HandleIntent("host", Intent{Type: "votekick", TargetID: "guest"}); err != nil {
			t.Fatalf("vote %d error = %v", i+1, err)
		}
	}
	vote, ok := findKickVote(r.Snapshot("host"), "guest")
	if !ok || vote.Votes != 1 {
		t.Errorf("vote = %+v (found %v), want exactly one vote", vote, ok)
	}
	if playerView(t, r, "guest").Bankrupt {
		t.Error("target was kicked by a repeated vote")
	}
}

// TestSnapshotOmitsVotesForConnectedSeats pins the snapshot contract that
// kickVotes is only published for paused seats, even if a stale tally survives
// a reconnect that forgot to clear it.
func TestSnapshotOmitsVotesForConnectedSeats(t *testing.T) {
	m := newTestManager(t)
	r, clients := startGameWithClients(t, m, "host", "guest", "third")
	r.RemoveClient(clients["guest"])
	if err := r.HandleIntent("host", Intent{Type: "votekick", TargetID: "guest"}); err != nil {
		t.Fatalf("votekick error = %v", err)
	}
	if _, ok := findKickVote(r.Snapshot("host"), "guest"); !ok {
		t.Fatal("no vote for the paused seat, want one entry")
	}

	// Reconnect the seat without clearing the tally, as a missing delete would.
	r.mu.Lock()
	r.game.SetConnected(r.game.PlayerIndex("guest"), true)
	count := r.kickVoteCountLocked("guest")
	r.mu.Unlock()
	if count != 1 {
		t.Fatalf("stored vote count = %d, want 1", count)
	}

	snap := r.Snapshot("host")
	if _, ok := findKickVote(snap, "guest"); ok {
		t.Errorf("kickVotes = %+v contains a connected target, want paused seats only", snap.KickVotes)
	}
}

func TestVotekickOnCurrentPlayerAdvancesTurn(t *testing.T) {
	m := newTestManager(t)
	r, clients := startGameWithClients(t, m, "host", "guest", "third")
	snap := r.Snapshot("host")
	if snap.Game == nil {
		t.Fatal("no game state")
	}
	targetID := snap.Game.Players[snap.Game.Current].ID
	r.RemoveClient(clients[targetID])
	for _, voter := range []string{"host", "guest", "third"} {
		if voter == targetID {
			continue
		}
		if err := r.HandleIntent(voter, Intent{Type: "votekick", TargetID: targetID}); err != nil {
			t.Fatalf("votekick by %q error = %v", voter, err)
		}
	}

	after := r.Snapshot("host")
	if after.Game == nil {
		t.Fatal("no game state after the kick")
	}
	if after.Game.Phase != game.PhasePlaying {
		t.Fatalf("phase = %q, want playing", after.Game.Phase)
	}
	current := after.Game.Players[after.Game.Current]
	if current.ID == targetID {
		t.Error("turn is still on the kicked player")
	}
	if current.Bankrupt {
		t.Error("the current player is bankrupt")
	}
}

func TestVotekickFinishesGame(t *testing.T) {
	m := newTestManager(t)
	r, clients := startGameWithClients(t, m, "host", "guest")
	r.RemoveClient(clients["guest"])
	if err := r.HandleIntent("host", Intent{Type: "votekick", TargetID: "guest"}); err != nil {
		t.Fatalf("votekick error = %v", err)
	}
	snap := r.Snapshot("host")
	if snap.Game == nil {
		t.Fatal("no game state")
	}
	if snap.Game.Phase != game.PhaseFinished {
		t.Fatalf("phase = %q, want finished", snap.Game.Phase)
	}
	if got := snap.Game.Players[snap.Game.Winner].ID; got != "host" {
		t.Errorf("winner = %q, want host", got)
	}
	if len(snap.KickVotes) != 0 {
		t.Errorf("kickVotes = %+v after the game ended, want empty", snap.KickVotes)
	}
}

func TestTwoPausedSeatsKeepIndependentVotes(t *testing.T) {
	m := newTestManager(t)
	r, clients := startGameWithClients(t, m, "host", "guest", "third", "fourth")
	r.RemoveClient(clients["guest"])
	r.RemoveClient(clients["fourth"])
	if err := r.HandleIntent("host", Intent{Type: "votekick", TargetID: "guest"}); err != nil {
		t.Fatalf("host votekick error = %v", err)
	}
	if err := r.HandleIntent("third", Intent{Type: "votekick", TargetID: "fourth"}); err != nil {
		t.Fatalf("third votekick error = %v", err)
	}
	snap := r.Snapshot("host")
	vote, ok := findKickVote(snap, "guest")
	if !ok || vote.Votes != 1 || !vote.Voted {
		t.Errorf("guest vote = %+v (found %v), want one vote by host", vote, ok)
	}
	vote, ok = findKickVote(snap, "fourth")
	if !ok || vote.Votes != 1 || vote.Voted {
		t.Errorf("fourth vote = %+v (found %v), want one vote by third", vote, ok)
	}
	if playerView(t, r, "guest").Bankrupt || playerView(t, r, "fourth").Bankrupt {
		t.Fatal("a kick fired before its own quorum was reached")
	}

	if err := r.HandleIntent("third", Intent{Type: "votekick", TargetID: "guest"}); err != nil {
		t.Fatalf("second votekick for guest error = %v", err)
	}
	if !playerView(t, r, "guest").Bankrupt {
		t.Fatal("guest was not kicked")
	}
	snap = r.Snapshot("host")
	if _, ok := findKickVote(snap, "guest"); ok {
		t.Error("votes for the kicked seat were not cleared")
	}
	vote, ok = findKickVote(snap, "fourth")
	if !ok || vote.Votes != 1 {
		t.Errorf("votes for the other paused seat were lost: %+v", snap.KickVotes)
	}
	if playerView(t, r, "fourth").Bankrupt {
		t.Error("the second target was kicked without its own quorum")
	}
}
