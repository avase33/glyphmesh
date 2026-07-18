package crdt

import (
	"encoding/json"
	"testing"
)

func serialize(d *Doc) string {
	b, _ := json.Marshal(d.Snapshot())
	return string(b)
}

func TestConvergesRegardlessOfOrder(t *testing.T) {
	ops := []ShapeOp{
		{ID: "s1", Replica: "a", Lamport: 1, Kind: "rectangle", Points: []float64{0, 0, 10, 10}},
		{ID: "s1", Replica: "b", Lamport: 3, Kind: "rectangle", Points: []float64{0, 0, 20, 20}},
		{ID: "s2", Replica: "a", Lamport: 2, Kind: "circle"},
		{ID: "s1", Replica: "a", Lamport: 2, Kind: "rectangle", Points: []float64{5, 5, 15, 15}},
	}

	d1 := New()
	for _, op := range ops {
		d1.Apply(op)
	}

	// apply to a second doc in reverse order
	d2 := New()
	for i := len(ops) - 1; i >= 0; i-- {
		d2.Apply(ops[i])
	}

	if serialize(d1) != serialize(d2) {
		t.Fatalf("did not converge:\n d1=%s\n d2=%s", serialize(d1), serialize(d2))
	}
	// s1 should be b@3 (highest lamport)
	for _, op := range d1.Snapshot() {
		if op.ID == "s1" && (op.Replica != "b" || op.Lamport != 3) {
			t.Fatalf("s1 resolved wrong: %+v", op)
		}
	}
}

func TestIdempotent(t *testing.T) {
	d := New()
	op := ShapeOp{ID: "x", Replica: "a", Lamport: 5, Kind: "line"}
	if !d.Apply(op) {
		t.Fatal("first apply should change state")
	}
	if d.Apply(op) {
		t.Fatal("re-applying the same op should be a no-op")
	}
}

func TestLamportTieBreaksOnReplica(t *testing.T) {
	d := New()
	d.Apply(ShapeOp{ID: "x", Replica: "a", Lamport: 7})
	d.Apply(ShapeOp{ID: "x", Replica: "z", Lamport: 7}) // same clock, larger replica wins
	snap := d.Snapshot()
	if snap[0].Replica != "z" {
		t.Fatalf("tie should go to replica z, got %s", snap[0].Replica)
	}
}

func TestTombstoneHidesFromLive(t *testing.T) {
	d := New()
	d.Apply(ShapeOp{ID: "x", Replica: "a", Lamport: 1, Kind: "line"})
	d.Apply(ShapeOp{ID: "x", Replica: "a", Lamport: 2, Deleted: true})
	if len(d.Live()) != 0 {
		t.Fatalf("deleted shape should not be live")
	}
	if len(d.Snapshot()) != 1 {
		t.Fatalf("tombstone should remain in snapshot")
	}
}
