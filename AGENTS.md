# Agent Notes

Short project guidance for coding agents working in this repository.

## Product Shape

BinCrypt is a pastebin with browser-side encryption for password-protected pastes. In encrypted mode, the server stores ciphertext and never receives the decryption password. In no-password mode, content is intentionally stored server-side as unencrypted paste data.

## Build Options

```bash
go build .             # SQLite, Postgres, memory rate limiting, Redis
go build -tags gcp .   # Adds GCS, Firestore, Secret Manager, Cloud Logging
```

## Defaults

- Storage: SQLite at `./bincrypt.db`
- Rate limiting: in-memory token bucket
- Secrets: environment variables, with optional Secret Manager in GCP builds

## Useful Commands

```bash
./setup.sh
docker compose --profile sqlite up --build

cd src && go build -o ../bincrypt .
go test ./...
go vet ./...
```

## Architecture

- `src/main.go`: backend selection, config loading, server boot.
- `src/server.go`: router and HTTP server setup.
- `src/middleware.go`: logging, recovery, rate limiting, request limits, validation, CORS, CSP, nonce injection.
- `src/handlers_paste.go`: paste create/read/view handlers.
- `src/handlers_invoice.go`: optional BTCPay handlers.
- `src/services/`: storage backends, rate limiters, deletion queue, BTCPay client.
- `src/static/`: browser UI and encryption/decryption logic.
- `tests/go/`: package-level behavior tests.

## Security Rules

- Do not log raw paste IDs, full paste URLs, passwords, tokens, API keys, webhook signatures, or plaintext paste contents.
- Keep `/api/metrics` behind `METRICS_TOKEN` whenever metrics are enabled.
- Keep CSP nonce placeholders as `{{CSP_NONCE}}` when adding inline scripts/styles.
- Keep encrypted-paste guarantees intact. Server-side plaintext handling belongs only to the explicit no-password mode.
- Prefer small, auditable changes with tests around touched boundaries.
