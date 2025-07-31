# BinCrypt - Zero-Knowledge Encrypted Pastebin

A secure, serverless pastebin with client-side encryption using Google Cloud Storage.

## Features

- 🔐 **Zero-Knowledge Encryption**: All encryption happens client-side using AES-256-GCM
- ☁️ **Serverless Architecture**: Uses only Google Cloud Storage - no database required
- 🔥 **Burn After Read**: Single-view self-destructing pastes
- ⚡ **Rate Limiting**: GCS-based distributed rate limiting
- 💰 **Premium Features**: Bitcoin payments via BTCPay Server
- 🎨 **Syntax Highlighting**: Support for 18+ programming languages
- 📱 **Responsive Design**: Works on all devices

## Architecture

```
src/
├── handlers/       # HTTP request handlers
├── middleware/     # HTTP middleware (logging, recovery, rate limiting)
├── models/         # Data models
├── services/       # Business logic services
├── server/         # HTTP server setup
├── static/         # Frontend assets (HTML, CSS, JS)
└── main.go         # Application entry point
```

## Quick Start

### Prerequisites

- Go 1.24+
- Firebase CLI (`npm install -g firebase-tools`)
- Google Cloud SDK (for deployment)

### Local Development

1. Clone the repository:
```bash
git clone https://github.com/yourusername/bincrypt.git
cd bincrypt
```

2. Copy the environment variables:
```bash
cp .env.example .env
# Edit .env with your configuration
```

3. Run with Firebase emulator:
```bash
./tools/run.sh
```

The app will be available at http://localhost:8080

### Development Mode

For hot reload during development:
```bash
./tools/dev.sh
```

### Building

To build the binary:
```bash
./tools/build.sh
```

## Configuration

Environment variables:

- `PORT` - Server port (default: 8080)
- `BUCKET_NAME` - Google Cloud Storage bucket name
- `STORAGE_EMULATOR_HOST` - Firebase emulator host (for local dev)
- `BTCPAY_ENDPOINT` - BTCPay Server URL
- `BTCPAY_APIKEY` - BTCPay API key
- `BTCPAY_WEBHOOK_SECRET` - Webhook signature secret
- `FIREBASE_*` - Firebase configuration

## Deployment

### Google Cloud Run

Deploy to Cloud Run:
```bash
./tools/deploy.sh
```

### Docker

Build and run with Docker:
```bash
docker build -t bincrypt .
docker run -p 8080:8080 --env-file .env bincrypt
```

## API Endpoints

- `POST /api/paste` - Create encrypted paste
- `GET /p/{id}` - View paste
- `POST /api/invoice` - Create premium invoice
- `POST /api/payhook` - Payment webhook
- `GET /api/health` - Health check
- `GET /api/metrics` - System metrics

## Security

- All encryption is performed client-side
- Server never sees plaintext content
- Pastes are stored encrypted in Google Cloud Storage
- Rate limiting prevents abuse
- HMAC signature verification for payment webhooks

## License

MIT License - see LICENSE file for details