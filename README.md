# stuff to fix later:

```bash
go run ./src/server/cmd/migrate up        # ok
go run ./src/server/cmd/migrate down      # ok, and tables are gone
sqlite3 src/server/data/social.db ".tables"   # only schema_migrations
go run ./src/server/cmd/migrate up        # ok again, which proves every .down.sql is correct

go run ./src/server/cmd/server            # "migrations up to date" version=15, then "listening"
curl -i localhost:8080/api/health         # 200 {"status":"ok"}
curl localhost:8080/api/nope              # {"error":{"code":"not_found",...}}
# Ctrl+C -> "shutting down", then start it again: no errors, nothing applied twice
sqlite3 src/server/data/social.db "PRAGMA foreign_key_check;"   # prints nothing
```

# ignore

```js 
// Copy as fetch
fetch("http://localhost:8080/", {
  "headers": {
    "accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
    "accept-language": "en-GB,en-US;q=0.9,en;q=0.8",
    "cache-control": "max-age=0",
    "upgrade-insecure-requests": "1"
  },
  "body": null,
  "method": "GET",
  "mode": "cors",
  "credentials": "omit"
});
```

```js
// Copy as fetch (Node.js)
fetch("http://localhost:8080/", {
  "headers": {
    "accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
    "accept-language": "en-GB,en-US;q=0.9,en;q=0.8",
    "cache-control": "max-age=0",
    "upgrade-insecure-requests": "1"
  },
  "body": null,
  "method": "GET"
});
```
# run two images on the same container on dockerfile? and also, 