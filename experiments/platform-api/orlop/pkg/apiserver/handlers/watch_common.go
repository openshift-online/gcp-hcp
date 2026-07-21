package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/thetechnick/orlop/pkg/apiserver/constants"
	"github.com/thetechnick/orlop/pkg/apiserver/storage"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// watchConfig holds watch configuration parsed from request.
type watchConfig struct {
	resourceVersion      string
	allowWatchBookmarks  bool
	sendInitialEvents    bool
	resourceVersionMatch string
	timeoutSeconds       string
}

// parseWatchConfig extracts watch parameters from HTTP request.
func parseWatchConfig(r *http.Request) watchConfig {
	query := r.URL.Query()
	return watchConfig{
		resourceVersion:      query.Get(constants.QueryParamResourceVersion),
		allowWatchBookmarks:  query.Get(constants.QueryParamAllowWatchBookmarks) == "true",
		sendInitialEvents:    query.Get(constants.QueryParamSendInitialEvents) == "true",
		resourceVersionMatch: query.Get(constants.QueryParamResourceVersionMatch),
		timeoutSeconds:       query.Get(constants.QueryParamTimeoutSeconds),
	}
}

// applyWatchTimeout applies timeout to context if specified.
func applyWatchTimeout(ctx context.Context, timeoutSeconds string) context.Context {
	if timeoutSeconds == "" {
		return ctx
	}

	timeout, err := strconv.ParseInt(timeoutSeconds, 10, 64)
	if err != nil || timeout <= 0 {
		return ctx
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	_ = cancel // cancel is handled by the parent context or watch stop
	return ctxWithTimeout
}

// watchStreamer handles the streaming watch protocol.
type watchStreamer struct {
	writer                 http.ResponseWriter
	flusher                http.Flusher
	encoder                *json.Encoder
	gvk                    schema.GroupVersionKind
	lastResourceVersion    string
	currentResourceVersion string
	initialBookmarkSent    bool
	listIsEmpty            bool
}

// newWatchStreamer creates a new watch streamer and sets up HTTP response.
func newWatchStreamer(w http.ResponseWriter, gvk schema.GroupVersionKind, currentRV string, isEmpty bool) (*watchStreamer, error) {
	// Set headers for streaming
	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	w.Header().Set(constants.HeaderTransferEncoding, constants.TransferEncodingChunked)
	w.WriteHeader(http.StatusOK)

	// Get flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("response writer does not support flushing")
	}
	flusher.Flush()

	return &watchStreamer{
		writer:                 w,
		flusher:                flusher,
		encoder:                json.NewEncoder(w),
		gvk:                    gvk,
		lastResourceVersion:    currentRV,
		currentResourceVersion: currentRV,
		initialBookmarkSent:    false,
		listIsEmpty:            isEmpty,
	}, nil
}

// sendEvent sends a watch event (ADDED, MODIFIED, DELETED).
func (ws *watchStreamer) sendEvent(eventType storage.EventType, obj interface{}) error {
	watchEvent := map[string]interface{}{
		"type":   string(eventType),
		"object": obj,
	}
	if err := ws.encoder.Encode(watchEvent); err != nil {
		return err
	}
	ws.flusher.Flush()
	return nil
}

// sendBookmark sends a BOOKMARK event with optional annotations.
func (ws *watchStreamer) sendBookmark(annotations map[string]interface{}) error {
	metadata := map[string]interface{}{
		"resourceVersion": ws.lastResourceVersion,
	}
	if annotations != nil {
		metadata["annotations"] = annotations
	}

	bookmarkObj := map[string]interface{}{
		"apiVersion":            ws.gvk.GroupVersion().String(),
		"kind":                  ws.gvk.Kind,
		constants.FieldMetadata: metadata,
	}

	watchEvent := map[string]interface{}{
		"type":   string(storage.EventBookmark),
		"object": bookmarkObj,
	}

	if err := ws.encoder.Encode(watchEvent); err != nil {
		return err
	}
	ws.flusher.Flush()
	return nil
}

// sendInitialBookmarkIfCaughtUp sends initial bookmark if already at current RV or list is empty.
func (ws *watchStreamer) sendInitialBookmarkIfCaughtUp(requestedRV string, allowBookmarks bool) error {
	if !allowBookmarks || ws.initialBookmarkSent {
		return nil
	}

	requestedRVInt, _ := strconv.ParseInt(requestedRV, 10, 64)
	currentRVInt, _ := strconv.ParseInt(ws.currentResourceVersion, 10, 64)

	if requestedRVInt >= currentRVInt || ws.listIsEmpty {
		ws.initialBookmarkSent = true
		return ws.sendBookmark(nil)
	}

	return nil
}

// checkAndSendCatchupBookmark sends bookmark when catching up to initial RV.
func (ws *watchStreamer) checkAndSendCatchupBookmark(eventRV string, allowBookmarks bool) error {
	if !allowBookmarks || ws.initialBookmarkSent {
		return nil
	}

	eventRVInt, _ := strconv.ParseInt(eventRV, 10, 64)
	currentRVInt, _ := strconv.ParseInt(ws.currentResourceVersion, 10, 64)

	if eventRVInt >= currentRVInt {
		ws.initialBookmarkSent = true
		return ws.sendBookmark(nil)
	}

	return nil
}

// objectTransformer is called for each object before sending to client.
// Allows converting/transforming objects (e.g., private to public conversion).
type objectTransformer func(client.Object) (interface{}, error)

// streamWatch runs the watch event loop with optional object transformation.
func streamWatch(
	ctx context.Context,
	streamer *watchStreamer,
	eventCh <-chan storage.ResourceEvent,
	config watchConfig,
	listOpts storage.ListOptions,
	store storage.ResourceStore,
	transformer objectTransformer,
) {
	// Setup periodic bookmarks
	var bookmarkTicker *time.Ticker
	var bookmarkCh <-chan time.Time
	if config.allowWatchBookmarks {
		bookmarkTicker = time.NewTicker(30 * time.Second)
		bookmarkCh = bookmarkTicker.C
		defer bookmarkTicker.Stop()
	}

	// Send initial events if requested
	if config.sendInitialEvents {
		list, err := store.List(ctx, listOpts)
		if err == nil {
			items, _ := meta.ExtractList(list)
			for _, item := range items {
				obj, ok := item.(client.Object)
				if !ok {
					continue
				}

				// Transform object if needed
				var sendObj interface{} = obj
				if transformer != nil {
					transformed, err := transformer(obj)
					if err != nil {
						continue // Skip failed transformations
					}
					sendObj = transformed
				}

				if err := streamer.sendEvent(storage.EventAdded, sendObj); err != nil {
					return
				}
			}

			// Send bookmark marking end of initial events
			if config.allowWatchBookmarks {
				streamer.sendBookmark(map[string]interface{}{
					constants.AnnotationInitialEventsEnd: "true",
				})
				streamer.initialBookmarkSent = true
			}
		}
	}

	// Send initial bookmark if already caught up
	requestedRV := config.resourceVersion
	if requestedRV == "" {
		requestedRV = "0"
	}
	if !config.sendInitialEvents {
		streamer.sendInitialBookmarkIfCaughtUp(requestedRV, config.allowWatchBookmarks)
	}

	// Main event loop
	for {
		select {
		case <-ctx.Done():
			return

		case <-bookmarkCh:
			// Send periodic bookmark
			if err := streamer.sendBookmark(nil); err != nil {
				return
			}

		case event, ok := <-eventCh:
			if !ok {
				return
			}

			// Update last resource version
			streamer.lastResourceVersion = event.ResourceVersion

			// Transform object if needed
			obj, ok := event.Object.(client.Object)
			if !ok {
				continue
			}

			var sendObj interface{} = obj
			if transformer != nil {
				transformed, err := transformer(obj)
				if err != nil {
					continue // Skip failed transformations
				}
				sendObj = transformed
			}

			// Send event
			if err := streamer.sendEvent(event.Type, sendObj); err != nil {
				return
			}

			// Send catchup bookmark if needed
			if err := streamer.checkAndSendCatchupBookmark(event.ResourceVersion, config.allowWatchBookmarks); err != nil {
				return
			}
		}
	}
}
