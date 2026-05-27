package rtm

import "sync"

// Bus is a tiny in-process event fan-out. Subscribers get every event with
// non-blocking delivery — slow consumers drop, they don't backpressure the
// WebSocket reader.
type Bus struct {
	mu   sync.RWMutex
	subs map[chan Event]struct{}
}

func NewBus() *Bus { return &Bus{subs: map[chan Event]struct{}{}} }

// Subscribe returns a channel that receives events. Caller must call
// Unsubscribe when done; closing the returned channel directly is not safe.
func (b *Bus) Subscribe(buf int) chan Event {
	if buf <= 0 {
		buf = 64
	}
	ch := make(chan Event, buf)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *Bus) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	if _, ok := b.subs[ch]; ok {
		delete(b.subs, ch)
		close(ch)
	}
	b.mu.Unlock()
}

// Publish delivers ev to all subscribers. Drops to slow consumers.
func (b *Bus) Publish(ev Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- ev:
		default:
			// Slow consumer — drop.
		}
	}
}

func (b *Bus) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs)
}
