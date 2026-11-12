package presence

import (
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/room"
)

// Lock order (must always be kept): hub.mu → room.Manager.mu → room.Room.mu.
//
// The hub may hold hub.mu while calling Manager.Get, Manager.JoinLobby and
// Room.LobbyInfo; Manager → Room already exists (manager.go, removeAbandoned).
// Invariant: nothing in the room package ever calls into the hub (no observer,
// no callback), so no reverse edge exists and the order cannot deadlock.
//
// Sink.Send must be non-blocking and must never call back into the hub
// synchronously: it only appends to a buffered channel or signals its writer
// loop to close the connection, the same contract as room.Client.Send. If a
// later feature needs a room → hub notification, it must dispatch
// asynchronously (channel or goroutine), never inline while room.Room.mu is
// held.
//
// Presence is derived from seats, not from presence connections: a user counts
// as online only while they have a start-page connection and are not currently
// in a room (D2). "In a room" means a seat plus either a live room connection
// or a game that has not finished (Manager.InRoomSeats / Manager.UserRoom). A
// finished game leaves only a stale seat behind, so a user back on the start
// page counts as online and invitable again; a lobby or running-game seat still
// counts until the user leaves, even without a room connection.
//
// One push takes a single pass over the room data (see pass) instead of one
// Manager.UserRoom scan per online user and room.

// Sink is the transport of one presence connection.
type Sink interface {
	Send(msg any)
	Close()
}

// userPresence is the start-page presence of one user, deduplicated across
// tabs. name is the currently published display name.
type userPresence struct {
	name  string
	conns map[*Client]struct{}
}

// invite is one pending invitation. The embedded Invitation is what the
// invitee sees; toID and toName stay on the hub side.
type invite struct {
	Invitation
	toID   string // invitee
	toName string // invitee display name at invite time (fallback for SentInvite.Name)
}

// roomInfo is the cached lobby view of one room code inside one pass.
type roomInfo struct {
	members map[string]string // user id → lobby name
	open    bool              // lobby phase, not closed, fewer than maxPlayers
	exists  bool
}

func (i roomInfo) isMember(userID string) bool {
	_, ok := i.members[userID]
	return ok
}

// pass caches the room data of one push. rooms holds the lobby views per code
// and seats the in-room users (a seat plus either a live room connection or a
// game that has not finished) of every room. Both are computed lazily, so a
// push that only needs lobby views does not scan the seats and vice versa. One
// pass per push keeps the online list linear instead of one Manager scan per
// user and room.
type pass struct {
	rooms map[string]roomInfo
	seats map[string]string
}

func newPass() *pass {
	return &pass{rooms: map[string]roomInfo{}}
}

// seatsLocked returns the in-room users of the current pass. Caller holds h.mu.
func (h *Hub) seatsLocked(p *pass) map[string]string {
	if p.seats == nil {
		p.seats = h.dir.InRoomSeats()
	}
	return p.seats
}

// Hub owns the start-page presence, the room-lobby subscriptions and the
// pending invitations. All state lives in memory.
type Hub struct {
	mu      sync.Mutex
	users   map[string]*userPresence        // start-page connections, deduplicated per user
	rooms   map[string]map[*Client]struct{} // room-code scoped connections
	invites map[string]*invite              // by invitation id
	dir     *room.Manager
	now     func() time.Time
	newID   func() (string, error)
}

// Client is the hub-side view of one presence connection.
type Client struct {
	UserID string
	Room   string // "" for the start page
	Name   string

	sink      Sink
	lastLobby string // fingerprint of the last LobbyState sent on this connection
	lastInbox string // fingerprint of the last InboxState sent on this connection
	closeOnce sync.Once
}

// Send queues a message on the connection. It never blocks.
func (c *Client) Send(msg any) { c.sink.Send(msg) }

// Close closes the connection once.
func (c *Client) Close() {
	c.closeOnce.Do(func() { c.sink.Close() })
}

// NewHub creates a hub that validates invitations against dir.
func NewHub(dir *room.Manager) *Hub {
	return &Hub{
		users:   map[string]*userPresence{},
		rooms:   map[string]map[*Client]struct{}{},
		invites: map[string]*invite{},
		dir:     dir,
		now:     time.Now,
		newID:   newID,
	}
}

// newID returns a random 16 character hex identifier. A failing random source
// is reported instead of producing an identifier every invitation would share.
func newID() (string, error) {
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// Connect registers one presence connection. An empty room code means the start
// page; any other value scopes the connection to that room's lobby. Room codes
// are canonicalised the same way room.Manager.Get does it, so two tabs that
// spell the same room differently share one lobby.
func (h *Hub) Connect(userID, roomCode, name string, sink Sink) *Client {
	roomCode = strings.ToUpper(strings.TrimSpace(roomCode))
	c := &Client{UserID: userID, Room: roomCode, Name: name, sink: sink}
	h.mu.Lock()
	defer h.mu.Unlock()
	if roomCode == "" {
		p, ok := h.users[userID]
		if !ok {
			p = &userPresence{name: name, conns: map[*Client]struct{}{}}
			h.users[userID] = p
		}
		p.conns[c] = struct{}{}
		h.sendInboxLocked(c, h.inboxStateLocked(userID))
		h.pushAllLobbiesLocked()
		return c
	}
	conns, ok := h.rooms[roomCode]
	if !ok {
		conns = map[*Client]struct{}{}
		h.rooms[roomCode] = conns
	}
	conns[c] = struct{}{}
	h.pushLobbyLocked(roomCode, newPass())
	return c
}

// Disconnect removes a presence connection. The last start-page connection of a
// user ends their presence; a room-scoped connection never counts as presence.
func (h *Hub) Disconnect(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c.Room == "" {
		p, ok := h.users[c.UserID]
		if !ok {
			return
		}
		delete(p.conns, c)
		if len(p.conns) > 0 {
			return
		}
		delete(h.users, c.UserID)
		h.pushAllLobbiesLocked()
		return
	}
	conns, ok := h.rooms[c.Room]
	if !ok {
		return
	}
	delete(conns, c)
	if len(conns) == 0 {
		delete(h.rooms, c.Room)
	}
}

// HandleIntent applies one client command. Scope is enforced here: invite and
// cancel_invite need a room-scoped connection, accept_invite, decline_invite
// and set_name need a start-page connection.
func (h *Hub) HandleIntent(c *Client, in Intent) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch in.Type {
	case "invite":
		return h.inviteLocked(c, in)
	case "cancel_invite":
		return h.cancelInviteLocked(c, in)
	case "accept_invite":
		return h.acceptInviteLocked(c, in)
	case "decline_invite":
		return h.declineInviteLocked(c, in)
	case "set_name":
		return h.setNameLocked(c, in)
	default:
		return &Error{Code: "unknown_intent", Message: "Unbekannte Aktion."}
	}
}

// SendError sends a rejected request to one connection.
func (h *Hub) SendError(c *Client, err error) {
	c.Send(ErrorMessage{Type: "error", Error: &ErrorBody{Code: errorCode(err), Message: err.Error()}})
}

func errorCode(err error) string {
	var pe *Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	var re *room.Error
	if errors.As(err, &re) {
		return re.Code
	}
	return "internal"
}

// Reconcile drops invitations whose room no longer accepts them and re-pushes
// changed states. It is exported so tests need no timer.
func (h *Hub) Reconcile() {
	h.mu.Lock()
	defer h.mu.Unlock()
	p := newPass()
	changed := map[string]struct{}{}
	for id, inv := range h.invites {
		info := h.roomInfoLocked(inv.Room, p)
		if !info.exists || !info.open || info.isMember(inv.toID) {
			delete(h.invites, id)
			changed[inv.toID] = struct{}{}
		}
	}
	for userID := range changed {
		h.pushInboxLocked(userID)
	}
	for code := range h.rooms {
		h.pushLobbyLocked(code, p)
	}
}

// Run reconciles once per second until done is closed.
func (h *Hub) Run(done <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			h.Reconcile()
		}
	}
}

func (h *Hub) inviteLocked(c *Client, in Intent) error {
	if c.Room == "" {
		return &Error{Code: "invalid_scope", Message: "Einladungen gibt es nur in der Lobby."}
	}
	info := h.roomInfoLocked(c.Room, newPass())
	if !info.exists || !info.open {
		return &Error{Code: "lobby_closed", Message: "Diese Lobby nimmt keine Einladungen mehr an."}
	}
	if !info.isMember(c.UserID) {
		return &Error{Code: "not_a_member", Message: "Du bist nicht in dieser Lobby."}
	}
	target := strings.TrimSpace(in.UserID)
	// Membership is checked before presence: a room member can never be
	// invited, whether or not a start-page tab is open (this is also what
	// rejects inviting yourself).
	if info.isMember(target) {
		return &Error{Code: "already_member", Message: "Dieser Spieler ist schon in dieser Lobby."}
	}
	p, online := h.users[target]
	if target == "" || !online {
		return &Error{Code: "not_online", Message: "Dieser Spieler ist nicht online."}
	}
	// D2: a user who is currently in another room is not available, even with a
	// start-page tab open. A seat left behind in a finished game does not
	// count. A crafted invite must not reach an in-room user; the room is
	// deliberately not leaked.
	if _, seated := h.dir.UserRoom(target); seated {
		return &Error{Code: "not_online", Message: "Dieser Spieler ist nicht online."}
	}
	for _, inv := range h.invites {
		if inv.Room == c.Room && inv.toID == target {
			return &Error{Code: "already_invited", Message: "Dieser Spieler ist bereits eingeladen."}
		}
	}
	fromName := info.members[c.UserID]
	if fromName == "" {
		fromName = c.Name
	}
	id, err := h.newID()
	if err != nil {
		return &Error{Code: "internal", Message: "Einladung konnte nicht erzeugt werden."}
	}
	inv := &invite{
		Invitation: Invitation{
			ID:        id,
			Room:      c.Room,
			FromID:    c.UserID,
			FromName:  room.NormalizeName(fromName, c.Name),
			CreatedAt: h.now(),
		},
		toID:   target,
		toName: p.name,
	}
	h.invites[inv.ID] = inv
	h.pushInboxLocked(target)
	h.pushLobbyCodeLocked(c.Room)
	return nil
}

func (h *Hub) cancelInviteLocked(c *Client, in Intent) error {
	if c.Room == "" {
		return &Error{Code: "invalid_scope", Message: "Einladungen gibt es nur in der Lobby."}
	}
	inv, ok := h.invites[in.InviteID]
	if !ok || inv.Room != c.Room {
		return &Error{Code: "invite_not_found", Message: "Diese Einladung gibt es nicht mehr."}
	}
	if inv.FromID != c.UserID {
		return &Error{Code: "not_sender", Message: "Nur der Absender kann die Einladung abbrechen."}
	}
	delete(h.invites, inv.ID)
	h.pushInboxLocked(inv.toID)
	h.pushLobbyCodeLocked(inv.Room)
	return nil
}

func (h *Hub) declineInviteLocked(c *Client, in Intent) error {
	if c.Room != "" {
		return &Error{
			Code:    "invalid_scope",
			Message: "Einladungen lassen sich nur auf der Startseite ablehnen.",
		}
	}
	inv, ok := h.invites[in.InviteID]
	if !ok || inv.toID != c.UserID {
		return &Error{Code: "invite_not_found", Message: "Diese Einladung gibt es nicht mehr."}
	}
	delete(h.invites, inv.ID)
	h.pushInboxLocked(inv.toID)
	h.pushLobbyCodeLocked(inv.Room)
	return nil
}

// acceptInviteLocked joins the caller to the invited room. The invitation is
// consumed only by room-state failures and success; client-side rejections
// (an invalid colour, an existing seat in another room) keep it so the invitee
// can retry. The inbox push always happens before the error reaches the caller.
func (h *Hub) acceptInviteLocked(c *Client, in Intent) error {
	if c.Room != "" {
		return &Error{
			Code:    "invalid_scope",
			Message: "Einladungen lassen sich nur auf der Startseite annehmen.",
		}
	}
	inv, ok := h.invites[in.InviteID]
	if !ok || inv.toID != c.UserID {
		h.pushInboxLocked(c.UserID)
		return &Error{Code: "invite_not_found", Message: "Diese Einladung gibt es nicht mehr."}
	}
	if in.Color != "" && !room.ValidColor(in.Color) {
		return &Error{Code: "invalid_color", Message: "Unbekannte Spielfarbe."}
	}
	// A seat in another room that is still in use keeps the invitation; a stale
	// seat in a finished game or a spectator tab in another room does not block.
	if code, seated := h.dir.UserRoom(c.UserID); seated && code != inv.Room {
		return &Error{Code: "already_in_room", Message: "Du bist schon in einer anderen Lobby."}
	}
	delete(h.invites, inv.ID)
	defer func() {
		h.pushInboxLocked(c.UserID)
		h.pushLobbyCodeLocked(inv.Room)
	}()
	name := room.NormalizeName(in.Name, c.Name)
	_, snap, err := h.dir.JoinLobby(inv.Room, c.UserID, name, in.Color)
	if err != nil {
		return err
	}
	c.Send(Accepted{Type: "accepted", Room: snap})
	return nil
}

// setNameLocked publishes a new start-page display name for the caller. The
// intent has no target: a userId sent along is ignored, and a room-scoped
// connection is rejected before the name is touched.
func (h *Hub) setNameLocked(c *Client, in Intent) error {
	if c.Room != "" {
		return &Error{
			Code:    "invalid_scope",
			Message: "Der Name lässt sich nur auf der Startseite ändern.",
		}
	}
	name := room.NormalizeName(in.Name, c.Name)
	c.Name = name
	if p, ok := h.users[c.UserID]; ok {
		p.name = name
	}
	h.pushAllLobbiesLocked()
	return nil
}

// roomInfoLocked returns the lobby view of one room code, cached per pass.
// Caller holds h.mu.
func (h *Hub) roomInfoLocked(code string, p *pass) roomInfo {
	if info, ok := p.rooms[code]; ok {
		return info
	}
	info := roomInfo{members: map[string]string{}}
	if rm, ok := h.dir.Get(code); ok {
		lobby := rm.LobbyInfo()
		info.exists = true
		info.open = lobby.Open
		for _, m := range lobby.Members {
			info.members[m.UserID] = m.Name
		}
	}
	p.rooms[code] = info
	return info
}

// lobbyStateLocked computes the lobby state of one room code. p carries the
// room views and the in-room users of the current pass. Caller holds h.mu.
func (h *Hub) lobbyStateLocked(code string, p *pass) LobbyState {
	state := emptyLobbyState()
	info := h.roomInfoLocked(code, p)
	if !info.exists {
		return state
	}
	seats := h.seatsLocked(p)
	for userID, u := range h.users {
		// D2: "online in the lobby" means on the start page and not currently
		// in a room. The in-room snapshot also excludes the members of this
		// room and ignores stale seats in finished games.
		if _, seated := seats[userID]; seated {
			continue
		}
		state.Online = append(state.Online, OnlineUser{UserID: userID, Name: u.name})
	}
	sort.Slice(state.Online, func(i, j int) bool {
		a, b := strings.ToLower(state.Online[i].Name), strings.ToLower(state.Online[j].Name)
		if a != b {
			return a < b
		}
		return state.Online[i].UserID < state.Online[j].UserID
	})
	for _, inv := range h.invites {
		if inv.Room != code {
			continue
		}
		state.Sent = append(state.Sent, SentInvite{
			ID:        inv.ID,
			UserID:    inv.toID,
			Name:      h.inviteeNameLocked(inv),
			FromID:    inv.FromID,
			FromName:  inv.Invitation.FromName,
			CreatedAt: inv.CreatedAt,
		})
	}
	sort.Slice(state.Sent, func(i, j int) bool {
		a, b := state.Sent[i], state.Sent[j]
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.Before(b.CreatedAt)
		}
		return a.ID < b.ID
	})
	return state
}

// inviteeNameLocked prefers the current start-page name of the invitee, so a
// rename also shows up in an invitation created earlier.
func (h *Hub) inviteeNameLocked(inv *invite) string {
	if p, ok := h.users[inv.toID]; ok {
		return p.name
	}
	return inv.toName
}

// inboxStateLocked lists every invitation of one user. Caller holds h.mu.
func (h *Hub) inboxStateLocked(userID string) InboxState {
	state := InboxState{Type: "inbox", Invites: []Invitation{}}
	for _, inv := range h.invites {
		if inv.toID == userID {
			state.Invites = append(state.Invites, inv.Invitation)
		}
	}
	sort.Slice(state.Invites, func(i, j int) bool {
		a, b := state.Invites[i], state.Invites[j]
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.Before(b.CreatedAt)
		}
		return a.ID < b.ID
	})
	return state
}

func emptyLobbyState() LobbyState {
	return LobbyState{Type: "lobby", Online: []OnlineUser{}, Sent: []SentInvite{}}
}

// pushLobbyLocked sends the current lobby state of one room to its connections.
// A connection whose user is not a member of the room gets the empty state, and
// a connection whose state did not change gets nothing.
func (h *Hub) pushLobbyLocked(code string, p *pass) {
	conns := h.rooms[code]
	if len(conns) == 0 {
		return
	}
	state := h.lobbyStateLocked(code, p)
	members := h.roomInfoLocked(code, p).members
	for c := range conns {
		out := state
		if _, ok := members[c.UserID]; !ok {
			out = emptyLobbyState()
		}
		h.sendLobbyLocked(c, out)
	}
}

func (h *Hub) pushLobbyCodeLocked(code string) {
	h.pushLobbyLocked(code, newPass())
}

func (h *Hub) pushAllLobbiesLocked() {
	p := newPass()
	for code := range h.rooms {
		h.pushLobbyLocked(code, p)
	}
}

func (h *Hub) sendLobbyLocked(c *Client, state LobbyState) {
	fingerprint, err := json.Marshal(state)
	if err != nil {
		return
	}
	if string(fingerprint) == c.lastLobby {
		return
	}
	c.lastLobby = string(fingerprint)
	c.Send(state)
}

// pushInboxLocked sends the current inbox to every start-page connection of one
// user, skipping the connections that already have that list.
func (h *Hub) pushInboxLocked(userID string) {
	p, ok := h.users[userID]
	if !ok {
		return
	}
	state := h.inboxStateLocked(userID)
	for c := range p.conns {
		h.sendInboxLocked(c, state)
	}
}

func (h *Hub) sendInboxLocked(c *Client, state InboxState) {
	fingerprint, err := json.Marshal(state)
	if err != nil {
		return
	}
	if string(fingerprint) == c.lastInbox {
		return
	}
	c.lastInbox = string(fingerprint)
	c.Send(state)
}
