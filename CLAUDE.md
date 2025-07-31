# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## BinCrypt - Zero-Knowledge Encrypted Pastebin

BinCrypt is a secure pastebin service with client-side encryption using AES-256-GCM. It uses Google Cloud Storage as its sole backend (NO PostgreSQL database).

## Avoiding File Proliferation

To maintain a clean and manageable codebase, adhere to the following guidelines:

- **Flat Structure**: Keep the directory structure as flat as possible. Avoid creating new directories unless absolutely necessary.
- **Reuse Existing Files**: Before creating a new file, consider if the functionality can be integrated into an existing file. Group related functions and logic together.
- **Consolidate Logic**: Where feasible, consolidate similar logic into shared modules or services to reduce redundancy.
- **Minimal Dependencies**: Limit the use of external libraries and dependencies. Prefer native solutions and built-in functionalities.
- **Documentation**: Use existing documentation files to add new information. Avoid creating separate documentation files for small updates.
- **Review and Refactor**: Regularly review the codebase to identify opportunities for refactoring and consolidation. Remove obsolete or redundant files.

By following these principles, we ensure that the codebase remains simple, efficient, and easy to navigate, preventing unnecessary complexity and file proliferation.


## Essential Commands

### Development
```bash
# Start local development with Firebase emulator
./run.sh

# Build the Go binary
cd src && go build -o ../bincrypt . && cd ..

# Run tests (when implemented)
cd src && go test ./... && cd ..
```

### Environment Setup
1. Copy `.env.example` to `.env`
2. Configure Firebase emulator: `STORAGE_EMULATOR_HOST=localhost:9199`
3. Set `BUCKET_NAME` to match your Firebase project

## Architecture Overview

### Zero-Knowledge Design
- **Client-side encryption**: All encryption/decryption happens in browser using AES-256-GCM
- **Server never sees plaintext**: Server only stores encrypted blobs in GCS
- **No database**: Everything stored as objects in Google Cloud Storage

### Key Components

1. **Storage Layer** (`src/services/storage.go`)
   - All data stored in GCS with metadata
   - Paste content: `pastes/{id}/content`
   - Paste metadata: `pastes/{id}/metadata.json`
   - Rate limiting: `ratelimits/{identifier}/{bucket}.json`

2. **Handlers** (`src/handlers/`)
   - `paste.go`: Handles paste creation/viewing
     - `ViewPaste`: Serves HTML viewer page
     - `GetPasteAPI`: Returns JSON data for AJAX
     - `CreatePaste`: Stores encrypted content
   - `invoice.go`: BTCPay Server integration
     - **CRITICAL**: Payment webhook lacks authentication
   - `config.go`: Serves client configuration

3. **Frontend** (`src/static/`)
   - `index.html`: Main paste creation page
   - `viewer.html`: Paste viewing page
   - `app.js`: Encryption/decryption logic
   - `coinzilla-ads.js`: Ad integration

### Security Considerations

1. **Rate Limiting**: Uses GCS objects (has race conditions in production)
2. **CSP Headers**: Currently allows `unsafe-inline` and `unsafe-eval`
3. **Payment Webhooks**: Missing BTCPay signature verification
4. **CORS**: Not explicitly configured

### API Routes

- `POST /api/paste` - Create encrypted paste
- `GET /p/{id}` - View paste HTML page
- `GET /api/paste/{id}` - Get paste JSON data
- `POST /api/invoice` - Create premium invoice
- `POST /api/payhook` - Payment webhook (unauthenticated)
- `GET /api/config` - Client configuration
- `GET /api/health` - Health check

### Environment Variables

Critical for production:
- `BUCKET_NAME` - GCS bucket name
- `BTCPAY_ENDPOINT` - BTCPay Server URL
- `BTCPAY_APIKEY` - BTCPay API key
- `BTCPAY_WEBHOOK_SECRET` - For webhook verification (not implemented)
- `FIREBASE_*` - Firebase configuration
- `COINZILLA_*` - Ad zone IDs

### Production Deployment Issues

Before deploying to production, these MUST be fixed:
1. Implement BTCPay webhook signature verification
2. Remove `unsafe-inline` and `unsafe-eval` from CSP
3. Replace GCS-based rate limiting with Redis/in-memory
4. Add input validation middleware
5. Implement proper secrets management

### Testing

Currently no tests exist. When implementing:
- Test encryption/decryption roundtrip
- Test rate limiting logic
- Test payment flow
- Test paste expiration

### Common Development Tasks

```bash
# Check for build errors
cd src && go build . && cd ..

# Format Go code
cd src && go fmt ./... && cd ..

# Run locally with hot reload (not implemented)
# Consider using air or similar tool
```

### Important Notes

1. **No Database**: All state in GCS - be careful with consistency
2. **Embedded Files**: Static files embedded with `go:embed`
3. **Firebase Emulator**: Required for local development
4. **Client-Side Focus**: Most logic happens in browser
5. **Security First**: Always consider zero-knowledge architecture when making changes