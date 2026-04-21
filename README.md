# BinCrypt

Encrypted pastebin for short-lived text sharing.

Password-protected pastes are encrypted and decrypted in the browser with AES-256-GCM before anything is sent to the server. No-password pastes are intentionally stored as unencrypted paste data and should be treated as link-accessible public content.

No independent third-party security audit has been completed. 
Review the code before using it for anything high-stakes.

## What It Does

- Browser-side encrypted text pastes
- Optional no-password paste mode
- Burn-after-read links
- Time-based expiration
- Syntax highlighting
- SQLite by default, with PostgreSQL and Google Cloud Storage options
- In-memory rate limiting by default, with Redis and Firestore options
- Optional BTCPay invoice endpoints

## Run It

```bash
./setup.sh
```

Or run it directly:

```bash
cp .env.example .env
cd src
go run .
```

The app listens on `http://localhost:8080` by default.

## Build

```bash
# SQLite/Postgres, memory/Redis rate limiting
cd src && go build -o ../bincrypt .

# Include GCS, Firestore, Secret Manager, Cloud Logging
cd src && go build -tags gcp -o ../bincrypt .
```

## Configuration

Common settings live in [.env.example](.env.example).

```bash
PORT=8080
STORAGE_BACKEND=sqlite
SQLITE_PATH=./bincrypt.db

RATE_LIMITER_TYPE=memory

METRICS_ENABLED=false
METRICS_TOKEN=
```

Set `METRICS_TOKEN` before enabling `METRICS_ENABLED=true`.

## API

Create an encrypted paste:

```http
POST /api/paste
Content-Type: application/json

{
  "ciphertext": "base64-encoded-encrypted-data",
  "expiry_seconds": 604800,
  "burn_after_read": false
}
```

Create a no-password paste:

```http
POST /api/paste
Content-Type: application/json

{
  "plaintext": "your content here",
  "expiry_seconds": 604800,
  "burn_after_read": false
}
```

Fetch paste data:

```http
GET /api/paste/{id}
```

Health check:

```http
GET /api/health
```

## Notes

- The decryption key lives in the URL fragment when you share the full encrypted link.
- Anyone with the full encrypted URL can decrypt the paste.
- Browser history, extensions, screenshots, and compromised devices are outside BinCrypt's protection boundary.
- No-password pastes are readable by anyone with the link.
- Paste IDs and paste paths are redacted or hashed in logs.

## Development

```bash
go test ./...
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

## License

BinCrypt is dual-licensed:

- AGPL-3.0 for the open-source version. See [LICENSE](LICENSE).
- Commercial Apache-2.0 licenses are available from the maintainer. See [LICENSE-APACHE](LICENSE-APACHE).
