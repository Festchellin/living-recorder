# Simplify Recording Flow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the "Recording Task" concept, support one-click start/stop recording without pre-configuration.

**Architecture:** Modify `RecorderService.Start()` to accept nil task (use config defaults). Add `StartAll()`/`StopAll()` to RecorderService and corresponding HTTP handlers. Remove Tasks page from frontend navigation and routes.

**Tech Stack:** Go 1.25 + Gin + GORM (backend), React 18 + react-router-dom (frontend)

---

### Task 1: Add default recording params to config

**Files:**
- Modify: `backend/config/config.go:51-56`
- Modify: `backend/config/config.yaml:23-26`

- [ ] **Step 1: Add new fields to RecorderConfig struct**

In `backend/config/config.go`, add default recording parameter fields to `RecorderConfig`:

```go
type RecorderConfig struct {
	MaxParallel          int    `mapstructure:"max_parallel"`
	RestartOnFailure     int    `mapstructure:"restart_on_failure"`
	HealthCheckInterval  int    `mapstructure:"health_check_interval"`
	DefaultVideoCodec    string `mapstructure:"default_video_codec"`
	DefaultAudioCodec    string `mapstructure:"default_audio_codec"`
	DefaultOutputTemplate string `mapstructure:"default_output_template"`
	StorageLocalPath     string // injected at runtime from storage.local.path
}
```

- [ ] **Step 2: Add defaults in setDefaults()**

```go
v.SetDefault("recorder.default_video_codec", "copy")
v.SetDefault("recorder.default_audio_codec", "copy")
v.SetDefault("recorder.default_output_template", "{name}/{date}_{time}.mp4")
```

- [ ] **Step 3: Update config.yaml**

```yaml
recorder:
  max_parallel: 10
  restart_on_failure: 3
  health_check_interval: 30
  default_video_codec: "copy"
  default_audio_codec: "copy"
  default_output_template: "{name}/{date}_{time}.mp4"
```

- [ ] **Step 4: Verify build**

Run: `go build ./...` from `backend/`
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add backend/config/config.go backend/config/config.yaml
git commit -m "feat: add default recording params to config"
```

---

### Task 2: Modify RecorderService to support taskless start

**Files:**
- Modify: `backend/services/recorder.go`

- [ ] **Step 1: Modify Start() to accept nil task**

Change the `Start()` method so that when `task` is nil, it builds a default `RecordTask` using `s.cfg`:

```go
func (s *RecorderService) Start(streamID uint, task *models.RecordTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.streams[streamID]; exists {
		return fmt.Errorf("stream %d is already recording", streamID)
	}

	if len(s.streams) >= s.cfg.MaxParallel {
		return fmt.Errorf("max parallel recordings reached (%d)", s.cfg.MaxParallel)
	}

	var stream models.Stream
	if err := s.db.First(&stream, streamID).Error; err != nil {
		return fmt.Errorf("stream not found: %w", err)
	}

	if task == nil {
		task = &models.RecordTask{
			StreamID:       streamID,
			OutputTemplate: s.cfg.DefaultOutputTemplate,
			VideoCodec:     s.cfg.DefaultVideoCodec,
			AudioCodec:     s.cfg.DefaultAudioCodec,
			StorageType:    "local",
		}
	}

	args := s.buildFFmpegArgs(&stream, task)
	// ... rest stays the same
```

Make sure the `task.ID` is 0 for default tasks (it will be used in `watchProcess` to create a `RecordLog` — default tasks have `TaskID: 0` which is fine).

- [ ] **Step 2: Verify build**

Run: `go build ./...` from `backend/`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add backend/services/recorder.go
git commit -m "feat: support taskless start with config defaults"
```

---

### Task 3: Add StartAll / StopAll to RecorderService

**Files:**
- Modify: `backend/services/recorder.go`

- [ ] **Step 1: Add StartAll() method**

```go
type BatchResult struct {
	Success int      `json:"success"`
	Errors  []string `json:"errors,omitempty"`
}

func (s *RecorderService) StartAll() BatchResult {
	var streams []models.Stream
	s.db.Where("enabled = ?", true).Find(&streams)

	result := BatchResult{}
	for _, stream := range streams {
		if s.IsRecording(stream.ID) {
			continue
		}
		if err := s.Start(stream.ID, nil); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", stream.Name, err))
			continue
		}
		result.Success++
	}
	return result
}
```

- [ ] **Step 2: Add StopAll() method**

```go
func (s *RecorderService) StopAll() BatchResult {
	s.mu.RLock()
	active := make([]uint, 0, len(s.streams))
	for id := range s.streams {
		active = append(active, id)
	}
	s.mu.RUnlock()

	result := BatchResult{}
	for _, id := range active {
		if err := s.Stop(id); err != nil {
			// get stream name for error message
			var stream models.Stream
			if s.db.First(&stream, id).Error == nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", stream.Name, err))
			} else {
				result.Errors = append(result.Errors, fmt.Sprintf("stream %d: %v", id, err))
			}
			continue
		}
		result.Success++
	}
	return result
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./...` from `backend/`
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add backend/services/recorder.go
git commit -m "feat: add StartAll and StopAll to RecorderService"
```

---

### Task 4: Add StartAll / StopAll HTTP handlers and routes

**Files:**
- Modify: `backend/handlers/stream.go`
- Modify: `backend/routes/routes.go`

- [ ] **Step 1: Add handlers in stream.go**

```go
func (h *StreamHandler) StartAll(c *gin.Context) {
	result := h.recorder.StartAll()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

func (h *StreamHandler) StopAll(c *gin.Context) {
	result := h.recorder.StopAll()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}
```

- [ ] **Step 2: Modify Start() handler to support taskless start**

Change the Start handler to not require a task:

```go
func (h *StreamHandler) Start(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var task models.RecordTask
	if err := h.db.Where("stream_id = ? AND enabled = ?", id, true).First(&task).Error; err != nil {
		// No task configured, start with defaults (pass nil)
		if err := h.recorder.Start(uint(id), nil); err != nil {
			c.JSON(http.StatusConflict, gin.H{"code": 1, "message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "recording started (defaults)"})
		return
	}
	if err := h.recorder.Start(uint(id), &task); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "recording started"})
}
```

- [ ] **Step 3: Add routes**

In `backend/routes/routes.go`, add after the existing stream routes:

```go
api.POST("/streams/start-all", streamHandler.StartAll)
api.POST("/streams/stop-all", streamHandler.StopAll)
```

- [ ] **Step 4: Verify build**

Run: `go build ./...` from `backend/`
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add backend/handlers/stream.go backend/routes/routes.go
git commit -m "feat: add start-all/stop-all API endpoints"
```

---

### Task 5: Remove Tasks page from frontend navigation

**Files:**
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/components/Layout.tsx`

- [ ] **Step 1: Remove Tasks import and route from App.tsx**

Remove the `Tasks` import line and the `/tasks` Route:

```tsx
import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { Layout } from '@/components/Layout'
import Dashboard from '@/pages/Dashboard'
import Streams from '@/pages/Streams'
import StreamDetail from '@/pages/StreamDetail'
import Settings from '@/pages/Settings'

export default function App() {
  return (
    <BrowserRouter>
      <Layout>
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/streams" element={<Streams />} />
          <Route path="/streams/:id" element={<StreamDetail />} />
          <Route path="/settings" element={<Settings />} />
        </Routes>
      </Layout>
    </BrowserRouter>
  )
}
```

- [ ] **Step 2: Remove Tasks nav item from Layout.tsx**

```tsx
import { Link, useLocation } from 'react-router-dom'
import { Button } from '@/components/ui/button'

const navItems = [
  { path: '/', label: 'Dashboard', icon: '📊' },
  { path: '/streams', label: 'Streams', icon: '📡' },
  { path: '/settings', label: 'Settings', icon: '⚙️' },
]
```

- [ ] **Step 3: Verify frontend builds**

Run: `npx tsc --noEmit` from `frontend/`
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add frontend/src/App.tsx frontend/src/components/Layout.tsx
git commit -m "feat: remove Tasks page from navigation"
```

---

### Task 6: Add Start All / Stop All buttons to Streams page

**Files:**
- Modify: `frontend/src/pages/Streams.tsx`
- Modify: `frontend/src/lib/api.ts`

- [ ] **Step 1: Add startAll/stopAll to api.ts**

```ts
startAll: () => request<{success: number; errors?: string[]}>('/api/streams/start-all', { method: 'POST' }),
stopAll: () => request<{success: number; errors?: string[]}>('/api/streams/stop-all', { method: 'POST' }),
```

- [ ] **Step 2: Add toolbar with One-Click buttons in Streams.tsx**

Modify the `Streams` component to add a toolbar row between the page title and the stream grid:

```tsx
import { Button } from '@/components/ui/button'

const handleStartAll = async () => {
  try {
    const result = await api.streams.startAll()
    console.log(`Started ${result.success} streams`)
    load()
  } catch (e) {
    console.error('Start all failed', e)
  }
}

const handleStopAll = async () => {
  try {
    const result = await api.streams.stopAll()
    console.log(`Stopped ${result.success} streams`)
    load()
  } catch (e) {
    console.error('Stop all failed', e)
  }
}

// Add to the JSX, after the title row and before the grid:
<div className="flex justify-between items-center">
  <h2 className="text-2xl font-bold">Streams</h2>
  <div className="flex gap-2">
    <Button variant="default" onClick={handleStartAll}>▶ Start All</Button>
    <Button variant="destructive" onClick={handleStopAll}>■ Stop All</Button>
    <Dialog open={open} onOpenChange={setOpen}>
      ...
    </Dialog>
  </div>
</div>
```

- [ ] **Step 3: Verify frontend builds**

Run: `npx tsc --noEmit` from `frontend/`
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add frontend/src/pages/Streams.tsx frontend/src/lib/api.ts
git commit -m "feat: add Start All / Stop All buttons to Streams page"
```

---

### Task 7: Rebuild binaries

**Files:**
- Build: `build/living-recorder-linux`, `build/living-recorder.exe`

- [ ] **Step 1: Build frontend**

```bash
cd frontend && npm run build && cp -r dist ../backend/embed/
```

- [ ] **Step 2: Cross-compile Linux and Windows**

```bash
cd backend
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ../build/living-recorder-linux .
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o ../build/living-recorder.exe .
```

- [ ] **Step 3: Verify binaries exist**

```bash
ls -lh ../build/living-recorder-linux ../build/living-recorder.exe
```

- [ ] **Step 4: Commit**

```bash
git add build/
git commit -m "build: rebuild binaries with simplified recording flow"
```
