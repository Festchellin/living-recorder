# Living Recorder — Agent Instructions

## Quick start
```sh
docker compose up -d --build                                 # full stack
cd frontend && npm install && npm run dev                     # frontend dev (Vite)
cd backend && go run .                                        # backend dev (Gin :8080)
```

## Build
```sh
./build.sh                       # native Linux amd64 (system ffmpeg)
./build.sh embed linux           # Linux amd64 with embedded ffmpeg (-tags embedffmpeg)
./build.sh embed all             # all platforms + embedded ffmpeg
```
- Version auto-detected from git tag, override with `VERSION=1.2.3 ./build.sh`.
- Frontend dist is copied to `backend/embed/dist/` and served via Go `embed.FS`.
- Build with embedded ffmpeg requires pre-downloaded binaries in `backend/embed/ffmpeg/`.

## Testing
```sh
cd backend && go test ./handlers/ -v -count=1
```
- Only backend tests exist (in-memory SQLite, pure Go, no CGO needed).
- No frontend test framework is set up.

## Lint / Typecheck
```sh
cd frontend && npm run lint       # eslint . --ext ts,tsx --max-warnings 0
cd frontend && npx tsc --noEmit   # TypeScript typecheck
```

## Architecture
- **Backend**: Go 1.25, Gin, GORM + SQLite (pure Go driver, no CGO), Viper config.
- **Frontend**: React 18 + TypeScript + Tailwind CSS + Vite. Path alias `@/` → `./src/`.
- **Video**: FFmpeg spawning for recording; mpegts.js + WebSocket (MPEG-TS) for live preview.
- **Storage**: Local FS or S3-compatible (minio-go). Switch via `storage.default` in config.
- The backend embeds the built frontend SPA via `//go:embed all:embed/dist`.
- Service singletons: RecorderService, SchedulerService, MonitorService, LogWriter.

## API pattern
All under `/api`, response shape `{"code": 0, "data": ...}`. SPA fallback: non-API 404 routes serve `index.html`.

## Key directories
| Path | Purpose |
|------|---------|
| `backend/handlers/` | HTTP handlers (Gin context) |
| `backend/services/` | Core logic (recorder, scheduler, monitor, storage, logger) |
| `backend/models/` | GORM models (Stream, Group, RecordTask, RecordLog) |
| `frontend/src/pages/` | Page components (7 routes) |
| `frontend/src/components/` | Shared UI components |
| `frontend/src/hooks/` | Custom React hooks |

## Design system
Glass/iridescent dark theme via CSS variables + Tailwind. shadcn/ui pattern (Radix primitives, CVA, clsx, tailwind-merge, lucide-react). Animations: shimmer, float, morph, pulse-soft, gradient.

## Style notes
- Chinese-locale project (UI, README, docs all in Chinese).
- Frontend: `strict: true` TypeScript, `noUnusedLocals/Parameters` on.
- Backend: `CGO_ENABLED=0` for cross-compilation; build tags `embedffmpeg` toggle embedded FFmpeg extraction.
- SQLite uses WAL + synchronous=NORMAL for performance on slow disks.
