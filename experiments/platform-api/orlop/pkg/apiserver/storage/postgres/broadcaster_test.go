package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/thetechnick/orlop/pkg/apiserver/storage"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func setupTestBroadcaster(t *testing.T) (*PostgresBroadcaster, func()) {
	connString := os.Getenv("POSTGRES_TEST_URL")
	if connString == "" {
		connString = "postgres://localhost/orlop_test?sslmode=disable"
	}

	db, cleanup := setupTestDB(t)
	if db == nil {
		return nil, func() {}
	}

	// Create test scheme
	scheme := runtime.NewScheme()
	gvk := schema.GroupVersionKind{
		Group:   "test.example.com",
		Version: "v1",
		Kind:    "TestObject",
	}

	broadcaster, err := NewPostgresBroadcaster(context.Background(), PostgresBroadcasterConfig{
		DB:          db,
		ConnString:  connString,
		ChannelName: "test_events",
		TableName:   "event_log",
		Scheme:      scheme,
		GVK:         gvk,
	})
	if err != nil {
		cleanup()
		t.Fatalf("Failed to create broadcaster: %v", err)
	}

	broadcastCleanup := func() {
		broadcaster.Close()
		cleanup()
	}

	return broadcaster, broadcastCleanup
}

func TestPostgresBroadcaster_Broadcast(t *testing.T) {
	t.Run("broadcasts event to subscribers", func(t *testing.T) {
		broadcaster, cleanup := setupTestBroadcaster(t)
		defer cleanup()

		// Subscribe
		eventCh, stop, err := broadcaster.Subscribe("0")
		if err != nil {
			t.Fatalf("Subscribe() failed: %v", err)
		}
		defer stop()

		// Broadcast event
		obj := newTestObject(withName("test"), withNamespace("default"))
		event := storage.ResourceEvent{
			Type:            storage.EventAdded,
			Object:          obj,
			ResourceVersion: "1",
		}

		broadcaster.Broadcast(event)

		// Wait for event
		select {
		case received := <-eventCh:
			if received.Type != storage.EventAdded {
				t.Errorf("Expected ADDED event, got %s", received.Type)
			}
			if received.ResourceVersion != "1" {
				t.Errorf("Expected resourceVersion 1, got %s", received.ResourceVersion)
			}
		case <-time.After(2 * time.Second):
			t.Error("Event not received")
		}
	})

	t.Run("stores event in database", func(t *testing.T) {
		broadcaster, cleanup := setupTestBroadcaster(t)
		defer cleanup()

		obj := newTestObject(withName("test"), withNamespace("default"))
		event := storage.ResourceEvent{
			Type:            storage.EventModified,
			Object:          obj,
			ResourceVersion: "5",
		}

		broadcaster.Broadcast(event)

		// Give it time to write to database
		time.Sleep(100 * time.Millisecond)

		// Query database
		var count int
		err := broadcaster.db.QueryRow(`
			SELECT COUNT(*) FROM event_log
			WHERE event_type = 'MODIFIED' AND resource_version = 5
		`).Scan(&count)

		if err != nil {
			t.Fatalf("Failed to query event log: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 event in database, got %d", count)
		}
	})

	t.Run("broadcasts to multiple subscribers", func(t *testing.T) {
		broadcaster, cleanup := setupTestBroadcaster(t)
		defer cleanup()

		// Create 3 subscribers
		ch1, stop1, _ := broadcaster.Subscribe("0")
		defer stop1()
		ch2, stop2, _ := broadcaster.Subscribe("0")
		defer stop2()
		ch3, stop3, _ := broadcaster.Subscribe("0")
		defer stop3()

		// Broadcast event
		obj := newTestObject(withName("test"), withNamespace("default"))
		event := storage.ResourceEvent{
			Type:            storage.EventDeleted,
			Object:          obj,
			ResourceVersion: "10",
		}

		broadcaster.Broadcast(event)

		// All subscribers should receive it
		timeout := time.After(2 * time.Second)
		received := 0

		for i := 0; i < 3; i++ {
			select {
			case <-ch1:
				received++
			case <-ch2:
				received++
			case <-ch3:
				received++
			case <-timeout:
				t.Fatalf("Only %d/3 subscribers received event", received)
			}
		}

		if received != 3 {
			t.Errorf("Expected 3 subscribers to receive event, got %d", received)
		}
	})
}

func TestPostgresBroadcaster_Subscribe(t *testing.T) {
	t.Run("subscribes and receives events", func(t *testing.T) {
		broadcaster, cleanup := setupTestBroadcaster(t)
		defer cleanup()

		eventCh, stop, err := broadcaster.Subscribe("0")
		if err != nil {
			t.Fatalf("Subscribe() failed: %v", err)
		}
		defer stop()

		if eventCh == nil {
			t.Error("Event channel is nil")
		}
	})

	t.Run("stop function closes channel", func(t *testing.T) {
		broadcaster, cleanup := setupTestBroadcaster(t)
		defer cleanup()

		eventCh, stop, _ := broadcaster.Subscribe("0")

		stop()

		// Channel should be closed
		select {
		case _, ok := <-eventCh:
			if ok {
				t.Error("Expected channel to be closed")
			}
		case <-time.After(time.Second):
			t.Error("Channel not closed after stop()")
		}
	})

	t.Run("returns error when broadcaster is closed", func(t *testing.T) {
		broadcaster, cleanup := setupTestBroadcaster(t)
		defer cleanup()

		broadcaster.Close()

		_, _, err := broadcaster.Subscribe("0")
		if err == nil {
			t.Error("Expected error when subscribing to closed broadcaster")
		}
	})
}

func TestPostgresBroadcaster_HistoricalEvents(t *testing.T) {
	t.Run("sends historical events on subscribe", func(t *testing.T) {
		broadcaster, cleanup := setupTestBroadcaster(t)
		defer cleanup()

		// Broadcast some events first
		for i := 1; i <= 3; i++ {
			obj := newTestObject(withName("test"), withNamespace("default"))
			event := storage.ResourceEvent{
				Type:            storage.EventAdded,
				Object:          obj,
				ResourceVersion: string(rune('0' + i)),
			}
			broadcaster.Broadcast(event)
		}

		// Give time for database writes
		time.Sleep(200 * time.Millisecond)

		// Subscribe with resourceVersion "0" to get all historical events
		eventCh, stop, err := broadcaster.Subscribe("0")
		if err != nil {
			t.Fatalf("Subscribe() failed: %v", err)
		}
		defer stop()

		// Should receive historical events
		// Note: This is a simplified test - actual implementation needs proper object reconstruction
		received := 0
		timeout := time.After(2 * time.Second)

		for received < 3 {
			select {
			case event := <-eventCh:
				if event.Type == storage.EventAdded {
					received++
				}
			case <-timeout:
				t.Fatalf("Only received %d/3 historical events", received)
			}
		}
	})
}

func TestPostgresBroadcaster_Close(t *testing.T) {
	t.Run("closes broadcaster and all subscriptions", func(t *testing.T) {
		broadcaster, cleanup := setupTestBroadcaster(t)
		defer cleanup()

		// Create subscriptions
		ch1, _, _ := broadcaster.Subscribe("0")
		ch2, _, _ := broadcaster.Subscribe("0")

		// Close broadcaster
		err := broadcaster.Close()
		if err != nil {
			t.Errorf("Close() failed: %v", err)
		}

		// All channels should be closed
		select {
		case _, ok := <-ch1:
			if ok {
				t.Error("Channel 1 not closed")
			}
		case <-time.After(time.Second):
			t.Error("Channel 1 not closed in time")
		}

		select {
		case _, ok := <-ch2:
			if ok {
				t.Error("Channel 2 not closed")
			}
		case <-time.After(time.Second):
			t.Error("Channel 2 not closed in time")
		}
	})

	t.Run("close is idempotent", func(t *testing.T) {
		broadcaster, cleanup := setupTestBroadcaster(t)
		defer cleanup()

		err1 := broadcaster.Close()
		err2 := broadcaster.Close()

		if err1 != nil {
			t.Errorf("First Close() failed: %v", err1)
		}
		if err2 != nil {
			t.Errorf("Second Close() failed: %v", err2)
		}
	})
}

func TestPostgresBroadcaster_PruneOldEvents(t *testing.T) {
	t.Run("removes old events", func(t *testing.T) {
		broadcaster, cleanup := setupTestBroadcaster(t)
		defer cleanup()

		// Insert old event directly
		ctx := context.Background()
		_, err := broadcaster.db.ExecContext(ctx, `
			INSERT INTO event_log (event_type, resource_version, object_data, created_at)
			VALUES ('ADDED', 1, '{}', NOW() - INTERVAL '8 days')
		`)
		if err != nil {
			t.Fatalf("Failed to insert old event: %v", err)
		}

		// Insert recent event
		broadcaster.Broadcast(storage.ResourceEvent{
			Type:            storage.EventAdded,
			Object:          newTestObject(withName("recent"), withNamespace("default")),
			ResourceVersion: "2",
		})

		time.Sleep(100 * time.Millisecond)

		// Prune events older than 7 days
		err = broadcaster.PruneOldEvents(ctx, 7*24*time.Hour)
		if err != nil {
			t.Fatalf("PruneOldEvents() failed: %v", err)
		}

		// Query remaining events
		var count int
		err = broadcaster.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM event_log").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to count events: %v", err)
		}

		if count != 1 {
			t.Errorf("Expected 1 event after pruning, got %d", count)
		}
	})
}

func TestPostgresBroadcaster_EventLogSchema(t *testing.T) {
	t.Run("creates event log table with indexes", func(t *testing.T) {
		broadcaster, cleanup := setupTestBroadcaster(t)
		defer cleanup()

		// Verify table exists
		var exists bool
		err := broadcaster.db.QueryRow(`
			SELECT EXISTS (
				SELECT FROM information_schema.tables
				WHERE table_name = 'event_log'
			)
		`).Scan(&exists)
		if err != nil || !exists {
			t.Error("Event log table was not created")
		}

		// Verify indexes exist
		var indexCount int
		err = broadcaster.db.QueryRow(`
			SELECT COUNT(*) FROM pg_indexes
			WHERE tablename = 'event_log'
		`).Scan(&indexCount)
		if err != nil || indexCount < 2 {
			t.Errorf("Expected at least 2 indexes on event_log, got %d", indexCount)
		}
	})
}

func TestPostgresBroadcaster_MultiInstance(t *testing.T) {
	t.Run("events propagate between instances", func(t *testing.T) {
		connString := os.Getenv("POSTGRES_TEST_URL")
		if connString == "" {
			connString = "postgres://localhost/orlop_test?sslmode=disable"
		}

		db, cleanup := setupTestDB(t)
		if db == nil {
			return
		}
		defer cleanup()

		// Create test scheme
		scheme := runtime.NewScheme()
		gvk := schema.GroupVersionKind{
			Group:   "test.example.com",
			Version: "v1",
			Kind:    "TestObject",
		}

		// Create two broadcaster instances
		ctx := context.Background()
		b1, err := NewPostgresBroadcaster(ctx, PostgresBroadcasterConfig{
			DB:          db,
			ConnString:  connString,
			ChannelName: "multi_test",
			TableName:   "event_log_multi",
			Scheme:      scheme,
			GVK:         gvk,
		})
		if err != nil {
			t.Fatalf("Failed to create broadcaster 1: %v", err)
		}
		defer b1.Close()

		b2, err := NewPostgresBroadcaster(ctx, PostgresBroadcasterConfig{
			DB:          db,
			ConnString:  connString,
			ChannelName: "multi_test",
			TableName:   "event_log_multi",
			Scheme:      scheme,
			GVK:         gvk,
		})
		if err != nil {
			t.Fatalf("Failed to create broadcaster 2: %v", err)
		}
		defer b2.Close()

		// Subscribe on instance 2
		eventCh, stop, _ := b2.Subscribe("0")
		defer stop()

		// Broadcast from instance 1
		obj := newTestObject(withName("test"), withNamespace("default"))
		event := storage.ResourceEvent{
			Type:            storage.EventAdded,
			Object:          obj,
			ResourceVersion: "1",
		}

		b1.Broadcast(event)

		// Instance 2 should receive the event via NOTIFY
		select {
		case received := <-eventCh:
			if received.Type != storage.EventAdded {
				t.Errorf("Expected ADDED event, got %s", received.Type)
			}
		case <-time.After(3 * time.Second):
			t.Error("Event not propagated to second instance")
		}

		// Clean up
		db.Exec("DROP TABLE IF EXISTS event_log_multi CASCADE")
	})
}
