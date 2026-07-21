# orlop-gc - Garbage Collector

Standalone garbage collector for Orlop API objects with owner references.

## Overview

The `orlop-gc` binary implements Kubernetes-style cascading deletion by automatically
deleting dependent objects when their owners no longer exist. It runs as a separate
process and periodically scans all resources to find and delete orphaned objects.

## How It Works

1. **Periodic Scanning**: Every `--interval` (default 30s), scans all resources
2. **Owner Check**: For each object with `metadata.ownerReferences`, verifies owners exist
3. **Cascading Delete**: Deletes objects whose owners are missing
4. **Logging**: Reports what was checked and deleted each cycle

## Usage

```bash
# Connect to PostgreSQL backend
orlop-gc \
  --db-host=localhost \
  --db-port=5432 \
  --db-name=orlop \
  --db-user=orlop \
  --db-password=secret \
  --interval=30s \
  --v=1
```

## Flags

- `--interval`: Garbage collection interval (default: 30s)
- `--db-host`: PostgreSQL host (default: localhost)
- `--db-port`: PostgreSQL port (default: 5432)
- `--db-name`: PostgreSQL database name (default: orlop)
- `--db-user`: PostgreSQL user (default: orlop)
- `--db-password`: PostgreSQL password
- `--db-sslmode`: PostgreSQL SSL mode (default: disable)
- `--v`: Log verbosity level (0=info, 1=debug)

## Owner References

Objects are automatically deleted when their owner is deleted if they have
`metadata.ownerReferences` set:

```yaml
apiVersion: test.orlop.thetechnick.ninja/v1
kind: Object
metadata:
  name: dependent-object
  ownerReferences:
  - apiVersion: test.orlop.thetechnick.ninja/v1
    kind: Object
    name: owner-object
    uid: abc-123-def-456
spec:
  publicField: value
```

When `owner-object` is deleted, `dependent-object` will be automatically
deleted by the garbage collector.

## Deployment

The garbage collector should be deployed as a separate process/pod that has
access to the same storage backend (PostgreSQL) as the API server.

### Docker Example

```dockerfile
FROM golang:1.26 AS builder
WORKDIR /workspace
COPY . .
RUN go build -o orlop-gc ./cmd/orlop-gc

FROM gcr.io/distroless/base
COPY --from=builder /workspace/orlop-gc /orlop-gc
ENTRYPOINT ["/orlop-gc"]
```

### Kubernetes Example

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: orlop-gc
spec:
  replicas: 1
  selector:
    matchLabels:
      app: orlop-gc
  template:
    metadata:
      labels:
        app: orlop-gc
    spec:
      containers:
      - name: gc
        image: orlop-gc:latest
        args:
        - --db-host=postgres
        - --db-name=orlop
        - --db-user=orlop
        - --db-password=$(DB_PASSWORD)
        - --interval=30s
        - --v=1
        env:
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: postgres-secret
              key: password
```

## Resource Configuration

Currently, the list of resources to garbage collect is hardcoded in `main.go`.
In a production deployment, this would typically be:

1. Loaded from a configuration file
2. Discovered via the API server's discovery endpoint
3. Passed as command-line arguments

## Performance Considerations

- **Interval**: Set based on acceptable latency for cascading deletes
- **Single Instance**: Only one GC instance should run to avoid conflicts
- **Database Load**: Each cycle does one LIST per resource type
- **Batch Size**: Currently processes all objects; consider pagination for large datasets

## Metrics

The GC logs the following metrics each cycle:
- `duration`: How long the GC cycle took
- `checked`: Number of objects examined
- `deleted`: Number of orphaned objects deleted

Future enhancements could expose Prometheus metrics.
