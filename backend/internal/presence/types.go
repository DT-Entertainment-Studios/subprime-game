// Package presence tracks the signed-in users who wait on the start page and
// carries room invitations over a dedicated WebSocket channel (Spec.md §6).
package presence

import (
	"time"

	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/room"
)

// Intent is one client command on the presence channel. Type is one of invite,
// cancel_invite, accept_invite, decline_invite or set_name.
type Intent struct {
	Type     string `json:"type"`
	UserID   string `json:"userId"`   // invite
	InviteID string `json:"inviteId"` // cancel_invite | accept_invite | decline_invite
	Name     string `json:"name"`     // accept_invite | set_name
	Color    string `json:"color"`    // accept_invite
}

// OnlineUser is one start-page user as seen from a room lobby.
type OnlineUser struct {
	UserID string `json:"userId"`
	Name   string `json:"name"`
}

// Invitation is one pending invitation as seen by the invitee.
type Invitation struct {
	ID        string    `json:"id"`
	Room      string    `json:"room"`
	FromID    string    `json:"fromId"`
	FromName  string    `json:"fromName"`
	CreatedAt time.Time `json:"createdAt"`
}

// SentInvite is one pending invitation as seen by the inviting room.
type SentInvite struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Name      string    `json:"name"`
	FromID    string    `json:"fromId"`
	FromName  string    `json:"fromName"`
	CreatedAt time.Time `json:"createdAt"`
}

// LobbyState is pushed to a room-scoped connection.
type LobbyState struct {
	Type   string       `json:"type"` // "lobby"
	Online []OnlineUser `json:"online"`
	Sent   []SentInvite `json:"sent"`
}

// InboxState is pushed to a start-page connection.
type InboxState struct {
	Type    string       `json:"type"` // "inbox"
	Invites []Invitation `json:"invites"`
}

// Accepted is the reply to a successful accept_invite.
type Accepted struct {
	Type string        `json:"type"` // "accepted"
	Room room.Snapshot `json:"room"`
}

// ErrorMessage is a rejected request on the presence channel.
type ErrorMessage struct {
	Type  string     `json:"type"` // "error"
	Error *ErrorBody `json:"error"`
}

// ErrorBody is the error payload of a presence message.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error is a rejected presence request; only Code and Message matter because
// the channel is a WebSocket, not HTTP.
type Error struct{ Code, Message string }

func (e *Error) Error() string { return e.Message }
