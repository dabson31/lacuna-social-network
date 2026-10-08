# go-sqlite3 is a C library, meaning the build needs CGO and a C toolchain
# we add gcc + musl-dev for the C compiler
# we could also use llvm maybe
FROM golang:1.27-alpine AS build

RUN apk add --no-cache gcc musl-dev

WORKDIR /src

# download deps as their own layer, docker caches them
COPY go.mod go.sum ./
RUN go mod download

# actual source
COPY . .

# CGO_ENABLED=1 is the whole reason for the C toolchain above
RUN CGO_ENABLED=1 go build -o /out/server ./src/server/cmd/server

FROM alpine:3.20

# Never run as root. -D = no password, a fixed uid keeps file ownership stable.
# a user thats not the "owner" (there own owner) and doesnt need sudo priv
RUN adduser -D -u 10001 app

WORKDIR /app

# migrator reads the .sql files from disk at runtime (file:// source)
COPY --from=build /src/src/server/pkg/db/migrations/sqlite ./migrations

# compiled server
COPY --from=build /out/server ./server

# the DB file and uploaded images live here; owned by the non-root user. it works with a restart i think
RUN mkdir -p /app/data/uploads && chown -R app:app /app

USER app
EXPOSE 8080

ENTRYPOINT ["/app/server"]
