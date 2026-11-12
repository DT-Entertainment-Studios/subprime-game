package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/auth"
	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/board"
	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/config"
	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/game"
	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/presence"
	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/room"
)

func newTestServer(t *testing.T, staticDir string) *Server {
	t.Helper()
	cat, err := board.New()
	if err != nil {
		t.Fatalf("board.New() error = %v", err)
	}
	cfg := config.Config{
		Port:      8080,
		StaticDir: staticDir,
		Auth: config.AuthHeaders{
			User:   "X-Forwarded-User",
			Name:   "X-Forwarded-Preferred-Username",
			Email:  "X-Forwarded-Email",
			Groups: "X-Forwarded-Groups",
		},
		LogLevel: "error",
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	rooms := room.NewManager(cat)
	return New(cfg, auth.NewStore(), rooms, presence.NewHub(rooms), logger)
}

func do(t *testing.T, s *Server, method, path, user, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if user != "" {
		req.Header.Set("X-Forwarded-User", user)
		req.Header.Set("X-Forwarded-Preferred-Username", user+" name")
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func decodeRoom(t *testing.T, rec *httptest.ResponseRecorder) room.Snapshot {
	t.Helper()
	var payload struct {
		Room room.Snapshot `json:"room"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode room: %v (%s)", err, rec.Body.String())
	}
	return payload.Room
}

func decodeActiveRoom(t *testing.T, rec *httptest.ResponseRecorder) *room.RoomRef {
	t.Helper()
	var payload struct {
		Room *room.RoomRef `json:"room"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode active room: %v (%s)", err, rec.Body.String())
	}
	return payload.Room
}

func TestHealth(t *testing.T) {
	s := newTestServer(t, "")
	rec := do(t, s, http.MethodGet, "/api/health", "", "")
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAuthRequired(t *testing.T) {
	s := newTestServer(t, "")
	tests := []struct {
		name string
		path string
		user string
		want int
	}{
		{name: "me without identity", path: "/api/me", user: "", want: http.StatusUnauthorized},
		{name: "me with identity", path: "/api/me", user: "u1", want: http.StatusOK},
		{name: "me room without identity", path: "/api/me/room", user: "", want: http.StatusUnauthorized},
		{name: "rooms without identity", path: "/api/rooms", user: "", want: http.StatusUnauthorized},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			method := http.MethodGet
			if tc.path == "/api/rooms" {
				method = http.MethodPost
			}
			rec := do(t, s, method, tc.path, tc.user, "")
			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestCreateRoom(t *testing.T) {
	s := newTestServer(t, "")
	rec := do(t, s, http.MethodPost, "/api/rooms", "u1", `{"displayName":"Anna","color":"blau"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
	snap := decodeRoom(t, rec)
	if len(snap.Code) != 6 {
		t.Errorf("code = %q, want 6 letters", snap.Code)
	}
	if snap.Phase != "lobby" {
		t.Errorf("phase = %q, want lobby", snap.Phase)
	}
	if len(snap.Seats) != 1 || snap.Seats[0].Name != "Anna" || snap.Seats[0].Color != "blau" {
		t.Errorf("seats = %+v", snap.Seats)
	}
	if snap.You.Role != "player" || !snap.You.IsHost {
		t.Errorf("you = %+v, want host player", snap.You)
	}
}

func TestCreateRoomInvalidColor(t *testing.T) {
	s := newTestServer(t, "")
	rec := do(t, s, http.MethodPost, "/api/rooms", "u1", `{"displayName":"Anna","color":"neon"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestJoinRoom(t *testing.T) {
	s := newTestServer(t, "")
	code := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "u1", "")).Code

	tests := []struct {
		name  string
		code  string
		user  string
		body  string
		want  int
		seats int
	}{
		{name: "unknown room", code: "ZZZZZZ", user: "u2", body: "", want: http.StatusNotFound},
		{name: "join", code: code, user: "u2", body: `{"displayName":"Ben","color":"gruen"}`, want: http.StatusOK, seats: 2},
		{name: "rejoin updates", code: code, user: "u2", body: `{"displayName":"Ben2"}`, want: http.StatusOK, seats: 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, s, http.MethodPost, "/api/rooms/"+tc.code+"/join", tc.user, tc.body)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tc.want, rec.Body.String())
			}
			if tc.seats > 0 {
				if got := len(decodeRoom(t, rec).Seats); got != tc.seats {
					t.Errorf("seats = %d, want %d", got, tc.seats)
				}
			}
		})
	}
}

func TestCreateRoomAlreadyInRoom(t *testing.T) {
	s := newTestServer(t, "")
	first := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "u1", "")).Code

	rec := do(t, s, http.MethodPost, "/api/rooms", "u1", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if got := responseErrorCode(t, rec); got != "already_in_room" {
		t.Errorf("code = %q, want already_in_room", got)
	}
	if msg := responseErrorMessage(t, rec); !strings.Contains(msg, first) {
		t.Errorf("message = %q, want the existing room code %q", msg, first)
	}
}

func TestJoinRoomAlreadyInRoom(t *testing.T) {
	s := newTestServer(t, "")
	first := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "u1", "")).Code
	second := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "u2", "")).Code

	rec := do(t, s, http.MethodPost, "/api/rooms/"+second+"/join", "u1", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if got := responseErrorCode(t, rec); got != "already_in_room" {
		t.Errorf("code = %q, want already_in_room", got)
	}
	if msg := responseErrorMessage(t, rec); !strings.Contains(msg, first) {
		t.Errorf("message = %q, want the existing room code %q", msg, first)
	}

	// Rejoining the same room stays allowed.
	rec = do(t, s, http.MethodPost, "/api/rooms/"+first+"/join", "u1", "")
	if rec.Code != http.StatusOK {
		t.Errorf("rejoin status = %d, want %d (%s)", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestRoomFull(t *testing.T) {
	s := newTestServer(t, "")
	code := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "host", "")).Code
	for i := 0; i < 7; i++ {
		rec := do(t, s, http.MethodPost, "/api/rooms/"+code+"/join", string(rune('a'+i)), "")
		if rec.Code != http.StatusOK {
			t.Fatalf("join %d status = %d (%s)", i, rec.Code, rec.Body.String())
		}
	}
	rec := do(t, s, http.MethodPost, "/api/rooms/"+code+"/join", "extra", "")
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestGetRoomAndMethodNotAllowed(t *testing.T) {
	s := newTestServer(t, "")
	code := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "u1", "")).Code

	rec := do(t, s, http.MethodGet, "/api/rooms/"+code, "u2", "")
	if rec.Code != http.StatusOK {
		t.Errorf("GET room status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := decodeRoom(t, rec).You.Role; got != "spectator" {
		t.Errorf("role = %q, want spectator", got)
	}

	rec = do(t, s, http.MethodDelete, "/api/rooms/"+code, "u1", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if allow := rec.Header().Get("Allow"); allow != http.MethodGet {
		t.Errorf("Allow = %q, want GET", allow)
	}
}

func TestMyRoom(t *testing.T) {
	// startRoom creates a lobby as u1, joins u2 and starts the game.
	startRoom := func(t *testing.T, s *Server) string {
		t.Helper()
		code := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "u1", "")).Code
		join := do(t, s, http.MethodPost, "/api/rooms/"+code+"/join", "u2", "")
		if join.Code != http.StatusOK {
			t.Fatalf("join status = %d (%s)", join.Code, join.Body.String())
		}
		rm, ok := s.rooms.Get(code)
		if !ok {
			t.Fatalf("room %q not found", code)
		}
		if err := rm.HandleIntent("u1", room.Intent{Type: "start_game"}); err != nil {
			t.Fatalf("start_game error = %v", err)
		}
		return code
	}

	tests := []struct {
		name  string
		setup func(t *testing.T, s *Server) (user, code, phase string)
	}{
		{
			name: "no room",
			setup: func(t *testing.T, s *Server) (string, string, string) {
				return "u1", "", ""
			},
		},
		{
			name: "lobby",
			setup: func(t *testing.T, s *Server) (string, string, string) {
				code := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "u1", "")).Code
				return "u1", code, "lobby"
			},
		},
		{
			name: "playing",
			setup: func(t *testing.T, s *Server) (string, string, string) {
				return "u2", startRoom(t, s), "playing"
			},
		},
		{
			name: "finished without connection",
			setup: func(t *testing.T, s *Server) (string, string, string) {
				code := startRoom(t, s)
				rm, ok := s.rooms.Get(code)
				if !ok {
					t.Fatalf("room %q not found", code)
				}
				if err := rm.HandleIntent("u2", room.Intent{Type: "leave_room"}); err != nil {
					t.Fatalf("leave_room error = %v", err)
				}
				return "u1", "", ""
			},
		},
		{
			name: "spectator",
			setup: func(t *testing.T, s *Server) (string, string, string) {
				code := startRoom(t, s)
				join := do(t, s, http.MethodPost, "/api/rooms/"+code+"/join", "u3", "")
				if join.Code != http.StatusOK {
					t.Fatalf("spectator join status = %d (%s)", join.Code, join.Body.String())
				}
				return "u3", "", ""
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestServer(t, "")
			user, code, phase := tc.setup(t, s)
			rec := do(t, s, http.MethodGet, "/api/me/room", user, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusOK, rec.Body.String())
			}
			if code == "" {
				if body := strings.TrimSpace(rec.Body.String()); body != `{"room":null}` {
					t.Errorf("body = %s, want {\"room\":null}", body)
				}
				if ref := decodeActiveRoom(t, rec); ref != nil {
					t.Errorf("room = %+v, want nil", ref)
				}
				return
			}
			ref := decodeActiveRoom(t, rec)
			if ref == nil {
				t.Fatalf("room = nil, want {%s %s}", code, phase)
			}
			if ref.Code != code || ref.Phase != phase {
				t.Errorf("room = %+v, want {%s %s}", ref, code, phase)
			}
		})
	}
}

func TestMyRoomMethodNotAllowed(t *testing.T) {
	s := newTestServer(t, "")
	rec := do(t, s, http.MethodPost, "/api/me/room", "u1", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if allow := rec.Header().Get("Allow"); allow != http.MethodGet {
		t.Errorf("Allow = %q, want GET", allow)
	}
}

func TestStaticServing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("subprime-spa"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}
	s := newTestServer(t, dir)

	rec := do(t, s, http.MethodGet, "/app.js", "", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "console.log") {
		t.Errorf("asset = %d %q", rec.Code, rec.Body.String())
	}
	rec = do(t, s, http.MethodGet, "/room/ABCDEF", "", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "subprime-spa") {
		t.Errorf("spa fallback = %d %q", rec.Code, rec.Body.String())
	}
}

func TestWebSocketFlow(t *testing.T) {
	s := newTestServer(t, "")
	code := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "host", "")).Code
	do(t, s, http.MethodPost, "/api/rooms/"+code+"/join", "guest", "")

	httpServer := httptest.NewServer(s.Handler())
	defer httpServer.Close()

	dial := func(user string) *websocket.Conn {
		t.Helper()
		header := http.Header{}
		header.Set("X-Forwarded-User", user)
		header.Set("X-Forwarded-Preferred-Username", user+" name")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		url := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/ws?room=" + code
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, header)
		if err != nil {
			t.Fatalf("dial %s: %v", user, err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		return conn
	}
	readState := func(conn *websocket.Conn) room.Snapshot {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			var msg struct {
				Type  string        `json:"type"`
				Room  room.Snapshot `json:"room"`
				Error *struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := conn.ReadJSON(&msg); err != nil {
				t.Fatalf("read: %v", err)
			}
			if msg.Type == "error" {
				t.Fatalf("server error: %+v", msg.Error)
			}
			if msg.Type == "state" {
				return msg.Room
			}
		}
		t.Fatal("no state message received")
		return room.Snapshot{}
	}
	readUntil := func(conn *websocket.Conn, pred func(room.Snapshot) bool) room.Snapshot {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if snap := readState(conn); pred(snap) {
				return snap
			}
		}
		t.Fatal("condition not reached")
		return room.Snapshot{}
	}

	hostConn := dial("host")
	guestConn := dial("guest")
	if snap := readState(hostConn); snap.Phase != "lobby" {
		t.Fatalf("phase = %q, want lobby", snap.Phase)
	}

	if err := hostConn.WriteJSON(room.Intent{Type: "start_game"}); err != nil {
		t.Fatalf("write start: %v", err)
	}
	snap := readUntil(hostConn, func(s room.Snapshot) bool { return s.Phase == "playing" })
	if snap.Game == nil {
		t.Fatal("no game state after start")
	}

	current := snap.Game.Players[snap.Game.Current].ID
	active := hostConn
	if current == "guest" {
		active = guestConn
	}
	if err := active.WriteJSON(room.Intent{Type: "roll"}); err != nil {
		t.Fatalf("write roll: %v", err)
	}
	snap = readUntil(active, func(s room.Snapshot) bool {
		if s.Game == nil {
			return false
		}
		for _, event := range s.Game.Events {
			if event.Type == "roll" {
				return true
			}
		}
		return false
	})
	if snap.Game == nil {
		t.Fatal("no game state after roll")
	}
}

// presenceMessage is one message on the presence channel, whichever type it is.
type presenceMessage struct {
	Type    string                `json:"type"`
	Online  []presence.OnlineUser `json:"online"`
	Sent    []presence.SentInvite `json:"sent"`
	Invites []presence.Invitation `json:"invites"`
	Room    room.Snapshot         `json:"room"`
	Error   *presence.ErrorBody   `json:"error"`
}

// dialPresence opens a presence socket for one user; displayName is sent as the
// preferred-username header and query is the raw query string including the
// leading "?" (or ""). The display name never travels in the URL; the call with
// name=Grace deliberately exercises the ignored legacy parameter.
func dialPresence(
	t *testing.T, srv *httptest.Server, user, displayName, query string,
) *websocket.Conn {
	t.Helper()
	header := http.Header{}
	header.Set("X-Forwarded-User", user)
	header.Set("X-Forwarded-Preferred-Username", displayName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/ws/presence" + query
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, header)
	if err != nil {
		t.Fatalf("dial presence %s: %v", user, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func readPresence(t *testing.T, conn *websocket.Conn) presenceMessage {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	var msg presenceMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read presence: %v", err)
	}
	return msg
}

func readPresenceUntil(t *testing.T, conn *websocket.Conn, what string,
	pred func(presenceMessage) bool) presenceMessage {
	t.Helper()
	var lastErr *presence.ErrorBody
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		msg := readPresence(t, conn)
		if msg.Type == "error" {
			lastErr = msg.Error
		}
		if pred(msg) {
			return msg
		}
	}
	if lastErr != nil {
		t.Fatalf("no %s message received (last error: %s: %s)", what, lastErr.Code, lastErr.Message)
	}
	t.Fatalf("no %s message received", what)
	return presenceMessage{}
}

func TestPresenceRequiresAuth(t *testing.T) {
	s := newTestServer(t, "")
	rec := do(t, s, http.MethodGet, "/api/ws/presence", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestPresenceUnknownRoom(t *testing.T) {
	s := newTestServer(t, "")
	rec := do(t, s, http.MethodGet, "/api/ws/presence?room=ZZZZZZ", "u1", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error: %v (%s)", err, rec.Body.String())
	}
	if payload.Error.Code != "room_not_found" {
		t.Errorf("code = %q, want room_not_found", payload.Error.Code)
	}
}

func TestPresenceClosedRoomNotFound(t *testing.T) {
	s := newTestServer(t, "")
	code := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "host", "")).Code
	rec := do(t, s, http.MethodPost, "/api/rooms/"+code+"/leave", "host", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("leave status = %d, want %d", rec.Code, http.StatusNoContent)
	}

	rec = do(t, s, http.MethodGet, "/api/ws/presence?room="+code, "host", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error: %v (%s)", err, rec.Body.String())
	}
	if payload.Error.Code != "room_not_found" {
		t.Errorf("code = %q, want room_not_found", payload.Error.Code)
	}
}

func TestPresenceInviteFlow(t *testing.T) {
	s := newTestServer(t, "")
	body := `{"displayName":"Host","color":"blau"}`
	code := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "host", body)).Code
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	host := dialPresence(t, srv, "host", "Host", "?room="+code)
	invitee := dialPresence(t, srv, "guest", "Ben", "")

	if msg := readPresence(t, host); msg.Type != "lobby" || len(msg.Online) != 0 {
		t.Fatalf("host first message = %+v, want empty lobby", msg)
	}
	if msg := readPresence(t, invitee); msg.Type != "inbox" || len(msg.Invites) != 0 {
		t.Fatalf("invitee first message = %+v, want empty inbox", msg)
	}
	readPresenceUntil(t, host, "online list with Ben", func(m presenceMessage) bool {
		return m.Type == "lobby" && len(m.Online) == 1 &&
			m.Online[0].UserID == "guest" && m.Online[0].Name == "Ben"
	})

	if err := host.WriteJSON(presence.Intent{Type: "invite", UserID: "guest"}); err != nil {
		t.Fatalf("write invite: %v", err)
	}
	invitation := readPresenceUntil(t, invitee, "invitation", func(m presenceMessage) bool {
		return m.Type == "inbox" && len(m.Invites) == 1
	}).Invites[0]
	if invitation.Room != code || invitation.FromID != "host" {
		t.Errorf("invitation = %+v, want room %s from host", invitation, code)
	}
	readPresenceUntil(t, host, "sent invitation", func(m presenceMessage) bool {
		return m.Type == "lobby" && len(m.Sent) == 1 &&
			m.Sent[0].UserID == "guest" && m.Sent[0].Name == "Ben"
	})

	if err := invitee.WriteJSON(presence.Intent{
		Type:     "accept_invite",
		InviteID: invitation.ID,
		Name:     "Ben",
		Color:    "gruen",
	}); err != nil {
		t.Fatalf("write accept: %v", err)
	}
	accepted := readPresenceUntil(t, invitee, "accepted", func(m presenceMessage) bool {
		return m.Type == "accepted"
	})
	if len(accepted.Room.Seats) != 2 {
		t.Fatalf("seats = %+v, want 2", accepted.Room.Seats)
	}
	var joined *room.Seat
	for i := range accepted.Room.Seats {
		if accepted.Room.Seats[i].UserID == "guest" {
			joined = &accepted.Room.Seats[i]
		}
	}
	if joined == nil || joined.Name != "Ben" || joined.Color != "gruen" {
		t.Errorf("joined seat = %+v, want Ben/gruen", joined)
	}
	readPresenceUntil(t, host, "lobby after accept", func(m presenceMessage) bool {
		if m.Type != "lobby" || len(m.Sent) != 0 {
			return false
		}
		for _, u := range m.Online {
			if u.UserID == "guest" {
				return false
			}
		}
		return true
	})
}

func TestPresenceCancelFlow(t *testing.T) {
	s := newTestServer(t, "")
	code := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "host", "")).Code
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	host := dialPresence(t, srv, "host", "Host", "?room="+code)
	invitee := dialPresence(t, srv, "guest", "Ben", "")
	readPresence(t, host)
	readPresence(t, invitee)
	readPresenceUntil(t, host, "online list with Ben", func(m presenceMessage) bool {
		return m.Type == "lobby" && len(m.Online) == 1
	})

	if err := host.WriteJSON(presence.Intent{Type: "invite", UserID: "guest"}); err != nil {
		t.Fatalf("write invite: %v", err)
	}
	invitation := readPresenceUntil(t, invitee, "invitation", func(m presenceMessage) bool {
		return m.Type == "inbox" && len(m.Invites) == 1
	}).Invites[0]
	readPresenceUntil(t, host, "sent invitation", func(m presenceMessage) bool {
		return m.Type == "lobby" && len(m.Sent) == 1
	})

	cancel := presence.Intent{Type: "cancel_invite", InviteID: invitation.ID}
	if err := host.WriteJSON(cancel); err != nil {
		t.Fatalf("write cancel: %v", err)
	}
	readPresenceUntil(t, invitee, "empty inbox", func(m presenceMessage) bool {
		return m.Type == "inbox" && len(m.Invites) == 0
	})
	readPresenceUntil(t, host, "empty sent list", func(m presenceMessage) bool {
		return m.Type == "lobby" && len(m.Sent) == 0
	})
}

func TestPresenceSetName(t *testing.T) {
	s := newTestServer(t, "")
	code := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "host", "")).Code
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	host := dialPresence(t, srv, "host", "Host", "?room="+code)
	// The legacy display-name query parameter is ignored: the socket starts with
	// the identity name ("Ben") even though the URL asks for "Grace" (4.1, 6).
	legacyQuery := "?" + url.Values{"name": {"Grace"}}.Encode()
	start := dialPresence(t, srv, "guest", "Ben", legacyQuery)
	readPresence(t, host)
	readPresence(t, start)
	readPresenceUntil(t, host, "online list with Ben", func(m presenceMessage) bool {
		return m.Type == "lobby" && len(m.Online) == 1 && m.Online[0].Name == "Ben"
	})

	if err := start.WriteJSON(presence.Intent{Type: "set_name", Name: "Benjamin"}); err != nil {
		t.Fatalf("write set_name: %v", err)
	}
	readPresenceUntil(t, host, "renamed online list", func(m presenceMessage) bool {
		return m.Type == "lobby" && len(m.Online) == 1 && m.Online[0].Name == "Benjamin"
	})

	if err := host.WriteJSON(presence.Intent{Type: "set_name", Name: "Chef"}); err != nil {
		t.Fatalf("write room set_name: %v", err)
	}
	msg := readPresenceUntil(t, host, "invalid_scope error", func(m presenceMessage) bool {
		return m.Type == "error"
	})
	if msg.Error == nil || msg.Error.Code != "invalid_scope" {
		t.Errorf("error = %+v, want invalid_scope", msg.Error)
	}
}

// responseErrorCode returns the machine readable code of an HTTP error body.
func responseErrorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error body: %v (%s)", err, rec.Body.String())
	}
	return payload.Error.Code
}

// responseErrorMessage returns the human readable message of an HTTP error body.
func responseErrorMessage(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var payload struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error body: %v (%s)", err, rec.Body.String())
	}
	return payload.Error.Message
}

// findPlayer returns the public game view of one player.
func findPlayer(snap room.Snapshot, userID string) (game.PlayerView, bool) {
	if snap.Game == nil {
		return game.PlayerView{}, false
	}
	for _, p := range snap.Game.Players {
		if p.ID == userID {
			return p, true
		}
	}
	return game.PlayerView{}, false
}

// dialRoom opens the room WebSocket for one user.
func dialRoom(t *testing.T, srv *httptest.Server, user, code string) *websocket.Conn {
	t.Helper()
	header := http.Header{}
	header.Set("X-Forwarded-User", user)
	header.Set("X-Forwarded-Preferred-Username", user+" name")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/ws?room=" + code
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, header)
	if err != nil {
		t.Fatalf("dial room %s: %v", user, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// readRoomMessage reads one room frame and returns its raw JSON plus the
// snapshot. An error frame fails the test.
func readRoomMessage(t *testing.T, conn *websocket.Conn) ([]byte, room.Snapshot) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	var raw json.RawMessage
	if err := conn.ReadJSON(&raw); err != nil {
		t.Fatalf("read room message: %v", err)
	}
	var msg struct {
		Type  string        `json:"type"`
		Room  room.Snapshot `json:"room"`
		Error *struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("decode room message: %v (%s)", err, raw)
	}
	if msg.Type == "error" {
		t.Fatalf("server error: %+v", msg.Error)
	}
	return []byte(raw), msg.Room
}

// readRoomUntil reads states until pred matches and returns the matching raw
// frame and snapshot.
func readRoomUntil(t *testing.T, conn *websocket.Conn, what string,
	pred func(room.Snapshot) bool) ([]byte, room.Snapshot) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		raw, snap := readRoomMessage(t, conn)
		if pred(snap) {
			return raw, snap
		}
	}
	t.Fatalf("no %s state received", what)
	return nil, room.Snapshot{}
}

func TestLeaveRoom(t *testing.T) {
	s := newTestServer(t, "")
	code := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "host", "")).Code
	rec := do(t, s, http.MethodPost, "/api/rooms/"+code+"/join", "guest", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("join status = %d (%s)", rec.Code, rec.Body.String())
	}

	rec = do(t, s, http.MethodPost, "/api/rooms/"+code+"/leave", "host", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("leave status = %d, want %d (%s)", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Errorf("leave body = %q, want empty", rec.Body.String())
	}

	snap := decodeRoom(t, do(t, s, http.MethodGet, "/api/rooms/"+code, "guest", ""))
	if len(snap.Seats) != 1 || snap.Seats[0].UserID != "guest" || !snap.Seats[0].Host {
		t.Errorf("seats = %+v, want guest as host", snap.Seats)
	}
	if snap.HostID != "guest" {
		t.Errorf("hostId = %q, want guest", snap.HostID)
	}

	rec = do(t, s, http.MethodPost, "/api/rooms/"+code+"/leave", "guest", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("guest leave status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	rec = do(t, s, http.MethodGet, "/api/rooms/"+code, "guest", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET closed room status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if got := responseErrorCode(t, rec); got != "room_not_found" {
		t.Errorf("code = %q, want room_not_found", got)
	}
	rec = do(t, s, http.MethodPost, "/api/rooms/"+code+"/join", "guest", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("join closed room status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestLeaveRoomNonMemberIsNoOp(t *testing.T) {
	s := newTestServer(t, "")
	code := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "host", "")).Code

	rec := do(t, s, http.MethodPost, "/api/rooms/"+code+"/leave", "watcher", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("leave status = %d, want %d (%s)", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Errorf("leave body = %q, want empty", rec.Body.String())
	}

	snap := decodeRoom(t, do(t, s, http.MethodGet, "/api/rooms/"+code, "host", ""))
	if len(snap.Seats) != 1 || snap.Seats[0].UserID != "host" {
		t.Errorf("seats = %+v, want the host seat unchanged", snap.Seats)
	}
	if snap.HostID != "host" {
		t.Errorf("hostId = %q, want host", snap.HostID)
	}
}

func TestLeaveRoomUnknownOrClosed(t *testing.T) {
	s := newTestServer(t, "")
	rec := do(t, s, http.MethodPost, "/api/rooms/ZZZZZZ/leave", "u1", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown room status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if got := responseErrorCode(t, rec); got != "room_not_found" {
		t.Errorf("code = %q, want room_not_found", got)
	}

	code := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "host", "")).Code
	rec = do(t, s, http.MethodPost, "/api/rooms/"+code+"/leave", "host", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("leave status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	rec = do(t, s, http.MethodPost, "/api/rooms/"+code+"/leave", "host", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("closed room status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if got := responseErrorCode(t, rec); got != "room_not_found" {
		t.Errorf("code = %q, want room_not_found", got)
	}
}

func TestLeaveRoomMethodNotAllowed(t *testing.T) {
	s := newTestServer(t, "")
	code := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "host", "")).Code
	rec := do(t, s, http.MethodGet, "/api/rooms/"+code+"/leave", "host", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if allow := rec.Header().Get("Allow"); allow != http.MethodPost {
		t.Errorf("Allow = %q, want POST", allow)
	}
}

func TestLeaveRoomRunningGame(t *testing.T) {
	s := newTestServer(t, "")
	code := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "host", "")).Code
	do(t, s, http.MethodPost, "/api/rooms/"+code+"/join", "guest", "")
	rm, ok := s.rooms.Get(code)
	if !ok {
		t.Fatalf("room %q not found", code)
	}
	if err := rm.HandleIntent("host", room.Intent{Type: "start_game"}); err != nil {
		t.Fatalf("start_game error = %v", err)
	}

	rec := do(t, s, http.MethodPost, "/api/rooms/"+code+"/leave", "guest", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if got := responseErrorCode(t, rec); got != "game_started" {
		t.Errorf("code = %q, want game_started", got)
	}
	if !rm.HasMember("guest") {
		t.Error("running-game leave freed the seat, want the seat kept")
	}
}

func TestLeaveRoomLastMemberPushesEmptySeats(t *testing.T) {
	s := newTestServer(t, "")
	code := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "host", "")).Code
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	conn := dialRoom(t, srv, "host", code)
	if _, snap := readRoomMessage(t, conn); len(snap.Seats) != 1 {
		t.Fatalf("initial seats = %+v, want one", snap.Seats)
	}

	rec := do(t, s, http.MethodPost, "/api/rooms/"+code+"/leave", "host", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("leave status = %d, want %d", rec.Code, http.StatusNoContent)
	}

	raw, snap := readRoomUntil(t, conn, "empty seats", func(s room.Snapshot) bool {
		return len(s.Seats) == 0
	})
	if !strings.Contains(string(raw), `"seats":[]`) {
		t.Errorf("pushed state = %s, want \"seats\":[]", raw)
	}
	if strings.Contains(string(raw), `"seats":null`) {
		t.Errorf("pushed state = %s, want no null seats", raw)
	}
	if snap.Phase != "lobby" {
		t.Errorf("phase = %q, want lobby", snap.Phase)
	}
}

func TestAbandonRoomLobby(t *testing.T) {
	s := newTestServer(t, "")
	code := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "host", "")).Code
	do(t, s, http.MethodPost, "/api/rooms/"+code+"/join", "guest", "")

	rec := do(t, s, http.MethodPost, "/api/rooms/"+code+"/abandon", "host", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("abandon status = %d, want %d (%s)", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Errorf("abandon body = %q, want empty", rec.Body.String())
	}

	snap := decodeRoom(t, do(t, s, http.MethodGet, "/api/rooms/"+code, "guest", ""))
	if len(snap.Seats) != 1 || snap.Seats[0].UserID != "guest" || !snap.Seats[0].Host {
		t.Errorf("seats = %+v, want guest as host", snap.Seats)
	}
	if snap.HostID != "guest" {
		t.Errorf("hostId = %q, want guest", snap.HostID)
	}

	// AC4: the user can create a room again after the final leave.
	rec = do(t, s, http.MethodPost, "/api/rooms", "host", "")
	if rec.Code != http.StatusCreated {
		t.Errorf("create after abandon status = %d, want %d (%s)",
			rec.Code, http.StatusCreated, rec.Body.String())
	}
}

func TestAbandonRoomRunningGame(t *testing.T) {
	s := newTestServer(t, "")
	code := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "host", "")).Code
	do(t, s, http.MethodPost, "/api/rooms/"+code+"/join", "guest", "")
	rm, ok := s.rooms.Get(code)
	if !ok {
		t.Fatalf("room %q not found", code)
	}
	if err := rm.HandleIntent("host", room.Intent{Type: "start_game"}); err != nil {
		t.Fatalf("start_game error = %v", err)
	}

	rec := do(t, s, http.MethodPost, "/api/rooms/"+code+"/abandon", "guest", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("abandon status = %d, want %d (%s)", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if rm.HasMember("guest") {
		t.Error("HasMember(guest) = true after abandon, want the membership removed")
	}
	snap := decodeRoom(t, do(t, s, http.MethodGet, "/api/rooms/"+code, "host", ""))
	p, ok := findPlayer(snap, "guest")
	if !ok || !p.Bankrupt {
		t.Errorf("guest player = %+v (found %v), want bankrupt", p, ok)
	}

	rec = do(t, s, http.MethodPost, "/api/rooms", "guest", "")
	if rec.Code != http.StatusCreated {
		t.Errorf("create after abandon status = %d, want %d (%s)",
			rec.Code, http.StatusCreated, rec.Body.String())
	}
}

// TestAbandonRoomAllowsJoinAfterwards exercises AC4's join path directly: the
// final leave frees the user from the room gate so they can join another room.
func TestAbandonRoomAllowsJoinAfterwards(t *testing.T) {
	s := newTestServer(t, "")
	codeA := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "u1", "")).Code
	codeB := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "u2", "")).Code

	rec := do(t, s, http.MethodPost, "/api/rooms/"+codeB+"/join", "u1", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("join before abandon status = %d, want %d (%s)",
			rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if got := responseErrorCode(t, rec); got != "already_in_room" {
		t.Errorf("join before abandon code = %q, want already_in_room", got)
	}

	rec = do(t, s, http.MethodPost, "/api/rooms/"+codeA+"/abandon", "u1", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("abandon status = %d, want %d (%s)", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	rec = do(t, s, http.MethodPost, "/api/rooms/"+codeB+"/join", "u1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("join after abandon status = %d, want %d (%s)",
			rec.Code, http.StatusOK, rec.Body.String())
	}
	if snap := decodeRoom(t, rec); len(snap.Seats) != 2 {
		t.Errorf("seats = %d after join, want 2", len(snap.Seats))
	}
}

func TestAbandonRoomFinishedGame(t *testing.T) {
	s := newTestServer(t, "")
	code := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "host", "")).Code
	do(t, s, http.MethodPost, "/api/rooms/"+code+"/join", "guest", "")
	rm, ok := s.rooms.Get(code)
	if !ok {
		t.Fatalf("room %q not found", code)
	}
	if err := rm.HandleIntent("host", room.Intent{Type: "start_game"}); err != nil {
		t.Fatalf("start_game error = %v", err)
	}
	if err := rm.HandleIntent("guest", room.Intent{Type: "leave_room"}); err != nil {
		t.Fatalf("leave_room error = %v", err)
	}
	if got := rm.Phase(); got != "finished" {
		t.Fatalf("phase = %q, want finished", got)
	}

	rec := do(t, s, http.MethodPost, "/api/rooms/"+code+"/abandon", "host", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("abandon status = %d, want %d (%s)", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if rm.HasMember("host") {
		t.Error("HasMember(host) = true after abandon, want the stale seat removed")
	}
}

func TestAbandonRoomNonMemberNoOp(t *testing.T) {
	s := newTestServer(t, "")
	code := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "host", "")).Code

	rec := do(t, s, http.MethodPost, "/api/rooms/"+code+"/abandon", "watcher", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("abandon status = %d, want %d (%s)", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Errorf("abandon body = %q, want empty", rec.Body.String())
	}

	snap := decodeRoom(t, do(t, s, http.MethodGet, "/api/rooms/"+code, "host", ""))
	if len(snap.Seats) != 1 || snap.Seats[0].UserID != "host" {
		t.Errorf("seats = %+v, want the host seat unchanged", snap.Seats)
	}
	if snap.HostID != "host" {
		t.Errorf("hostId = %q, want host", snap.HostID)
	}
}

func TestAbandonRoomUnknownOrClosed(t *testing.T) {
	s := newTestServer(t, "")
	rec := do(t, s, http.MethodPost, "/api/rooms/ZZZZZZ/abandon", "u1", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown room status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if got := responseErrorCode(t, rec); got != "room_not_found" {
		t.Errorf("code = %q, want room_not_found", got)
	}

	code := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "host", "")).Code
	rec = do(t, s, http.MethodPost, "/api/rooms/"+code+"/abandon", "host", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("abandon status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	rec = do(t, s, http.MethodPost, "/api/rooms/"+code+"/abandon", "host", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("closed room status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if got := responseErrorCode(t, rec); got != "room_not_found" {
		t.Errorf("code = %q, want room_not_found", got)
	}
}

func TestAbandonRoomMethodNotAllowed(t *testing.T) {
	s := newTestServer(t, "")
	code := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "host", "")).Code
	rec := do(t, s, http.MethodGet, "/api/rooms/"+code+"/abandon", "host", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if allow := rec.Header().Get("Allow"); allow != http.MethodPost {
		t.Errorf("Allow = %q, want POST", allow)
	}
}

func TestAbandonRoomPushesState(t *testing.T) {
	s := newTestServer(t, "")
	code := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "host", "")).Code
	do(t, s, http.MethodPost, "/api/rooms/"+code+"/join", "guest", "")
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	conn := dialRoom(t, srv, "guest", code)
	if _, snap := readRoomMessage(t, conn); len(snap.Seats) != 2 {
		t.Fatalf("initial seats = %+v, want two", snap.Seats)
	}

	rec := do(t, s, http.MethodPost, "/api/rooms/"+code+"/abandon", "host", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("abandon status = %d, want %d", rec.Code, http.StatusNoContent)
	}

	_, snap := readRoomUntil(t, conn, "one seat", func(s room.Snapshot) bool {
		return len(s.Seats) == 1
	})
	if snap.Seats[0].UserID != "guest" || !snap.Seats[0].Host {
		t.Errorf("seats = %+v, want guest as host", snap.Seats)
	}
	if snap.HostID != "guest" {
		t.Errorf("hostId = %q, want guest", snap.HostID)
	}
}

func TestGetClosedRoomNotFound(t *testing.T) {
	s := newTestServer(t, "")
	code := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "host", "")).Code
	rec := do(t, s, http.MethodPost, "/api/rooms/"+code+"/leave", "host", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("leave status = %d, want %d", rec.Code, http.StatusNoContent)
	}

	rec = do(t, s, http.MethodGet, "/api/rooms/"+code, "host", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if got := responseErrorCode(t, rec); got != "room_not_found" {
		t.Errorf("code = %q, want room_not_found", got)
	}

	rec = do(t, s, http.MethodGet, "/api/ws?room="+code, "host", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("WS status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if got := responseErrorCode(t, rec); got != "room_not_found" {
		t.Errorf("WS code = %q, want room_not_found", got)
	}
}

func TestVotekickOverWebSocket(t *testing.T) {
	s := newTestServer(t, "")
	code := decodeRoom(t, do(t, s, http.MethodPost, "/api/rooms", "host", "")).Code
	do(t, s, http.MethodPost, "/api/rooms/"+code+"/join", "guest", "")
	do(t, s, http.MethodPost, "/api/rooms/"+code+"/join", "third", "")
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	host := dialRoom(t, srv, "host", code)
	guest := dialRoom(t, srv, "guest", code)
	third := dialRoom(t, srv, "third", code)
	readRoomMessage(t, host)
	readRoomMessage(t, guest)
	readRoomMessage(t, third)

	if err := host.WriteJSON(room.Intent{Type: "start_game"}); err != nil {
		t.Fatalf("write start_game: %v", err)
	}
	readRoomUntil(t, host, "playing", func(s room.Snapshot) bool { return s.Phase == "playing" })
	readRoomUntil(t, third, "playing", func(s room.Snapshot) bool { return s.Phase == "playing" })

	// Closing the target's socket pauses the seat; wait until the server
	// observed it before voting.
	if err := guest.Close(); err != nil {
		t.Fatalf("close guest: %v", err)
	}
	readRoomUntil(t, host, "guest disconnected", func(s room.Snapshot) bool {
		p, ok := findPlayer(s, "guest")
		return ok && !p.Connected
	})

	if err := host.WriteJSON(room.Intent{Type: "votekick", TargetID: "guest"}); err != nil {
		t.Fatalf("write host votekick: %v", err)
	}
	readRoomUntil(t, third, "one vote", func(s room.Snapshot) bool {
		for _, vote := range s.KickVotes {
			if vote.UserID == "guest" && vote.Votes == 1 {
				return true
			}
		}
		return false
	})

	if err := third.WriteJSON(room.Intent{Type: "votekick", TargetID: "guest"}); err != nil {
		t.Fatalf("write third votekick: %v", err)
	}
	_, snap := readRoomUntil(t, host, "guest bankrupt", func(s room.Snapshot) bool {
		p, ok := findPlayer(s, "guest")
		return ok && p.Bankrupt
	})
	if snap.Phase != "playing" {
		t.Errorf("phase = %q, want playing", snap.Phase)
	}
	if len(snap.KickVotes) != 0 {
		t.Errorf("kickVotes = %+v after the kick, want empty", snap.KickVotes)
	}
}
