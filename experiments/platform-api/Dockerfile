FROM golang:1.26 AS builder

WORKDIR /workspace
COPY orlop/ orlop/
COPY platform-api/ platform-api/

WORKDIR /workspace/platform-api
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /platform-api-server ./cmd/platform-api-server

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /platform-api-server /platform-api-server
USER 65532:65532
ENTRYPOINT ["/platform-api-server"]
