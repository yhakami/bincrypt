# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Multi-host frontend deployment templates for Firebase Hosting, Vercel, and Netlify.
- `scripts/prepare-static.sh` for static export packaging.
- CI security scanning workflow (`govulncheck`).
- Maintainer-facing release checklist and support policy docs.
- GitHub issue templates for bug reports and feature requests.

### Changed
- Release workflow now runs verification checks before publishing assets.
- Repository ignore rules now include generated binaries and local distribution artifacts.
- GitHub Actions workflows now pin third-party actions to immutable commit SHAs.

## [1.0.0] - Initial public release

### Added
- Browser-side encrypted pastebin with AES-256-GCM for password-protected pastes.
- Burn-after-read support with backend race-condition protections.
- Multiple storage backends: SQLite, PostgreSQL, and Google Cloud Storage.
- Multiple rate limiter backends: memory, Redis, and Firestore.
- Docker-based local development and production deployment paths.
