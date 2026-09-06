package rebuilder

import (
	"sync"

	"github.com/liapoldus/liapoldus/backend/internal/application/build"
)

// Hub fans out DevRebuildEvent to every subscriber (live WS clients) and keeps
// the last event per site so a client that connects mid-session still gets the
// current bundle state immediately (relay on join).
type Hub struct {
	mu   sync.RWMutex
	subs map[chan build.DevRebuildEvent]struct{}
	last map[string]build.DevRebuildEvent
}

func NewHub() *Hub {
	return &Hub{
		subs: make(map[chan build.DevRebuildEvent]struct{}),
		last: make(map[string]build.DevRebuildEvent),
	}
}

var _ build.DevEventHub = (*Hub)(nil)

// Publish stores the event as the site's current state and delivers it to
// every subscriber without blocking on slow receivers.
func (h *Hub) Publish(event build.DevRebuildEvent) {
	h.mu.Lock()
	h.last[event.SiteID] = event
	subs := make([]chan build.DevRebuildEvent, 0, len(h.subs))
	for ch := range h.subs {
		subs = append(subs, ch)
	}
	h.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- event:
		default:
			// Receiver is slow: drop rather than stall the build loop.
		}
	}
}

// Subscribe registers a new consumer. The returned function removes it; call
// it once the connection closes.
func (h *Hub) Subscribe() (<-chan build.DevRebuildEvent, func()) {
	ch := make(chan build.DevRebuildEvent, 16)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()

	unsubscribe := func() {
		h.mu.Lock()
		if _, ok := h.subs[ch]; ok {
			delete(h.subs, ch)
			close(ch)
		}
		h.mu.Unlock()
	}
	return ch, unsubscribe
}

// Current returns the latest published event for the site (ok=false when the
// site never emitted one).
func (h *Hub) Current(siteID string) (build.DevRebuildEvent, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	event, ok := h.last[siteID]
	return event, ok
}
