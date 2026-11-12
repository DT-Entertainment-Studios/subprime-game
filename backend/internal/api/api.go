// Package api exposes the HTTP and WebSocket surface of the Subprime server
// (Spec.md §6).
package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/auth"
	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/config"
	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/game"
	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/presence"
	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/room"
	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/ws"
)

// Server is the HTTP handler of the application.
type Server struct {
	cfg    config.Config
	store  *auth.Store
	rooms  *room.Manager
	hub    *presence.Hub
	log    *slog.Logger
	auth   func(http.Handler) http.Handler
	static http.Handler
}

// New creates the server.
func New(cfg config.Config, store *auth.Store, rooms *room.Manager, hub *presence.Hub,
	log *slog.Logger) *Server {
	s := &Server{
		cfg:   cfg,
		store: store,
		rooms: rooms,
		hub:   hub,
		log:   log,
		auth:  auth.Middleware(cfg.Auth, store),
	}
	s.static = staticHandler(cfg.StaticDir)
	return s
}

// Handler returns the root handler.
func (s *Server) Handler() http.Handler { return s }

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		s.serveAPI(w, r)
		return
	}
	s.static.ServeHTTP(w, r)
}

func (s *Server) serveAPI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/health" {
		s.health(w, r)
		return
	}
	s.auth(http.HandlerFunc(s.apiRoutes)).ServeHTTP(w, r)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) apiRoutes(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/api/me":
		s.me(w, r)
	case r.URL.Path == "/api/me/room":
		s.myRoom(w, r)
	case r.URL.Path == "/api/ws":
		s.websocket(w, r)
	case r.URL.Path == "/api/ws/presence":
		s.presenceSocket(w, r)
	case r.URL.Path == "/api/rooms":
		s.createRoom(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/rooms/"):
		s.roomRoutes(w, r)
	default:
		writeError(w, http.StatusNotFound, "not_found", "Unbekannter Endpunkt.")
	}
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	identity, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Nicht angemeldet.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"id":     identity.ID,
			"name":   identity.Name,
			"email":  identity.Email,
			"groups": identity.Groups,
		},
	})
}

// myRoom reports the caller's currently blocking room (the same rule as
// create/join/invites) as {"room":null} or {"room":{"code":…,"phase":…}}.
func (s *Server) myRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	identity, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Nicht angemeldet.")
		return
	}
	ref, ok := s.rooms.UserRoomRef(identity.ID)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"room": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"room": ref})
}

type roomRequest struct {
	DisplayName string `json:"displayName"`
	Color       string `json:"color"`
}

func (s *Server) createRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	identity, _ := auth.FromContext(r.Context())
	var body roomRequest
	if err := decodeBody(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "Ungültiger Anfrageinhalt.")
		return
	}
	if body.Color != "" && !room.ValidColor(body.Color) {
		writeError(w, http.StatusBadRequest, "invalid_color", "Unbekannte Spielfarbe.")
		return
	}
	rm, err := s.rooms.Create(identity.ID, room.NormalizeName(body.DisplayName, identity.Name), body.Color)
	if err != nil {
		s.writeRoomError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"room": rm.Snapshot(identity.ID)})
}

func (s *Server) roomRoutes(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/rooms/")
	parts := strings.Split(rest, "/")
	if len(parts) == 1 && parts[0] != "" {
		s.getRoom(w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "join" {
		s.joinRoom(w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "leave" {
		s.leaveRoom(w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "abandon" {
		s.abandonRoom(w, r, parts[0])
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "Unbekannter Endpunkt.")
}

func (s *Server) getRoom(w http.ResponseWriter, r *http.Request, code string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	identity, _ := auth.FromContext(r.Context())
	rm, ok := s.rooms.Get(code)
	if !ok {
		writeError(w, http.StatusNotFound, "room_not_found", "Unbekannter Raum.")
		return
	}
	if rm.Closed() {
		writeError(w, http.StatusNotFound, "room_not_found", "Unbekannter Raum.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"room": rm.Snapshot(identity.ID)})
}

func (s *Server) joinRoom(w http.ResponseWriter, r *http.Request, code string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	identity, _ := auth.FromContext(r.Context())
	var body roomRequest
	if err := decodeBody(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "Ungültiger Anfrageinhalt.")
		return
	}
	if body.Color != "" && !room.ValidColor(body.Color) {
		writeError(w, http.StatusBadRequest, "invalid_color", "Unbekannte Spielfarbe.")
		return
	}
	_, snap, err := s.rooms.Join(code, identity.ID, room.NormalizeName(body.DisplayName, identity.Name), body.Color)
	if err != nil {
		s.writeRoomError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"room": snap})
}

// leaveRoom frees the caller's lobby seat. It has no request body and answers
// 204 No Content; a user without a seat is a no-op.
func (s *Server) leaveRoom(w http.ResponseWriter, r *http.Request, code string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	identity, _ := auth.FromContext(r.Context())
	rm, ok := s.rooms.Get(code)
	if !ok {
		writeError(w, http.StatusNotFound, "room_not_found", "Unbekannter Raum.")
		return
	}
	if err := rm.LeaveLobby(identity.ID); err != nil {
		s.writeRoomError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// abandonRoom finally gives up the caller's seat (204, no body). A room that is
// already gone or closed answers 404 room_not_found; the client treats that as
// success, like leave.
func (s *Server) abandonRoom(w http.ResponseWriter, r *http.Request, code string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	identity, _ := auth.FromContext(r.Context())
	rm, ok := s.rooms.Get(code)
	if !ok {
		writeError(w, http.StatusNotFound, "room_not_found", "Unbekannter Raum.")
		return
	}
	if err := rm.Abandon(identity.ID); err != nil {
		s.writeRoomError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) websocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	identity, _ := auth.FromContext(r.Context())
	code := r.URL.Query().Get("room")
	if strings.TrimSpace(code) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Raumcode fehlt.")
		return
	}
	rm, ok := s.rooms.Get(code)
	if !ok {
		writeError(w, http.StatusNotFound, "room_not_found", "Unbekannter Raum.")
		return
	}
	if rm.Closed() {
		writeError(w, http.StatusNotFound, "room_not_found", "Unbekannter Raum.")
		return
	}
	conn, err := ws.Upgrade(w, r)
	if err != nil {
		s.log.Warn("websocket upgrade failed", "error", err)
		return
	}
	client := room.NewClient(conn, identity.ID)
	rm.AddClient(client)
	client.Start()
	defer func() {
		rm.RemoveClient(client)
		client.Close()
	}()
	for {
		var intent room.Intent
		if err := conn.ReadJSON(&intent); err != nil {
			if !errors.Is(err, io.EOF) {
				s.log.Debug("websocket closed", "error", err, "user", identity.ID)
			}
			return
		}
		if err := rm.HandleIntent(identity.ID, intent); err != nil {
			rm.SendError(client, errorCode(err), err.Error())
		}
	}
}

func (s *Server) presenceSocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	identity, _ := auth.FromContext(r.Context())
	code := r.URL.Query().Get("room")
	if strings.TrimSpace(code) != "" {
		rm, ok := s.rooms.Get(code)
		if !ok || rm.Closed() {
			writeError(w, http.StatusNotFound, "room_not_found", "Unbekannter Raum.")
			return
		}
	}
	name := room.NormalizeName(identity.Name, "")
	conn, err := ws.Upgrade(w, r)
	if err != nil {
		s.log.Warn("presence websocket upgrade failed", "error", err)
		return
	}
	sink := presence.NewWSConn(conn)
	client := s.hub.Connect(identity.ID, code, name, sink)
	sink.Start()
	defer func() {
		s.hub.Disconnect(client)
		client.Close()
	}()
	for {
		var intent presence.Intent
		if err := conn.ReadJSON(&intent); err != nil {
			if !errors.Is(err, io.EOF) {
				s.log.Debug("presence websocket closed", "error", err, "user", identity.ID)
			}
			return
		}
		if err := s.hub.HandleIntent(client, intent); err != nil {
			s.hub.SendError(client, err)
		}
	}
}

func (s *Server) writeRoomError(w http.ResponseWriter, err error) {
	var re *room.Error
	if errors.As(err, &re) {
		writeError(w, re.Status, re.Code, re.Message)
		return
	}
	s.log.Error("internal error", "error", err)
	writeError(w, http.StatusInternalServerError, "internal", "Interner Fehler.")
}

func errorCode(err error) string {
	var pe *presence.Error
	if errors.As(err, &pe) {
		return pe.Code
	}
	var re *room.Error
	if errors.As(err, &re) {
		return re.Code
	}
	var ge *game.RuleError
	if errors.As(err, &ge) {
		return ge.Code
	}
	return "internal"
}

func decodeBody(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"message": message,
			"code":    code,
		},
	})
}

func methodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Methode nicht erlaubt.")
}

// staticHandler serves the built SPA from disk and falls back to index.html.
func staticHandler(dir string) http.Handler {
	if strings.TrimSpace(dir) == "" {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusNotFound, "not_found", "Es ist kein Frontend konfiguriert.")
		})
	}
	fs := http.FileServer(http.Dir(dir))
	index := filepath.Join(dir, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		clean := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(clean); err == nil && !info.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, index)
	})
}
