FROM registry.access.redhat.com/ubi9/go-toolset:latest AS builder
USER root
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -o /build/hyperfleet-adapters-go ./cmd/...

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /build/hyperfleet-adapters-go /hyperfleet-adapters-go
ENTRYPOINT ["/hyperfleet-adapters-go"]
