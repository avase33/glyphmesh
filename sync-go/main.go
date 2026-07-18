// Command glyphmesh-sync is the multiplayer CRDT sync server. It speaks a
// from-scratch WebSocket protocol (internal/ws), keeps one LWW-Element-Map per
// room (internal/crdt), and fans shape ops + cursors out to collaborators.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"glyphmesh/sync/internal/room"
	"glyphmesh/sync/internal/ws"
)

func main() {
	addr := os.Getenv("GLYPHMESH_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	mgr := room.NewManager()

	mux := http.NewServeMux()

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := ws.Upgrade(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		roomID := r.URL.Query().Get("room")
		if roomID == "" {
			roomID = "default"
		}
		replica := r.URL.Query().Get("replica")
		if replica == "" {
			replica = "anon"
		}
		mgr.Serve(conn, roomID, replica)
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		rooms, clients := mgr.Stats()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"rooms":   rooms,
			"clients": clients,
		})
	})

	log.Printf("glyphmesh-sync on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
