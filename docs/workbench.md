# Workbench Development Guide

This guide explains how to run the Go API and the React/TypeScript workbench
together for local development.

## Prerequisites

- Go 1.25+
- Node.js 22+ and npm 10+
- The Go module is at the repository root. The frontend lives in `web/`.

## First-time setup

```powershell
# Go dependencies (automatic on first build)
go build ./...

# Frontend dependencies
npm install
```

## Database

The Go backend reads `TTS_DATABASE_URL` and applies additive migrations
automatically on startup. The default URL is
`sqlite:///./data/go-dev.sqlite3`.

## Run the Go API

```powershell
$env:TTS_DATABASE_URL = "sqlite:///./data/go-dev.sqlite3"
$env:TTS_STATIC_DIR = "" # leave empty for development; the proxy handles routing
go run ./cmd/tkshop migrate
go run ./cmd/tkshop serve --addr :8080
```

The server exposes:

- `GET /healthz` and `GET /products` for legacy clients
- `GET /api/v1/trends`, `GET /api/v1/products/{id}`, `GET /api/v1/products/{id}/metrics`
- `GET /api/v1/categories`, `GET /api/v1/category-mappings`,
  `PATCH /api/v1/products/{id}/category`
- `GET /api/v1/imports`, `POST /api/v1/imports/csv`, `GET /api/v1/imports/{id}`

## Run the frontend in development

```powershell
cd web
npm install
npm run dev
```

The Vite dev server runs on `http://localhost:5173` and proxies `/api` to the
Go API on `http://localhost:8080`.

## Sample CSV import

A representative fixture lives at `data/sample_amazon.csv`. From the workbench
Imports page, upload that file or run the CLI directly:

```powershell
go run ./cmd/tkshop import-csv --file data/sample_amazon.csv --region US
```

## Production build

```powershell
cd web
npm run build

# Serve the production build with the Go server
$env:TTS_STATIC_DIR = "./web/dist"
go run ./cmd/tkshop serve --addr :8080
```

The Go server only serves files inside the configured directory and rejects
path-traversal attempts.

## Tests

```powershell
# Backend
go test ./...

# Frontend
cd web
npm test          # vitest
npm run build     # tsc + vite build
```

## Adding a new canonical category

Canonical categories live in the `internal/category` package. Append a new
seed to `DefaultSeeds()` and rebuild. The seed is applied with
`INSERT OR IGNORE`, so existing categories are preserved.

## Adding a new source mapping

Mappings can be inserted via the database directly or via the upcoming admin
endpoint. They follow the schema in `source_category_mappings`.

## Browser verification checklist

- Trending Products: desktop table renders all columns without overlap
- Trending Products: mobile viewport stacks product rows
- Product Detail: insufficient-history chart shows the empty state
- Categories: unresolved list shows products without confident assignment
- Imports: upload a CSV with 100+ rows and verify pagination of history

## Deployment notes

The first release is intended for local development and single-user
evaluation. The HTTP API does not implement authentication or tenancy. Do
not expose the server to the public internet without adding those controls
first.