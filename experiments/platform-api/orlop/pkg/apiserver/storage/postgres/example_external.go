package postgres

import (
	"context"
	"fmt"
	"sync"

	"github.com/thetechnick/orlop/pkg/apiserver/storage"
)

// ExternalDBBroadcaster is an example implementation of storage.EventBroadcaster
// that uses an external database's change stream (like MongoDB change streams,
// PostgreSQL LISTEN/NOTIFY, or similar mechanisms).
//
// This is a reference implementation showing how to integrate with external
// event sources. Real implementations would connect to actual databases.
type ExternalDBBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[int]chan storage.ResourceEvent
	nextID      int
	closed      bool
	ctx         context.Context
	cancel      context.CancelFunc

	// External database connection (example)
	// In real implementation, this would be:
	// - *mongo.ChangeStream for MongoDB
	// - *pgx.Conn with LISTEN for PostgreSQL
	// - *nats.Subscription for NATS
	// - etc.
	dbConnection interface{}
}

// NewExternalDBBroadcaster creates a broadcaster connected to an external database.
// In a real implementation, this would accept database-specific connection parameters.
func NewExternalDBBroadcaster(dbConnection interface{}) *ExternalDBBroadcaster {
	ctx, cancel := context.WithCancel(context.Background())

	b := &ExternalDBBroadcaster{
		subscribers:  make(map[int]chan storage.ResourceEvent),
		ctx:          ctx,
		cancel:       cancel,
		dbConnection: dbConnection,
	}

	// Start listening to database change stream in background
	go b.listenToDatabase()

	return b
}

// listenToDatabase monitors the external database for changes.
// This is an example - real implementation would use database-specific APIs.
func (b *ExternalDBBroadcaster) listenToDatabase() {
	// Example pseudo-code for different databases:
	//
	// MongoDB:
	//   changeStream := collection.Watch(ctx, pipeline)
	//   for changeStream.Next(ctx) {
	//       var event bson.M
	//       changeStream.Decode(&event)
	//       b.Broadcast(convertToResourceEvent(event))
	//   }
	//
	// PostgreSQL:
	//   conn.Exec("LISTEN resource_changes")
	//   for notification := range conn.WaitForNotification(ctx) {
	//       event := parseNotification(notification)
	//       b.Broadcast(event)
	//   }
	//
	// NATS:
	//   sub, _ := nc.Subscribe("resource.changes", func(msg *nats.Msg) {
	//       event := parseMessage(msg)
	//       b.Broadcast(event)
	//   })

	<-b.ctx.Done()
}

// Broadcast sends an event to all active subscribers.
// This method is typically called by the database listener goroutine.
func (b *ExternalDBBroadcaster) Broadcast(event storage.ResourceEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return
	}

	// Send to all subscribers
	for _, ch := range b.subscribers {
		select {
		case ch <- event:
		default:
			// Don't block if subscriber is slow
			// In production, might want to disconnect slow subscribers
		}
	}
}

// Subscribe creates a new watch starting from the given resource version.
// For external databases, this might query historical events first.
func (b *ExternalDBBroadcaster) Subscribe(sinceResourceVersion string) (<-chan storage.ResourceEvent, func(), error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, nil, fmt.Errorf("broadcaster is closed")
	}

	id := b.nextID
	b.nextID++

	ch := make(chan storage.ResourceEvent, 100)
	b.subscribers[id] = ch

	// In a real implementation, query database for historical events
	// since the requested resource version:
	//
	// MongoDB:
	//   resumeToken := convertResourceVersionToResumeToken(sinceResourceVersion)
	//   changeStream := collection.Watch(ctx, pipeline, options.ChangeStream().SetResumeAfter(resumeToken))
	//
	// PostgreSQL:
	//   rows := conn.Query("SELECT * FROM event_log WHERE id > $1 ORDER BY id", sinceResourceVersion)
	//   for rows.Next() {
	//       event := scanEvent(rows)
	//       ch <- event
	//   }

	// Create stop function
	stopFunc := func() {
		b.unsubscribe(id)
	}

	return ch, stopFunc, nil
}

// unsubscribe removes a subscriber.
func (b *ExternalDBBroadcaster) unsubscribe(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if ch, exists := b.subscribers[id]; exists {
		close(ch)
		delete(b.subscribers, id)
	}
}

// Close shuts down the broadcaster and all active subscriptions.
func (b *ExternalDBBroadcaster) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil
	}

	b.closed = true
	b.cancel() // Stop database listener

	// Close all subscriber channels
	for id, ch := range b.subscribers {
		close(ch)
		delete(b.subscribers, id)
	}

	// In real implementation, close database connection:
	// - changeStream.Close() for MongoDB
	// - conn.Close() for PostgreSQL
	// - sub.Unsubscribe() for NATS

	return nil
}

// Verify that ExternalDBBroadcaster implements storage.EventBroadcaster.
var _ storage.EventBroadcaster = (*ExternalDBBroadcaster)(nil)

// Example factory functions for different database backends:

// NewMongoDBBroadcaster creates a broadcaster using MongoDB change streams.
// func NewMongoDBBroadcaster(client *mongo.Client, database, collection string) storage.EventBroadcaster {
//     // Real implementation would:
//     // 1. Get collection: coll := client.Database(database).Collection(collection)
//     // 2. Create change stream: stream := coll.Watch(ctx, pipeline)
//     // 3. Listen for changes and call Broadcast()
//     return NewExternalDBBroadcaster(client)
// }

// NewPostgrePostgresBroadcaster creates a broadcaster using PostgreSQL LISTEN/NOTIFY.
// func NewPostgrePostgresBroadcaster(connString string) storage.EventBroadcaster {
//     // Real implementation would:
//     // 1. Connect: conn := pgx.Connect(ctx, connString)
//     // 2. Execute: conn.Exec("LISTEN resource_changes")
//     // 3. Wait for notifications and call Broadcast()
//     return NewExternalDBBroadcaster(conn)
// }

// NewNATSBroadcaster creates a broadcaster using NATS messaging.
// func NewNATSBroadcaster(url, subject string) storage.EventBroadcaster {
//     // Real implementation would:
//     // 1. Connect: nc := nats.Connect(url)
//     // 2. Subscribe: nc.Subscribe(subject, handler)
//     // 3. Handler calls Broadcast() for each message
//     return NewExternalDBBroadcaster(nc)
// }
