# RideNow

RideNow is the backend service for the RideNow ride-hailing product. This
repository is the founding scaffold: a minimal but fully runnable Go service
with a health endpoint, a SQLite-backed data layer, one passing test, and CI.
Feature stories build on top of this skeleton.

## Stack

- **Language:** Go 1.23+
- **Router:** [chi](https://github.com/go-chi/chi) on top of `net/http`
- **Database:** SQLite via [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)
  — pure Go, no CGO. Tests use an in-memory (`:memory:`) database.

## Layout

```
.
├── cmd/ridenow/        # main entrypoint (wiring, graceful shutdown)
├── internal/
│   ├── db/             # SQLite connection helper
│   └── server/         # chi router, handlers, and tests
├── .github/workflows/  # CI pipeline
├── go.mod
├── Makefile
└── README.md
```

## Getting started

Requires Go 1.23 or newer.

```sh
# Resolve dependencies and generate go.sum
go mod tidy

# Run the tests
make test        # or: go test -race ./...

# Run the service
make run         # or: go run ./cmd/ridenow
```

The server listens on `:8080` by default:

```sh
curl -s localhost:8080/healthz
# {"status":"ok","db":"ok"}
```

## Configuration

| Env var          | Default      | Description                                 |
| ---------------- | ------------ | ------------------------------------------- |
| `RIDENOW_ADDR`   | `:8080`      | Address the HTTP server binds to.           |
| `RIDENOW_DB_DSN` | `ridenow.db` | SQLite DSN. Use `:memory:` for ephemeral.   |

## Endpoints

| Method | Path       | Description                                    |
| ------ | ---------- | ---------------------------------------------- |
| GET    | `/healthz` | Liveness/readiness check; pings the database.  |

## Development

```sh
make fmt    # gofmt -w .
make vet    # go vet ./...
make test   # go test -race ./...
```

CI runs formatting checks, `go vet`, build, and tests on every push and pull
request — see `.github/workflows/ci.yml`.
