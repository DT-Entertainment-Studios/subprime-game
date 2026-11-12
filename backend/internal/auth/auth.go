// Package auth extracts the identity from the OAuth2 proxy headers and keeps
// the lazily provisioned users in memory (Spec.md §2).
package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/config"
)

type contextKey struct{}

// Identity is one authenticated proxy identity.
type Identity struct {
	ID     string
	Name   string
	Email  string
	Groups []string
}

// User is a lazily provisioned in-memory user.
type User struct {
	Identity
	FirstSeen time.Time
}

// Store holds all users for the lifetime of the process.
type Store struct {
	mu    sync.Mutex
	users map[string]*User
	now   func() time.Time
}

// NewStore creates an empty user store.
func NewStore() *Store {
	return &Store{users: map[string]*User{}, now: time.Now}
}

// Upsert provisions a user on first sight and updates display data.
func (s *Store) Upsert(id Identity) *User {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id.ID]
	if !ok {
		u = &User{Identity: id, FirstSeen: s.now()}
		s.users[id.ID] = u
		return u
	}
	u.Identity = id
	return u
}

// Get returns a provisioned user.
func (s *Store) Get(id string) (*User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	return u, ok
}

// Middleware enforces the identity header and stores the user in the request
// context. A missing or blank user id yields HTTP 401.
func Middleware(headers config.AuthHeaders, store *Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := strings.TrimSpace(r.Header.Get(headers.User))
			if id == "" {
				writeUnauthorized(w)
				return
			}
			name := strings.TrimSpace(r.Header.Get(headers.Name))
			if name == "" {
				name = id
			}
			identity := Identity{
				ID:     id,
				Name:   name,
				Email:  strings.TrimSpace(r.Header.Get(headers.Email)),
				Groups: splitGroups(r.Header.Get(headers.Groups)),
			}
			store.Upsert(identity)
			ctx := context.WithValue(r.Context(), contextKey{}, identity)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func splitGroups(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if field = strings.TrimSpace(field); field != "" {
			out = append(out, field)
		}
	}
	return out
}

// FromContext returns the identity stored by Middleware.
func FromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(contextKey{}).(Identity)
	return id, ok
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"message": "Nicht angemeldet.",
			"code":    "unauthorized",
		},
	})
}
