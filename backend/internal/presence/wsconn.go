package presence

import (
	"sync"
	"time"

	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/ws"
)

const (
	sendBuffer = 32
	pingPeriod = 25 * time.Second
)

// WSConn is the presence transport over one WebSocket connection. It follows
// room.Client: one buffered send channel, one writer goroutine, a periodic ping
// and a close that runs once.
type WSConn struct {
	conn      *ws.Conn
	send      chan any
	drop      chan struct{}
	done      chan struct{}
	closeOnce sync.Once
}

// NewWSConn wraps an upgraded connection.
func NewWSConn(conn *ws.Conn) *WSConn {
	return &WSConn{
		conn: conn,
		send: make(chan any, sendBuffer),
		drop: make(chan struct{}, 1),
		done: make(chan struct{}),
	}
}

// Start launches the writer goroutine.
func (c *WSConn) Start() { go c.writeLoop() }

func (c *WSConn) writeLoop() {
	ticker := time.NewTicker(pingPeriod)
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
func (c *WSConn) Send(msg any) {
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

// Close closes the connection once.
func (c *WSConn) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.conn.Close()
	})
}
