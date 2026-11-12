package room

import (
	crand "crypto/rand"
	"encoding/binary"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/board"
)

// Manager owns all rooms.
type Manager struct {
	mu    sync.Mutex
	rooms map[string]*Room
	cat   *board.Catalog
	now   func() time.Time
	seed  func() *rand.Rand
}

// NewManager creates a room manager.
func NewManager(cat *board.Catalog) *Manager {
	return &Manager{
		rooms: map[string]*Room{},
		cat:   cat,
		now:   time.Now,
		seed:  secureRand,
	}
}

func secureRand() *rand.Rand {
	var b [8]byte
	_, _ = crand.Read(b[:])
	return rand.New(rand.NewSource(int64(binary.LittleEndian.Uint64(b[:]))))
}

// Create creates a new lobby with the host as first member. A user who still
// holds a seat in a live room must return there first: holding two live rooms
// would make presence and invitations nondeterministic.
func (m *Manager) Create(hostID, name, color string) (*Room, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.userRoomLocked(hostID); ok {
		return nil, alreadyInRoom(existing)
	}
	var code string
	for attempt := 0; attempt < 20; attempt++ {
		candidate, err := randomCode()
		if err != nil {
			return nil, &Error{Status: 500, Code: "internal", Message: "Raumcode konnte nicht erzeugt werden."}
		}
		if _, exists := m.rooms[candidate]; !exists {
			code = candidate
			break
		}
	}
	if code == "" {
		return nil, &Error{Status: 500, Code: "internal", Message: "Kein freier Raumcode verfügbar."}
	}
	r := newRoom(code, m.cat, m.now, m.seed)
	r.addHostLocked(hostID, name, color)
	m.rooms[code] = r
	return r, nil
}

// Get finds a room by code (case-insensitive).
func (m *Manager) Get(code string) (*Room, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rooms[strings.ToUpper(strings.TrimSpace(code))]
	return r, ok
}

// InRoomSeats returns the room code of every user who is currently in a room:
// they hold a seat and either have a live room connection or their game has not
// finished. A finished game leaves only a stale seat behind, so it is
// deliberately excluded.
//
// The snapshot takes m.mu once and then one Room.mu per room, so a caller that
// pushes many lobbies needs a single pass instead of one UserRoom per user.
//
// Lock order: m.mu → Room.mu, the same order removeAbandoned uses. Nothing in
// the room package calls back into the presence hub.
func (m *Manager) InRoomSeats() map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	seats := make(map[string]string, len(m.rooms))
	for code, r := range m.rooms {
		for _, userID := range r.inRoomIDs() {
			seats[userID] = code
		}
	}
	return seats
}

// UserRoom returns the code of a room the user is currently in (see InRoom),
// and false when the user holds no seat or only a stale seat in a finished
// game. The presence hub uses it to derive "online" from rooms (D2) instead of
// presence connections.
//
// A seat alone is not always enough: a finished game leaves a stale seat, so a
// disconnected seat there must not hide a start-page user or reject invitations
// to them. A lobby or running-game seat still counts until the user leaves.
//
// Lock order: m.mu → Room.mu, the same order removeAbandoned uses. Nothing in
// the room package calls back into the presence hub.
func (m *Manager) UserRoom(userID string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.userRoomLocked(userID)
}

// userRoomLocked applies the UserRoom rule. The caller must hold m.mu.
func (m *Manager) userRoomLocked(userID string) (string, bool) {
	for code, r := range m.rooms {
		if r.InRoom(userID) {
			return code, true
		}
	}
	return "", false
}

// RoomRef is the blocking room of one user as reported to the start page.
type RoomRef struct {
	Code  string `json:"code"`
	Phase string `json:"phase"`
}

// UserRoomRef returns the room the user is currently in (same rule as
// UserRoom) together with its phase.
func (m *Manager) UserRoomRef(userID string) (RoomRef, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for code, r := range m.rooms {
		if phase, ok := r.InRoomPhase(userID); ok {
			return RoomRef{Code: code, Phase: phase}, true
		}
	}
	return RoomRef{}, false
}

// alreadyInRoom rejects a second live room and names the room the user must
// return to. The message only mentions the caller's own room, so it does not
// leak another room's existence.
func alreadyInRoom(code string) *Error {
	return forbidden("already_in_room",
		"Du bist noch in Raum "+code+". Kehre zuerst dorthin zurück.")
}

// Join adds the user to the room and returns the room plus the user's snapshot.
// A user who still holds a seat in a different live room is rejected; rejoining
// the same room (reconnect or spectate) stays allowed.
func (m *Manager) Join(code, userID, name, color string) (*Room, Snapshot, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	r, ok := m.Get(code)
	if !ok {
		return nil, Snapshot{}, notFound("Unbekannter Raum.")
	}
	if existing, ok := m.UserRoom(userID); ok && existing != code {
		return nil, Snapshot{}, alreadyInRoom(existing)
	}
	snap, err := r.Join(userID, name, color)
	if err != nil {
		return nil, Snapshot{}, err
	}
	return r, snap, nil
}

// JoinLobby adds the user to an open lobby and returns the room plus the
// user's snapshot. It rejects started, closed and full rooms. Like Join it
// rejects a user who still holds a seat in a different live room, so the
// single-live-room rule does not depend on the caller pre-checking it.
func (m *Manager) JoinLobby(code, userID, name, color string) (*Room, Snapshot, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	r, ok := m.Get(code)
	if !ok {
		return nil, Snapshot{}, notFound("Unbekannter Raum.")
	}
	if existing, ok := m.UserRoom(userID); ok && existing != code {
		return nil, Snapshot{}, alreadyInRoom(existing)
	}
	snap, err := r.JoinLobby(userID, name, color)
	if err != nil {
		return nil, Snapshot{}, err
	}
	return r, snap, nil
}

// Run ticks all rooms until the context is done: it resolves expired auctions
// and pending decisions.
func (m *Manager) Run(done <-chan struct{}) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	cleanup := time.NewTicker(30 * time.Second)
	defer cleanup.Stop()
	for {
		select {
		case <-done:
			return
		case now := <-ticker.C:
			m.eachRoom(func(r *Room) { r.tick(now) })
		case <-cleanup.C:
			m.removeAbandoned()
		}
	}
}

func (m *Manager) eachRoom(fn func(*Room)) {
	m.mu.Lock()
	rooms := make([]*Room, 0, len(m.rooms))
	for _, r := range m.rooms {
		rooms = append(rooms, r)
	}
	m.mu.Unlock()
	for _, r := range rooms {
		fn(r)
	}
}

func (m *Manager) removeAbandoned() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for code, r := range m.rooms {
		if r.abandoned() {
			delete(m.rooms, code)
		}
	}
}

// RoomCount returns the number of live rooms (used by tests).
func (m *Manager) RoomCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.rooms)
}
