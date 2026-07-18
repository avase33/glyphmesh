// Package crdt implements a LWW-Element-Map: a conflict-free replicated map of
// shape id -> shape, where concurrent edits are resolved last-writer-wins on
// the pair (Lamport clock, replica id). Apply is commutative and idempotent, so
// replicas that see the same set of ops converge regardless of order.
package crdt

import (
	"sort"
	"sync"
)

// ShapeOp is the CRDT unit (see proto/protocol.md). A tombstone is Deleted=true.
type ShapeOp struct {
	ID      string    `json:"id"`
	Replica string    `json:"replica"`
	Lamport uint64    `json:"lamport"`
	Deleted bool      `json:"deleted"`
	Kind    string    `json:"kind,omitempty"`
	Points  []float64 `json:"points,omitempty"`
	Color   string    `json:"color,omitempty"`
}

// dominates reports whether a should win over b.
func (a ShapeOp) dominates(b ShapeOp) bool {
	if a.Lamport != b.Lamport {
		return a.Lamport > b.Lamport
	}
	return a.Replica > b.Replica
}

// Doc is a replicated document.
type Doc struct {
	mu     sync.Mutex
	shapes map[string]ShapeOp
	clock  uint64
}

func New() *Doc {
	return &Doc{shapes: make(map[string]ShapeOp)}
}

// Apply merges an op and reports whether it changed local state.
func (d *Doc) Apply(op ShapeOp) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if op.Lamport > d.clock {
		d.clock = op.Lamport
	}
	cur, ok := d.shapes[op.ID]
	if !ok || op.dominates(cur) {
		d.shapes[op.ID] = op
		return true
	}
	return false
}

// Tick advances and returns the local Lamport clock.
func (d *Doc) Tick() uint64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.clock++
	return d.clock
}

// Snapshot returns every op (including tombstones), sorted by id, so two docs
// with the same history serialise identically.
func (d *Doc) Snapshot() []ShapeOp {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]ShapeOp, 0, len(d.shapes))
	for _, op := range d.shapes {
		out = append(out, op)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Live returns only the non-deleted shapes, sorted by id.
func (d *Doc) Live() []ShapeOp {
	out := []ShapeOp{}
	for _, op := range d.Snapshot() {
		if !op.Deleted {
			out = append(out, op)
		}
	}
	return out
}
