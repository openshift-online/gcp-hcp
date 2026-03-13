# etcd-benchmark-tool

Container image for generating etcd write pressure on HCP management clusters. Used by the `start-benchmark` Cloud Workflow ([GCP-468](https://issues.redhat.com/browse/GCP-468)) and the etcd Overload Detection and Automated Response demo.

## Image

`quay.io/cveiga/etcd-benchmark-tool:v3.5.21-3`

> **Note**: This is a temporary personal repo. The image will be moved to an official registry once one is established for the project.

## What it does

On startup, the container launches parallel `benchmark put` workers that write 10KB values to etcd via the `etcd-client` service. The container runs until the total key count is reached or the pod/Deployment is deleted.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `WORKERS` | 1 | Number of parallel PUT workers |
| `RANGE_WORKERS` | 0 | Number of parallel RANGE workers |
| `CLIENTS` | 50 | Concurrent gRPC clients per worker |
| `KEY_SIZE` | 256 | Key size in bytes |
| `VAL_SIZE` | 10240 | Value size in bytes |

### Presets (used by the start-benchmark workflow)

| Preset | WORKERS | RANGE_WORKERS | CLIENTS | Use case |
|--------|---------|---------------|---------|----------|
| light | 1 | 0 | 50 | Gentle write pressure, good for testing alerts |
| medium | 4 | 2 | 100 | Moderate load (writes + reads), triggers WARNING |
| heavy | 8 | 3 | 200 | Full blast, triggers CRITICAL quickly |

## TLS Requirements

The container expects etcd client TLS certificates mounted at:

- `/etc/etcd/tls/client/etcd-client.crt` — from Secret `etcd-client-tls`
- `/etc/etcd/tls/client/etcd-client.key` — from Secret `etcd-client-tls`
- `/etc/etcd/tls/etcd-ca/ca.crt` — from ConfigMap `etcd-ca`

These are standard across all HCP namespaces.

## Build

```bash
podman build --platform linux/amd64 -t quay.io/cveiga/etcd-benchmark-tool:v3.5.21-3 .
podman push quay.io/cveiga/etcd-benchmark-tool:v3.5.21-3
```

## Manual usage (without workflow)

```bash
kubectl apply -n <hcp-namespace> -f - <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: etcd-benchmark
spec:
  replicas: 1
  selector:
    matchLabels:
      app: etcd-benchmark
  template:
    metadata:
      labels:
        app: etcd-benchmark
    spec:
      containers:
        - name: benchmark
          image: quay.io/cveiga/etcd-benchmark-tool:v3.5.21-3
          imagePullPolicy: Always
          env:
            - name: WORKERS
              value: "1"
            - name: CLIENTS
              value: "50"
          volumeMounts:
            - name: client-tls
              mountPath: /etc/etcd/tls/client
              readOnly: true
            - name: etcd-ca
              mountPath: /etc/etcd/tls/etcd-ca
              readOnly: true
      volumes:
        - name: client-tls
          secret:
            secretName: etcd-client-tls
            defaultMode: 0640
        - name: etcd-ca
          configMap:
            name: etcd-ca
            defaultMode: 0644
EOF
```

Stop by deleting the Deployment:

```bash
kubectl delete deployment etcd-benchmark -n <hcp-namespace>
```

## Monitoring etcd during benchmark

Watch etcd DB size and health while the benchmark is running:

```bash
NS=<hcp-namespace>
watch -n 5 "kubectl exec -n $NS etcd-0 -c etcd -- etcdctl \
  --endpoints=https://etcd-client:2379 \
  --cert=/etc/etcd/tls/client/etcd-client.crt \
  --key=/etc/etcd/tls/client/etcd-client.key \
  --cacert=/etc/etcd/tls/etcd-ca/ca.crt \
  endpoint status -w table 2>/dev/null"
```

Other useful etcdctl commands (run via `kubectl exec -n $NS etcd-0 -c etcd -- etcdctl [flags] <command>`):

| Command | Description |
|---------|-------------|
| `endpoint status -w table` | DB size, leader, raft index |
| `endpoint health -w table` | Health check with latency |
| `member list -w table` | Cluster membership |
| `endpoint status -w json` | Machine-parseable status (for automation) |

## Recovery (compact + defrag)

After stopping the benchmark, compact and defrag etcd to reclaim disk space:

```bash
NS=<hcp-namespace>

# Helper function (paste into your shell)
etcdctl_exec() {
  kubectl exec -n "$NS" etcd-0 -c etcd -- etcdctl \
    --endpoints=https://etcd-client:2379 \
    --cert=/etc/etcd/tls/client/etcd-client.crt \
    --key=/etc/etcd/tls/client/etcd-client.key \
    --cacert=/etc/etcd/tls/etcd-ca/ca.crt \
    "$@"
}

# 1. Get current revision
REV=$(etcdctl_exec endpoint status -w json 2>/dev/null \
  | grep -o '"revision":[0-9]*' | head -1 | cut -d: -f2)

# 2. Compact to current revision (removes old revisions)
etcdctl_exec compact "$REV"

# 3. Defrag to reclaim disk space
etcdctl_exec defrag --command-timeout=120s
```

**If defrag fails with "no space left on device"**: Defrag creates a temporary copy of the DB file, requiring ~2x the current DB size in free disk space. Expand the PVC first:

```bash
kubectl patch pvc data-etcd-0 -n "$NS" --type=merge \
  -p '{"spec":{"resources":{"requests":{"storage":"20Gi"}}}}'

# Wait for expansion to complete
kubectl get pvc data-etcd-0 -n "$NS" -o jsonpath='{.status.capacity.storage}'

# Retry defrag
etcdctl_exec defrag --command-timeout=120s
```
