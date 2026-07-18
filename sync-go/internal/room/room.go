// Package room hosts collaborative canvases. Each room owns one CRDT document
// and a set of connected clients; ops are applied to the doc and fanned out to
// every other client, cursors are forwarded live, and new joiners get a
// snapshot of the current live shapes.
package room

import (
	"encoding/json"
	"sync"

	"glyphmesh/sync/internal/crdt"
	"glyphmesh/sync/internal/ws"
)

type Client struct {
	conn    *ws.Conn
	Replica string
	out     chan []byte
}

func (c *Client) send(b []byte) {
	select {
	case c.out <- b:
	default:
		// drop for a slow client rather than block the room
	}
}

type Room struct {
	id      string
	mu      sync.RWMutex
	doc     *crdt.Doc
	clients map[*Client]struct{}
}

func newRoom(id string) *Room {
	return &Room{id: id, doc: crdt.New(), clients: make(map[*Client]struct{})}
}

type envelope struct {
	Type string         `json:"type"`
	Room string         `json:"room"`
	Op   *crdt.ShapeOp  `json:"op,omitempty"`
	Ops  []crdt.ShapeOp `json:"ops,omitempty"`
}

func (r *Room) join(c *Client) {
	r.mu.Lock()
	r.clients[c] = struct{}{}
	r.mu.Unlock()

	snap := envelope{Type: "snapshot", Room: r.id, Ops: r.doc.Live()}
	if b, err := json.Marshal(snap); err == nil {
		c.send(b)
	}
}

func (r *Room) leave(c *Client) {
	r.mu.Lock()
	delete(r.clients, c)
	r.mu.Unlock()
	close(c.out)
}

func (r *Room) broadcast(msg []byte, except *Client) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for c := range r.clients {
		if c != except {
			c.send(msg)
		}
	}
}

// handle processes one raw client message.
func (r *Room) handle(raw []byte, from *Client) {
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return
	}
	switch env.Type {
	case "op":
		if env.Op == nil {
			return
		}
		if r.doc.Apply(*env.Op) {
			r.broadcast(raw, from)
		}
	case "cursor":
		r.broadcast(raw, from)
	}
}

// Clients returns the current client count (for /healthz).
func (r *Room) Clients() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients)
}

// Manager owns all rooms.
type Manager struct {
	mu    sync.Mutex
	rooms map[string]*Room
}

func NewManager() *Manager {
	return &Manager{rooms: make(map[string]*Room)}
}

func (m *Manager) get(id string) *Room {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rooms[id]
	if !ok {
		r = newRoom(id)
		m.rooms[id] = r
	}
	return r
}

func (m *Manager) Stats() (rooms, clients int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.rooms {
		clients += r.Clients()
	}
	return len(m.rooms), clients
}

// Serve runs the read/write loops for one upgraded connection.
func (m *Manager) Serve(conn *ws.Conn, roomID, replica string) {
	r := m.get(roomID)
	c := &Client{conn: conn, Replica: replica, out: make(chan []byte, 64)}

	// writer
	go func() {
		for msg := range c.out {
			if err := conn.WriteText(msg); err != nil {
				return
			}
		}
	}()

	r.join(c)
	defer func() {
		r.leave(c)
		conn.Close()
	}()

	for {
		msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		r.handle(msg, c)
	}
}
