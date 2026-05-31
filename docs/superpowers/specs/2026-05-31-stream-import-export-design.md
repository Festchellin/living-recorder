# Stream Import/Export Design

Date: 2026-05-31

## Summary

Add import/export functionality for stream (signal source) configurations, supporting JSON, CSV, Excel (.xlsx), and Text (JSON Lines) formats. The backend handles all file parsing and generation using Go; the frontend handles file selection and download.

## Data Model Changes

Add `Remark` field to `models.Stream`:

```
Stream {
  id         : uint
  name       : string     (required)
  url        : string     (required)
  protocol   : string     (required)  — rtsp / rtmp / flv / hls
  enabled    : bool       (default: true)
  status     : string     (runtime, not exported)
  remark     : string     (NEW, size:512)  — optional remark/description
  group_id   : *uint      (nullable FK)
  group      : *Group     (relation)
  created_at : time.Time
  updated_at : time.Time
}
```

## Group Path Handling

Groups form a self-referential hierarchy via `ParentID`. For import/export we use a forward-slash-delimited path string:

- Export: traverse from leaf group up to root via `ParentID`, join segments with `/`
  - Example: `ParentID=3 → "一楼"`, `ID=3.ParentID=1 → "东侧"` → path `"一楼/东侧"`
- Import: split path by `/`, find-or-create each segment sequentially under its parent
  - Example: `"一楼/东侧"` → find-or-create root group "一楼" → under it, find-or-create "东侧"
  - Stream is associated with the leaf group

## API Endpoints

### POST /api/streams/export

Export selected streams in the requested format.

Request (JSON body):
```json
{
  "format": "json" | "csv" | "xlsx" | "txt",
  "ids": [1, 2, 3]        // optional; omit or empty = export all
}
```

Response: file download.
- Content-Type:
  - json: `application/json`
  - csv: `text/csv`
  - xlsx: `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`
  - txt: `text/plain`
- Content-Disposition: `attachment; filename="streams-{format}-{timestamp}.{ext}"`

Fields exported in all formats (same mapping):
| Field        | Description                |
|--------------|----------------------------|
| name         | Stream name                |
| url          | Stream URL                 |
| protocol     | rtsp/rtmp/flv/hls          |
| enabled      | true/false                 |
| group_path   | e.g. "一楼/东侧" or empty  |
| remark       | Description/notes          |

### POST /api/streams/import

Import streams from an uploaded file.

Request: multipart/form-data with file field `"file"`.
- Accepted extensions: `.json`, `.csv`, `.xlsx`, `.txt`
- Max file size: ~10 MB (Gin default)

Response:
```json
{
  "success": 8,
  "skipped": 2,
  "errors": [
    { "line": 3, "message": "重复的 URL: rtsp://..." },
    { "line": 7, "message": "无效的协议: udp" }
  ]
}
```

Import rules:
- Each record is validated independently (name required, URL required, protocol must be one of rtsp/rtmp/flv/hls)
- If another stream with the same **name** or **URL** already exists → skip (counted in `skipped`)
- Group path → find-or-create group hierarchy
- `enabled` defaults to `true` if not specified

## Format Specifications

### JSON (.json)

Array of objects with the 6 fields. Example:
```json
[
  {
    "name": "摄像头A",
    "url": "rtsp://192.168.1.100:554/stream1",
    "protocol": "rtsp",
    "enabled": true,
    "group_path": "一楼/东侧",
    "remark": "主入口监控"
  }
]
```

### CSV (.csv)

Header row + data rows. Comma-separated, quoted if needed.

```csv
name,url,protocol,enabled,group_path,remark
摄像头A,rtsp://192.168.1.100:554/stream1,rtsp,true,一楼/东侧,主入口监控
```

### Excel (.xlsx)

Single sheet "Streams" with header row in bold. Same columns as CSV. Auto-sized column widths.

### Text / JSON Lines (.txt)

One JSON object per line (newline-delimited JSON).

```json
{"name":"摄像头A","url":"rtsp://192.168.1.100:554/stream1","protocol":"rtsp","enabled":true,"group_path":"一楼/东侧","remark":"主入口监控"}
{"name":"摄像头B","url":"rtsp://192.168.1.100:554/stream2","protocol":"rtsp","enabled":true,"group_path":"一楼/西侧","remark":""}
```

## Backend Implementation

### New file: `backend/handlers/stream_io.go`

- `Export(c *gin.Context)` — handler for POST /api/streams/export
- `Import(c *gin.Context)` — handler for POST /api/streams/import
- `exportJSON(streams []models.Stream, format string)` — returns io.Reader
- `exportCSV(streams []models.Stream)` — returns io.Reader
- `exportXLSX(streams []models.Stream)` — returns []byte
- `exportTXT(streams []models.Stream)` — returns io.Reader (JSON Lines)
- `parseImportFile(header *multipart.FileHeader)` — detect format, parse into []ImportStream
- `importStreams(imports []ImportStream)` — process each, return ImportResult

### Helper structs

```go
type ExportRequest struct {
    Format string `json:"format" binding:"required"`
    IDs    []uint `json:"ids"`    // empty = all
}

type ImportStream struct {
    Name      string `json:"name"`
    URL       string `json:"url"`
    Protocol  string `json:"protocol"`
    Enabled   bool   `json:"enabled"`
    GroupPath string `json:"group_path"`
    Remark    string `json:"remark"`
}

type ImportResult struct {
    Success int                `json:"success"`
    Skipped int                `json:"skipped"`
    Errors  []ImportError      `json:"errors,omitempty"`
}

type ImportError struct {
    Line    int    `json:"line"`
    Message string `json:"message"`
}
```

### Group path helpers

- `getGroupPath(db *gorm.DB, group *models.Group) string` — traverse parent chain, build "/"-separated path
- `findOrCreateGroupPath(db *gorm.DB, path string) (*models.Group, error)` — split by "/", find-or-create each level

### Routes registered in `backend/routes/routes.go`

```go
api.POST("/streams/export", streamHandler.Export)
api.POST("/streams/import", streamHandler.Import)
```

### New Go dependency

```
github.com/xuri/excelize/v2
```

## Frontend Implementation

### Type changes (`frontend/src/lib/api.ts`)

```typescript
export interface Stream {
  id: number
  name: string
  url: string
  protocol: string
  enabled: boolean
  status: string
  remark: string          // NEW
  group_id: number | null
  group?: Group
  created_at: string
  updated_at: string
}
```

### API client additions (`frontend/src/lib/api.ts`)

```typescript
export interface ImportResult {
  success: number
  skipped: number
  errors?: { line: number; message: string }[]
}

streams: {
  // ... existing methods ...

  export: (format: string, ids?: number[]) =>
    fetch('/api/streams/export', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ format, ids }),
    }).then(r => {
      const filename = (r.headers.get('Content-Disposition') || '').match(/filename="?(.+?)"?$/)?.[1] || `streams.${format}`
      return r.blob().then(blob => ({ blob, filename }))
    }),

  import: (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return request<ImportResult>('/api/streams/import', {
      method: 'POST',
      body: form,
    })
  },
}
```

### UI changes (`frontend/src/pages/Streams.tsx`)

**Top toolbar**: Add "导入" and "导出" buttons next to "Start All" / "Stop All".

**Checkboxes**: Add a checkbox column / per-stream-card checkbox for selection.

**Export dialog**: Format picker (JSON/CSV/XLSX/TXT) + "Export All" / "Export Selected" + confirm.

**Import flow**: Click "导入" → file picker (accept `.json,.csv,.xlsx,.txt`) → upload → show result in a result dialog with success/skipped/error counts.

**Create/Edit dialog**: Add a `remark` text field (optional, multiline input).

## Error Handling

- Export: 400 if format is not one of json/csv/xlsx/txt. 200 with empty array if no streams match.
- Import: 400 if format not recognized or file unparseable. Always return structured `ImportResult` on 200 (partial success is expected).
- Gin's `MaxMultipartMemory` default (32 MB) is sufficient; no explicit limit needed.

## File Summary

| File | Action |
|------|--------|
| `backend/models/stream.go` | Edit — add `Remark string` field |
| `backend/handlers/stream_io.go` | Create — Export/Import handlers & helpers |
| `backend/routes/routes.go` | Edit — register export/import routes |
| `backend/go.mod` | Edit — add excelize dependency |
| `frontend/src/lib/api.ts` | Edit — add Stream.remark, export/import methods |
| `frontend/src/pages/Streams.tsx` | Edit — add checkboxes, import/export buttons, dialogs |
