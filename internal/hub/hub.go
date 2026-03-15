package hub

import "sync"

type Event struct {
	TaskID int32
	Data   any // carries monitor.ItemEvent without importing monitor
}

type Hub struct {
	mu   sync.RWMutex
	subs map[chan Event]struct{}
}

func New() *Hub {
	return &Hub{subs: make(map[chan Event]struct{})}
}

// Publish sends an event to all subscribers. Non-blocking: if a subscriber's
// channel is full, the event is dropped for that subscriber.
func (h *Hub) Publish(evt Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subs {
		select {
		case ch <- evt:
		default:
		}
	}
}

// Subscribe returns a channel that receives hub events. The caller must
// eventually call Unsubscribe to avoid leaking the channel.
func (h *Hub) Subscribe(bufSize int) <-chan Event {
	ch := make(chan Event, bufSize)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber channel and closes it.
func (h *Hub) Unsubscribe(ch <-chan Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for sub := range h.subs {
		if sub == ch {
			delete(h.subs, sub)
			close(sub)
			return
		}
	}
}
