package storage

import (
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EventBroadcaster defines the interface for broadcasting watch events.
// Implementations can use in-memory channels, database change streams,
// message queues, or any other event distribution mechanism.
type EventBroadcaster interface {
	// Broadcast sends an event to all active watchers.
	// Implementations should be non-blocking and handle slow consumers.
	Broadcast(event ResourceEvent)

	// Subscribe creates a new watch starting from the given resource version.
	// Returns:
	// - A channel that receives ResourceEvent
	// - A stop function to end the watch and clean up resources
	// - An error if the watch cannot be created
	//
	// If sinceResourceVersion is provided, implementations should send
	// historical events that occurred after that version, if available.
	Subscribe(sinceResourceVersion string) (<-chan ResourceEvent, func(), error)

	// Close shuts down the broadcaster and all active subscriptions.
	Close() error
}

// EventBroadcasterFactory creates an EventBroadcaster instance.
// This allows different implementations to be plugged in based on deployment needs.
type EventBroadcasterFactory func() EventBroadcaster

// WatchFilter filters watch events based on client options.
type WatchFilter interface {
	// Matches returns true if the event should be sent to the watcher.
	Matches(event ResourceEvent, opts client.ListOptions) bool
}

// DefaultWatchFilter implements filtering based on namespace and label selector.
type DefaultWatchFilter struct{}

// Matches implements WatchFilter interface.
func (f *DefaultWatchFilter) Matches(event ResourceEvent, opts client.ListOptions) bool {
	// BOOKMARK events always pass through
	if event.Type == EventBookmark {
		return true
	}

	obj := event.Object
	if obj == nil {
		return false
	}

	// Filter by namespace
	if opts.Namespace != "" && obj.GetNamespace() != opts.Namespace {
		return false
	}

	// Filter by label selector
	if opts.LabelSelector != nil {
		objLabels := obj.GetLabels()
		if !opts.LabelSelector.Matches(labels.Set(objLabels)) {
			return false
		}
	}

	return true
}
