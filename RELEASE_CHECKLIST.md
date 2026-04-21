# Release Checklist

Use this checklist before publishing a new `v*` tag.

## 1. Preflight

- Confirm `main` is green in CI (`Tests`, `Security`, `Docker`).
- Confirm no unresolved security reports for the target release.
- Confirm `README.md`, `.env.example`, and deployment docs match current behavior.
- Update `CHANGELOG.md` for the release scope.

## 2. Local Verification

- Run:
  - `go vet ./...`
  - `go test -race ./...`
  - `./scripts/prepare-static.sh`
- Smoke test core endpoints:
  - `POST /api/paste`
  - `GET /api/paste/{id}`
  - `GET /api/health`

## 3. Dependency and Security Review

- Run `go mod tidy` (if needed) and ensure no accidental dependency drift.
- Run `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`.
- Review advisories in required modules and document any accepted residual risk.

## 4. Release Build and Tag

- Create and push an annotated semver tag:
  - `git tag -a vX.Y.Z -m "Release vX.Y.Z"`
  - `git push origin vX.Y.Z`
- Verify GitHub Release artifacts are generated:
  - Linux/macOS/Windows binaries
  - `checksums.txt`

## 5. Post-Release

- Validate install/run instructions against released artifacts.
- Announce release notes and highlight upgrade or migration steps.
- Open follow-up issues for deferred items discovered during release prep.
