# Contributing to BinCrypt

Thanks for considering a contribution! BinCrypt is a pastebin with browser-side encryption, written in Go. This guide explains how to get set up, how to propose changes, and the expectations for incoming patches.

## Contribution License

BinCrypt is dual-licensed: the open-source version is AGPL-3.0, and commercial Apache-2.0 licenses are available from the maintainer. By submitting a contribution, you agree that:

- Your contribution is licensed under **AGPL-3.0** as part of the open-source BinCrypt project.
- You grant the maintainer the right to include your contribution in commercial BinCrypt distributions licensed under **Apache-2.0**.

If you are not comfortable with this dual-licensing model, please do not submit contributions.

## Getting Started

1. **Fork & Clone**
   ```bash
   git clone https://github.com/<your-user>/bincrypt.git
   cd bincrypt
   ```
2. **Tooling**
   - Go 1.25+
   - Docker + Docker Compose (for local stack)
   - Firebase CLI (optional, used for the storage emulator)
3. **Environment**
   ```bash
   cp .env.example .env
   # Fill in local values (or leave the defaults for emulator use)
   ./setup.sh  # chooses storage backend and launches docker-compose
   ```
   If you choose the GCS emulator profile, `./setup.sh` will generate `firebase.json` plus `storage.rules` (deny-all template) and `storage.rules.dev` (emulator-only). `firebase.json` is configured to use the dev rules for local emulators; never deploy `storage.rules.dev`, and replace `storage.rules` with your production policy before deploying.

## Development Workflow

1. **Format & Lint**
   ```bash
   cd src
   gofmt -w .
   go vet ./...
   ```
2. **Unit Tests**
   ```bash
   go test ./...
   ```
   Some tests require emulators or external services. When possible, guard new tests with environment variables so CI can run in a hermetic environment.
3. **Static Assets**
   - Update scripts/styles in `src/static/`.
   - Regenerate Subresource Integrity hashes for any CDN assets you change.
   - Run `npm` or other asset pipelines in a temporary directory; do not commit build artefacts.
4. **Secrets**
   - Never commit `.env` or real credentials.
   - Use placeholders and update `.env.example` if new configuration keys are required.

## Pull Requests

- Keep PRs focused; split across multiple branches if the change set is large.
- Include tests for new functionality or a justification when tests are not applicable.
- Update documentation (README, SECURITY.md) when behaviour or configuration changes.
- Update `CHANGELOG.md` for user-facing or operational changes.
- Reference related issues and describe the testing performed.
- Do not force-push shared feature branches without coordination.

## Code Style

- Prefer standard library functionality over new dependencies.
- Aim for small, composable functions. Add comments only when intent is not obvious.
- Use structured logging via `src/logger`; ensure sensitive data is not logged.
- Follow existing patterns for error wrapping (`fmt.Errorf("context: %w", err)`).

## Reporting Vulnerabilities

See [SECURITY.md](SECURITY.md) for private disclosure instructions. Never open a public issue for an unpatched security vulnerability.

Thank you for helping improve BinCrypt!
