# Multi-View Preview — Design Spec

## Overview
A multi-camera preview grid page with selectable layouts (1/2/4/8/16/32), drag-and-drop stream assignment from a tree-grouped sidebar, and live WebRTC WHEP playback in each cell.

---

## Page Layout

```
┌──────────────────────────────────────────────┐
│  [1] [2] [4] [8] [16] [32]   预览控制台       │
├───────────┬──────────────────────────────────┤
│ 信号源     │  ┌──────┬──────┬──────┐          │
│           │  │ 大门  │ 后门  │ 空   │          │
│ 摄像头A   │  ├──────┼──────┼──────┤          │
│  ├ 大门   │  │ 客厅  │ 空   │ 空   │          │
│  ├ 后门   │  └──────┴──────┴──────┘          │
│ 摄像头B   │                                   │
│  ├ 室内   │   拖拽信号源到格子即可预览          │
│  │ ├ 客厅 │                                   │
│  │ └ 卧室 │                                   │
│  └ 室外   │                                   │
└───────────┴──────────────────────────────────┘
```

---

## Components

### useWebRTC(streamId, mediamtxUrl, maxRetries)
Extracted hook from existing `StreamPreview.tsx`.
- Creates RTCPeerConnection with STUN server
- POST SDP offer to `{mediamtxUrl}/lr/{streamId}/whep`
- Retry loop (1s interval, maxRetries times)
- Returns: `{ status: string, videoRef: RefObject<HTMLVideoElement> }`
- Cleanup: closes PC on unmount

### PreviewCell
- Accepts: `streamId?`, `streamName?`, `mediamtxUrl`, `maxRetries`, `onDrop`, `onRemove`
- If `streamId` set: shows video via `useWebRTC`, calls `POST /api/streams/:id/preview` on assign, `DELETE` on remove
- Drop target for HTML5 DnD
- Shows stream name overlay + connection status
- Empty state: dashed border + "拖拽信号源到此" placeholder

### PreviewPage
Main page at `/preview` route.

**State:**
- `mode: 1 | 2 | 4 | 8 | 16 | 32` — current grid layout
- `cells: CellState[]` — array of cell assignments, length = mode
- `CellState = { streamId?: number; streamName?: string }`
- `streams: Stream[]` — all streams loaded from API
- `groups: Group[]` — all groups loaded for tree sidebar
- `mediamtxUrl`, `maxRetries` — loaded from config

**Layout switching:**
- On mode change, `cells` array is resized
- Extra cells (beyond new mode) are **saved** in a separate map
- Switching to a larger mode restores previously saved assignments
- This ensures stream assignments persist across layout changes

**Cleanup:**
- On unmount, stops all active previews in cells via API calls

---

## Grid Layouts

| Mode | Columns | Rows |
|------|---------|------|
| 1    | 1       | 1    |
| 2    | 2       | 1    |
| 4    | 2       | 2    |
| 8    | 4       | 2    |
| 16   | 4       | 4    |
| 32   | 8       | 4    |

Rendered as CSS Grid: `grid-template-columns: repeat(N, 1fr)`

---

## Stream Sidebar

- Left panel (~240px wide), scrollable
- Streams loaded from API, grouped by buildTree(groups)
- Each group header shown with group name
- Stream items indented under their group
- Each item: `draggable`, drag data carries `streamId` + `streamName`
- Search/filter input at top (optional, for UX)

---

## Preview Lifecycle

On cell assignment change:
1. If old streamId exists → `DELETE /api/streams/:oldId/preview`
2. If new streamId exists → `POST /api/streams/:newId/preview`
3. WebRTC connection starts automatically via hook

On page unmount:
- Iterate all assigned cells → `DELETE /api/streams/:id/preview` for each

---

## Backend Changes
None required. Existing `POST/DELETE /api/streams/:id/preview` endpoints + MediaMTX integration are sufficient.

## Frontend Changes Summary

| File | Status | Description |
|------|--------|-------------|
| `src/hooks/useWebRTC.ts` | Create | Extract WHEP connection logic from StreamPreview |
| `src/components/PreviewCell.tsx` | Create | Single grid cell with video + DnD |
| `src/components/PreviewSidebar.tsx` | Create | Stream list panel grouped by tree |
| `src/pages/Preview.tsx` | Create | Main multi-view page |
| `src/App.tsx` | Modify | Add `/preview` route |
| `src/components/Layout.tsx` | Modify | Add nav link |
| `src/components/StreamPreview.tsx` | Refactor | Use shared `useWebRTC` hook |
