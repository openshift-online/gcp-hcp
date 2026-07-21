package memory

import (
	"fmt"
	"testing"
	"time"

	"github.com/thetechnick/orlop/pkg/apiserver/storage"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func makeEvent(typ storage.EventType, rv string) storage.ResourceEvent {
	obj := &unstructured.Unstructured{}
	obj.SetName("obj-" + rv)
	obj.SetNamespace("default")
	return storage.ResourceEvent{
		Type:            typ,
		Object:          obj,
		ResourceVersion: rv,
	}
}

func TestWatchBuffer_AddAndGetAll(t *testing.T) {
	t.Run("returns all buffered events", func(t *testing.T) {
		buf := NewWatchBuffer(10)
		buf.Add(makeEvent(storage.EventAdded, "1"))
		buf.Add(makeEvent(storage.EventModified, "2"))
		buf.Add(makeEvent(storage.EventDeleted, "3"))

		events, err := buf.GetEventsSince("")
		if err != nil {
			t.Fatalf("GetEventsSince(\"\") failed: %v", err)
		}
		if len(events) != 3 {
			t.Fatalf("expected 3 events, got %d", len(events))
		}
		if events[0].ResourceVersion != "1" {
			t.Errorf("expected first event RV=1, got %s", events[0].ResourceVersion)
		}
		if events[2].ResourceVersion != "3" {
			t.Errorf("expected last event RV=3, got %s", events[2].ResourceVersion)
		}
	})
}

func TestWatchBuffer_GetEventsSince_EmptyRV(t *testing.T) {
	buf := NewWatchBuffer(10)
	buf.Add(makeEvent(storage.EventAdded, "1"))
	buf.Add(makeEvent(storage.EventAdded, "2"))

	events, err := buf.GetEventsSince("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 events for empty RV, got %d", len(events))
	}
}

func TestWatchBuffer_GetEventsSince_ZeroRV(t *testing.T) {
	buf := NewWatchBuffer(10)
	buf.Add(makeEvent(storage.EventAdded, "1"))
	buf.Add(makeEvent(storage.EventAdded, "2"))

	events, err := buf.GetEventsSince("0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 events for RV=0, got %d", len(events))
	}
}

func TestWatchBuffer_GetEventsSince_FiltersByRV(t *testing.T) {
	buf := NewWatchBuffer(10)
	buf.Add(makeEvent(storage.EventAdded, "1"))
	buf.Add(makeEvent(storage.EventModified, "2"))
	buf.Add(makeEvent(storage.EventDeleted, "3"))
	buf.Add(makeEvent(storage.EventAdded, "4"))

	tests := []struct {
		name     string
		sinceRV  string
		wantLen  int
		wantFirst string
	}{
		{
			name:      "since RV 2 returns events 3 and 4",
			sinceRV:   "2",
			wantLen:   2,
			wantFirst: "3",
		},
		{
			name:    "since RV 4 returns nothing",
			sinceRV: "4",
			wantLen: 0,
		},
		{
			name:      "since RV 1 returns events 2, 3, 4",
			sinceRV:   "1",
			wantLen:   3,
			wantFirst: "2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := buf.GetEventsSince(tt.sinceRV)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(events) != tt.wantLen {
				t.Fatalf("expected %d events, got %d", tt.wantLen, len(events))
			}
			if tt.wantLen > 0 && events[0].ResourceVersion != tt.wantFirst {
				t.Errorf("expected first event RV=%s, got %s", tt.wantFirst, events[0].ResourceVersion)
			}
		})
	}
}

func TestWatchBuffer_GetEventsSince_InvalidRV(t *testing.T) {
	buf := NewWatchBuffer(10)
	buf.Add(makeEvent(storage.EventAdded, "1"))

	_, err := buf.GetEventsSince("not-a-number")
	if err == nil {
		t.Fatal("expected error for invalid resource version, got nil")
	}
}

func TestWatchBuffer_Wraparound(t *testing.T) {
	buf := NewWatchBuffer(3)

	// Add 5 events to a buffer of size 3; oldest 2 should be overwritten
	for i := 1; i <= 5; i++ {
		buf.Add(makeEvent(storage.EventAdded, fmt.Sprintf("%d", i)))
	}

	events, err := buf.GetEventsSince("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events after wraparound, got %d", len(events))
	}

	// The remaining events should be 3, 4, 5
	rvs := make([]string, len(events))
	for i, e := range events {
		rvs[i] = e.ResourceVersion
	}
	for _, rv := range []string{"3", "4", "5"} {
		found := false
		for _, got := range rvs {
			if got == rv {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected RV %s in buffer after wraparound, got %v", rv, rvs)
		}
	}
}

// --- Watcher tests ---

func TestWatcher_SubscribeAndBroadcast(t *testing.T) {
	w := NewWatcher(10)
	defer w.Close()

	ch, stop, err := w.Subscribe("")
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	defer stop()

	event := makeEvent(storage.EventAdded, "1")
	w.Broadcast(event)

	select {
	case got := <-ch:
		if got.Type != storage.EventAdded {
			t.Errorf("expected ADDED event, got %s", got.Type)
		}
		if got.ResourceVersion != "1" {
			t.Errorf("expected RV=1, got %s", got.ResourceVersion)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for broadcast event")
	}
}

func TestWatcher_Count(t *testing.T) {
	w := NewWatcher(10)
	defer w.Close()

	if w.Count() != 0 {
		t.Fatalf("expected 0 subscribers initially, got %d", w.Count())
	}

	_, stop1, err := w.Subscribe("")
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	if w.Count() != 1 {
		t.Errorf("expected 1 subscriber, got %d", w.Count())
	}

	_, stop2, err := w.Subscribe("")
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	if w.Count() != 2 {
		t.Errorf("expected 2 subscribers, got %d", w.Count())
	}

	stop1()
	if w.Count() != 1 {
		t.Errorf("expected 1 subscriber after unsubscribe, got %d", w.Count())
	}

	stop2()
	if w.Count() != 0 {
		t.Errorf("expected 0 subscribers after all unsubscribed, got %d", w.Count())
	}
}

func TestWatcher_Unsubscribe_ClosesChannel(t *testing.T) {
	w := NewWatcher(10)
	defer w.Close()

	ch, stop, err := w.Subscribe("")
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	stop()

	// Channel should be closed after unsubscribe
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to be closed after unsubscribe")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for channel close")
	}
}

func TestWatcher_Close_ClosesAllChannels(t *testing.T) {
	w := NewWatcher(10)

	ch1, _, err := w.Subscribe("")
	if err != nil {
		t.Fatalf("Subscribe 1 failed: %v", err)
	}
	ch2, _, err := w.Subscribe("")
	if err != nil {
		t.Fatalf("Subscribe 2 failed: %v", err)
	}

	w.Close()

	// Both channels should be closed
	select {
	case _, ok := <-ch1:
		if ok {
			t.Error("expected ch1 to be closed after Close()")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ch1 close")
	}

	select {
	case _, ok := <-ch2:
		if ok {
			t.Error("expected ch2 to be closed after Close()")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ch2 close")
	}
}

func TestWatcher_SubscribeWithHistory(t *testing.T) {
	w := NewWatcher(10)
	defer w.Close()

	// Broadcast some events before subscribing
	w.Broadcast(makeEvent(storage.EventAdded, "1"))
	w.Broadcast(makeEvent(storage.EventModified, "2"))
	w.Broadcast(makeEvent(storage.EventDeleted, "3"))

	// Subscribe since RV "1" should replay events 2 and 3
	ch, stop, err := w.Subscribe("1")
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	defer stop()

	var received []storage.ResourceEvent
	for i := 0; i < 2; i++ {
		select {
		case e := <-ch:
			received = append(received, e)
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for historical event %d", i)
		}
	}

	if len(received) != 2 {
		t.Fatalf("expected 2 historical events, got %d", len(received))
	}
	if received[0].ResourceVersion != "2" {
		t.Errorf("expected first replayed event RV=2, got %s", received[0].ResourceVersion)
	}
	if received[1].ResourceVersion != "3" {
		t.Errorf("expected second replayed event RV=3, got %s", received[1].ResourceVersion)
	}
}

func TestWatcher_SubscribeOnClosedWatcher(t *testing.T) {
	w := NewWatcher(10)
	w.Close()

	ch, stop, err := w.Subscribe("")
	if err == nil {
		t.Fatal("expected error subscribing to closed watcher, got nil")
	}
	if ch != nil {
		t.Error("expected nil channel for closed watcher")
	}
	if stop != nil {
		t.Error("expected nil stop func for closed watcher")
	}
}

func TestWatcher_MultipleSubscribersReceiveSameEvent(t *testing.T) {
	w := NewWatcher(10)
	defer w.Close()

	ch1, stop1, err := w.Subscribe("")
	if err != nil {
		t.Fatalf("Subscribe 1 failed: %v", err)
	}
	defer stop1()

	ch2, stop2, err := w.Subscribe("")
	if err != nil {
		t.Fatalf("Subscribe 2 failed: %v", err)
	}
	defer stop2()

	event := makeEvent(storage.EventAdded, "42")
	w.Broadcast(event)

	for i, ch := range []<-chan storage.ResourceEvent{ch1, ch2} {
		select {
		case got := <-ch:
			if got.ResourceVersion != "42" {
				t.Errorf("subscriber %d: expected RV=42, got %s", i+1, got.ResourceVersion)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: timed out waiting for event", i+1)
		}
	}
}
