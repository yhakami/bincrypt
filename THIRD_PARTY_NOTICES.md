# Third-Party Notices

BinCrypt includes or depends on third-party software components. The following table summarises the primary dependencies and their licenses. Each project retains its own copyright.

| Component | Source / Homepage | License |
|-----------|-------------------|---------|
| Alpine.js | https://alpinejs.dev | MIT |
| Prism.js | https://prismjs.com | MIT |
| Prism Autoloader Plugin | https://prismjs.com | MIT |
| highlight.js | https://highlightjs.org | BSD-3-Clause |
| Flatpickr | https://flatpickr.js.org | MIT |
| Firebase JavaScript SDK | https://github.com/firebase/firebase-js-sdk | Apache-2.0 |
| Google Fonts (Inter, JetBrains Mono) | https://fonts.google.com | SIL Open Font License 1.1 |
| Gorilla Mux (`github.com/gorilla/mux`) | https://github.com/gorilla/mux | BSD-3-Clause |
| go-redis (`github.com/redis/go-redis/v9`) | https://github.com/redis/go-redis | BSD-2-Clause |
| pgx (`github.com/jackc/pgx/v5`) | https://github.com/jackc/pgx | MIT |
| go-sqlite3 (`github.com/mattn/go-sqlite3`) | https://github.com/mattn/go-sqlite3 | MIT |
| Google Cloud Go SDK (`cloud.google.com/go/*`) | https://cloud.google.com/go | Apache-2.0 |
| Google APIs Client (`google.golang.org/api`) | https://github.com/googleapis/google-api-go-client | BSD-3-Clause |
| gRPC (`google.golang.org/grpc`) | https://grpc.io | Apache-2.0 |
| OpenTelemetry (`go.opentelemetry.io/*`) | https://opentelemetry.io | Apache-2.0 |
| Redis Docker Image (`redis:7-alpine`) | https://hub.docker.com/_/redis | Various (see upstream image) |
| PostgreSQL Docker Image (`postgres:16-alpine`) | https://hub.docker.com/_/postgres | PostgreSQL |
| golang Docker Image (`golang:1.25-alpine`) | https://hub.docker.com/_/golang | BSD-style |
| alpine Docker Image (`alpine:latest`) | https://hub.docker.com/_/alpine | MIT |

This list is not exhaustive. To generate a complete inventory, run a license scanner such as:

```bash
go install github.com/google/go-licenses@latest
go-licenses report ./... > third_party_report.txt
```

For front-end dependencies included via CDN, consult the upstream project sites for the latest license info before pinning new versions.
