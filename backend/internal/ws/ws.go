// Package ws is a thin wrapper around gorilla/websocket with the limits and
// helpers the game server needs.
package ws

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	readLimit  = 1 << 20
	writeWait  = 10 * time.Second
	pongWait   = 90 * time.Second
	pingPeriod = 30 * time.Second
)

// Conn is a single WebSocket connection.
type Conn struct {
	c    *websocket.Conn
	mu   sync.Mutex
	once sync.Once
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	// Identity comes from the reverse proxy headers, never from the origin.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Upgrade upgrades an HTTP request to a WebSocket connection.
func Upgrade(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	c.SetReadLimit(readLimit)
	_ = c.SetReadDeadline(time.Now().Add(pongWait))
	c.SetPongHandler(func(string) error {
		return c.SetReadDeadline(time.Now().Add(pongWait))
	})
	return &Conn{c: c}, nil
}

// ReadJSON reads one JSON message.
func (c *Conn) ReadJSON(v any) error { return c.c.ReadJSON(v) }

// WriteJSON writes one JSON message.
func (c *Conn) WriteJSON(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.c.SetWriteDeadline(time.Now().Add(writeWait))
	return c.c.WriteJSON(v)
}

// Ping sends a WebSocket ping frame.
func (c *Conn) Ping() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.c.SetWriteDeadline(time.Now().Add(writeWait))
	return c.c.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait))
}

// Close closes the connection once.
func (c *Conn) Close() error {
	var err error
	c.once.Do(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		_ = c.c.SetWriteDeadline(time.Now().Add(writeWait))
		err = c.c.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			time.Now().Add(writeWait))
		_ = c.c.Close()
	})
	return err
}
