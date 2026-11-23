# Build the backend and ship it on a minimal Red Hat runtime (per
# gcp-hcp-infra's ubi-minimal pattern; see tools/etcd-benchmark-tool/Dockerfile).
FROM registry.access.redhat.com/ubi9/go-toolset:1.26@sha256:0e3894cea4fc077084245e8aa2befc8a7d91a4f60fbf850f0b5b555cbe62ee26 AS build
USER 0
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest@sha256:08d4c70f1baf6537a0728656cf7942d561e74a35916509d205b6519861266352

LABEL org.opencontainers.image.title="marketplace-signup-backend" \
      org.opencontainers.image.description="Interim marketplace sign-up backend (experiment)" \
      org.opencontainers.image.source="https://github.com/openshift-online/gcp-hcp" \
      org.opencontainers.image.vendor="Red Hat" \
      vendor="Red Hat, Inc." \
      com.redhat.component="marketplace-signup-backend" \
      summary="Marketplace sign-up backend" \
      distribution-scope="private"

RUN microdnf install -y --nodocs ca-certificates && microdnf clean all && rm -rf /var/cache/dnf /var/cache/yum

COPY --from=build /out/server /server

USER 1001
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 CMD curl -f http://localhost:8080/healthz || exit 1
ENTRYPOINT ["/server"]
