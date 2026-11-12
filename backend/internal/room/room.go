// Package room manages lobbies, running games, connected clients and the
// realtime protocol.
package room

import (
	crand "crypto/rand"
	"math/big"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/board"
	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/game"
	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/ws"
)

// Colors are the token colors a player can choose.
var Colors = []string{"rot", "blau", "gruen", "gelb", "rosa", "orange", "lila", "schwarz"}

const maxNameLength = 24

var maxPlayers = game.DefaultRules.MaxPlayers

// ValidColor reports whether c is a selectable token color.
func ValidColor(c string) bool {
	for _, color := range Colors {
		if color == c {
			return true
		}
	}
	return false
}

// NormalizeName trims a display name and caps it at maxNameLength runes.
func NormalizeName(name, fallback string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = fallback
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "Spieler"
	}
	runes := []rune(name)
	if len(runes) > maxNameLength {
		name = string(runes[:maxNameLength])
	}
	return name
}

// Error is a rejected request with an HTTP status for the REST layer.
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

func badRequest(code, message string) *Error {
	return &Error{Status: 400, Code: code, Message: message}
}

func forbidden(code, message string) *Error {
	return &Error{Status: 403, Code: code, Message: message}
}

func notFound(message string) *Error {
	return &Error{Status: 404, Code: "room_not_found", Message: message}
}

// Member is a lobby seat.
type Member struct {
	UserID   string
	Name     string
	Color    string
	Host     bool
	JoinedAt time.Time
}

// Seat is the public lobby seat view.
type Seat struct {
	UserID    string `json:"userId"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	Host      bool   `json:"host"`
	Connected bool   `json:"connected"`
}

// LobbyMember is the minimal member view the presence hub needs.
type LobbyMember struct {
	UserID string
	Name   string
}

// LobbyInfo is a consistent view of a room's lobby.
type LobbyInfo struct {
	Members []LobbyMember
	Open    bool // lobby phase, not closed, fewer than maxPlayers members
}

// Viewer describes the calling user's role in the room.
type Viewer struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	Seat   int    `json:"seat"`
	IsHost bool   `json:"isHost"`
}

// KickVote is the votekick tally for one paused seat as seen by one viewer.
type KickVote struct {
	UserID string `json:"userId"` // target seat
	Votes  int    `json:"votes"`  // effective votes (connected, non-eliminated voters)
	Voted  bool   `json:"voted"`  // this viewer voted for the target
}

// Snapshot is the full room state sent to one viewer.
type Snapshot struct {
	Code      string      `json:"code"`
	Phase     string      `json:"phase"`
	HostID    string      `json:"hostId"`
	CreatedAt time.Time   `json:"createdAt"`
	Seats     []Seat      `json:"seats"`     // always non-nil
	KickVotes []KickVote  `json:"kickVotes"` // always non-nil; entries only while playing
	You       Viewer      `json:"you"`
	CanStart  bool        `json:"canStart"`
	Game      *game.State `json:"game,omitempty"`
}

// Intent is one client command.
type Intent struct {
	Type      string                  `json:"type"`
	Amount    int                     `json:"amount"`
	Tile      int                     `json:"tile"`
	Option    string                  `json:"option"`
	Trade     *game.TradeInput        `json:"trade"`
	TradeID   string                  `json:"tradeId"`
	Accept    bool                    `json:"accept"`
	Name      string                  `json:"name"`
	Color     string                  `json:"color"`
	TargetID  string                  `json:"targetId"` // votekick
	Autopilot *game.AutopilotSettings `json:"autopilot"`
}

// ErrorBody is the error payload of a WebSocket message.
type ErrorBody struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

type outbound struct {
	Type  string     `json:"type"`
	Room  *Snapshot  `json:"room,omitempty"`
	Error *ErrorBody `json:"error,omitempty"`
}

// Client is one WebSocket connection.
type Client struct {
	conn      *ws.Conn
	UserID    string
	room      *Room
	send      chan outbound
	drop      chan struct{}
	done      chan struct{}
	closeOnce sync.Once
}

// NewClient wraps a connection.
func NewClient(conn *ws.Conn, userID string) *Client {
	return &Client{
		conn:   conn,
		UserID: userID,
		send:   make(chan outbound, 32),
		drop:   make(chan struct{}, 1),
		done:   make(chan struct{}),
	}
}

// Start launches the writer goroutine.
func (c *Client) Start() {
	go c.writeLoop()
}

func (c *Client) writeLoop() {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-c.drop:
			// The send buffer overflowed; close from the writer goroutine so
			// Send never waits on the connection mutex.
			c.Close()
			return
		case msg := <-c.send:
			if err := c.conn.WriteJSON(msg); err != nil {
				c.Close()
				return
			}
		case <-ticker.C:
			if err := c.conn.Ping(); err != nil {
				c.Close()
				return
			}
		}
	}
}

// Send queues a message. It never blocks and never takes the connection mutex;
// a client that cannot keep up is signalled for closure by its writer loop.
// Closing here instead would block the caller (and any Room.mu holder) on the
// writer's in-flight write deadline.
func (c *Client) Send(msg outbound) {
	select {
	case <-c.done:
		return
	default:
	}
	select {
	case c.send <- msg:
	default:
		select {
		case c.drop <- struct{}{}:
		default:
		}
	}
}

// Close closes the connection once. It is safe to call repeatedly and from
// either the writer loop or the connection handler.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.conn.Close()
	})
}

// Room is one lobby or running game.
type Room struct {
	mu sync.Mutex

	code      string
	hostID    string
	createdAt time.Time
	cat       *board.Catalog
	members   map[string]*Member
	order     []string
	game      *game.Game
	clients   map[*Client]struct{}
	closed    bool
	votes     map[string]map[string]struct{} // target user id -> voter user ids

	now     func() time.Time
	rngSeed func() *rand.Rand
}

// Code returns the join code.
func (r *Room) Code() string { return r.code }

// Phase returns lobby, playing or finished.
func (r *Room) Phase() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.phaseLocked()
}

// phaseLocked applies the Phase rule. The caller must hold r.mu.
func (r *Room) phaseLocked() string {
	if r.game == nil {
		return "lobby"
	}
	return string(r.game.Phase)
}

func (r *Room) snapshotLocked(userID string) Snapshot {
	snap := Snapshot{
		Code:      r.code,
		Phase:     "lobby",
		HostID:    r.hostID,
		CreatedAt: r.createdAt,
		Seats:     make([]Seat, 0, len(r.order)),
		KickVotes: make([]KickVote, 0),
		You:       Viewer{ID: userID, Seat: -1},
	}
	member, isMember := r.members[userID]
	if isMember {
		snap.You.Name = member.Name
		snap.You.IsHost = member.Host
		snap.You.Role = "player"
	}
	if r.game != nil {
		snap.Phase = string(r.game.Phase)
		// Resolve the viewer's seat first: StateFor includes that seat's private
		// autopilot configuration and only its shared view for everyone else.
		// Spectators get no seat and therefore the spectator state.
		seat := r.game.PlayerIndex(userID)
		state := r.game.StateFor(seat)
		snap.Game = &state
		if seat >= 0 {
			snap.You.Role = "player"
			snap.You.Seat = seat
			snap.You.Name = r.game.Players[seat].Name
		} else {
			snap.You.Role = "spectator"
		}
	} else {
		snap.You.Role = "spectator"
		if isMember {
			snap.You.Role = "player"
		}
		for _, id := range r.order {
			m := r.members[id]
			if m == nil {
				continue
			}
			snap.Seats = append(snap.Seats, Seat{
				UserID:    m.UserID,
				Name:      m.Name,
				Color:     m.Color,
				Host:      m.Host,
				Connected: r.connectedLocked(m.UserID),
			})
		}
		snap.CanStart = isMember && member.Host && len(r.members) >= game.DefaultRules.MinPlayers
	}
	if r.game != nil && r.game.Phase == game.PhasePlaying {
		for _, p := range r.game.Players {
			// Votes are only ever published for paused seats, matching
			// votekickLocked: a connected target is not kickable.
			if p.Bankrupt || p.Connected {
				continue
			}
			count := r.kickVoteCountLocked(p.ID)
			if count == 0 {
				continue
			}
			_, voted := r.votes[p.ID][userID]
			snap.KickVotes = append(snap.KickVotes, KickVote{
				UserID: p.ID,
				Votes:  count,
				Voted:  voted,
			})
		}
	}
	return snap
}

// HasMember reports whether userID currently holds a seat in the room. It is a
// raw seat check: seats outlive the room tab (a closed tab or a finished game
// keeps them), so presence must use InRoom instead.
func (r *Room) HasMember(userID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.members[userID] != nil
}

// InRoom reports whether userID is currently in the room: they hold a seat and
// either have a live room connection or the game has not finished. A finished
// game leaves only a stale seat behind, so a user back on the start page counts
// as available again; a lobby or running-game seat counts even without a
// connection until the user leaves.
func (r *Room) InRoom(userID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.inRoomLocked(userID)
}

// inRoomLocked applies the InRoom rule. The caller must hold r.mu.
func (r *Room) inRoomLocked(userID string) bool {
	if r.members[userID] == nil {
		return false
	}
	if r.connectedLocked(userID) {
		return true
	}
	return r.game == nil || r.game.Phase != game.PhaseFinished
}

// InRoomPhase reports whether userID is currently in the room (see InRoom) and
// returns the phase from the same critical section.
func (r *Room) InRoomPhase(userID string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.inRoomLocked(userID) {
		return "", false
	}
	return r.phaseLocked(), true
}

// inRoomIDs returns the users currently in the room (see InRoom). It returns a
// fresh slice; the caller must not hold r.mu.
func (r *Room) inRoomIDs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	ids := make([]string, 0, len(r.order))
	for _, id := range r.order {
		if r.inRoomLocked(id) {
			ids = append(ids, id)
		}
	}
	return ids
}

// LobbyInfo reports the lobby members and whether the room still accepts
// invitations.
func (r *Room) LobbyInfo() LobbyInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	info := LobbyInfo{
		Members: make([]LobbyMember, 0, len(r.order)),
		Open:    r.game == nil && !r.closed && len(r.members) < maxPlayers,
	}
	for _, id := range r.order {
		m := r.members[id]
		if m == nil {
			continue
		}
		info.Members = append(info.Members, LobbyMember{UserID: m.UserID, Name: m.Name})
	}
	return info
}

// Snapshot returns the room state for one user.
func (r *Room) Snapshot(userID string) Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshotLocked(userID)
}

func (r *Room) connectedLocked(userID string) bool {
	for c := range r.clients {
		if c.UserID == userID {
			return true
		}
	}
	return false
}

// AddClient registers a connection, marks the player connected and sends the
// initial state.
func (r *Room) AddClient(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c.room = r
	r.clients[c] = struct{}{}
	if r.game != nil {
		if idx := r.game.PlayerIndex(c.UserID); idx >= 0 {
			r.game.SetConnected(idx, true)
			delete(r.votes, c.UserID)
		}
	}
	c.Send(outbound{Type: "state", Room: ptr(r.snapshotLocked(c.UserID))})
	r.broadcastLocked()
}

// RemoveClient unregisters a connection.
func (r *Room) RemoveClient(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.clients[c]; !ok {
		return
	}
	delete(r.clients, c)
	if r.game != nil && !r.connectedLocked(c.UserID) {
		if idx := r.game.PlayerIndex(c.UserID); idx >= 0 {
			r.game.SetConnected(idx, false)
		}
	}
	r.broadcastLocked()
}

func ptr[T any](v T) *T { return &v }

func (r *Room) broadcastLocked() {
	for c := range r.clients {
		c.Send(outbound{Type: "state", Room: ptr(r.snapshotLocked(c.UserID))})
	}
}

// SendError sends an error to one connection.
func (r *Room) SendError(c *Client, code, message string) {
	c.Send(outbound{Type: "error", Error: &ErrorBody{Code: code, Message: message}})
}

func (r *Room) tick(now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.game == nil {
		return
	}
	if r.game.Tick(now) {
		r.broadcastLocked()
	}
}

// HandleIntent applies one client command and broadcasts the new state.
func (r *Room) HandleIntent(userID string, in Intent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	err := r.applyLocked(userID, in)
	r.broadcastLocked()
	return err
}

func (r *Room) applyLocked(userID string, in Intent) error {
	switch in.Type {
	case "start_game":
		return r.startLocked(userID)
	case "set_profile":
		return r.setProfileLocked(userID, in.Name, in.Color)
	case "leave_room":
		return r.leaveLocked(userID)
	case "votekick":
		return r.votekickLocked(userID, in.TargetID)
	}

	if r.game == nil {
		return badRequest("not_in_game", "Das Spiel läuft noch nicht.")
	}
	idx := r.game.PlayerIndex(userID)
	if idx < 0 {
		return forbidden("spectator", "Zuschauer können nicht eingreifen.")
	}
	// Trades are allowed out of turn as well: the engine validates them (a trade
	// that involves the active player is only allowed before rolling).
	switch in.Type {
	case "propose_trade":
		if in.Trade == nil {
			return badRequest("invalid_trade", "Handelsdaten fehlen.")
		}
		_, err := r.game.ProposeTrade(idx, *in.Trade)
		return err
	case "respond_trade":
		return r.game.RespondTrade(idx, in.TradeID, in.Accept)
	case "set_autopilot":
		// Allowed out of turn and in every turn phase: the seat owns its own
		// autopilot configuration, including while it is not the active player.
		if in.Autopilot == nil {
			return badRequest("invalid_autopilot", "Autopilot-Daten fehlen.")
		}
		return r.game.SetAutopilot(idx, *in.Autopilot)
	}

	if idx != r.game.Current {
		// Auction bids are also allowed out of turn; the engine validates them.
		// Everything else needs the active seat.
		switch in.Type {
		case "bid":
			return r.game.Bid(idx, in.Amount)
		default:
			return forbidden("not_your_turn", "Du bist nicht am Zug.")
		}
	}
	switch in.Type {
	case "roll":
		return r.game.Roll(idx)
	case "buy":
		return r.game.Buy(idx)
	case "decline":
		return r.game.Decline(idx)
	case "end_turn":
		return r.game.EndTurn(idx)
	case "bid":
		return r.game.Bid(idx, in.Amount)
	case "jail_option":
		return r.game.JailOption(idx, in.Option)
	case "build":
		return r.game.Build(idx, in.Tile)
	case "sell_building":
		return r.game.SellBuilding(idx, in.Tile)
	case "mortgage":
		return r.game.Mortgage(idx, in.Tile)
	case "unmortgage":
		return r.game.Unmortgage(idx, in.Tile)
	case "sell_to_bank":
		return r.game.SellToBank(idx, in.Tile)
	case "give_up":
		return r.game.GiveUp(idx)
	case "forfeit":
		return r.game.Forfeit(idx)
	default:
		return badRequest("unknown_intent", "Unbekannte Aktion.")
	}
}

func (r *Room) setProfileLocked(userID, name, color string) error {
	if r.game != nil {
		return forbidden("game_started", "Das Spiel läuft bereits.")
	}
	m := r.members[userID]
	if m == nil {
		return forbidden("not_a_member", "Du bist nicht in dieser Lobby.")
	}
	m.Name = NormalizeName(name, m.Name)
	if color != "" {
		if !ValidColor(color) {
			return badRequest("invalid_color", "Unbekannte Spielfarbe.")
		}
		for _, other := range r.members {
			if other.UserID != userID && other.Color == color {
				return badRequest("color_taken", "Diese Spielfarbe ist bereits vergeben.")
			}
		}
		m.Color = color
	}
	return nil
}

func (r *Room) leaveLocked(userID string) error {
	m := r.members[userID]
	if m == nil {
		return nil
	}
	if r.game != nil {
		idx := r.game.PlayerIndex(userID)
		if idx >= 0 && !r.game.Players[idx].Bankrupt {
			_ = r.game.Leave(idx)
			delete(r.members, userID)
			return nil
		}
		delete(r.members, userID)
		return nil
	}
	r.leaveLobbyLocked(userID)
	return nil
}

// leaveLobbyLocked frees a lobby seat, promotes the next member to host and
// closes the room when the last member leaves. The caller must hold r.mu.
func (r *Room) leaveLobbyLocked(userID string) {
	m := r.members[userID]
	if m == nil {
		return
	}
	delete(r.members, userID)
	r.order = removeString(r.order, userID)
	if len(r.members) == 0 {
		r.closed = true
	} else if m.Host {
		next := r.members[r.order[0]]
		next.Host = true
		r.hostID = next.UserID
	}
}

// Closed reports whether the lobby was closed because its last member left.
// Every lobby entry point must check it instead of map presence: the room
// object survives until the manager's abandoned-room cleanup.
func (r *Room) Closed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closed
}

// LeaveLobby frees the caller's lobby seat through the REST leave path. A user
// without a seat gets nil (the request is a no-op). A running game only allows
// leaving the view, not the seat, and a finished game has nothing to leave.
func (r *Room) LeaveLobby(userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return notFound("Unbekannter Raum.")
	}
	if r.game != nil {
		if r.game.Phase == game.PhaseFinished {
			return badRequest("game_over", "Das Spiel ist beendet.")
		}
		return forbidden("game_started", "Das Spiel läuft bereits.")
	}
	if r.members[userID] == nil {
		return nil
	}
	r.leaveLobbyLocked(userID)
	r.broadcastLocked()
	return nil
}

// Abandon finally gives up the caller's seat through the REST path. It reuses
// leaveLocked, the same seat-removal path as the room WebSocket's leave_room
// intent: a lobby seat is freed with host promotion/last-member close, a running
// game eliminates the player through Game.Leave and removes the membership, and
// a finished game drops the stale membership. A caller without a seat is a
// no-op.
func (r *Room) Abandon(userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return notFound("Unbekannter Raum.")
	}
	if r.members[userID] == nil {
		return nil
	}
	if err := r.leaveLocked(userID); err != nil {
		return err
	}
	r.broadcastLocked()
	return nil
}

// votekickLocked records a vote against a paused seat. The caller must hold
// r.mu. The kick fires once every other connected, non-eliminated player has
// voted; a successful kick eliminates the target exactly like giving up.
func (r *Room) votekickLocked(voterID, targetID string) error {
	if r.game == nil {
		return badRequest("not_in_game", "Votekick ist nur im laufenden Spiel möglich.")
	}
	if r.game.Phase != game.PhasePlaying {
		return badRequest("game_over", "Das Spiel ist beendet.")
	}
	voterIdx := r.game.PlayerIndex(voterID)
	if voterIdx < 0 {
		return forbidden("spectator", "Zuschauer können nicht abstimmen.")
	}
	if r.game.Players[voterIdx].Bankrupt {
		return forbidden("eliminated", "Ausgeschiedene Spieler können nicht abstimmen.")
	}
	targetIdx := r.game.PlayerIndex(targetID)
	if targetID == "" || targetIdx < 0 || targetIdx == voterIdx {
		return badRequest("invalid_target", "Ungültiges Ziel für den Votekick.")
	}
	if r.game.Players[targetIdx].Bankrupt {
		return badRequest("invalid_target", "Der Spieler ist bereits ausgeschieden.")
	}
	if r.game.Players[targetIdx].Connected {
		return badRequest("target_connected", "Der Spieler ist verbunden.")
	}
	voters := r.votes[targetID]
	if voters == nil {
		voters = map[string]struct{}{}
		r.votes[targetID] = voters
	}
	voters[voterID] = struct{}{}
	if !r.votesUnanimousLocked(targetID, targetIdx) {
		return nil
	}
	delete(r.votes, targetID)
	return r.game.Leave(targetIdx)
}

// votesUnanimousLocked reports whether every eligible voter (the other
// connected, non-eliminated players) voted for targetID. At least one vote is
// required.
func (r *Room) votesUnanimousLocked(targetID string, targetIdx int) bool {
	if r.game == nil {
		return false
	}
	voters := r.votes[targetID]
	eligible := 0
	for i, p := range r.game.Players {
		if i == targetIdx || p.Bankrupt || !p.Connected {
			continue
		}
		if _, ok := voters[p.ID]; !ok {
			return false
		}
		eligible++
	}
	return eligible > 0
}

// kickVoteCountLocked counts the effective votes for a target: only voters that
// are still non-eliminated, connected players count. A disconnected voter's
// vote stays in the map but is neither counted nor required (A1).
func (r *Room) kickVoteCountLocked(targetID string) int {
	if r.game == nil {
		return 0
	}
	count := 0
	for voterID := range r.votes[targetID] {
		idx := r.game.PlayerIndex(voterID)
		if idx < 0 {
			continue
		}
		p := r.game.Players[idx]
		if p.Bankrupt || !p.Connected {
			continue
		}
		count++
	}
	return count
}

func removeString(list []string, value string) []string {
	out := list[:0]
	for _, item := range list {
		if item != value {
			out = append(out, item)
		}
	}
	return out
}

func (r *Room) startLocked(userID string) error {
	if r.game != nil {
		return badRequest("already_started", "Das Spiel läuft bereits.")
	}
	if userID != r.hostID {
		return forbidden("not_host", "Nur der Gastgeber kann starten.")
	}
	if len(r.members) < game.DefaultRules.MinPlayers {
		return badRequest("not_enough_players", "Es braucht mindestens zwei Spieler.")
	}
	order := append([]string(nil), r.order...)
	rng := r.rngSeed()
	rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
	players := make([]*game.Player, 0, len(order))
	for _, id := range order {
		m := r.members[id]
		players = append(players, &game.Player{
			ID:        m.UserID,
			Name:      m.Name,
			Color:     m.Color,
			Connected: r.connectedLocked(m.UserID),
		})
	}
	g, err := game.New(r.cat, players, game.Options{Rand: rng, Now: r.now})
	if err != nil {
		return badRequest("cannot_start", "Das Spiel konnte nicht gestartet werden.")
	}
	r.game = g
	return nil
}

// Join adds a user to the lobby or reconnects/spectates a running game.
func (r *Room) Join(userID, name, color string) (Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.game != nil {
		if idx := r.game.PlayerIndex(userID); idx >= 0 {
			r.game.SetConnected(idx, true)
			delete(r.votes, userID)
			if m := r.members[userID]; m != nil {
				if name != "" {
					m.Name = NormalizeName(name, m.Name)
				}
				if color != "" && ValidColor(color) {
					m.Color = color
				}
			}
		}
		return r.snapshotLocked(userID), nil
	}
	if r.closed {
		return Snapshot{}, notFound("Unbekannter Raum.")
	}
	return r.joinLobbyLocked(userID, name, color)
}

// JoinLobby joins a user to a room that is still an open lobby. Unlike Join it
// rejects a started room (game_started, 403), a closed room (room_not_found,
// 404) and a full room (room_full, 403) instead of returning a spectator
// snapshot.
func (r *Room) JoinLobby(userID, name, color string) (Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.game != nil {
		return Snapshot{}, forbidden("game_started", "Das Spiel läuft bereits.")
	}
	if r.closed {
		return Snapshot{}, notFound("Unbekannter Raum.")
	}
	return r.joinLobbyLocked(userID, name, color)
}

// joinLobbyLocked adds the user to the lobby or updates an existing member's
// profile. The caller must hold r.mu.
func (r *Room) joinLobbyLocked(userID, name, color string) (Snapshot, error) {
	if m := r.members[userID]; m != nil {
		if name != "" {
			m.Name = NormalizeName(name, m.Name)
		}
		if color != "" && ValidColor(color) {
			m.Color = color
		}
		return r.snapshotLocked(userID), nil
	}
	if len(r.members) >= maxPlayers {
		return Snapshot{}, forbidden("room_full", "Diese Lobby ist voll.")
	}
	name = NormalizeName(name, userID)
	if color == "" || !ValidColor(color) {
		color = r.freeColorLocked()
	}
	for _, m := range r.members {
		if m.Color == color && m.UserID != userID {
			color = r.freeColorLocked()
			break
		}
	}
	m := &Member{
		UserID:   userID,
		Name:     name,
		Color:    color,
		Host:     userID == r.hostID,
		JoinedAt: r.now(),
	}
	r.members[userID] = m
	r.order = append(r.order, userID)
	return r.snapshotLocked(userID), nil
}

func (r *Room) freeColorLocked() string {
	used := map[string]bool{}
	for _, m := range r.members {
		used[m.Color] = true
	}
	for _, color := range Colors {
		if !used[color] {
			return color
		}
	}
	return Colors[0]
}

// CreateRoom builds a lobby with the given host as first member.
func newRoom(code string, cat *board.Catalog, now func() time.Time, seed func() *rand.Rand) *Room {
	return &Room{
		code:      code,
		cat:       cat,
		createdAt: now(),
		members:   map[string]*Member{},
		clients:   map[*Client]struct{}{},
		votes:     map[string]map[string]struct{}{},
		now:       now,
		rngSeed:   seed,
	}
}

// addHostLocked provisions the host seat.
func (r *Room) addHostLocked(userID, name, color string) {
	name = NormalizeName(name, userID)
	if color == "" || !ValidColor(color) {
		color = Colors[0]
	}
	r.hostID = userID
	r.members[userID] = &Member{
		UserID:   userID,
		Name:     name,
		Color:    color,
		Host:     true,
		JoinedAt: r.now(),
	}
	r.order = append(r.order, userID)
}

func randomCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	const length = 6
	buf := make([]byte, length)
	for i := range buf {
		n, err := crand.Int(crand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		buf[i] = alphabet[n.Int64()]
	}
	return string(buf), nil
}

// abandoned reports whether the room has neither members nor connections.
func (r *Room) abandoned() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.members) == 0 && len(r.clients) == 0
}
