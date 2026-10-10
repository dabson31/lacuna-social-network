# the backend image: the go process, the migrations it reads at boot, and nothing else.
FROM golang:1.27-alpine AS build

#! go-sqlite3 is a C library, so the build needs CGO and a C toolchain
RUN apk add --no-cache gcc musl-dev

WORKDIR /src

# deps as their own layer, so a source change does not re-download the module cache
COPY go.mod go.sum ./
RUN go mod download

COPY src/server ./src/server

# CGO_ENABLED=1 is the whole reason for the C toolchain above
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/server ./src/server/cmd/server

FROM alpine:3.20

LABEL org.opencontainers.image.title="social-network-backend"
LABEL org.opencontainers.image.description="Go API, WebSocket hub and SQLite store for the social network"

# never run as root. -D = no password, and a fixed uid keeps file ownership stable across
# rebuilds, which matters because the data volume below is chowned to it
RUN adduser -D -u 10001 app

WORKDIR /app

# the migrator reads the .sql files off disk at runtime through its file:// source, so they ship with the binary rather than being embedded
COPY --from=build /src/src/server/pkg/db/migrations/sqlite ./migrations

COPY --from=build /out/server ./server

# the db file and the uploaded images live here, and this path is the volume mount point, so it
# has to exist and be writable by the non-root user before the volume is attached
RUN mkdir -p /app/data/uploads && chown -R app:app /app

USER app
EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=15s --retries=5 \
  CMD wget --quiet --tries=1 --spider http://127.0.0.1:8080/api/health || exit 1

ENTRYPOINT ["/app/server"]
