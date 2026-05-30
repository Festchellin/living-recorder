# Preview Quality Control Design

## Overview

Allow users to control preview stream quality per-cell: preset quick-switch (流畅/画质) + expandable custom parameters (resolution, fps, CRF).

## Preset Values

| Parameter | 流畅 (Smooth) | 画质 (Quality) |
|-----------|--------------|----------------|
| Resolution | 360p (640×360) | 720p (1280×720) |
| Framerate | 10 | 25 |
| CRF | 35 | 23 |
| Encoder preset | `ultrafast` / HW speed | `veryfast` / HW balanced |

## Custom Parameters (Advanced)

Three dropdowns, override presets when set:

- **Resolution**: 360p / 480p / 720p / 1080p (source)
- **Framerate**: 5 / 10 / 15 / 25 / 30
- **Quality**: Low (CRF 35) / Medium (CRF 28) / High (CRF 23) / Highest (CRF 18)

## Backend

### API

WebSocket URL accepts query parameters:

```
GET /api/preview/{id}/ws?width=1280&height=720&fps=25&crf=23
```

- All params optional, defaults: `width=640, height=360, fps=10, crf=35` (current hardcoded values)
- When ffmpeg receives `crf=0` (highest quality), use lossless. Disabled in presets.

### PreviewWS Changes

1. Parse query params `width`, `height`, `fps`, `crf` from `c.Request.URL.Query()`
2. Replace hardcoded `scale=-2:360` → `scale={width}:{height}` (maintain aspect ratio)
3. Replace hardcoded `-r 10` → `-r {fps}`
4. Replace hardcoded `-crf 35` → `-crf {crf}` (software only; HW encoders use `-global_quality` or `-qp`)
5. Hardware encoder paths: adjust QP/quality equivalent per encoder

### Encoder-specific Quality Mapping

| Encoder | Quality param | CRF 18 | CRF 23 | CRF 28 | CRF 35 |
|---------|-------------|--------|--------|--------|--------|
| libx264 | `-crf` | 18 | 23 | 28 | 35 |
| h264_vaapi | `-qp` | 18 | 23 | 28 | 35 |
| h264_nvenc | `-qp` | 18 | 23 | 28 | 35 |
| h264_qsv | `-global_quality` | 18 | 23 | 28 | 35 |
| h264_amf | `-qp_i` / `-qp_p` | 18 | 23 | 28 | 35 |
| h264_videotoolbox | `-quality` | (not linear, skip) | | | |

For videotoolbox: ignore custom CRF, use `-quality` based on preset only.

## Frontend

### Components

**`PreviewCell`** — add gear icon button, toggles popover:

```
┌─────────────────────────┐
│ 预览画质                  │
│                          │
│  [ 流畅 ]  [ 画质 ]       │
│                          │
│  ▼ 高级设置               │
│  分辨率: [360p       ▼]  │
│  帧率:   [10 fps     ▼]  │
│  画质:   [中         ▼]  │
└─────────────────────────┘
```

- Preset buttons: two-state toggle, mutually exclusive
- Advanced section: collapsible, three `<select>` elements
- Selecting a custom value automatically switches to "自定义" preset state
- Swapping preset resets custom values to preset defaults

**New file**: `PreviewQualityPopover.tsx` — the popover content component.

### Hook Changes

`useMpegtsPreview(streamId, config?)` — new optional `config` param:

```typescript
interface PreviewConfig {
  width: number
  height: number
  fps: number
  crf: number
}
```

- Config change → disconnect old WS, connect new WS with updated query params
- Internal state holds config per cell
- Config stored as React state in `PreviewCell`, not persisted

### UX Behavior

- Clicking "流畅" or "画质" preset resets all params to that preset's defaults
- Changing any custom dropdown unselects both presets (implicit "自定义" state)
- User can click a preset again at any time to override custom values
- Popover closes on click outside or Escape key; settings remain applied while cell is connected

### State Management

- Each `PreviewCell` manages its own `previewConfig` state
- No global state or URL sync
- Config resets on page refresh (same as cell layout)

## Files Changed

| File | Change |
|------|--------|
| `backend/handlers/stream.go` | Parse query params, dynamic ffmpeg args |
| `frontend/src/hooks/useMpegts.ts` | Accept `PreviewConfig`, append to WS URL |
| `frontend/src/components/PreviewCell.tsx` | Gear icon, popover trigger |
| `frontend/src/components/PreviewQualityPopover.tsx` | New: preset + custom controls |
