package presence

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/board"
	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/game"
	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/room"
)

// fakeSink records the messages of one presence connection.
type fakeSink struct {
	msgs   []any
	closed bool
}

func (s *fakeSink) Send(msg any) { s.msgs = append(s.msgs, msg) }

func (s *fakeSink) Close() { s.closed = true }

func (s *fakeSink) reset() { s.msgs = nil }

func (s *fakeSink) count() int { return len(s.msgs) }

func (s *fakeSink) lastLobby(t *testing.T) LobbyState {
	t.Helper()
	for i := len(s.msgs) - 1; i >= 0; i-- {
		if state, ok := s.msgs[i].(LobbyState); ok {
			return state
		}
	}
	t.Fatalf("no LobbyState in %d messages: %v", len(s.msgs), s.msgs)
	return LobbyState{}
}

func (s *fakeSink) lastInbox(t *testing.T) InboxState {
	t.Helper()
	for i := len(s.msgs) - 1; i >= 0; i-- {
		if state, ok := s.msgs[i].(InboxState); ok {
			return state
		}
	}
	t.Fatalf("no InboxState in %d messages: %v", len(s.msgs), s.msgs)
	return InboxState{}
}

func (s *fakeSink) lastAccepted(t *testing.T) Accepted {
	t.Helper()
	for i := len(s.msgs) - 1; i >= 0; i-- {
		if msg, ok := s.msgs[i].(Accepted); ok {
			return msg
		}
	}
	t.Fatalf("no Accepted in %d messages: %v", len(s.msgs), s.msgs)
	return Accepted{}
}

func (s *fakeSink) lastError(t *testing.T) ErrorMessage {
	t.Helper()
	for i := len(s.msgs) - 1; i >= 0; i-- {
		if msg, ok := s.msgs[i].(ErrorMessage); ok {
			return msg
		}
	}
	t.Fatalf("no ErrorMessage in %d messages: %v", len(s.msgs), s.msgs)
	return ErrorMessage{}
}

// newTestHub builds a hub with deterministic ids and timestamps.
func newTestHub(t *testing.T) (*Hub, *room.Manager) {
	t.Helper()
	cat, err := board.New()
	if err != nil {
		t.Fatalf("board.New() error = %v", err)
	}
	dir := room.NewManager(cat)
	h := NewHub(dir)
	ids := 0
	h.newID = func() (string, error) {
		ids++
		return fmt.Sprintf("invite-%d", ids), nil
	}
	clock := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	h.now = func() time.Time {
		clock = clock.Add(time.Second)
		return clock
	}
	return h, dir
}

func newRoom(t *testing.T, dir *room.Manager, hostID, hostName string) *room.Room {
	t.Helper()
	r, err := dir.Create(hostID, hostName, "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	return r
}

func joinRoom(t *testing.T, dir *room.Manager, code, userID, name string) {
	t.Helper()
	if _, _, err := dir.JoinLobby(code, userID, name, ""); err != nil {
		t.Fatalf("JoinLobby(%q) error = %v", userID, err)
	}
}

func connectStart(t *testing.T, h *Hub, userID, name string) (*Client, *fakeSink) {
	t.Helper()
	sink := &fakeSink{}
	return h.Connect(userID, "", name, sink), sink
}

func connectLobby(t *testing.T, h *Hub, userID, code, name string) (*Client, *fakeSink) {
	t.Helper()
	sink := &fakeSink{}
	return h.Connect(userID, code, name, sink), sink
}

func mustInvite(t *testing.T, h *Hub, c *Client, userID string) {
	t.Helper()
	if err := h.HandleIntent(c, Intent{Type: "invite", UserID: userID}); err != nil {
		t.Fatalf("invite(%q) error = %v", userID, err)
	}
}

// inviteID returns the single invitation of an invitee inbox.
func inviteID(t *testing.T, sink *fakeSink) string {
	t.Helper()
	inbox := sink.lastInbox(t)
	if len(inbox.Invites) != 1 {
		t.Fatalf("invites = %+v, want exactly one", inbox.Invites)
	}
	return inbox.Invites[0].ID
}

// acceptIntent builds the accept_invite command used by the invitation tests.
func acceptIntent(id, name, color string) Intent {
	return Intent{Type: "accept_invite", InviteID: id, Name: name, Color: color}
}

func onlineNames(state LobbyState) []string {
	names := make([]string, 0, len(state.Online))
	for _, user := range state.Online {
		names = append(names, user.Name)
	}
	return names
}

func wantOnline(t *testing.T, state LobbyState, want ...string) {
	t.Helper()
	got := onlineNames(state)
	if len(got) != len(want) {
		t.Fatalf("online = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("online = %v, want %v", got, want)
		}
	}
}

// rejectionCode extracts the machine readable code of a presence or room error.
func rejectionCode(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("error = nil, want a rejection")
	}
	var pe *Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	var re *room.Error
	if errors.As(err, &re) {
		return re.Code
	}
	t.Fatalf("error %v carries no code", err)
	return ""
}

func wantError(t *testing.T, err error, code string) {
	t.Helper()
	if got := rejectionCode(t, err); got != code {
		t.Errorf("error code = %q, want %q (%v)", got, code, err)
	}
}

func findSeat(snap room.Snapshot, userID string) *room.Seat {
	for i := range snap.Seats {
		if snap.Seats[i].UserID == userID {
			return &snap.Seats[i]
		}
	}
	return nil
}

func TestPresenceDedupesTabs(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	_, hostSink := connectLobby(t, h, "host", r.Code(), "Anna")

	tab1, _ := connectStart(t, h, "ben", "Ben")
	tab2, _ := connectStart(t, h, "ben", "Ben")

	state := hostSink.lastLobby(t)
	wantOnline(t, state, "Ben")

	hostSink.reset()
	h.Disconnect(tab1)
	if got := hostSink.count(); got != 0 {
		t.Errorf("pushed %d messages after closing one tab, want 0", got)
	}

	h.Disconnect(tab2)
	state = hostSink.lastLobby(t)
	wantOnline(t, state)
}

func TestOnlineListExcludesRoomMembers(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	joinRoom(t, dir, r.Code(), "ben", "Ben")

	_, hostSink := connectLobby(t, h, "host", r.Code(), "Anna")
	connectStart(t, h, "ben", "Ben") // a member with a second tab on the start page
	connectStart(t, h, "cara", "Cara")

	wantOnline(t, hostSink.lastLobby(t), "Cara")
}

func TestLobbyStateForNonMemberIsEmpty(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	_, watcherSink := connectLobby(t, h, "watcher", r.Code(), "Watcher")
	connectStart(t, h, "ben", "Ben")

	state := watcherSink.lastLobby(t)
	wantOnline(t, state)
	if len(state.Sent) != 0 {
		t.Errorf("sent = %+v, want empty", state.Sent)
	}
	if got := watcherSink.count(); got != 1 {
		t.Errorf("watcher received %d messages, want only the initial state", got)
	}
}

func TestInviteAppearsInInboxAndSentList(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	hostConn, hostSink := connectLobby(t, h, "host", r.Code(), "Anna")
	_, benSink := connectStart(t, h, "ben", "Ben")

	mustInvite(t, h, hostConn, "ben")

	inbox := benSink.lastInbox(t)
	if len(inbox.Invites) != 1 {
		t.Fatalf("invites = %+v, want one", inbox.Invites)
	}
	inv := inbox.Invites[0]
	if inv.ID == "" {
		t.Error("invitation has no id")
	}
	if inv.Room != r.Code() || inv.FromID != "host" || inv.FromName != "Anna" {
		t.Errorf("invitation = %+v", inv)
	}

	sent := hostSink.lastLobby(t).Sent
	if len(sent) != 1 {
		t.Fatalf("sent = %+v, want one", sent)
	}
	sentInvite := sent[0]
	if sentInvite.ID != inv.ID || sentInvite.UserID != "ben" ||
		sentInvite.Name != "Ben" || sentInvite.FromID != "host" {
		t.Errorf("sent[0] = %+v", sentInvite)
	}
}

func TestInviteRejectsOfflineMemberAndDuplicate(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	joinRoom(t, dir, r.Code(), "ben", "Ben")
	joinRoom(t, dir, r.Code(), "dave", "Dave")
	hostConn, _ := connectLobby(t, h, "host", r.Code(), "Anna")
	daveConn, _ := connectLobby(t, h, "dave", r.Code(), "Dave")
	connectStart(t, h, "cara", "Cara")

	wantError(t, h.HandleIntent(hostConn, Intent{Type: "invite", UserID: "ghost"}), "not_online")
	wantError(t, h.HandleIntent(hostConn, Intent{Type: "invite", UserID: "ben"}), "already_member")

	mustInvite(t, h, hostConn, "cara")
	wantError(t, h.HandleIntent(daveConn, Intent{Type: "invite", UserID: "cara"}), "already_invited")
}

func TestInviteRequiresMembership(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	connectStart(t, h, "ben", "Ben")
	watcherConn, _ := connectLobby(t, h, "watcher", r.Code(), "Watcher")
	wantError(t, h.HandleIntent(watcherConn, Intent{Type: "invite", UserID: "ben"}), "not_a_member")

	full := newRoom(t, dir, "host2", "Host2")
	for i := 1; i < game.DefaultRules.MaxPlayers; i++ {
		joinRoom(t, dir, full.Code(), fmt.Sprintf("u%d", i), fmt.Sprintf("U%d", i))
	}
	fullConn, _ := connectLobby(t, h, "host2", full.Code(), "Host2")
	wantError(t, h.HandleIntent(fullConn, Intent{Type: "invite", UserID: "ben"}), "lobby_closed")

	closed := newRoom(t, dir, "host3", "Host3")
	if err := closed.HandleIntent("host3", room.Intent{Type: "leave_room"}); err != nil {
		t.Fatalf("leave_room error = %v", err)
	}
	closedConn, _ := connectLobby(t, h, "host3", closed.Code(), "Host3")
	wantError(t, h.HandleIntent(closedConn, Intent{Type: "invite", UserID: "ben"}), "lobby_closed")
}

func TestCancel(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	joinRoom(t, dir, r.Code(), "dave", "Dave")
	hostConn, hostSink := connectLobby(t, h, "host", r.Code(), "Anna")
	daveConn, daveSink := connectLobby(t, h, "dave", r.Code(), "Dave")
	_, benSink := connectStart(t, h, "ben", "Ben")

	mustInvite(t, h, hostConn, "ben")
	id := inviteID(t, benSink)

	wantError(t, h.HandleIntent(daveConn, Intent{Type: "cancel_invite", InviteID: id}), "not_sender")
	if got := len(benSink.lastInbox(t).Invites); got != 1 {
		t.Fatalf("invites = %d after a foreign cancel, want 1", got)
	}

	if err := h.HandleIntent(hostConn, Intent{Type: "cancel_invite", InviteID: id}); err != nil {
		t.Fatalf("cancel_invite error = %v", err)
	}
	if got := len(benSink.lastInbox(t).Invites); got != 0 {
		t.Errorf("invites = %d after cancel, want 0", got)
	}
	if got := len(hostSink.lastLobby(t).Sent); got != 0 {
		t.Errorf("sent = %d after cancel, want 0", got)
	}
	if got := len(daveSink.lastLobby(t).Sent); got != 0 {
		t.Errorf("sent on the other member = %d after cancel, want 0", got)
	}

	mustInvite(t, h, hostConn, "ben")
	if got := len(benSink.lastInbox(t).Invites); got != 1 {
		t.Errorf("invites = %d after a new invite, want 1", got)
	}
}

func TestDecline(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	hostConn, hostSink := connectLobby(t, h, "host", r.Code(), "Anna")
	benConn, benSink := connectStart(t, h, "ben", "Ben")

	mustInvite(t, h, hostConn, "ben")
	id := inviteID(t, benSink)

	if err := h.HandleIntent(benConn, Intent{Type: "decline_invite", InviteID: id}); err != nil {
		t.Fatalf("decline_invite error = %v", err)
	}
	if got := len(benSink.lastInbox(t).Invites); got != 0 {
		t.Errorf("invites = %d after decline, want 0", got)
	}
	if got := len(hostSink.lastLobby(t).Sent); got != 0 {
		t.Errorf("sent = %d after decline, want 0", got)
	}

	mustInvite(t, h, hostConn, "ben")
}

func TestAccept(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	hostConn, hostSink := connectLobby(t, h, "host", r.Code(), "Anna")
	benConn, benSink := connectStart(t, h, "ben", "Ben")

	mustInvite(t, h, hostConn, "ben")
	id := inviteID(t, benSink)

	if err := h.HandleIntent(benConn, acceptIntent(id, "Benno", "blau")); err != nil {
		t.Fatalf("accept_invite error = %v", err)
	}
	accepted := benSink.lastAccepted(t)
	if accepted.Room.Code != r.Code() {
		t.Errorf("accepted room = %q, want %q", accepted.Room.Code, r.Code())
	}
	if accepted.Room.You.Name != "Benno" || accepted.Room.You.Role != "player" {
		t.Errorf("you = %+v, want player Benno", accepted.Room.You)
	}
	seat := findSeat(accepted.Room, "ben")
	if seat == nil {
		t.Fatalf("seats = %+v, want ben", accepted.Room.Seats)
	}
	if seat.Name != "Benno" || seat.Color != "blau" {
		t.Errorf("seat = %+v, want Benno in blau", *seat)
	}
	if got := len(benSink.lastInbox(t).Invites); got != 0 {
		t.Errorf("invites = %d after accept, want 0", got)
	}
	if got := len(hostSink.lastLobby(t).Sent); got != 0 {
		t.Errorf("sent = %d after accept, want 0", got)
	}
}

func TestAcceptAfterStart(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	joinRoom(t, dir, r.Code(), "dave", "Dave")
	hostConn, hostSink := connectLobby(t, h, "host", r.Code(), "Anna")
	benConn, benSink := connectStart(t, h, "ben", "Ben")

	mustInvite(t, h, hostConn, "ben")
	id := inviteID(t, benSink)

	if err := r.HandleIntent("host", room.Intent{Type: "start_game"}); err != nil {
		t.Fatalf("start_game error = %v", err)
	}
	benSink.reset()
	hostSink.reset()

	wantError(t, h.HandleIntent(benConn, acceptIntent(id, "Ben", "blau")), "game_started")
	if got := len(benSink.lastInbox(t).Invites); got != 0 {
		t.Errorf("invites = %d after a failed accept, want 0", got)
	}
	if got := len(hostSink.lastLobby(t).Sent); got != 0 {
		t.Errorf("sent = %d after a failed accept, want 0", got)
	}
}

func TestAcceptWhenFull(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	joinRoom(t, dir, r.Code(), "dave", "Dave")
	hostConn, _ := connectLobby(t, h, "host", r.Code(), "Anna")
	benConn, benSink := connectStart(t, h, "ben", "Ben")

	mustInvite(t, h, hostConn, "ben")
	id := inviteID(t, benSink)

	for i := 2; i < game.DefaultRules.MaxPlayers; i++ {
		joinRoom(t, dir, r.Code(), fmt.Sprintf("u%d", i), fmt.Sprintf("U%d", i))
	}
	wantError(t, h.HandleIntent(benConn, acceptIntent(id, "Ben", "")), "room_full")
	if got := len(benSink.lastInbox(t).Invites); got != 0 {
		t.Errorf("invites = %d after a failed accept, want 0", got)
	}
}

func TestAcceptWhenRoomClosed(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	hostConn, _ := connectLobby(t, h, "host", r.Code(), "Anna")
	benConn, benSink := connectStart(t, h, "ben", "Ben")

	mustInvite(t, h, hostConn, "ben")
	id := inviteID(t, benSink)

	if err := r.HandleIntent("host", room.Intent{Type: "leave_room"}); err != nil {
		t.Fatalf("leave_room error = %v", err)
	}
	// No Reconcile: the accept itself must reject the stale invitation.
	wantError(t, h.HandleIntent(benConn, acceptIntent(id, "Ben", "")), "room_not_found")
	if got := len(benSink.lastInbox(t).Invites); got != 0 {
		t.Errorf("invites = %d after a failed accept, want 0", got)
	}
}

func TestAcceptRejectsInvalidColor(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	hostConn, hostSink := connectLobby(t, h, "host", r.Code(), "Anna")
	benConn, benSink := connectStart(t, h, "ben", "Ben")

	mustInvite(t, h, hostConn, "ben")
	id := inviteID(t, benSink)

	// A client-side rejection keeps the invitation: only room-state failures
	// consume it.
	wantError(t, h.HandleIntent(benConn, acceptIntent(id, "Ben", "neon")), "invalid_color")
	if got := len(benSink.lastInbox(t).Invites); got != 1 {
		t.Errorf("invites = %d after a rejected accept, want 1", got)
	}
	if got := len(hostSink.lastLobby(t).Sent); got != 1 {
		t.Errorf("sent = %d after a rejected accept, want 1", got)
	}
	if _, ok := h.invites[id]; !ok {
		t.Error("invitation was deleted by an invalid color")
	}

	// The kept invitation still works with a valid colour.
	if err := h.HandleIntent(benConn, acceptIntent(id, "Ben", "blau")); err != nil {
		t.Fatalf("accept_invite after invalid colour error = %v", err)
	}
	if got := len(benSink.lastInbox(t).Invites); got != 0 {
		t.Errorf("invites = %d after accept, want 0", got)
	}
}

func TestAcceptWhileInAnotherRoom(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	other := newRoom(t, dir, "olaf", "Olaf")
	hostConn, hostSink := connectLobby(t, h, "host", r.Code(), "Anna")
	connectLobby(t, h, "ben", other.Code(), "Ben")
	benConn, benSink := connectStart(t, h, "ben", "Ben")

	mustInvite(t, h, hostConn, "ben")
	id := inviteID(t, benSink)
	// The invitation was created while Ben was available; then he entered
	// another room before accepting.
	joinRoom(t, dir, other.Code(), "ben", "Ben")

	wantError(t, h.HandleIntent(benConn, acceptIntent(id, "Ben", "")), "already_in_room")
	// A client-side rejection keeps the invitation.
	if got := len(benSink.lastInbox(t).Invites); got != 1 {
		t.Errorf("invites = %d after a rejected accept, want 1", got)
	}
	if got := len(hostSink.lastLobby(t).Sent); got != 1 {
		t.Errorf("sent = %d after a rejected accept, want 1", got)
	}
	if _, ok := h.invites[id]; !ok {
		t.Error("invitation was deleted by an already_in_room rejection")
	}

	// Once the other room is left, the kept invitation still works.
	if err := other.HandleIntent("ben", room.Intent{Type: "leave_room"}); err != nil {
		t.Fatalf("leave_room error = %v", err)
	}
	if err := h.HandleIntent(benConn, acceptIntent(id, "Ben", "blau")); err != nil {
		t.Fatalf("accept_invite after leaving the other room error = %v", err)
	}
	if got := benSink.lastAccepted(t).Room.Code; got != r.Code() {
		t.Errorf("accepted room = %q, want %q", got, r.Code())
	}
	if got := len(benSink.lastInbox(t).Invites); got != 0 {
		t.Errorf("invites = %d after accept, want 0", got)
	}
	if got := len(hostSink.lastLobby(t).Sent); got != 0 {
		t.Errorf("sent = %d after accept, want 0", got)
	}
}

func TestMultipleInvitations(t *testing.T) {
	h, dir := newTestHub(t)
	anna := newRoom(t, dir, "anna", "Anna")
	dave := newRoom(t, dir, "dave", "Dave")
	annaConn, annaSink := connectLobby(t, h, "anna", anna.Code(), "Anna")
	daveConn, daveSink := connectLobby(t, h, "dave", dave.Code(), "Dave")
	benConn, benSink := connectStart(t, h, "ben", "Ben")

	mustInvite(t, h, annaConn, "ben")
	mustInvite(t, h, daveConn, "ben")

	inbox := benSink.lastInbox(t)
	if len(inbox.Invites) != 2 {
		t.Fatalf("invites = %+v, want two", inbox.Invites)
	}
	if inbox.Invites[0].Room != anna.Code() || inbox.Invites[1].Room != dave.Code() {
		t.Errorf("invitation order = %+v, want anna then dave", inbox.Invites)
	}

	id := inbox.Invites[0].ID
	if err := h.HandleIntent(benConn, acceptIntent(id, "Ben", "rot")); err != nil {
		t.Fatalf("accept_invite error = %v", err)
	}
	rest := benSink.lastInbox(t).Invites
	if len(rest) != 1 || rest[0].Room != dave.Code() {
		t.Errorf("remaining invites = %+v, want the dave invitation", rest)
	}
	if got := len(annaSink.lastLobby(t).Sent); got != 0 {
		t.Errorf("anna sent = %d after accept, want 0", got)
	}
	if got := len(daveSink.lastLobby(t).Sent); got != 1 {
		t.Errorf("dave sent = %d after accept, want 1", got)
	}
}

func TestReconcileDropsInvalidInvitations(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, dir *room.Manager, r *room.Room)
	}{
		{
			name: "room started",
			setup: func(t *testing.T, dir *room.Manager, r *room.Room) {
				joinRoom(t, dir, r.Code(), "dave", "Dave")
				if err := r.HandleIntent("host", room.Intent{Type: "start_game"}); err != nil {
					t.Fatalf("start_game error = %v", err)
				}
			},
		},
		{
			name: "room full",
			setup: func(t *testing.T, dir *room.Manager, r *room.Room) {
				for i := 1; i < game.DefaultRules.MaxPlayers; i++ {
					joinRoom(t, dir, r.Code(), fmt.Sprintf("u%d", i), fmt.Sprintf("U%d", i))
				}
			},
		},
		{
			name: "room closed",
			setup: func(t *testing.T, dir *room.Manager, r *room.Room) {
				if err := r.HandleIntent("host", room.Intent{Type: "leave_room"}); err != nil {
					t.Fatalf("leave_room error = %v", err)
				}
			},
		},
		{
			name: "invitee joined",
			setup: func(t *testing.T, dir *room.Manager, r *room.Room) {
				joinRoom(t, dir, r.Code(), "ben", "Ben")
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, dir := newTestHub(t)
			r := newRoom(t, dir, "host", "Anna")
			hostConn, hostSink := connectLobby(t, h, "host", r.Code(), "Anna")
			_, benSink := connectStart(t, h, "ben", "Ben")
			mustInvite(t, h, hostConn, "ben")

			tc.setup(t, dir, r)
			h.Reconcile()

			if got := len(benSink.lastInbox(t).Invites); got != 0 {
				t.Errorf("invites = %d after reconcile, want 0", got)
			}
			if got := len(hostSink.lastLobby(t).Sent); got != 0 {
				t.Errorf("sent = %d after reconcile, want 0", got)
			}
		})
	}
}

func TestReconcilePushesOnlyOnChange(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	_, hostSink := connectLobby(t, h, "host", r.Code(), "Anna")
	benConn, benSink := connectStart(t, h, "ben", "Ben")

	hostSink.reset()
	benSink.reset()
	h.Reconcile()
	if got := hostSink.count(); got != 0 {
		t.Fatalf("reconcile pushed %d unchanged lobby messages, want 0", got)
	}
	if got := benSink.count(); got != 0 {
		t.Fatalf("reconcile pushed %d unchanged inbox messages, want 0", got)
	}

	// A rename reaches the room exactly once, not on every tick.
	if err := h.HandleIntent(benConn, Intent{Type: "set_name", Name: "Benedikt"}); err != nil {
		t.Fatalf("set_name error = %v", err)
	}
	if got := hostSink.count(); got != 1 {
		t.Fatalf("set_name pushed %d messages, want exactly 1", got)
	}
	hostSink.reset()
	h.Reconcile()
	if got := hostSink.count(); got != 0 {
		t.Fatalf("reconcile re-pushed %d messages after a rename, want 0", got)
	}

	// A membership change the hub never observed is picked up on the next tick.
	joinRoom(t, dir, r.Code(), "ben", "Ben")
	hostSink.reset()
	h.Reconcile()
	if got := hostSink.count(); got != 1 {
		t.Fatalf("reconcile pushed %d messages after a join, want exactly 1", got)
	}
	wantOnline(t, hostSink.lastLobby(t))
}

func TestSetNameUpdatesOnlineList(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	hostConn, hostSink := connectLobby(t, h, "host", r.Code(), "Anna")
	benConn, _ := connectStart(t, h, "ben", "Ben")
	connectStart(t, h, "cara", "Cara")
	mustInvite(t, h, hostConn, "ben")

	hostSink.reset()
	if err := h.HandleIntent(benConn, Intent{Type: "set_name", Name: "Benedikt"}); err != nil {
		t.Fatalf("set_name error = %v", err)
	}
	state := hostSink.lastLobby(t)
	wantOnline(t, state, "Benedikt", "Cara")
	if len(state.Sent) != 1 || state.Sent[0].Name != "Benedikt" {
		t.Errorf("sent = %+v, want the new name", state.Sent)
	}
}

func TestSetNameScopeAndTarget(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	roomConn, roomSink := connectLobby(t, h, "host", r.Code(), "Anna")
	benConn, _ := connectStart(t, h, "ben", "Ben")
	connectStart(t, h, "cara", "Cara")

	wantError(t, h.HandleIntent(roomConn, Intent{Type: "set_name", Name: "Neu"}), "invalid_scope")
	if _, ok := h.users["host"]; ok {
		t.Error("a room-scoped connection registered start-page presence")
	}

	// A foreign userId is ignored: only the caller is renamed.
	foreign := Intent{Type: "set_name", UserID: "cara", Name: "Benedikt"}
	if err := h.HandleIntent(benConn, foreign); err != nil {
		t.Fatalf("set_name error = %v", err)
	}
	if got := h.users["ben"].name; got != "Benedikt" {
		t.Errorf("ben name = %q, want Benedikt", got)
	}
	if got := h.users["cara"].name; got != "Cara" {
		t.Errorf("cara name = %q, want Cara", got)
	}

	// An empty name keeps the previous one instead of blanking the list.
	if err := h.HandleIntent(benConn, Intent{Type: "set_name", Name: "   "}); err != nil {
		t.Fatalf("set_name error = %v", err)
	}
	if got := h.users["ben"].name; got != "Benedikt" {
		t.Errorf("ben name = %q after an empty set_name, want Benedikt", got)
	}
	wantOnline(t, roomSink.lastLobby(t), "Benedikt", "Cara")
}

func TestIntentScope(t *testing.T) {
	tests := []struct {
		name   string
		lobby  bool
		intent Intent
	}{
		{"invite from start page", false, Intent{Type: "invite", UserID: "cara"}},
		{"cancel from start page", false, Intent{Type: "cancel_invite", InviteID: "missing"}},
		{"accept from room", true, Intent{Type: "accept_invite", InviteID: "missing", Name: "Ben"}},
		{"decline from room", true, Intent{Type: "decline_invite", InviteID: "missing"}},
		{"set_name from room", true, Intent{Type: "set_name", Name: "Neu"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, dir := newTestHub(t)
			r := newRoom(t, dir, "host", "Anna")
			connectStart(t, h, "cara", "Cara")
			var client *Client
			if tc.lobby {
				client, _ = connectLobby(t, h, "host", r.Code(), "Anna")
			} else {
				client, _ = connectStart(t, h, "ben", "Ben")
			}
			wantError(t, h.HandleIntent(client, tc.intent), "invalid_scope")
		})
	}
}

func TestInvitationSurvivesInviteeOffline(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	hostConn, hostSink := connectLobby(t, h, "host", r.Code(), "Anna")
	benConn, _ := connectStart(t, h, "ben", "Ben")
	mustInvite(t, h, hostConn, "ben")

	h.Disconnect(benConn)
	h.Reconcile()
	wantOnline(t, hostSink.lastLobby(t))

	benConn, benSink := connectStart(t, h, "ben", "Ben")
	inbox := benSink.lastInbox(t)
	if len(inbox.Invites) != 1 {
		t.Fatalf("invites after reconnect = %+v, want one", inbox.Invites)
	}
	id := inbox.Invites[0].ID
	if err := h.HandleIntent(benConn, acceptIntent(id, "Ben", "blau")); err != nil {
		t.Fatalf("accept_invite after reconnect error = %v", err)
	}
	if got := benSink.lastAccepted(t).Room.Code; got != r.Code() {
		t.Errorf("accepted room = %q, want %q", got, r.Code())
	}
}

func TestSendError(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	conn, sink := connectLobby(t, h, "host", r.Code(), "Anna")
	sink.reset()

	h.SendError(conn, &Error{Code: "not_online", Message: "Dieser Spieler ist nicht online."})
	msg := sink.lastError(t)
	if msg.Type != "error" || msg.Error == nil {
		t.Fatalf("message = %+v, want an error", msg)
	}
	if msg.Error.Code != "not_online" || msg.Error.Message != "Dieser Spieler ist nicht online." {
		t.Errorf("error = %+v", msg.Error)
	}

	h.SendError(conn, &room.Error{Status: 403, Code: "room_full", Message: "Diese Lobby ist voll."})
	if got := sink.lastError(t).Error.Code; got != "room_full" {
		t.Errorf("code = %q, want room_full", got)
	}
}

func TestOnlineListExcludesSeatedElsewhere(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	other := newRoom(t, dir, "olaf", "Olaf")
	joinRoom(t, dir, other.Code(), "ben", "Ben")

	_, hostSink := connectLobby(t, h, "host", r.Code(), "Anna")
	connectStart(t, h, "ben", "Ben") // in another room, but has a start-page tab
	connectStart(t, h, "cara", "Cara")

	wantOnline(t, hostSink.lastLobby(t), "Cara")
}

func TestInviteRejectsSeatedElsewhere(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	other := newRoom(t, dir, "olaf", "Olaf")
	joinRoom(t, dir, other.Code(), "ben", "Ben")

	hostConn, hostSink := connectLobby(t, h, "host", r.Code(), "Anna")
	_, benSink := connectStart(t, h, "ben", "Ben")

	wantError(t, h.HandleIntent(hostConn, Intent{Type: "invite", UserID: "ben"}), "not_online")
	if len(h.invites) != 0 {
		t.Errorf("invites = %d, want none", len(h.invites))
	}
	if got := len(benSink.lastInbox(t).Invites); got != 0 {
		t.Errorf("invitee inbox = %d, want 0", got)
	}
	if got := len(hostSink.lastLobby(t).Sent); got != 0 {
		t.Errorf("sent = %d, want 0", got)
	}
}

func TestFinishedSeatKeepsUserOnline(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	finished := newRoom(t, dir, "olaf", "Olaf")
	joinRoom(t, dir, finished.Code(), "ben", "Ben")
	if err := finished.HandleIntent("olaf", room.Intent{Type: "start_game"}); err != nil {
		t.Fatalf("start_game error = %v", err)
	}
	// Leaving bankrupts the guest and finishes the two-player game. Olaf keeps
	// a seat, but it is a stale seat in a finished room.
	if err := finished.HandleIntent("ben", room.Intent{Type: "leave_room"}); err != nil {
		t.Fatalf("leave_room error = %v", err)
	}
	if got := finished.Phase(); got != "finished" {
		t.Fatalf("phase = %q, want finished", got)
	}

	hostConn, hostSink := connectLobby(t, h, "host", r.Code(), "Anna")
	_, olafSink := connectStart(t, h, "olaf", "Olaf")

	wantOnline(t, hostSink.lastLobby(t), "Olaf")
	mustInvite(t, h, hostConn, "olaf")
	if got := len(olafSink.lastInbox(t).Invites); got != 1 {
		t.Errorf("invites = %d, want one", got)
	}
	if got := len(hostSink.lastLobby(t).Sent); got != 1 {
		t.Errorf("sent = %d, want one", got)
	}
}

func TestFinishedSeatWithRoomConnectionStaysHidden(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	finished := newRoom(t, dir, "olaf", "Olaf")
	joinRoom(t, dir, finished.Code(), "ben", "Ben")
	if err := finished.HandleIntent("olaf", room.Intent{Type: "start_game"}); err != nil {
		t.Fatalf("start_game error = %v", err)
	}
	if err := finished.HandleIntent("ben", room.Intent{Type: "leave_room"}); err != nil {
		t.Fatalf("leave_room error = %v", err)
	}
	if got := finished.Phase(); got != "finished" {
		t.Fatalf("phase = %q, want finished", got)
	}

	// Olaf still watches the result: a finished-game seat with a live room
	// connection counts as in a room.
	client := room.NewClient(nil, "olaf")
	finished.AddClient(client)

	_, hostSink := connectLobby(t, h, "host", r.Code(), "Anna")
	connectStart(t, h, "olaf", "Olaf")

	wantOnline(t, hostSink.lastLobby(t))

	finished.RemoveClient(client)
	h.Reconcile()
	wantOnline(t, hostSink.lastLobby(t), "Olaf")
}

func TestWSConnSendSignalsDropWithoutClosing(t *testing.T) {
	c := NewWSConn(nil)
	for i := 0; i < sendBuffer; i++ {
		c.send <- i
	}
	c.Send("overflow")
	if got := len(c.drop); got != 1 {
		t.Fatalf("drop = %d, want 1", got)
	}
	select {
	case <-c.done:
		t.Fatal("Send closed the connection synchronously")
	default:
	}
	// A second overflow must not block on the full drop channel either.
	c.Send("overflow again")
	if got := len(c.drop); got != 1 {
		t.Errorf("drop = %d after a second overflow, want 1", got)
	}
}

func TestAcceptAsSpectatorOfAnotherRoom(t *testing.T) {
	h, dir := newTestHub(t)
	target := newRoom(t, dir, "host", "Anna")
	watched := newRoom(t, dir, "olaf", "Olaf")

	hostConn, _ := connectLobby(t, h, "host", target.Code(), "Anna")
	// A room-scoped connection without a seat is a spectator, not a member.
	connectLobby(t, h, "ben", watched.Code(), "Ben")
	benConn, benSink := connectStart(t, h, "ben", "Ben")

	mustInvite(t, h, hostConn, "ben")
	id := inviteID(t, benSink)

	if err := h.HandleIntent(benConn, acceptIntent(id, "Ben", "blau")); err != nil {
		t.Fatalf("accept_invite error = %v", err)
	}
	if got := benSink.lastAccepted(t).Room.Code; got != target.Code() {
		t.Errorf("accepted room = %q, want %q", got, target.Code())
	}
	if got := len(benSink.lastInbox(t).Invites); got != 0 {
		t.Errorf("invites = %d after accept, want 0", got)
	}
}

func TestInvitationSurvivesInviterLeaving(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	joinRoom(t, dir, r.Code(), "dave", "Dave")

	hostConn, _ := connectLobby(t, h, "host", r.Code(), "Anna")
	_, daveSink := connectLobby(t, h, "dave", r.Code(), "Dave")
	_, benSink := connectStart(t, h, "ben", "Ben")

	mustInvite(t, h, hostConn, "ben")
	if err := r.HandleIntent("host", room.Intent{Type: "leave_room"}); err != nil {
		t.Fatalf("leave_room error = %v", err)
	}
	h.Reconcile()

	if got := len(benSink.lastInbox(t).Invites); got != 1 {
		t.Errorf("invites = %d after the inviter left, want 1", got)
	}
	sent := daveSink.lastLobby(t).Sent
	if len(sent) != 1 {
		t.Fatalf("sent = %+v, want one", sent)
	}
	if sent[0].FromID != "host" || sent[0].FromName != "Anna" {
		t.Errorf("sent[0] = %+v, want the inviter name to survive", sent[0])
	}
}

func TestThirdPartyCannotAcceptOrDecline(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	hostConn, hostSink := connectLobby(t, h, "host", r.Code(), "Anna")
	_, benSink := connectStart(t, h, "ben", "Ben")
	caraConn, _ := connectStart(t, h, "cara", "Cara")

	mustInvite(t, h, hostConn, "ben")
	id := inviteID(t, benSink)

	wantError(t, h.HandleIntent(caraConn, acceptIntent(id, "Cara", "")), "invite_not_found")
	wantError(
		t,
		h.HandleIntent(caraConn, Intent{Type: "decline_invite", InviteID: id}),
		"invite_not_found",
	)
	if got := len(benSink.lastInbox(t).Invites); got != 1 {
		t.Errorf("invites = %d after foreign intents, want 1", got)
	}
	if got := len(hostSink.lastLobby(t).Sent); got != 1 {
		t.Errorf("sent = %d after foreign intents, want 1", got)
	}
	if _, ok := h.invites[id]; !ok {
		t.Error("invitation was changed by a third party")
	}
}

func TestCancelFromOtherRoom(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	other := newRoom(t, dir, "olaf", "Olaf")

	hostConn, _ := connectLobby(t, h, "host", r.Code(), "Anna")
	otherConn, _ := connectLobby(t, h, "olaf", other.Code(), "Olaf")
	_, benSink := connectStart(t, h, "ben", "Ben")

	mustInvite(t, h, hostConn, "ben")
	id := inviteID(t, benSink)

	wantError(
		t,
		h.HandleIntent(otherConn, Intent{Type: "cancel_invite", InviteID: id}),
		"invite_not_found",
	)
	if got := len(benSink.lastInbox(t).Invites); got != 1 {
		t.Errorf("invites = %d after a foreign cancel, want 1", got)
	}
	if _, ok := h.invites[id]; !ok {
		t.Error("invitation was deleted by a connection scoped to another room")
	}
}

func TestConnectCanonicalizesRoomCode(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")

	odd := " " + strings.ToLower(r.Code()) + " "
	c1, _ := connectLobby(t, h, "host", odd, "Anna")
	c2, _ := connectLobby(t, h, "host", r.Code(), "Anna")

	if c1.Room != r.Code() || c2.Room != r.Code() {
		t.Errorf("rooms = %q and %q, want %q", c1.Room, c2.Room, r.Code())
	}
	if got := len(h.rooms[r.Code()]); got != 2 {
		t.Errorf("connections in %q = %d, want 2", r.Code(), got)
	}
}

func TestInviteFailsWhenIDGenerationFails(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	hostConn, _ := connectLobby(t, h, "host", r.Code(), "Anna")
	_, benSink := connectStart(t, h, "ben", "Ben")

	h.newID = func() (string, error) { return "", errors.New("rand failed") }
	wantError(t, h.HandleIntent(hostConn, Intent{Type: "invite", UserID: "ben"}), "internal")
	if len(h.invites) != 0 {
		t.Errorf("invites = %d, want none", len(h.invites))
	}
	if got := len(benSink.lastInbox(t).Invites); got != 0 {
		t.Errorf("invitee inbox = %d, want 0", got)
	}
}

// TestAbandonRunningGameMakesUserInvitable covers AC4 for a running game: after
// the final leave the membership is gone, so the next reconcile lists the user
// as online and an invitation succeeds. No room -> hub callback is involved.
func TestAbandonRunningGameMakesUserInvitable(t *testing.T) {
	h, dir := newTestHub(t)
	gameRoom := newRoom(t, dir, "host", "Anna")
	joinRoom(t, dir, gameRoom.Code(), "dave", "Dave")
	if err := gameRoom.HandleIntent("host", room.Intent{Type: "start_game"}); err != nil {
		t.Fatalf("start_game error = %v", err)
	}

	lobby := newRoom(t, dir, "carol", "Cara")
	carolConn, carolSink := connectLobby(t, h, "carol", lobby.Code(), "Cara")
	_, daveSink := connectStart(t, h, "dave", "Dave")

	// Dave still holds his running-game seat: he is not listed and cannot be
	// invited even though a start-page tab is open.
	wantOnline(t, carolSink.lastLobby(t))
	wantError(t, h.HandleIntent(carolConn, Intent{Type: "invite", UserID: "dave"}), "not_online")

	if err := gameRoom.Abandon("dave"); err != nil {
		t.Fatalf("Abandon(dave) error = %v", err)
	}
	if code, ok := dir.UserRoom("dave"); ok {
		t.Fatalf("UserRoom(dave) = %q, %v after abandon, want no room", code, ok)
	}
	h.Reconcile()

	wantOnline(t, carolSink.lastLobby(t), "Dave")
	mustInvite(t, h, carolConn, "dave")
	if got := len(daveSink.lastInbox(t).Invites); got != 1 {
		t.Errorf("invites = %d after the abandon, want 1", got)
	}
}

// TestLeaveLobbyMakesUserInvitable covers the seat-free user of AC2: after a
// lobby leave the user is listed under "online" again and can be invited.
func TestLeaveLobbyMakesUserInvitable(t *testing.T) {
	h, dir := newTestHub(t)
	r := newRoom(t, dir, "host", "Anna")
	joinRoom(t, dir, r.Code(), "dave", "Dave")
	hostConn, hostSink := connectLobby(t, h, "host", r.Code(), "Anna")
	_, daveSink := connectStart(t, h, "dave", "Dave")

	// Dave still holds his seat: he is not listed and cannot be invited.
	wantOnline(t, hostSink.lastLobby(t))
	wantError(t, h.HandleIntent(hostConn, Intent{Type: "invite", UserID: "dave"}), "already_member")

	if err := r.LeaveLobby("dave"); err != nil {
		t.Fatalf("LeaveLobby(dave) error = %v", err)
	}
	h.Reconcile()

	wantOnline(t, hostSink.lastLobby(t), "Dave")
	mustInvite(t, h, hostConn, "dave")
	if got := len(daveSink.lastInbox(t).Invites); got != 1 {
		t.Errorf("invites = %d after the seat was freed, want 1", got)
	}
}
