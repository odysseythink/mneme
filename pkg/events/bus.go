package events

import (
	"sync"
	"time"
)

// Logger is the minimal sink for write errors and rotation events.
// Implementations: pkg/daemon/log.go fileLogger.
type Logger interface {
	Debug(component, msg string)
	Info(component, msg string)
	Warn(component, msg string)
	Error(component, msg string)
}

const subscriberBuffer = 64

// Bus is a single-process pub/sub with persistence + ring history.
// Skeleton: in-memory only; ring + jsonl wired in later tasks.
type Bus struct {
	log   Logger
	clock func() time.Time

	mu     sync.RWMutex
	subs   map[chan Event]struct{}
	closed bool
	ring   *ring
	jsonl  *jsonlWriter
}

func NewBus(home string, log Logger) (*Bus, error) {
	return NewBusWithClock(home, log, time.Now)
}

// NewBusWithClock allows test injection of the clock.
func NewBusWithClock(home string, log Logger, clock func() time.Time) (*Bus, error) {
	if clock == nil {
		clock = time.Now
	}
	b := &Bus{
		log:   log,
		clock: clock,
		subs:  map[chan Event]struct{}{},
		ring:  newRing(),
		jsonl: newJSONLWriter(home, clock, log),
	}
	return b, nil
}

// Publish fans out to all current subscribers. Non-blocking: drops on
// any subscriber whose buffer is full (Task 2 wires the drop counter).
func (b *Bus) Publish(e Event) {
	b.mu.RLock()
	closed := b.closed
	b.mu.RUnlock()
	if closed {
		return
	}
	b.ring.push(e)
	b.jsonl.append(e)

	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
			// drop on full
		}
	}
}

// Subscribe returns a buffered channel and an unsubscribe func.
// The caller must call unsub exactly once.
func (b *Bus) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, subscriberBuffer)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if _, ok := b.subs[ch]; ok {
			delete(b.subs, ch)
			close(ch)
		}
	}
}

// Tail returns up to limit events with TS > since, newest-first.
// limit is capped at 500.
func (b *Bus) Tail(limit int, since int64) []Event {
	if limit > 500 {
		limit = 500
	}
	return b.ring.tail(limit, since)
}

// Close stops new publishes and closes all subscriber channels.
func (b *Bus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	for ch := range b.subs {
		close(ch)
	}
	b.subs = nil
	if b.jsonl != nil {
		_ = b.jsonl.close()
	}
	return nil
}
