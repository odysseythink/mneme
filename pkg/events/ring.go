package events

import "sync"

const ringCapacity = 500

// ring is a fixed-size circular buffer of Events.
// Newest at the head. Concurrent-safe.
type ring struct {
	mu   sync.RWMutex
	buf  []Event // length 0..ringCapacity, in insertion order (oldest at index 0)
}

func newRing() *ring {
	return &ring{buf: make([]Event, 0, ringCapacity)}
}

func (r *ring) push(e Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.buf) < ringCapacity {
		r.buf = append(r.buf, e)
		return
	}
	// shift left by one, then append (cheap at cap=500, hot path at low rates)
	copy(r.buf, r.buf[1:])
	r.buf[ringCapacity-1] = e
}

// tail returns up to limit events whose TS > since, newest-first.
func (r *ring) tail(limit int, since int64) []Event {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if limit <= 0 {
		return nil
	}
	out := make([]Event, 0, limit)
	for i := len(r.buf) - 1; i >= 0 && len(out) < limit; i-- {
		if r.buf[i].TS > since {
			out = append(out, r.buf[i])
		}
	}
	return out
}
