# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Commands

### Full Local Development

```bash
npm install
./start.sh
```

`./start.sh` starts the Go backend on `127.0.0.1:9192` and the frontend on `0.0.0.0:9191`. By default it builds the frontend and runs Vite preview mode. Use hot reload with:

```bash
FRONTEND_MODE=dev ./start.sh --restart
```

Useful variants:

```bash
./start.sh --status
./start.sh --restart
./start.sh --stop
```

### Frontend

Run from the repository root:

```bash
npm run dev        # Vite dev server; default port comes from Vite unless overridden
npm run dev:raw    # Vite dev server on 127.0.0.1:5173
npm run build      # tsc -b && vite build
npm run preview    # Vite preview; vite.config.ts uses port 9191
npm run lint       # TypeScript no-emit check
npm test           # node --import tsx --test tests/*.test.ts
```

Run one frontend test:

```bash
node --import tsx --test tests/previewIntent.test.ts
```

### Backend

Run from `backend/` unless noted:

```bash
go run ./cmd/server
go test ./... -count=1
go build -o video-server ./cmd/server
```

Run one backend package or test:

```bash
go test ./internal/scanner -count=1
go test ./internal/scanner -run TestParse -count=1
```

The backend requires Go 1.23+ and uses vendored dependencies in `backend/vendor/`, so keep `go mod vendor` in sync after dependency changes.

### Release and Deployment

```bash
scripts/build-release.sh   # builds Linux amd64/arm64 release tarballs into release/
sudo bash install.sh       # prebuilt installer flow used by README
sudo bash deploy.sh        # build from current checkout and install systemd services
```

Docker uses the root `Dockerfile` and `docker-compose.yml`. The runtime image exposes port `9191` and stores persistent data under `/opt/video-site-91/data`.

## Architecture Overview

This is a private video aggregation site with a React/Vite frontend and a Go backend.

### Frontend

The frontend is a React 18 SPA under `src/`. `src/main.tsx` mounts `BrowserRouter`, `ToastProvider`, and `AuthProvider`, then renders `src/App.tsx`. `App.tsx` defines the public app routes (`/`, `/list`, `/shorts`, `/upload`, `/video/:id`) and admin routes under `/admin`; both main-site and admin pages are wrapped in `RequireAuth`, while `/login` is public.

Frontend API calls are split by surface:

- `src/data/videos.ts` calls the main authenticated API under `/api` and upload/proxy-related endpoints.
- `src/admin/api.ts` is the admin API client for `/admin/api`, always sending cookies and raising `UnauthorizedError` on `401`.

`vite.config.ts` proxies `/api`, `/p`, and `/admin/api` to `http://127.0.0.1:9192`, with frontend dev/preview served on port `9191` by default. The alias `@` maps to `src`.

Styling is plain CSS loaded from `src/main.tsx` in token/base/layout/navigation/search/video/admin layers. Shared UI lives in `src/components`, page-level screens in `src/pages`, and admin screens in `src/admin`.

### Backend

The backend entrypoint is `backend/cmd/server/main.go`. It loads `config.yaml` or `VIDEO_CONFIG`, creates the SQLite catalog and preview directories, builds the app state, registers API routes, starts the nightly runner, and then asynchronously attaches configured external drives so slow upstream login checks do not block port binding.

Important backend packages:

- `internal/config`: YAML config loading and first-run admin credential setup.
- `internal/catalog`: SQLite catalog, schema migration, video metadata, settings, tags, drive records, generation status, and deduplication state. It opens SQLite with WAL and a busy timeout.
- `internal/drives`: provider abstraction. Implementations include `quark`, `p115`, `pikpak`, `wopan`, `onedrive`, `localstorage`, `localupload`, and `spider91`.
- `internal/scanner`: recursively lists drive directories, parses filenames/tags, upserts catalog videos, applies skip-directory rules, and enqueues newly discovered videos.
- `internal/preview`: ffprobe/ffmpeg thumbnail and teaser generation workers. Generated assets are local files under the configured preview directory.
- `internal/fingerprint`: asynchronous sampled SHA-256 worker used for cross-drive duplicate detection.
- `internal/proxy`: `/p/*` media serving. Some providers redirect with `302` to signed CDN URLs, while providers requiring backend-held headers are reverse-proxied with Range support.
- `internal/api`: main API and admin API route handlers.
- `internal/nightly`: daily pipeline for drive scans, spider91 crawl, migration, queue drain, and duplicate asset cleanup.
- `internal/spider91migrate`: migration from spider91 downloads to a configured cloud drive.

### Runtime Flow

1. Admin adds or edits drives through `/admin/drives`, which persists drive config in the catalog.
2. The server attaches the drive implementation into the proxy registry and can trigger scans.
3. Scans convert provider files into catalog video rows, parse titles/authors/tags from filenames, and queue preview/fingerprint work.
4. The frontend lists videos through `/api/home`, `/api/list`, `/api/video/:id`, and streams media through `/p/*` endpoints.
5. The nightly runner performs the scheduled end-to-end maintenance pipeline; admins can trigger it manually through `/admin/api/jobs/nightly/run`.

### Configuration and Data

Backend defaults come from `backend/config.example.yaml`. On first backend start, `config.yaml` is created automatically if missing. Default local development paths are:

- Backend listen address: `127.0.0.1:9192`
- SQLite DB: `backend/data/video-site.db`
- Generated previews/thumbs: `backend/data/previews`

Docker and installer deployments rewrite config paths so data lives under `/opt/video-site-91/data` or the mounted `./data` directory.

`VIDEO_FRONTEND_DIR` controls where the Go server looks for built frontend assets. If unset, it serves `./dist` when present. Backend routes (`/api`, `/admin/api`, `/p`) are excluded from the SPA fallback.

## Notes for Changes

- Main-site API routes and proxy routes require authentication; only login/setup and `/api/settings/theme` are intentionally public.
- When adding a new drive provider, implement `internal/drives.Drive`, persist any needed config through catalog/admin APIs, attach it in `cmd/server`, and decide whether `/p/stream` should redirect or reverse-proxy in `internal/proxy`.
- Generated thumbnails and teasers are local runtime assets; do not treat them as source files.
- Frontend tests use Node's built-in test runner with `tsx`; TypeScript linting only checks `src` through the root `tsconfig.json`.
