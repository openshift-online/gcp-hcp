# Watch Event Broadcasting Architecture

## Overview

The watch event system has been refactored to use a pluggable interface pattern, allowing different event broadcasting implementations to be used based on deployment requirements.

## Core Interfaces

### EventBroadcaster

The main interface for event distribution:

```go
type EventBroadcaster interface {
    // Broadcast sends an event to all active watchers
    Broadcast(event WatchEvent)
    
    // Subscribe creates a new watch starting from a resource version
    // Returns: event channel, stop function, error
    Subscribe(sinceResourceVersion string) (<-chan WatchEvent, func(), error)
    
    // Close shuts down the broadcaster
    Close() error
}
```

### WatchFilter

Filters events based on client options:

```go
type WatchFilter interface {
    Matches(event WatchEvent, opts client.ListOptions) bool
}
```

## Implementations

### 1. In-Memory Broadcaster (Default)

**File:** `watch.go`

The `Watcher` type implements `EventBroadcaster` using in-memory channels and a circular buffer.

**Features:**
- Ring buffer stores last N events for catch-up
- Non-blocking broadcast to prevent slow consumers from blocking
- Automatic cleanup of closed subscriptions

**Use case:** Single-instance deployments, testing, development

**Example:**
```go
store := NewMemoryStore("objects", scheme, gvk)
// Uses in-memory Watcher by default
```

### 2. External Database Broadcaster

**File:** `watch_external_example.go`

Reference implementation showing how to integrate with external event sources.

**Supported backends** (examples):
- **MongoDB Change Streams**: Real-time notifications from MongoDB collections
- **PostgreSQL LISTEN/NOTIFY**: Built-in pub/sub for PostgreSQL
- **NATS**: Distributed messaging system
- **Redis Pub/Sub**: Event distribution via Redis
- **Kafka**: Event streaming platform

**Use case:** Multi-instance deployments where events need to be shared across instances

**Example:**
```go
// MongoDB backend
broadcaster := NewMongoDBBroadcaster(mongoClient, "mydb", "events")
store := NewMemoryStore("objects", scheme, gvk,
    WithBroadcaster(broadcaster))

// PostgreSQL backend
broadcaster := NewPostgreSQLBroadcaster("postgres://...")
store := NewMemoryStore("objects", scheme, gvk,
    WithBroadcaster(broadcaster))
```

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────┐
│                    API Clients                          │
│            (watching for resource changes)              │
└──────────────┬──────────────────────────────────────────┘
               │
               ▼
      ┌────────────────┐
      │ Watch Endpoint │
      │   (HTTP/SSE)   │
      └────────┬───────┘
               │
               ▼
       ┌───────────────┐
       │ MemoryStore   │
       │   .Watch()    │
       └───────┬───────┘
               │
               ▼
    ┌──────────────────────┐
    │ EventBroadcaster     │  ◄── Pluggable Interface
    │   (interface)        │
    └──────────┬───────────┘
               │
       ┌───────┴────────┐
       │                │
       ▼                ▼
┌─────────────┐  ┌──────────────────┐
│   Watcher   │  │ ExternalDB       │
│ (in-memory) │  │   Broadcaster    │
└──────┬──────┘  └────────┬─────────┘
       │                  │
       │                  ▼
       │         ┌─────────────────┐
       │         │  External DB    │
       │         │ (MongoDB/PG/    │
       │         │  NATS/Redis)    │
       │         └─────────────────┘
       │
       ▼
┌──────────────┐
│ Ring Buffer  │
│  (last N     │
│   events)    │
└──────────────┘
```

## Configuration Options

### Option 1: Default (In-Memory)

```go
store := NewMemoryStore(resourceType, scheme, gvk)
```

### Option 2: Custom Broadcaster Instance

```go
broadcaster := NewExternalDBBroadcaster(dbConnection)
store := NewMemoryStore(resourceType, scheme, gvk,
    WithBroadcaster(broadcaster))
```

### Option 3: Broadcaster Factory

```go
factory := func() EventBroadcaster {
    return NewMongoDBBroadcaster(client, "db", "collection")
}

store := NewMemoryStore(resourceType, scheme, gvk,
    WithBroadcasterFactory(factory))
```

## Event Flow

### 1. Resource Modification

```go
// Client creates an object
store.Create(obj)
  │
  ├─ Increment resourceVersion
  ├─ Store object in memory map
  └─ Broadcast event
       │
       └─> broadcaster.Broadcast(WatchEvent{
               Type: "ADDED",
               Object: obj,
               ResourceVersion: "123",
           })
```

### 2. Event Distribution

**In-Memory Broadcaster:**
```
Broadcast(event)
  │
  ├─ Add to ring buffer (for catch-up)
  └─ Send to all active subscribers
       │
       └─> For each subscriber channel:
             select {
               case ch <- event:  // Send
               default:           // Skip if full
             }
```

**External DB Broadcaster:**
```
Broadcast(event)
  │
  ├─ Write to database event log
  │   (MongoDB: insert into change stream)
  │   (PostgreSQL: INSERT + NOTIFY)
  │   (NATS: publish to subject)
  └─ Database propagates to all instances
```

### 3. Client Watch Subscription

```
client.Watch(opts, resourceVersion)
  │
  ├─ store.Watch(opts, resourceVersion)
  │   │
  │   ├─ broadcaster.Subscribe(resourceVersion)
  │   │   │
  │   │   ├─ Create event channel
  │   │   ├─ Send historical events (if available)
  │   │   └─ Return channel + stop function
  │   │
  │   └─ Start filtering goroutine
  │       │
  │       └─ For each event from broadcaster:
  │           ├─ Filter by namespace
  │           ├─ Filter by label selector
  │           └─ Send to client if matches
  │
  └─ Stream events to HTTP response
```

## Implementing a Custom Broadcaster

### Step 1: Implement EventBroadcaster Interface

```go
type MyCustomBroadcaster struct {
    // Your fields here
}

func (b *MyCustomBroadcaster) Broadcast(event WatchEvent) {
    // Send event to your backend
}

func (b *MyCustomBroadcaster) Subscribe(sinceResourceVersion string) (<-chan WatchEvent, func(), error) {
    ch := make(chan WatchEvent, 100)
    
    // Start listening for events
    // Send historical events if requested
    
    stopFunc := func() {
        // Cleanup
    }
    
    return ch, stopFunc, nil
}

func (b *MyCustomBroadcaster) Close() error {
    // Cleanup all resources
    return nil
}
```

### Step 2: Use in MemoryStore

```go
broadcaster := &MyCustomBroadcaster{...}
store := NewMemoryStore(resourceType, scheme, gvk,
    WithBroadcaster(broadcaster))
```

## Example: MongoDB Change Streams

```go
func NewMongoDBBroadcaster(
    client *mongo.Client,
    database, collection string,
) EventBroadcaster {
    ctx := context.Background()
    coll := client.Database(database).Collection(collection)
    
    b := &MongoDBBroadcaster{
        collection: coll,
        subscribers: make(map[int]chan WatchEvent),
    }
    
    // Start watching collection changes
    go func() {
        stream, _ := coll.Watch(ctx, mongo.Pipeline{})
        defer stream.Close(ctx)
        
        for stream.Next(ctx) {
            var changeEvent bson.M
            stream.Decode(&changeEvent)
            
            event := convertChangeEventToWatchEvent(changeEvent)
            b.Broadcast(event)
        }
    }()
    
    return b
}

func (b *MongoDBBroadcaster) Broadcast(event WatchEvent) {
    // Also write to database for historical replay
    b.collection.InsertOne(context.Background(), event)
    
    // Broadcast to active subscribers
    for _, ch := range b.subscribers {
        select {
        case ch <- event:
        default:
        }
    }
}
```

## Example: PostgreSQL LISTEN/NOTIFY

```go
func NewPostgreSQLBroadcaster(connString string) EventBroadcaster {
    conn, _ := pgx.Connect(context.Background(), connString)
    
    b := &PostgreSQLBroadcaster{
        conn: conn,
        subscribers: make(map[int]chan WatchEvent),
    }
    
    // Execute LISTEN
    conn.Exec(context.Background(), "LISTEN resource_changes")
    
    // Start listening for notifications
    go func() {
        for {
            notification, _ := conn.WaitForNotification(context.Background())
            event := parseNotification(notification)
            b.Broadcast(event)
        }
    }()
    
    return b
}

func (b *PostgreSQLBroadcaster) Broadcast(event WatchEvent) {
    // Insert into event log
    b.conn.Exec(context.Background(),
        "INSERT INTO events (type, object, resource_version) VALUES ($1, $2, $3)",
        event.Type, event.Object, event.ResourceVersion)
    
    // Broadcast to active subscribers in this instance
    for _, ch := range b.subscribers {
        select {
        case ch <- event:
        default:
        }
    }
}
```

## Benefits

### 1. Flexibility
- Switch between in-memory and external broadcasters without code changes
- Test with in-memory, deploy with external database

### 2. Scalability
- External broadcasters enable multi-instance deployments
- Events distributed across all API server instances

### 3. Reliability
- External databases can persist events for replay
- Survive API server restarts without losing events

### 4. Consistency
- All instances see the same event stream
- No need for client-side load balancing

## Migration Guide

### From Old Code

```go
// Old: hardcoded in-memory watcher
store := NewMemoryStore(resourceType, scheme, gvk)
```

### To New Code (In-Memory)

```go
// New: explicit in-memory (same behavior)
store := NewMemoryStore(resourceType, scheme, gvk)
// OR with explicit option
store := NewMemoryStore(resourceType, scheme, gvk,
    WithBroadcaster(NewWatcher(50)))
```

### To New Code (External DB)

```go
// New: use external database
broadcaster := NewMongoDBBroadcaster(client, "db", "events")
store := NewMemoryStore(resourceType, scheme, gvk,
    WithBroadcaster(broadcaster))
```

## Testing

### Unit Tests

```go
func TestWithCustomBroadcaster(t *testing.T) {
    mockBroadcaster := &MockBroadcaster{}
    store := NewMemoryStore("test", scheme, gvk,
        WithBroadcaster(mockBroadcaster))
    
    store.Create(obj)
    
    // Verify Broadcast was called
    if !mockBroadcaster.WasCalled("Broadcast") {
        t.Error("Broadcast not called")
    }
}
```

### Integration Tests

Existing integration tests continue to work with the default in-memory broadcaster.

## Performance Considerations

### In-Memory Broadcaster
- **Latency**: ~microseconds
- **Throughput**: High (limited by channel operations)
- **Scalability**: Single instance only

### External Database Broadcaster
- **Latency**: ~milliseconds (network + database)
- **Throughput**: Depends on database
- **Scalability**: Multiple instances

Choose based on deployment needs:
- **Development/Testing**: In-memory
- **Single Instance**: In-memory
- **Multi-Instance**: External database
- **Event Persistence**: External database
