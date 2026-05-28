package editor

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// streamSession buffers all SSE events for one chat turn so that clients can
// reconnect after closing the panel or navigating to a new page and still see
// the response stream live.
type streamSession struct {
	id string

	mu      sync.Mutex
	events  []json.RawMessage
	subs    map[chan struct{}]struct{}
	done    bool
	started time.Time
	ended   time.Time
}

func newStreamSession(id string) *streamSession {
	return &streamSession{
		id:      id,
		started: time.Now(),
		subs:    make(map[chan struct{}]struct{}),
	}
}

// Append serializes the payload as JSON and appends it to the buffered event
// list, notifying any active subscribers. Errors are silently ignored — the
// payloads here are always sent through marshalable map[string]any values.
func (s *streamSession) Append(payload map[string]any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	s.mu.Lock()
	s.events = append(s.events, data)
	s.notifyLocked()
	s.mu.Unlock()
}

// Finish marks the session as complete and wakes any subscribers so they can
// observe the terminal state and disconnect.
func (s *streamSession) Finish() {
	s.mu.Lock()
	if !s.done {
		s.done = true
		s.ended = time.Now()
	}
	s.notifyLocked()
	s.mu.Unlock()
}

func (s *streamSession) notifyLocked() {
	for ch := range s.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// Subscribe returns a notification channel signaled whenever new events are
// appended (or when the session finishes), and an unsubscribe func.
func (s *streamSession) Subscribe() (chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	// Prime the channel so the subscriber checks state immediately.
	select {
	case ch <- struct{}{}:
	default:
	}
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		delete(s.subs, ch)
		s.mu.Unlock()
	}
}

// Snapshot returns events appended at or after the given offset along with the
// current done flag. The returned slice is a copy — callers can iterate it
// without holding any lock.
func (s *streamSession) Snapshot(after int) ([]json.RawMessage, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []json.RawMessage
	if after < len(s.events) {
		out = make([]json.RawMessage, len(s.events)-after)
		copy(out, s.events[after:])
	}
	return out, s.done
}

// streamRegistry holds every active and recently finished stream session
// keyed by its opaque id.
type streamRegistry struct {
	mu       sync.Mutex
	sessions map[string]*streamSession
	ttl      time.Duration

	cleanupOnce sync.Once
}

func newStreamRegistry(ttl time.Duration) *streamRegistry {
	return &streamRegistry{
		sessions: make(map[string]*streamSession),
		ttl:      ttl,
	}
}

func (r *streamRegistry) Create() *streamSession {
	id := makeStreamID()
	s := newStreamSession(id)
	r.mu.Lock()
	r.sessions[id] = s
	r.mu.Unlock()
	r.startCleanupOnce()
	return s
}

func (r *streamRegistry) Get(id string) *streamSession {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sessions[id]
}

func (r *streamRegistry) startCleanupOnce() {
	r.cleanupOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				r.cleanup()
			}
		}()
	})
}

func (r *streamRegistry) cleanup() {
	cutoff := time.Now().Add(-r.ttl)
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, s := range r.sessions {
		s.mu.Lock()
		expired := s.done && !s.ended.IsZero() && s.ended.Before(cutoff)
		s.mu.Unlock()
		if expired {
			delete(r.sessions, id)
		}
	}
}

func makeStreamID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("s%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// defaultStreamRegistry holds finished sessions for 10 minutes so reconnects
// after a brief navigation still see the final events and persist the result.
var defaultStreamRegistry = newStreamRegistry(10 * time.Minute)
