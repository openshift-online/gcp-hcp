package memory

import (
	"container/ring"
	"fmt"
	"strconv"
	"sync"

	"github.com/thetechnick/orlop/pkg/apiserver/storage"
)


// WatchBuffer stores recent events for a resource type to allow watch synchronization.
type WatchBuffer struct {
	mu     sync.RWMutex
	events *ring.Ring // Circular buffer of storage.ResourceEvent
	size   int
}

// NewWatchBuffer creates a new watch buffer with the specified capacity.
func NewWatchBuffer(size int) *WatchBuffer {
	return &WatchBuffer{
		events: ring.New(size),
		size:   size,
	}
}

// Add adds an event to the buffer.
func (wb *WatchBuffer) Add(event storage.ResourceEvent) {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	wb.events.Value = event
	wb.events = wb.events.Next()
}

// GetEventsSince returns all events since the given resource version.
// Returns nil if the requested version is too old (not in buffer).
func (wb *WatchBuffer) GetEventsSince(resourceVersion string) ([]storage.ResourceEvent, error) {
	wb.mu.RLock()
	defer wb.mu.RUnlock()

	if resourceVersion == "" || resourceVersion == "0" {
		// Start from beginning - return all buffered events
		return wb.getAllEvents(), nil
	}

	requestedRV, err := strconv.ParseInt(resourceVersion, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid resourceVersion: %v", err)
	}

	var result []storage.ResourceEvent
	wb.events.Do(func(v interface{}) {
		if v == nil {
			return
		}
		event := v.(storage.ResourceEvent)
		eventRV, err := strconv.ParseInt(event.ResourceVersion, 10, 64)
		if err != nil {
			return
		}
		// Include events with RV > requested RV
		if eventRV > requestedRV {
			result = append(result, event)
		}
	})

	return result, nil
}

// getAllEvents returns all non-nil events in the buffer.
func (wb *WatchBuffer) getAllEvents() []storage.ResourceEvent {
	var result []storage.ResourceEvent
	wb.events.Do(func(v interface{}) {
		if v != nil {
			result = append(result, v.(storage.ResourceEvent))
		}
	})
	return result
}

// Watcher manages watch connections for a resource type.
// Implements EventBroadcaster interface for in-memory event distribution.
type Watcher struct {
	mu          sync.RWMutex
	buffer      *WatchBuffer
	subscribers map[int]chan storage.ResourceEvent
	nextID      int
	closed      bool
}

// NewWatcher creates a new watcher with event buffering.
// This is the default in-memory implementation of EventBroadcaster.
func NewWatcher(bufferSize int) *Watcher {
	return &Watcher{
		buffer:      NewWatchBuffer(bufferSize),
		subscribers: make(map[int]chan storage.ResourceEvent),
	}
}

// Verify that Watcher implements EventBroadcaster at compile time.
var _ storage.EventBroadcaster = (*Watcher)(nil)

// Broadcast sends an event to all subscribers and adds it to the buffer.
func (w *Watcher) Broadcast(event storage.ResourceEvent) {
	// Add to buffer first
	w.buffer.Add(event)

	// Then broadcast to active watchers
	w.mu.RLock()
	defer w.mu.RUnlock()

	for _, ch := range w.subscribers {
		select {
		case ch <- event:
		default:
			// Don't block if subscriber is slow
		}
	}
}

// Subscribe creates a new watch channel and optionally sends historical events.
// Implements EventBroadcaster interface.
func (w *Watcher) Subscribe(sinceResourceVersion string) (<-chan storage.ResourceEvent, func(), error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil, nil, fmt.Errorf("watcher is closed")
	}

	id := w.nextID
	w.nextID++

	// Buffered channel to avoid blocking broadcaster
	ch := make(chan storage.ResourceEvent, 100)
	w.subscribers[id] = ch

	// Send historical events if requested
	if sinceResourceVersion != "" {
		events, err := w.buffer.GetEventsSince(sinceResourceVersion)
		if err != nil {
			close(ch)
			delete(w.subscribers, id)
			return nil, nil, err
		}

		// Send historical events
		for _, event := range events {
			select {
			case ch <- event:
			default:
				// Channel full, close it
				close(ch)
				delete(w.subscribers, id)
				return nil, nil, fmt.Errorf("watch channel overflow")
			}
		}
	}

	// Create stop function
	stopFunc := func() {
		w.Unsubscribe(id)
	}

	return ch, stopFunc, nil
}

// SubscribeWithID creates a new watch channel and returns the subscription ID.
// Deprecated: Use Subscribe() which returns a stop function instead.
// This method is kept for backward compatibility with existing code.
func (w *Watcher) SubscribeWithID(sinceResourceVersion string) (<-chan storage.ResourceEvent, int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil, 0, fmt.Errorf("watcher is closed")
	}

	id := w.nextID
	w.nextID++

	// Buffered channel to avoid blocking broadcaster
	ch := make(chan storage.ResourceEvent, 100)
	w.subscribers[id] = ch

	// Send historical events if requested
	if sinceResourceVersion != "" {
		events, err := w.buffer.GetEventsSince(sinceResourceVersion)
		if err != nil {
			close(ch)
			delete(w.subscribers, id)
			return nil, 0, err
		}

		// Send historical events
		for _, event := range events {
			select {
			case ch <- event:
			default:
				// Channel full, close it
				close(ch)
				delete(w.subscribers, id)
				return nil, 0, fmt.Errorf("watch channel overflow")
			}
		}
	}

	return ch, id, nil
}

// Unsubscribe removes a subscriber.
func (w *Watcher) Unsubscribe(id int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if ch, exists := w.subscribers[id]; exists {
		close(ch)
		delete(w.subscribers, id)
	}
}

// Count returns the number of active subscribers.
func (w *Watcher) Count() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.subscribers)
}

// Close shuts down the watcher and all active subscriptions.
// Implements EventBroadcaster interface.
func (w *Watcher) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	w.closed = true

	// Close all subscriber channels
	for id, ch := range w.subscribers {
		close(ch)
		delete(w.subscribers, id)
	}

	return nil
}
