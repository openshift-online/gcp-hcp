FROM golang:1.23 AS builder
ARG ETCD_VER=v3.5.21
RUN git clone --depth 1 --branch ${ETCD_VER} https://github.com/etcd-io/etcd.git /tmp/etcd \
    && cd /tmp/etcd/tools/benchmark && go build -o /usr/local/bin/benchmark .
RUN curl -sL "https://github.com/etcd-io/etcd/releases/download/${ETCD_VER}/etcd-${ETCD_VER}-linux-amd64.tar.gz" \
    | tar xz --strip-components=1 -C /usr/local/bin "etcd-${ETCD_VER}-linux-amd64/etcdctl"

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
RUN microdnf install -y coreutils-single && microdnf clean all
COPY --from=builder /usr/local/bin/benchmark /usr/local/bin/etcdctl /usr/local/bin/

ENV WORKERS=1 RANGE_WORKERS=0 CLIENTS=50 KEY_SIZE=256 VAL_SIZE=10240

ENTRYPOINT ["bash", "-c", "\
for i in $(seq 1 $WORKERS); do \
  benchmark --endpoints=https://etcd-client:2379 \
    --cert=/etc/etcd/tls/client/etcd-client.crt \
    --key=/etc/etcd/tls/client/etcd-client.key \
    --cacert=/etc/etcd/tls/etcd-ca/ca.crt \
    --conns=$CLIENTS --clients=$CLIENTS \
    put --key-size=$KEY_SIZE --val-size=$VAL_SIZE \
    --key-space-size=2000000 --sequential-keys \
    --total=20000000 > /dev/null 2>&1 & \
done; \
for i in $(seq 1 $RANGE_WORKERS); do \
  benchmark --endpoints=https://etcd-client:2379 \
    --cert=/etc/etcd/tls/client/etcd-client.crt \
    --key=/etc/etcd/tls/client/etcd-client.key \
    --cacert=/etc/etcd/tls/etcd-ca/ca.crt \
    --conns=$CLIENTS --clients=$CLIENTS \
    range / --total=20000000 > /dev/null 2>&1 & \
done; \
echo \"$WORKERS put + $RANGE_WORKERS range workers launched with $CLIENTS clients each\"; \
wait"]
