# Build the backend and ship it on a minimal, non-root runtime image.
FROM golang:1.26 AS build
WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
# Static binary, no cgo: nothing to run from the userland of the base image.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/server"]
