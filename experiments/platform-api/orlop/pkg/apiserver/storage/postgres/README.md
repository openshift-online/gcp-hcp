# SQL Storage Backend

Complete PostgreSQL/SQL storage implementation for multi-instance deployments.

## Overview

The SQL storage backend provides:
- **Persistent storage** in PostgreSQL database
- **Multi-instance support** via shared database
- **Event broadcasting** using PostgreSQL LISTEN/NOTIFY
- **Historical event replay** from event log table
- **High availability** and horizontal scaling

## Components

### 1. SQLStore (`sql_storage.go`)

Implements `ResourceStore` interface using SQL database.

**Features:**
- Resources stored as JSONB for flexibility
- Metadata columns (namespace, name, labels) for efficient querying
- Automatic schema creation and migration
- GIN index on labels for fast label selector queries
- Resource version tracking

**Schema:**
```sql
CREATE TABLE resources_{type} (
    id SERIAL PRIMARY KEY,
    namespace VARCHAR(253) NOT NULL,
    name VARCHAR(253) NOT NULL,
    resource_version BIGINT NOT NULL,
    labels JSONB,
    data JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(namespace, name)
);
```

### 2. SQLBroadcaster (`sql_broadcaster.go`)

Implements `EventBroadcaster` interface using PostgreSQL.

**Features:**
- LISTEN/NOTIFY for real-time event distribution
- Event log table for historical replay
- Automatic reconnection on connection loss
- Event pruning to prevent unbounded growth
- Cross-instance event propagation

**Schema:**
```sql
CREATE TABLE event_log (
    id BIGSERIAL PRIMARY KEY,
    event_type VARCHAR(20) NOT NULL,
    resource_version BIGINT NOT NULL,
    object_data JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

## Usage

### Recommended: Use Storage Factory (API Server Integration)

The easiest way to use PostgreSQL storage is via the storage factory pattern:

```go
import (
    "context"
    "database/sql"
    
    _ "github.com/lib/pq"
    "github.com/thetechnick/orlop/pkg/apiserver"
    "github.com/thetechnick/orlop/pkg/apiserver/storage/postgres"
)

func main() {
    // Connect to PostgreSQL
    db, err := sql.Open("postgres", 
        "postgres://user:pass@localhost/orlop?sslmode=disable")
    if err != nil {
        panic(err)
    }
    defer db.Close()

    // Create PostgreSQL storage factory
    storageFactory := postgres.NewStorageFactory(postgres.StorageFactoryConfig{
        DB:         db,
        ConnString: "postgres://user:pass@localhost/orlop?sslmode=disable",
        Context:    context.Background(),
    })

    // Create API server with PostgreSQL storage
    server, err := apiserver.New(apiserver.Options{
        Address:          "0.0.0.0",
        PrivatePort:      8080,
        PublicPort:       8081,
        EnablePublicAPI:  true,
        PrivateScheme:    privateScheme,
        PublicScheme:     publicScheme,
        PrivateResources: privateResources,
        PublicResources:  publicResources,
        StorageFactory:   storageFactory, // Use PostgreSQL storage
    })
    if err != nil {
        panic(err)
    }

    if err := server.Run(); err != nil {
        panic(err)
    }
}
```

The storage factory automatically creates:
- A PostgresStore for each resource type
- A PostgresBroadcaster for each resource type with LISTEN/NOTIFY
- Proper table names and channel names per resource

### Manual Setup (Advanced)

For manual control over store and broadcaster creation:

```go
import (
    "context"
    "database/sql"
    
    _ "github.com/lib/pq"
    "github.com/thetechnick/orlop/pkg/apiserver/storage/postgres"
)

func main() {
    // Connect to PostgreSQL
    db, err := sql.Open("postgres", 
        "postgres://user:pass@localhost/orlop?sslmode=disable")
    if err != nil {
        panic(err)
    }
    defer db.Close()

    ctx := context.Background()

    // Create PostgreSQL broadcaster
    broadcaster, err := postgres.NewPostgresBroadcaster(ctx, 
        postgres.PostgresBroadcasterConfig{
            DB:          db,
            ConnString:  "postgres://user:pass@localhost/orlop?sslmode=disable",
            ChannelName: "resource_events",
            TableName:   "event_log",
            Scheme:      scheme,
            GVK:         gvk,
        })
    if err != nil {
        panic(err)
    }
    defer broadcaster.Close()

    // Create SQL store
    store, err := storage.NewPostgresStore(ctx,
        storage.SQLStoreConfig{
            DB:           db,
            ResourceType: "objects",
            Scheme:       scheme,
            GVK:          gvk,
            Broadcaster:  broadcaster,
            TableName:    "resources_objects",
        })
    if err != nil {
        panic(err)
    }

    // Use store normally
    store.Create(obj)
    store.Get("default", "my-object")
    store.List(client.ListOptions{Namespace: "default"})
    
    // Watch for changes
    eventCh, stop, _ := store.Watch(
        client.ListOptions{Namespace: "default"}, 
        "0")
    defer stop()
    
    for event := range eventCh {
        fmt.Printf("Event: %s %s\n", event.Type, event.Object.GetName())
    }
}
```

### Multi-Instance Deployment

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  API Server 1   │     │  API Server 2   │     │  API Server 3   │
│                 │     │                 │     │                 │
│  SQLStore       │     │  SQLStore       │     │  SQLStore       │
│  SQLBroadcaster │     │  SQLBroadcaster │     │  SQLBroadcaster │
└────────┬────────┘     └────────┬────────┘     └────────┬────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                        ┌────────▼────────┐
                        │   PostgreSQL    │
                        │                 │
                        │ • resources_*   │
                        │ • event_log     │
                        │ • LISTEN/NOTIFY │
                        └─────────────────┘
```

**Each instance:**
1. Connects to shared PostgreSQL database
2. Listens on NOTIFY channel for events
3. Writes to shared tables
4. Broadcasts events via NOTIFY

**Client benefits:**
- Load balancing across instances
- High availability (instance failures don't lose data)
- Consistent view across all instances

### Event Flow

#### 1. Create Resource (Instance 1)

```
Client → API Server 1
  │
  ├─ SQLStore.Create(obj)
  │   │
  │   ├─ INSERT INTO resources_objects (...)
  │   └─ broadcaster.Broadcast(event)
  │       │
  │       ├─ INSERT INTO event_log (...)
  │       └─ NOTIFY resource_events, '{...}'
  │
  └─ Response 201 Created
```

#### 2. Event Propagation

```
PostgreSQL NOTIFY
  │
  ├──> Instance 1: Receives notification → broadcasts to local watchers
  ├──> Instance 2: Receives notification → broadcasts to local watchers  
  └──> Instance 3: Receives notification → broadcasts to local watchers
```

#### 3. Watch from Instance 2

```
Client → API Server 2
  │
  ├─ SQLStore.Watch(opts, "0")
  │   │
  │   ├─ broadcaster.Subscribe("0")
  │   │   │
  │   │   ├─ SELECT FROM event_log WHERE rv > 0  (historical)
  │   │   └─ LISTEN resource_events  (live)
  │   │
  │   └─ Stream events to client
  │
  └─ SSE/WebSocket stream
```

### Configuration Options

#### SQLStoreConfig

```go
type SQLStoreConfig struct {
    DB           *sql.DB              // Required: database connection
    ResourceType string                // Required: e.g., "objects"
    Scheme       *runtime.Scheme      // Required: for object deserialization
    GVK          schema.GroupVersionKind // Required: resource GVK
    Broadcaster  EventBroadcaster     // Optional: for watch support
    TableName    string                // Optional: defaults to "resources_{type}"
}
```

#### SQLBroadcasterConfig

```go
type SQLBroadcasterConfig struct {
    DB          *sql.DB   // Required: database connection
    ConnString  string    // Required: for pq.Listener (must be full URL)
    ChannelName string    // Optional: defaults to "resource_events"
    TableName   string    // Optional: defaults to "event_log"
}
```

## Advanced Features

### Event Pruning

Prevent unbounded growth of event log:

```go
// Run periodically (e.g., daily cron)
err := broadcaster.PruneOldEvents(ctx, 7 * 24 * time.Hour)
if err != nil {
    log.Printf("Failed to prune events: %v", err)
}
```

### Label Selector Queries

Efficient label-based filtering using GIN indexes:

```go
opts := client.ListOptions{
    Namespace: "production",
    LabelSelector: labels.SelectorFromSet(labels.Set{
        "app": "web",
        "env": "prod",
    }),
}

list, err := store.List(opts)
```

PostgreSQL query executed:
```sql
SELECT data FROM resources_objects
WHERE namespace = 'production'
  AND labels @> '{"app":"web","env":"prod"}'
ORDER BY resource_version ASC
```

### Connection Pooling

Configure for high-throughput:

```go
db, _ := sql.Open("postgres", connString)

// Configure pool
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(5 * time.Minute)
```

### High Availability Setup

#### Primary-Replica Configuration

```go
// Write to primary
writeDB, _ := sql.Open("postgres", "postgres://primary:5432/orlop")

// Read from replica (for List/Get)
readDB, _ := sql.Open("postgres", "postgres://replica:5432/orlop")

// Create separate stores
writeStore, _ := storage.NewPostgresStore(ctx, storage.SQLStoreConfig{
    DB: writeDB,
    // ...
})

readStore, _ := storage.NewPostgresStore(ctx, storage.SQLStoreConfig{
    DB: readDB,
    // ...
})

// Use write store for mutations
writeStore.Create(obj)
writeStore.Update(obj)
writeStore.Delete(ns, name)

// Use read store for queries (reduces primary load)
readStore.Get(ns, name)
readStore.List(opts)
```

## Performance Considerations

### Indexes

The default schema creates these indexes:
- `UNIQUE(namespace, name)` - Fast lookups
- `idx_namespace` - Namespace filtering
- `idx_resource_version` - Watch catch-up
- `idx_labels (GIN)` - Label selector queries

For high-scale deployments, consider:
```sql
-- Composite index for namespace + label queries
CREATE INDEX idx_namespace_labels 
ON resources_objects(namespace) 
INCLUDE (labels);

-- Partial index for specific namespaces
CREATE INDEX idx_production_namespace 
ON resources_objects(namespace) 
WHERE namespace = 'production';
```

### Query Optimization

**Problem:** Slow label selector queries  
**Solution:** Use GIN index with `@>` operator

**Problem:** Watch catch-up queries slow  
**Solution:** Limit historical events to reasonable window

```go
// In production, limit catch-up to last hour
const maxCatchupWindow = time.Hour

sinceTime := time.Now().Add(-maxCatchupWindow)
query := `
    SELECT * FROM event_log 
    WHERE resource_version > $1 
      AND created_at > $2
    ORDER BY resource_version ASC
    LIMIT 1000
`
```

### Scaling Guidelines

| Metric | Single Instance | Multi-Instance |
|--------|----------------|----------------|
| QPS | ~1,000 | ~10,000+ |
| Concurrent Watches | ~100 | ~1,000+ |
| Database | 1 primary | Primary + replicas |
| Latency | <10ms | <50ms |

**Bottlenecks:**
1. **Database connections** - Use connection pooling
2. **NOTIFY throughput** - PostgreSQL handles ~10k NOTIFY/sec
3. **Event log size** - Prune regularly
4. **Lock contention** - Use `SELECT FOR UPDATE SKIP LOCKED`

## Migration from In-Memory

### Phase 1: Add SQL Backend

```go
// Keep existing in-memory store
memStore := storage.NewMemoryStore(...)

// Add SQL store
sqlStore, _ := storage.NewPostgresStore(...)

// Dual-write during migration
func create(obj client.Object) error {
    if err := memStore.Create(obj); err != nil {
        return err
    }
    if err := sqlStore.Create(obj); err != nil {
        log.Printf("SQL write failed: %v", err)
    }
    return nil
}
```

### Phase 2: Backfill Data

```go
// Copy all objects to SQL
list, _ := memStore.List(client.ListOptions{})
for _, item := range list.Items {
    sqlStore.Create(&item)
}
```

### Phase 3: Switch Primary

```go
// Read from SQL, fallback to memory
func get(ns, name string) (client.Object, error) {
    obj, err := sqlStore.Get(ns, name)
    if err == nil {
        return obj, nil
    }
    return memStore.Get(ns, name)
}
```

### Phase 4: Remove In-Memory

```go
// All operations use SQL
store := sqlStore
```

## Troubleshooting

### LISTEN Connection Lost

**Symptom:** Watchers stop receiving events

**Solution:** pq.Listener auto-reconnects. Check logs:
```go
listener := pq.NewListener(connString, 
    10*time.Second,  // Retry every 10s
    time.Minute,     // Max retry 1 min
    func(ev pq.ListenerEventType, err error) {
        log.Printf("Listener: %v, error: %v", ev, err)
    })
```

### Event Log Growth

**Symptom:** `event_log` table size growing unbounded

**Solution:** Regular pruning
```bash
# Cron job
0 2 * * * psql -c "DELETE FROM event_log WHERE created_at < NOW() - INTERVAL '7 days'"
```

### Slow Watch Catch-Up

**Symptom:** New watch connections take seconds to start

**Solution:** Limit historical replay
```go
// In broadcaster.sendHistoricalEvents()
LIMIT 1000  // Cap at 1000 events
```

### Lock Contention

**Symptom:** High wait times on INSERT/UPDATE

**Solution:** Reduce transaction isolation level
```go
db.ExecContext(ctx, "SET SESSION TRANSACTION ISOLATION LEVEL READ COMMITTED")
```

## Testing

### Unit Tests

```go
func TestSQLStore(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()
    
    store, _ := storage.NewPostgresStore(context.Background(),
        storage.SQLStoreConfig{
            DB:           db,
            ResourceType: "test",
            Scheme:       scheme,
            GVK:          gvk,
        })
    
    // Test CRUD operations
    obj := newTestObject()
    if err := store.Create(obj); err != nil {
        t.Fatal(err)
    }
    
    retrieved, _ := store.Get("default", "test")
    if retrieved.GetName() != "test" {
        t.Error("Object not found")
    }
}

func setupTestDB(t *testing.T) *sql.DB {
    db, err := sql.Open("postgres", 
        "postgres://localhost/orlop_test?sslmode=disable")
    if err != nil {
        t.Skip("PostgreSQL not available")
    }
    
    // Clean up after test
    t.Cleanup(func() {
        db.Exec("DROP TABLE IF EXISTS resources_test CASCADE")
        db.Exec("DROP TABLE IF EXISTS event_log CASCADE")
    })
    
    return db
}
```

### Integration Tests

```go
func TestMultiInstanceEvents(t *testing.T) {
    db := setupTestDB(t)
    
    // Create two instances
    store1, _ := storage.NewPostgresStore(ctx, config)
    store2, _ := storage.NewPostgresStore(ctx, config)
    
    // Watch from instance 2
    events, stop, _ := store2.Watch(client.ListOptions{}, "0")
    defer stop()
    
    // Create from instance 1
    store1.Create(obj)
    
    // Verify event received on instance 2
    select {
    case event := <-events:
        if event.Type != "ADDED" {
            t.Error("Expected ADDED event")
        }
    case <-time.After(time.Second):
        t.Error("Event not received")
    }
}
```

## Production Checklist

- [ ] Connection pooling configured
- [ ] Regular event log pruning scheduled
- [ ] Indexes created for common queries
- [ ] Monitoring for LISTEN connection health
- [ ] Backup strategy in place
- [ ] Replica lag monitoring (if using replicas)
- [ ] Load testing completed
- [ ] Disaster recovery tested
- [ ] Database credentials secured
- [ ] SSL/TLS enabled for connections

## See Also

- [WATCH_ARCHITECTURE.md](./WATCH_ARCHITECTURE.md) - Event broadcasting architecture
- [PostgreSQL LISTEN/NOTIFY](https://www.postgresql.org/docs/current/sql-notify.html)
- [JSONB Performance](https://www.postgresql.org/docs/current/datatype-json.html)
- [Connection Pooling](https://pkg.go.dev/database/sql#DB.SetMaxOpenConns)
