package events

import "sync"

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
	log Logger

	mu      sync.RWMutex
	subs    map[chan Event]struct{}
	closed  bool
}

// NewBus constructs a Bus rooted at home. home is unused in this task
// (used in Task 4 for the JSONL writer).
func NewBus(home string, log Logger) (*Bus, error) {
	return &Bus{
		log:  log,
		subs: map[chan Event]struct{}{},
	}, nil
}

// Publish fans out to all current subscribers. Non-blocking: drops on
// any subscriber whose buffer is full (Task 2 wires the drop counter).
func (b *Bus) Publish(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return
	}
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

// Close stops new publishes and closes all subscriber channels.
func (b *Bus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	for ch := range b.subs {
		close(ch)
	}
	b.subs = nil
	return nil
}
