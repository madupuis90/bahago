package hub

import (
	"slices"
	"sync"

	"bahago/internal/database/db"
)

// Hub fans out post-tick kingdom snapshots to subscribed SSE handlers.
type Hub struct {
	mu   sync.Mutex
	subs map[int][]chan db.Kingdom
}

// New returns a ready-to-use Hub.
func New() *Hub {
	return &Hub{subs: make(map[int][]chan db.Kingdom)}
}

// Subscribe registers a listener for the given kingdom ID and returns a
// receive-only channel and an unsubscribe function. The caller must call
// the cleanup function (typically deferred) to avoid a goroutine leak.
func (h *Hub) Subscribe(kingdomID int) (<-chan db.Kingdom, func()) {
	ch := make(chan db.Kingdom, 1)

	h.mu.Lock()
	h.subs[kingdomID] = append(h.subs[kingdomID], ch)
	h.mu.Unlock()

	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		list := h.subs[kingdomID]
		for i, s := range list {
			if s == ch {
				h.subs[kingdomID] = slices.Delete(list, i, i+1)
				break
			}
		}
		close(ch)
	}
}

// Publish delivers a kingdom snapshot to all active subscribers for that kingdom.
// Slow subscribers are skipped via a non-blocking send — they will receive the
// next tick instead.
func (h *Hub) Publish(k db.Kingdom) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ch := range h.subs[k.ID] {
		select {
		case ch <- k:
		default:
		}
	}
}
