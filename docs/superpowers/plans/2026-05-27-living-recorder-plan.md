# Living Recorder Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a standalone web application for recording live streams (RTSP/RTMP/FLV/HLS) via FFmpeg, with multi-stream support, configurable recording parameters, scheduled recording, and multiple storage backends.

**Architecture:** Single Go binary with embedded React frontend. FFmpeg managed as subprocess. Storage via pluggable interface (local disk / S3-compatible). Scheduling via robfig/cron. Real-time status via WebSocket.

**Tech Stack:** Go 1.22 + Gin + GORM + SQLite, React 18 + Vite + shadcn/ui + Tailwind CSS, FFmpeg, gorilla/websocket, robfig/cron, minio-go

---

## File Structure

```
living-recorder/
├── backend/
│   ├── main.go                        # Entry point, wires everything
│   ├── go.mod
│   ├── config/
│   │   ├── config.go                  # Viper-based config loading
│   │   └── config.yaml                # Default config file
│   ├── database/
│   │   └── database.go                # GORM init + AutoMigrate
│   ├── models/
│   │   ├── stream.go                  # Stream model
│   │   ├── record_task.go             # RecordTask model
│   │   └── record_log.go              # RecordLog model
│   ├── services/
│   │   ├── recorder.go                # RecorderService — FFmpeg process mgmt
│   │   ├── scheduler.go               # SchedulerService — cron scheduling
│   │   ├── storage.go                 # StorageBackend interface + impls
│   │   └── monitor.go                 # MonitorService — health + WebSocket push
│   ├── handlers/
│   │   ├── stream.go                  # Stream CRUD handlers
│   │   ├── task.go                    # Task CRUD handlers
│   │   ├── status.go                  # Status/config handlers
│   │   └── websocket.go               # WebSocket upgrade handler
│   ├── routes/
│   │   └── routes.go                  # Route registration
│   └── embed/
│       └── dist/
│           └── .gitkeep
├── frontend/
│   ├── package.json
│   ├── vite.config.ts
│   ├── tsconfig.json
│   ├── tailwind.config.js
│   ├── postcss.config.js
│   ├── index.html
│   └── src/
│       ├── main.tsx
│       ├── App.tsx
│       ├── lib/
│       │   ├── api.ts                 # API client
│       │   └── ws.ts                  # WebSocket client
│       ├── hooks/
│       │   └── useWebSocket.ts
│       ├── components/
│       │   ├── ui/                    # shadcn components
│       │   ├── StreamCard.tsx
│       │   ├── StatusBadge.tsx
│       │   └── Layout.tsx
│       └── pages/
│           ├── Dashboard.tsx
│           ├── Streams.tsx
│           ├── StreamDetail.tsx
│           ├── Tasks.tsx
│           └── Settings.tsx
└── docs/
    └── superpowers/
        └── specs/
            └── 2026-05-27-living-recorder-design.md
```

---

### Task 1: Project scaffolding

**Files:**
- Create: `backend/go.mod`
- Create: `backend/config/config.yaml`
- Create: `backend/embed/dist/.gitkeep`

- [ ] **Step 1: Create project root and Go module**

```bash
mkdir -p living-recorder/backend/{config,database,models,services,handlers,routes,embed/dist}
mkdir -p living-recorder/frontend
cd living-recorder/backend && go mod init living-recorder/backend
```

- [ ] **Step 2: Add Go dependencies**

```bash
cd living-recorder/backend
go get github.com/gin-gonic/gin
go get gorm.io/gorm
go get gorm.io/driver/sqlite
go get github.com/spf13/viper
go get github.com/gorilla/websocket
go get github.com/robfig/cron/v3
go get github.com/minio/minio-go/v7
go get github.com/google/uuid
```

- [ ] **Step 3: Create default config.yaml**

```yaml
server:
  port: "8080"
  mode: debug

database:
  driver: sqlite
  path: ./data/living-recorder.db

ffmpeg:
  path: ffmpeg

storage:
  default: local
  local:
    path: ./recordings
  s3:
    endpoint: ""
    access_key: ""
    secret_key: ""
    bucket: ""
    region: ""

recorder:
  max_parallel: 10
  restart_on_failure: 3
  health_check_interval: 30
```

Write to `backend/config/config.yaml`

- [ ] **Step 4: Create embed dist placeholder**

```bash
touch living-recorder/backend/embed/dist/.gitkeep
```

- [ ] **Step 5: Commit**

```bash
cd living-recorder && git init && git add -A && git commit -m "chore: scaffold project structure"
```

---

### Task 2: Config loading

**Files:**
- Create: `backend/config/config.go`

- [ ] **Step 1: Implement config structs and Load()**

```go
package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	FFmpeg   FFmpegConfig   `mapstructure:"ffmpeg"`
	Storage  StorageConfig  `mapstructure:"storage"`
	Recorder RecorderConfig `mapstructure:"recorder"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	Driver string `mapstructure:"driver"`
	Path   string `mapstructure:"path"`
}

type FFmpegConfig struct {
	Path string `mapstructure:"path"`
}

type StorageConfig struct {
	Default string          `mapstructure:"default"`
	Local   LocalStorageConfig `mapstructure:"local"`
	S3      S3StorageConfig    `mapstructure:"s3"`
}

type LocalStorageConfig struct {
	Path string `mapstructure:"path"`
}

type S3StorageConfig struct {
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	Bucket    string `mapstructure:"bucket"`
	Region    string `mapstructure:"region"`
}

type RecorderConfig struct {
	MaxParallel          int    `mapstructure:"max_parallel"`
	RestartOnFailure     int    `mapstructure:"restart_on_failure"`
	HealthCheckInterval  int    `mapstructure:"health_check_interval"`
	StorageLocalPath     string // injected at runtime from storage.local.path
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")

	v.ReadInConfig()

	setDefaults(v)
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	os.MkdirAll(cfg.Storage.Local.Path, 0755)
	os.MkdirAll(filepath.Dir(cfg.Database.Path), 0755)

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.port", "8080")
	v.SetDefault("server.mode", "release")
	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.path", "./data/living-recorder.db")
	v.SetDefault("ffmpeg.path", "ffmpeg")
	v.SetDefault("storage.default", "local")
	v.SetDefault("storage.local.path", "./recordings")
	v.SetDefault("recorder.max_parallel", 10)
	v.SetDefault("recorder.restart_on_failure", 3)
	v.SetDefault("recorder.health_check_interval", 30)
}
```

- [ ] **Step 2: Commit**

```bash
cd living-recorder && git add backend/config/config.go && git commit -m "feat: add config loading with viper"
```

---

### Task 3: Database + Models

**Files:**
- Create: `backend/models/stream.go`
- Create: `backend/models/record_task.go`
- Create: `backend/models/record_log.go`
- Create: `backend/database/database.go`

- [ ] **Step 1: Create Stream model**

```go
package models

import "time"

type Stream struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:255;not null" json:"name"`
	URL       string    `gorm:"size:1024;not null" json:"url"`
	Protocol  string    `gorm:"size:32;not null" json:"protocol"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	Status    string    `gorm:"size:32;default:idle" json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
```

- [ ] **Step 2: Create RecordTask model**

```go
package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

type JSON json.RawMessage

func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return string(j), nil
}

func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		*j = nil
		return nil
	}
	*j = append((*j)[:0], bytes...)
	return nil
}

type RecordTask struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	StreamID        uint      `gorm:"not null;index" json:"stream_id"`
	Stream          Stream    `gorm:"foreignKey:StreamID" json:"stream,omitempty"`
	OutputTemplate  string    `gorm:"size:512;not null;default:'{name}/{date}_{time}.mp4'" json:"output_template"`
	SegmentSec      int       `gorm:"default:0" json:"segment_sec"`
	VideoCodec      string    `gorm:"size:64;default:copy" json:"video_codec"`
	VideoBitrate    string    `gorm:"size:32" json:"video_bitrate"`
	Framerate       int       `gorm:"default:0" json:"framerate"`
	Resolution      string    `gorm:"size:32" json:"resolution"`
	AudioCodec      string    `gorm:"size:64;default:copy" json:"audio_codec"`
	AudioBitrate    string    `gorm:"size:32" json:"audio_bitrate"`
	StorageType     string    `gorm:"size:32;default:local" json:"storage_type"`
	StorageConfig   JSON      `gorm:"type:text" json:"storage_config,omitempty"`
	ScheduleCron    string    `gorm:"size:128" json:"schedule_cron"`
	ScheduleDuration int      `gorm:"default:0" json:"schedule_duration"`
	LoopRecord      bool      `gorm:"default:false" json:"loop_record"`
	Enabled         bool      `gorm:"default:true" json:"enabled"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
```

- [ ] **Step 3: Create RecordLog model**

```go
package models

import "time"

type RecordLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	StreamID  uint      `gorm:"not null;index" json:"stream_id"`
	Stream    Stream    `gorm:"foreignKey:StreamID" json:"stream,omitempty"`
	FilePath  string    `gorm:"size:1024" json:"file_path"`
	FileSize  int64     `gorm:"default:0" json:"file_size"`
	Duration  int       `gorm:"default:0" json:"duration"`
	Status    string    `gorm:"size:32;not null" json:"status"`
	ErrorMsg  string    `gorm:"size:1024" json:"error_msg"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
}
```

- [ ] **Step 4: Create database.go**

```go
package database

import (
	"living-recorder/backend/config"
	"living-recorder/backend/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func Init(cfg config.DatabaseConfig) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(cfg.Path), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	db.AutoMigrate(
		&models.Stream{},
		&models.RecordTask{},
		&models.RecordLog{},
	)

	return db, nil
}
```

- [ ] **Step 5: Verify build**

```bash
cd living-recorder/backend && go build ./...
```

- [ ] **Step 6: Commit**

```bash
cd living-recorder && git add backend/models backend/database && git commit -m "feat: add database and models"
```

---

### Task 4: Storage backend

**Files:**
- Create: `backend/services/storage.go`

- [ ] **Step 1: Define StorageBackend interface and implementations**

```go
package services

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"living-recorder/backend/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/credentials"
)

type FileInfo struct {
	Path     string
	Size     int64
	IsDir    bool
}

type StorageBackend interface {
	Save(ctx context.Context, srcPath, destPath string) error
	Delete(ctx context.Context, path string) error
	List(ctx context.Context, prefix string) ([]FileInfo, error)
	GetURL(ctx context.Context, path string) (string, error)
}

type LocalStorage struct {
	basePath string
}

func NewLocalStorage(cfg config.LocalStorageConfig) *LocalStorage {
	return &LocalStorage{basePath: cfg.Path}
}

func (s *LocalStorage) Save(_ context.Context, srcPath, destPath string) error {
	fullPath := filepath.Join(s.basePath, destPath)
	os.MkdirAll(filepath.Dir(fullPath), 0755)

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("create dest: %w", err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func (s *LocalStorage) Delete(_ context.Context, path string) error {
	return os.Remove(filepath.Join(s.basePath, path))
}

func (s *LocalStorage) List(_ context.Context, prefix string) ([]FileInfo, error) {
	entries, err := os.ReadDir(filepath.Join(s.basePath, prefix))
	if err != nil {
		return nil, err
	}
	var infos []FileInfo
	for _, e := range entries {
		info, _ := e.Info()
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		infos = append(infos, FileInfo{
			Path:  filepath.Join(prefix, e.Name()),
			Size:  size,
			IsDir: e.IsDir(),
		})
	}
	return infos, nil
}

func (s *LocalStorage) GetURL(_ context.Context, path string) (string, error) {
	return filepath.Join(s.basePath, path), nil
}

type S3Storage struct {
	client *minio.Client
	bucket string
}

func NewS3Storage(cfg config.S3StorageConfig) (*S3Storage, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, err
	}
	return &S3Storage{client: client, bucket: cfg.Bucket}, nil
}

func (s *S3Storage) Save(ctx context.Context, srcPath, destPath string) error {
	_, err := s.client.FPutObject(ctx, s.bucket, destPath, srcPath, minio.PutObjectOptions{})
	return err
}

func (s *S3Storage) Delete(ctx context.Context, path string) error {
	return s.client.RemoveObject(ctx, s.bucket, path, minio.RemoveObjectOptions{})
}

func (s *S3Storage) List(ctx context.Context, prefix string) ([]FileInfo, error) {
	objects := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: prefix})
	var infos []FileInfo
	for obj := range objects {
		infos = append(infos, FileInfo{
			Path:  obj.Key,
			Size:  obj.Size,
			IsDir: obj.Key[len(obj.Key)-1] == '/',
		})
	}
	return infos, nil
}

func (s *S3Storage) GetURL(ctx context.Context, path string) (string, error) {
	u, err := s.client.PresignedGetObject(ctx, s.bucket, path, 86400, nil)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
```

- [ ] **Step 2: Verify build**

```bash
cd living-recorder/backend && go build ./...
```

- [ ] **Step 3: Commit**

```bash
cd living-recorder && git add backend/services/storage.go && git commit -m "feat: add storage backend interface with local and s3 implementations"
```

---

### Task 5: Recorder service

**Files:**
- Create: `backend/services/recorder.go`

- [ ] **Step 1: Implement RecorderService**

```go
package services

import (
	"context"
	"fmt"
	"living-recorder/backend/config"
	"living-recorder/backend/models"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

type StreamProcess struct {
	StreamID  uint
	TaskID    uint
	Cmd       *exec.Cmd
	Cancel    context.CancelFunc
	StartedAt time.Time
	Status    string
}

type RecorderService struct {
	db       *gorm.DB
	cfg      config.RecorderConfig
	ffmpeg   string
	store    StorageBackend
	mu       sync.RWMutex
	streams  map[uint]*StreamProcess
}

func NewRecorderService(db *gorm.DB, cfg config.RecorderConfig, ffmpegPath string, store StorageBackend) *RecorderService {
	return &RecorderService{
		db:      db,
		cfg:     cfg,
		ffmpeg:  ffmpegPath,
		store:   store,
		streams: make(map[uint]*StreamProcess),
	}
}

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

	args := s.buildFFmpegArgs(&stream, task)
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, s.ffmpeg, args...)

	sp := &StreamProcess{
		StreamID:  streamID,
		TaskID:    task.ID,
		Cmd:       cmd,
		Cancel:    cancel,
		StartedAt: time.Now(),
		Status:    "recording",
	}
	s.streams[streamID] = sp

	s.db.Model(&stream).Update("status", "recording")

	go s.watchProcess(sp)
	go cmd.Run()

	return nil
}

func (s *RecorderService) Stop(streamID uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sp, exists := s.streams[streamID]
	if !exists {
		return fmt.Errorf("stream %d is not recording", streamID)
	}

	sp.Cancel()
	sp.Status = "stopping"

	return nil
}

func (s *RecorderService) IsRecording(streamID uint) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.streams[streamID]
	return exists
}

func (s *RecorderService) GetActiveStreams() []*StreamProcess {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*StreamProcess, 0, len(s.streams))
	for _, sp := range s.streams {
		result = append(result, sp)
	}
	return result
}

func (s *RecorderService) GetProcess(streamID uint) *StreamProcess {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.streams[streamID]
}

func (s *RecorderService) buildFFmpegArgs(stream *models.Stream, task *models.RecordTask) []string {
	args := []string{}

	switch stream.Protocol {
	case "rtsp":
		args = append(args, "-rtsp_transport", "tcp")
	case "rtmp", "flv":
	case "hls":
	default:
	}

	args = append(args, "-i", stream.URL)
	args = append(args, "-c:v", task.VideoCodec)

	if task.VideoBitrate != "" {
		args = append(args, "-b:v", task.VideoBitrate)
	}
	if task.Framerate > 0 {
		args = append(args, "-r", fmt.Sprintf("%d", task.Framerate))
	}
	if task.Resolution != "" {
		args = append(args, "-s", task.Resolution)
	}

	args = append(args, "-c:a", task.AudioCodec)
	if task.AudioBitrate != "" {
		args = append(args, "-b:a", task.AudioBitrate)
	}

	if task.SegmentSec > 0 {
		args = append(args, "-f", "segment")
		args = append(args, "-segment_time", fmt.Sprintf("%d", task.SegmentSec))
		args = append(args, "-reset_timestamps", "1")
		args = append(args, "-strftime", "1")
	}

	outputPath := s.buildOutputPath(stream, task)
	args = append(args, "-y", outputPath)

	return args
}

func (s *RecorderService) buildOutputPath(stream *models.Stream, task *models.RecordTask) string {
	tpl := task.OutputTemplate
	now := time.Now()
	name := stream.Name

	tpl = strings.ReplaceAll(tpl, "{name}", name)
	tpl = strings.ReplaceAll(tpl, "{date}", now.Format("20060102"))
	tpl = strings.ReplaceAll(tpl, "{time}", now.Format("150405"))
	tpl = strings.ReplaceAll(tpl, "{protocol}", stream.Protocol)
	tpl = strings.ReplaceAll(tpl, "{id}", fmt.Sprintf("%d", stream.ID))

	return filepath.Join(s.cfg.StorageLocalPath, tpl)
}

func (s *RecorderService) watchProcess(sp *StreamProcess) {
	err := sp.Cmd.Wait()

	s.mu.Lock()
	delete(s.streams, sp.StreamID)
	s.mu.Unlock()

	s.db.Model(&models.Stream{}).Where("id = ?", sp.StreamID).Update("status", "idle")

	log := models.RecordLog{
		StreamID:  sp.StreamID,
		Status:    "success",
		StartedAt: sp.StartedAt,
		EndedAt:   time.Now(),
	}
	if err != nil {
		if sp.Status == "stopping" {
			log.Status = "success"
		} else {
			log.Status = "failed"
			log.ErrorMsg = err.Error()
		}
	}
	s.db.Create(&log)
}
```

- [ ] **Step 2: Verify build**

```bash
cd living-recorder/backend && go build ./...
```

- [ ] **Step 3: Commit**

```bash
cd living-recorder && git add backend/services/recorder.go && git commit -m "feat: add recorder service with ffmpeg process management"
```

---

### Task 6: Scheduler service

**Files:**
- Create: `backend/services/scheduler.go`

- [ ] **Step 1: Implement SchedulerService**

```go
package services

import (
	"living-recorder/backend/models"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

type SchedulerService struct {
	db       *gorm.DB
	cron     *cron.Cron
	recorder *RecorderService
	tasks    map[uint]cron.EntryID
}

func NewSchedulerService(db *gorm.DB, recorder *RecorderService) *SchedulerService {
	return &SchedulerService{
		db:       db,
		cron:     cron.New(cron.WithSeconds()),
		recorder: recorder,
		tasks:    make(map[uint]cron.EntryID),
	}
}

func (s *SchedulerService) Start() {
	s.cron.Start()
	s.loadTasks()
}

func (s *SchedulerService) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
}

func (s *SchedulerService) loadTasks() {
	var tasks []models.RecordTask
	s.db.Where("enabled = ? AND schedule_cron != ''", true).Preload("Stream").Find(&tasks)
	for _, task := range tasks {
		s.AddTask(&task)
	}
}

func (s *SchedulerService) AddTask(task *models.RecordTask) error {
	if task.ScheduleCron == "" {
		return nil
	}

	taskID := task.ID
	duration := task.ScheduleDuration

	entryID, err := s.cron.AddFunc(task.ScheduleCron, func() {
		s.recorder.Start(task.StreamID, task)
		if duration > 0 {
			go func() {
				time.Sleep(time.Duration(duration) * time.Second)
				s.recorder.Stop(task.StreamID)
			}()
		}
	})
	if err != nil {
		return err
	}

	s.tasks[taskID] = entryID
	return nil
}

func (s *SchedulerService) RemoveTask(taskID uint) {
	if entryID, ok := s.tasks[taskID]; ok {
		s.cron.Remove(entryID)
		delete(s.tasks, taskID)
	}
}

func (s *SchedulerService) ReloadTask(task *models.RecordTask) {
	s.RemoveTask(task.ID)
	if task.Enabled && task.ScheduleCron != "" {
		s.AddTask(task)
	}
}
```

- [ ] **Step 2: Fix import (add `"time"`)**

- [ ] **Step 3: Verify build**

```bash
cd living-recorder/backend && go build ./...
```

- [ ] **Step 4: Commit**

```bash
cd living-recorder && git add backend/services/scheduler.go && git commit -m "feat: add scheduler service with cron support"
```

---

### Task 7: Monitor + WebSocket service

**Files:**
- Create: `backend/services/monitor.go`

- [ ] **Step 1: Implement MonitorService**

```go
package services

import (
	"encoding/json"
	"living-recorder/backend/config"
	"living-recorder/backend/models"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

type MonitorService struct {
	db       *gorm.DB
	recorder *RecorderService
	cfg      config.RecorderConfig
	clients  map[*websocket.Conn]bool
	mu       sync.RWMutex
}

type StatusMessage struct {
	Type            string                   `json:"type"`
	ActiveStreams   []*StreamProcess         `json:"active_streams,omitempty"`
	StreamStatus    *StreamStatusInfo        `json:"stream_status,omitempty"`
	StorageUsed     int64                    `json:"storage_used,omitempty"`
}

type StreamStatusInfo struct {
	StreamID   uint   `json:"stream_id"`
	Status     string `json:"status"`
	Duration   int    `json:"duration_sec"`
	FileSize   int64  `json:"file_size"`
}

func NewMonitorService(db *gorm.DB, recorder *RecorderService, cfg config.RecorderConfig) *MonitorService {
	return &MonitorService{
		db:       db,
		recorder: recorder,
		cfg:      cfg,
		clients:  make(map[*websocket.Conn]bool),
	}
}

func (s *MonitorService) AddClient(conn *websocket.Conn) {
	s.mu.Lock()
	s.clients[conn] = true
	s.mu.Unlock()
}

func (s *MonitorService) RemoveClient(conn *websocket.Conn) {
	s.mu.Lock()
	delete(s.clients, conn)
	s.mu.Unlock()
	conn.Close()
}

func (s *MonitorService) broadcast(msg StatusMessage) {
	data, _ := json.Marshal(msg)
	s.mu.RLock()
	defer s.mu.RUnlock()
	for client := range s.clients {
		client.WriteMessage(websocket.TextMessage, data)
	}
}

func (s *MonitorService) Start() {
	go func() {
		ticker := time.NewTicker(time.Duration(s.cfg.HealthCheckInterval) * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			s.collectAndBroadcast()
		}
	}()
}

func (s *MonitorService) collectAndBroadcast() {
	active := s.recorder.GetActiveStreams()

	var totalSize int64
	s.db.Model(&models.RecordLog{}).Select("COALESCE(SUM(file_size), 0)").Scan(&totalSize)

	msg := StatusMessage{
		Type:          "status",
		ActiveStreams: active,
		StorageUsed:   totalSize,
	}
	s.broadcast(msg)
}

func (s *MonitorService) NotifyStreamChange(streamID uint) {
	sp := s.recorder.GetProcess(streamID)
	info := &StreamStatusInfo{StreamID: streamID}
	if sp != nil {
		info.Status = sp.Status
		info.Duration = int(time.Since(sp.StartedAt).Seconds())
	}
	s.broadcast(StatusMessage{
		Type:         "stream_update",
		StreamStatus: info,
	})
}
```

- [ ] **Step 2: Verify build**

```bash
cd living-recorder/backend && go build ./...
```

- [ ] **Step 3: Commit**

```bash
cd living-recorder && git add backend/services/monitor.go && git commit -m "feat: add monitor service with websocket status push"
```

---

### Task 8: API handlers

**Files:**
- Create: `backend/handlers/stream.go`
- Create: `backend/handlers/task.go`
- Create: `backend/handlers/status.go`
- Create: `backend/handlers/websocket.go`

- [ ] **Step 1: Implement stream handlers**

```go
package handlers

import (
	"living-recorder/backend/models"
	"living-recorder/backend/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type StreamHandler struct {
	db       *gorm.DB
	recorder *services.RecorderService
}

func NewStreamHandler(db *gorm.DB, recorder *services.RecorderService) *StreamHandler {
	return &StreamHandler{db: db, recorder: recorder}
}

func (h *StreamHandler) List(c *gin.Context) {
	var streams []models.Stream
	h.db.Find(&streams)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": streams})
}

func (h *StreamHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var stream models.Stream
	if err := h.db.First(&stream, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "message": "stream not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": stream})
}

func (h *StreamHandler) Create(c *gin.Context) {
	var stream models.Stream
	if err := c.ShouldBindJSON(&stream); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": err.Error()})
		return
	}
	stream.Status = "idle"
	h.db.Create(&stream)
	c.JSON(http.StatusCreated, gin.H{"code": 0, "data": stream})
}

func (h *StreamHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var stream models.Stream
	if err := h.db.First(&stream, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "message": "stream not found"})
		return
	}
	var input models.Stream
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": err.Error()})
		return
	}
	h.db.Model(&stream).Updates(input)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": stream})
}

func (h *StreamHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if h.recorder.IsRecording(uint(id)) {
		c.JSON(http.StatusConflict, gin.H{"code": 1, "message": "stream is recording, stop first"})
		return
	}
	h.db.Delete(&models.Stream{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}

func (h *StreamHandler) Start(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var task models.RecordTask
	if err := h.db.Where("stream_id = ? AND enabled = ?", id, true).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "message": "no enabled task found for stream"})
		return
	}
	if err := h.recorder.Start(uint(id), &task); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "recording started"})
}

func (h *StreamHandler) Stop(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := h.recorder.Stop(uint(id)); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "recording stopped"})
}

func (h *StreamHandler) Logs(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var logs []models.RecordLog
	h.db.Where("stream_id = ?", id).Order("started_at desc").Limit(100).Find(&logs)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": logs})
}
```

- [ ] **Step 2: Implement task handlers**

```go
package handlers

import (
	"living-recorder/backend/models"
	"living-recorder/backend/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TaskHandler struct {
	db        *gorm.DB
	scheduler *services.SchedulerService
}

func NewTaskHandler(db *gorm.DB, scheduler *services.SchedulerService) *TaskHandler {
	return &TaskHandler{db: db, scheduler: scheduler}
}

func (h *TaskHandler) List(c *gin.Context) {
	var tasks []models.RecordTask
	h.db.Preload("Stream").Find(&tasks)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": tasks})
}

func (h *TaskHandler) Create(c *gin.Context) {
	var task models.RecordTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": err.Error()})
		return
	}
	h.db.Create(&task)
	h.scheduler.AddTask(&task)
	c.JSON(http.StatusCreated, gin.H{"code": 0, "data": task})
}

func (h *TaskHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var task models.RecordTask
	if err := h.db.First(&task, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "message": "task not found"})
		return
	}
	var input models.RecordTask
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": err.Error()})
		return
	}
	h.db.Model(&task).Updates(input)
	h.db.First(&task, id)
	h.scheduler.ReloadTask(&task)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": task})
}

func (h *TaskHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	h.scheduler.RemoveTask(uint(id))
	h.db.Delete(&models.RecordTask{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}
```

- [ ] **Step 3: Implement status handlers**

```go
package handlers

import (
	"living-recorder/backend/config"
	"living-recorder/backend/models"
	"living-recorder/backend/services"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type StatusHandler struct {
	db       *gorm.DB
	cfg      *config.Config
	recorder *services.RecorderService
}

func NewStatusHandler(db *gorm.DB, cfg *config.Config, recorder *services.RecorderService) *StatusHandler {
	return &StatusHandler{db: db, cfg: cfg, recorder: recorder}
}

func (h *StatusHandler) GetStatus(c *gin.Context) {
	var totalStreams int64
	var activeRecordings int64
	var totalLogs int64
	var totalSize int64

	h.db.Model(&models.Stream{}).Count(&totalStreams)
	h.db.Model(&models.Stream{}).Where("status = ?", "recording").Count(&activeRecordings)
	h.db.Model(&models.RecordLog{}).Count(&totalLogs)
	h.db.Model(&models.RecordLog{}).Select("COALESCE(SUM(file_size), 0)").Scan(&totalSize)

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"total_streams":     totalStreams,
		"active_recordings": activeRecordings,
		"total_recordings":  totalLogs,
		"storage_used":      totalSize,
		"max_parallel":      h.cfg.Recorder.MaxParallel,
		"version":           "1.0.0",
	}})
}

func (h *StatusHandler) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"ffmpeg_path":            h.cfg.FFmpeg.Path,
		"storage_default":        h.cfg.Storage.Default,
		"storage_local_path":     h.cfg.Storage.Local.Path,
		"max_parallel":           h.cfg.Recorder.MaxParallel,
		"restart_on_failure":     h.cfg.Recorder.RestartOnFailure,
		"health_check_interval":  h.cfg.Recorder.HealthCheckInterval,
	}})
}

func (h *StatusHandler) UpdateConfig(c *gin.Context) {
	var input struct {
		FFmpegPath          *string `json:"ffmpeg_path"`
		MaxParallel         *int    `json:"max_parallel"`
		RestartOnFailure    *int    `json:"restart_on_failure"`
		HealthCheckInterval *int    `json:"health_check_interval"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": err.Error()})
		return
	}
	if input.FFmpegPath != nil {
		h.cfg.FFmpeg.Path = *input.FFmpegPath
	}
	if input.MaxParallel != nil {
		h.cfg.Recorder.MaxParallel = *input.MaxParallel
	}
	if input.RestartOnFailure != nil {
		h.cfg.Recorder.RestartOnFailure = *input.RestartOnFailure
	}
	if input.HealthCheckInterval != nil {
		h.cfg.Recorder.HealthCheckInterval = *input.HealthCheckInterval
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "config updated"})
}
```

- [ ] **Step 4: Implement WebSocket handler**

```go
package handlers

import (
	"living-recorder/backend/services"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type WSHandler struct {
	monitor *services.MonitorService
}

func NewWSHandler(monitor *services.MonitorService) *WSHandler {
	return &WSHandler{monitor: monitor}
}

func (h *WSHandler) Handle(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %v", err)
		return
	}
	h.monitor.AddClient(conn)
	defer h.monitor.RemoveClient(conn)

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}
```

- [ ] **Step 5: Verify build**

```bash
cd living-recorder/backend && go build ./...
```

- [ ] **Step 6: Commit**

```bash
cd living-recorder && git add backend/handlers && git commit -m "feat: add api handlers for streams, tasks, status, and websocket"
```

---

### Task 9: Routes + main.go

**Files:**
- Create: `backend/routes/routes.go`
- Modify: `backend/main.go`

- [ ] **Step 1: Implement routes.go**

```go
package routes

import (
	"living-recorder/backend/config"
	"living-recorder/backend/handlers"
	"living-recorder/backend/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Setup(db *gorm.DB, cfg *config.Config, recorder *services.RecorderService, scheduler *services.SchedulerService, monitor *services.MonitorService) *gin.Engine {
	r := gin.Default()

	streamHandler := handlers.NewStreamHandler(db, recorder)
	taskHandler := handlers.NewTaskHandler(db, scheduler)
	statusHandler := handlers.NewStatusHandler(db, cfg, recorder)
	wsHandler := handlers.NewWSHandler(monitor)

	api := r.Group("/api")
	{
		api.GET("/streams", streamHandler.List)
		api.POST("/streams", streamHandler.Create)
		api.GET("/streams/:id", streamHandler.Get)
		api.PUT("/streams/:id", streamHandler.Update)
		api.DELETE("/streams/:id", streamHandler.Delete)
		api.POST("/streams/:id/start", streamHandler.Start)
		api.POST("/streams/:id/stop", streamHandler.Stop)
		api.GET("/streams/:id/logs", streamHandler.Logs)

		api.GET("/tasks", taskHandler.List)
		api.POST("/tasks", taskHandler.Create)
		api.PUT("/tasks/:id", taskHandler.Update)
		api.DELETE("/tasks/:id", taskHandler.Delete)

		api.GET("/status", statusHandler.GetStatus)
		api.GET("/config", statusHandler.GetConfig)
		api.PUT("/config", statusHandler.UpdateConfig)

		api.GET("/ws", wsHandler.Handle)
	}

	return r
}
```

- [ ] **Step 2: Implement main.go**

```go
package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"

	"living-recorder/backend/config"
	"living-recorder/backend/database"
	"living-recorder/backend/routes"
	"living-recorder/backend/services"

	"github.com/gin-gonic/gin"
)

//go:embed all:embed/dist
var staticFiles embed.FS

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.Init(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}

	var store services.StorageBackend
	switch cfg.Storage.Default {
	case "s3":
		s3store, err := services.NewS3Storage(cfg.Storage.S3)
		if err != nil {
			log.Fatalf("Failed to init S3 storage: %v", err)
		}
		store = s3store
	default:
		store = services.NewLocalStorage(cfg.Storage.Local)
	}

	cfg.Recorder.StorageLocalPath = cfg.Storage.Local.Path
	recorder := services.NewRecorderService(db, cfg.Recorder, cfg.FFmpeg.Path, store)
	scheduler := services.NewSchedulerService(db, recorder)
	monitor := services.NewMonitorService(db, recorder, cfg.Recorder)

	scheduler.Start()
	monitor.Start()

	gin.SetMode(cfg.Server.Mode)
	r := routes.Setup(db, cfg, recorder, scheduler, monitor)

	staticFS, _ := fs.Sub(staticFiles, "embed/dist")
	r.GET("/", func(c *gin.Context) {
		http.FileServer(http.FS(staticFS)).ServeHTTP(c.Writer, c.Request)
	})
	r.NoRoute(func(c *gin.Context) {
		if len(c.Request.URL.Path) >= 5 && c.Request.URL.Path[:5] == "/api/" {
			c.JSON(404, gin.H{"code": 1, "message": "not found"})
			return
		}
		c.Request.URL.Path = "/index.html"
		http.FileServer(http.FS(staticFS)).ServeHTTP(c.Writer, c.Request)
	})

	addr := ":" + cfg.Server.Port
	log.Printf("Starting server on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
```

- [ ] **Step 3: Verify build**

```bash
cd living-recorder/backend && go build -o ../living-recorder ./
```

- [ ] **Step 4: Commit**

```bash
cd living-recorder && git add backend/routes backend/main.go && git commit -m "feat: wire up routes and main entry point"
```

---

### Task 10: Frontend scaffolding

**Files:**
- Create: `frontend/package.json`
- Create: `frontend/vite.config.ts`
- Create: `frontend/tsconfig.json`
- Create: `frontend/tsconfig.node.json`
- Create: `frontend/tailwind.config.js`
- Create: `frontend/postcss.config.js`
- Create: `frontend/index.html`

- [ ] **Step 1: Create package.json**

```json
{
  "name": "living-recorder-frontend",
  "private": true,
  "version": "1.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite --host",
    "build": "tsc && vite build",
    "preview": "vite preview",
    "lint": "eslint . --ext ts,tsx --max-warnings 0"
  },
  "dependencies": {
    "react": "^18.3.1",
    "react-dom": "^18.3.1",
    "react-router-dom": "^6.23.1",
    "lucide-react": "^0.378.0",
    "@radix-ui/react-dialog": "^1.0.5",
    "@radix-ui/react-select": "^2.0.0",
    "@radix-ui/react-switch": "^1.0.3",
    "@radix-ui/react-toast": "^1.1.5",
    "@radix-ui/react-label": "^2.0.2",
    "class-variance-authority": "^0.7.0",
    "clsx": "^2.1.1",
    "tailwind-merge": "^2.3.0",
    "tailwindcss-animate": "^1.0.7"
  },
  "devDependencies": {
    "@types/react": "^18.3.3",
    "@types/react-dom": "^18.3.0",
    "@vitejs/plugin-react": "^4.3.1",
    "autoprefixer": "^10.4.19",
    "postcss": "^8.4.38",
    "tailwindcss": "^3.4.4",
    "typescript": "^5.4.5",
    "vite": "^5.2.13"
  }
}
```

- [ ] **Step 2: Create vite.config.ts**

```ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
    outDir: 'dist',
  },
})
```

- [ ] **Step 3: Create tsconfig.json**

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "baseUrl": ".",
    "paths": { "@/*": ["./src/*"] }
  },
  "include": ["src"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

- [ ] **Step 4: Create tsconfig.node.json**

```json
{
  "compilerOptions": {
    "composite": true,
    "skipLibCheck": true,
    "module": "ESNext",
    "moduleResolution": "bundler",
    "allowSyntheticDefaultImports": true
  },
  "include": ["vite.config.ts"]
}
```

- [ ] **Step 5: Create tailwind.config.js**

```js
/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {},
  },
  plugins: [require('tailwindcss-animate')],
}
```

- [ ] **Step 6: Create postcss.config.js**

```js
export default {
  plugins: {
    tailwindcss: {},
    autoprefixer: {},
  },
}
```

- [ ] **Step 7: Create index.html**

```html
<!DOCTYPE html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <link rel="icon" type="image/svg+xml" href="/vite.svg" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Living Recorder</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 8: Install dependencies**

```bash
cd living-recorder/frontend && npm install
```

- [ ] **Step 9: Commit**

```bash
cd living-recorder && git add frontend/package.json frontend/vite.config.ts frontend/tsconfig.json frontend/tsconfig.node.json frontend/tailwind.config.js frontend/postcss.config.js frontend/index.html && git commit -m "chore: scaffold frontend with vite + react + tailwind"
```

---

### Task 11: Frontend source files

**Files:**
- Create: `frontend/src/main.tsx`
- Create: `frontend/src/index.css`
- Create: `frontend/src/App.tsx`
- Create: `frontend/src/lib/api.ts`
- Create: `frontend/src/lib/ws.ts`
- Create: `frontend/src/hooks/useWebSocket.ts`
- Create: `frontend/src/components/Layout.tsx`
- Create: `frontend/src/components/StatusBadge.tsx`
- Create: `frontend/src/components/StreamCard.tsx`
- Create: `frontend/src/components/ui/` (shadcn-like components)

- [ ] **Step 1: Create main.tsx**

```tsx
import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import './index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
```

- [ ] **Step 2: Create index.css**

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

:root {
  --background: 0 0% 100%;
  --foreground: 222.2 84% 4.9%;
  --card: 0 0% 100%;
  --card-foreground: 222.2 84% 4.9%;
  --primary: 222.2 47.4% 11.2%;
  --primary-foreground: 210 40% 98%;
  --muted: 210 40% 96.1%;
  --muted-foreground: 215.4 16.3% 46.9%;
  --destructive: 0 84.2% 60.2%;
  --destructive-foreground: 210 40% 98%;
  --border: 214.3 31.8% 91.4%;
  --ring: 222.2 84% 4.9%;
  --radius: 0.5rem;
}
```

- [ ] **Step 3: Create lib/api.ts**

```ts
const BASE_URL = import.meta.env.VITE_API_URL || ''

export interface Stream {
  id: number
  name: string
  url: string
  protocol: string
  enabled: boolean
  status: string
  created_at: string
  updated_at: string
}

export interface RecordTask {
  id: number
  stream_id: number
  stream?: Stream
  output_template: string
  segment_sec: number
  video_codec: string
  video_bitrate: string
  framerate: number
  resolution: string
  audio_codec: string
  audio_bitrate: string
  storage_type: string
  schedule_cron: string
  schedule_duration: number
  loop_record: boolean
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface RecordLog {
  id: number
  stream_id: number
  file_path: string
  file_size: number
  duration: number
  status: string
  error_msg: string
  started_at: string
  ended_at: string
}

export interface Status {
  total_streams: number
  active_recordings: number
  total_recordings: number
  storage_used: number
  max_parallel: number
  version: string
}

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE_URL}${url}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  const body = await res.json()
  if (body.code !== 0) throw new Error(body.message)
  return body.data as T
}

export const api = {
  streams: {
    list: () => request<Stream[]>('/api/streams'),
    get: (id: number) => request<Stream>(`/api/streams/${id}`),
    create: (data: Partial<Stream>) =>
      request<Stream>('/api/streams', { method: 'POST', body: JSON.stringify(data) }),
    update: (id: number, data: Partial<Stream>) =>
      request<Stream>(`/api/streams/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
    delete: (id: number) => request<void>(`/api/streams/${id}`, { method: 'DELETE' }),
    start: (id: number) => request<void>(`/api/streams/${id}/start`, { method: 'POST' }),
    stop: (id: number) => request<void>(`/api/streams/${id}/stop`, { method: 'POST' }),
    logs: (id: number) => request<RecordLog[]>(`/api/streams/${id}/logs`),
  },
  tasks: {
    list: () => request<RecordTask[]>('/api/tasks'),
    create: (data: Partial<RecordTask>) =>
      request<RecordTask>('/api/tasks', { method: 'POST', body: JSON.stringify(data) }),
    update: (id: number, data: Partial<RecordTask>) =>
      request<RecordTask>(`/api/tasks/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
    delete: (id: number) => request<void>(`/api/tasks/${id}`, { method: 'DELETE' }),
  },
  status: {
    get: () => request<Status>('/api/status'),
  },
  config: {
    get: () => request<Record<string, unknown>>('/api/config'),
    update: (data: Record<string, unknown>) =>
      request<void>('/api/config', { method: 'PUT', body: JSON.stringify(data) }),
  },
}
```

- [ ] **Step 4: Create lib/ws.ts**

```ts
export type StatusMessage = {
  type: 'status' | 'stream_update'
  active_streams?: Array<{
    stream_id: number
    status: string
    started_at: string
  }>
  stream_status?: {
    stream_id: number
    status: string
    duration_sec: number
  }
  storage_used?: number
}

export function createWebSocket(onMessage: (msg: StatusMessage) => void): WebSocket {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/api/ws`
  const ws = new WebSocket(wsUrl)

  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data) as StatusMessage
      onMessage(msg)
    } catch {
      // ignore malformed messages
    }
  }

  ws.onclose = () => {
    setTimeout(() => {
      new WebSocket(wsUrl)
    }, 5000)
  }

  return ws
}
```

- [ ] **Step 5: Create hooks/useWebSocket.ts**

```ts
import { useEffect, useRef, useState } from 'react'
import { createWebSocket, StatusMessage } from '@/lib/ws'

export function useWebSocket() {
  const [lastMessage, setLastMessage] = useState<StatusMessage | null>(null)
  const wsRef = useRef<WebSocket | null>(null)

  useEffect(() => {
    wsRef.current = createWebSocket(setLastMessage)
    return () => {
      wsRef.current?.close()
    }
  }, [])

  return { lastMessage }
}
```

- [ ] **Step 6: Create components/ui/button.tsx, card.tsx, input.tsx, select.tsx, switch.tsx, badge.tsx, dialog.tsx, label.tsx, toast.tsx** (shadcn-style primitives — or use minimal inline versions)

Create `button.tsx`:
```tsx
import * as React from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'

const buttonVariants = cva(
  'inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50',
  {
    variants: {
      variant: {
        default: 'bg-primary text-primary-foreground shadow hover:bg-primary/90',
        destructive: 'bg-destructive text-destructive-foreground shadow-sm hover:bg-destructive/90',
        outline: 'border border-input bg-background shadow-sm hover:bg-muted',
        secondary: 'bg-muted text-muted-foreground shadow-sm hover:bg-muted/80',
        ghost: 'hover:bg-muted',
        link: 'text-primary underline-offset-4 hover:underline',
      },
      size: {
        default: 'h-9 px-4 py-2',
        sm: 'h-8 rounded-md px-3 text-xs',
        lg: 'h-10 rounded-md px-8',
        icon: 'h-9 w-9',
      },
    },
    defaultVariants: { variant: 'default', size: 'default' },
  },
)

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, ...props }, ref) => (
    <button className={cn(buttonVariants({ variant, size, className }))} ref={ref} {...props} />
  ),
)
Button.displayName = 'Button'
export { Button, buttonVariants }
```

Create `lib/utils.ts`:
```ts
import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
```

Create `card.tsx`, `badge.tsx`, `dialog.tsx`, `input.tsx`, `select.tsx`, `switch.tsx`, `label.tsx`, `toast.tsx` similarly — standard shadcn/ui patterns, adapted to manual setup.

- [ ] **Step 7: Create components/Layout.tsx**

```tsx
import { Link, useLocation } from 'react-router-dom'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'

const navItems = [
  { path: '/', label: 'Dashboard', icon: '📊' },
  { path: '/streams', label: 'Streams', icon: '📡' },
  { path: '/tasks', label: 'Tasks', icon: '⏰' },
  { path: '/settings', label: 'Settings', icon: '⚙️' },
]

export function Layout({ children }: { children: React.ReactNode }) {
  const location = useLocation()
  return (
    <div className="flex h-screen">
      <nav className="w-56 border-r bg-card p-4 flex flex-col gap-2">
        <h1 className="text-lg font-bold mb-4">Living Recorder</h1>
        {navItems.map((item) => (
          <Link key={item.path} to={item.path}>
            <Button
              variant={location.pathname === item.path ? 'default' : 'ghost'}
              className="w-full justify-start"
            >
              <span className="mr-2">{item.icon}</span>
              {item.label}
            </Button>
          </Link>
        ))}
      </nav>
      <main className="flex-1 overflow-auto p-6">{children}</main>
    </div>
  )
}
```

- [ ] **Step 8: Create components/StatusBadge.tsx**

```tsx
import { cn } from '@/lib/utils'

const statusColors: Record<string, string> = {
  idle: 'bg-gray-500',
  recording: 'bg-green-500 animate-pulse',
  error: 'bg-red-500',
  stopping: 'bg-yellow-500',
}

export function StatusBadge({ status }: { status: string }) {
  return (
    <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium">
      <span className={cn('w-2 h-2 rounded-full', statusColors[status] || 'bg-gray-500')} />
      {status}
    </span>
  )
}
```

- [ ] **Step 9: Create components/StreamCard.tsx**

```tsx
import { Stream } from '@/lib/api'
import { StatusBadge } from './StatusBadge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'

interface StreamCardProps {
  stream: Stream
  onStart: (id: number) => void
  onStop: (id: number) => void
  onEdit: (stream: Stream) => void
  onDelete: (id: number) => void
}

export function StreamCard({ stream, onStart, onStop, onEdit, onDelete }: StreamCardProps) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-base">{stream.name}</CardTitle>
        <StatusBadge status={stream.status} />
      </CardHeader>
      <CardContent className="space-y-2">
        <div className="text-sm text-muted-foreground truncate">{stream.url}</div>
        <div className="flex items-center gap-2">
          <span className="text-xs bg-muted px-2 py-0.5 rounded">{stream.protocol.toUpperCase()}</span>
          <span className="text-xs text-muted-foreground">ID: {stream.id}</span>
        </div>
        <div className="flex gap-2 pt-2">
          {stream.status === 'recording' ? (
            <Button size="sm" variant="destructive" onClick={() => onStop(stream.id)}>
              Stop
            </Button>
          ) : (
            <Button size="sm" variant="default" onClick={() => onStart(stream.id)}>
              Record
            </Button>
          )}
          <Button size="sm" variant="outline" onClick={() => onEdit(stream)}>
            Edit
          </Button>
          <Button size="sm" variant="ghost" onClick={() => onDelete(stream.id)}>
            Delete
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
```

- [ ] **Step 10: Create App.tsx with routing**

```tsx
import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { Layout } from '@/components/Layout'
import Dashboard from '@/pages/Dashboard'
import Streams from '@/pages/Streams'
import StreamDetail from '@/pages/StreamDetail'
import Tasks from '@/pages/Tasks'
import Settings from '@/pages/Settings'

export default function App() {
  return (
    <BrowserRouter>
      <Layout>
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/streams" element={<Streams />} />
          <Route path="/streams/:id" element={<StreamDetail />} />
          <Route path="/tasks" element={<Tasks />} />
          <Route path="/settings" element={<Settings />} />
        </Routes>
      </Layout>
    </BrowserRouter>
  )
}
```

- [ ] **Step 11: Create Dashboard page**

```tsx
import { useEffect, useState } from 'react'
import { api, Status, RecordLog } from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

export default function Dashboard() {
  const [status, setStatus] = useState<Status | null>(null)
  const [recentLogs, setRecentLogs] = useState<RecordLog[]>([])

  useEffect(() => {
    api.status.get().then(setStatus)
    api.streams.list().then((streams) => {
      if (streams.length > 0) {
        api.streams.logs(streams[0].id).then(setRecentLogs)
      }
    })
  }, [])

  return (
    <div className="space-y-6">
      <h2 className="text-2xl font-bold">Dashboard</h2>
      <div className="grid grid-cols-4 gap-4">
        <Card>
          <CardHeader><CardTitle className="text-sm">Total Streams</CardTitle></CardHeader>
          <CardContent><p className="text-2xl font-bold">{status?.total_streams ?? '-'}</p></CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle className="text-sm">Active Recordings</CardTitle></CardHeader>
          <CardContent><p className="text-2xl font-bold text-green-600">{status?.active_recordings ?? '-'}</p></CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle className="text-sm">Total Recordings</CardTitle></CardHeader>
          <CardContent><p className="text-2xl font-bold">{status?.total_recordings ?? '-'}</p></CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle className="text-sm">Storage Used</CardTitle></CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">
              {status ? `${(status.storage_used / 1024 / 1024 / 1024).toFixed(2)} GB` : '-'}
            </p>
          </CardContent>
        </Card>
      </div>
      <div>
        <h3 className="text-lg font-semibold mb-2">Recent Recordings</h3>
        {recentLogs.length === 0 ? (
          <p className="text-muted-foreground">No recordings yet</p>
        ) : (
          <div className="space-y-2">
            {recentLogs.slice(0, 10).map((log) => (
              <div key={log.id} className="flex justify-between items-center p-2 border rounded">
                <span>{log.file_path}</span>
                <span className="text-sm text-muted-foreground">{log.duration}s</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
```

- [ ] **Step 12: Create Streams page**

```tsx
import { useEffect, useState } from 'react'
import { api, Stream } from '@/lib/api'
import { StreamCard } from '@/components/StreamCard'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

const protocols = ['rtsp', 'rtmp', 'flv', 'hls']

export default function Streams() {
  const [streams, setStreams] = useState<Stream[]>([])
  const [editing, setEditing] = useState<Partial<Stream>>({})
  const [open, setOpen] = useState(false)

  const load = () => api.streams.list().then(setStreams)

  useEffect(() => { load() }, [])

  const handleSave = async () => {
    if (editing.id) {
      await api.streams.update(editing.id, editing)
    } else {
      await api.streams.create(editing)
    }
    setOpen(false)
    setEditing({})
    load()
  }

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <h2 className="text-2xl font-bold">Streams</h2>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button onClick={() => setEditing({ protocol: 'rtsp' })}>Add Stream</Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader><DialogTitle>{editing.id ? 'Edit Stream' : 'Add Stream'}</DialogTitle></DialogHeader>
            <div className="space-y-4">
              <div>
                <Label>Name</Label>
                <Input value={editing.name || ''} onChange={(e) => setEditing({ ...editing, name: e.target.value })} />
              </div>
              <div>
                <Label>URL</Label>
                <Input value={editing.url || ''} onChange={(e) => setEditing({ ...editing, url: e.target.value })} />
              </div>
              <div>
                <Label>Protocol</Label>
                <Select value={editing.protocol} onValueChange={(v) => setEditing({ ...editing, protocol: v })}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {protocols.map((p) => (
                      <SelectItem key={p} value={p}>{p.toUpperCase()}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <Button onClick={handleSave} className="w-full">
                {editing.id ? 'Update' : 'Create'}
              </Button>
            </div>
          </DialogContent>
        </Dialog>
      </div>
      <div className="grid grid-cols-3 gap-4">
        {streams.map((s) => (
          <StreamCard
            key={s.id}
            stream={s}
            onStart={async (id) => { await api.streams.start(id); load() }}
            onStop={async (id) => { await api.streams.stop(id); load() }}
            onEdit={(stream) => { setEditing(stream); setOpen(true) }}
            onDelete={async (id) => { await api.streams.delete(id); load() }}
          />
        ))}
      </div>
    </div>
  )
}
```

- [ ] **Step 13: Create StreamDetail page**

```tsx
import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api, Stream, RecordLog } from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { StatusBadge } from '@/components/StatusBadge'

export default function StreamDetail() {
  const { id } = useParams<{ id: string }>()
  const [stream, setStream] = useState<Stream | null>(null)
  const [logs, setLogs] = useState<RecordLog[]>([])

  useEffect(() => {
    if (!id) return
    api.streams.get(Number(id)).then(setStream)
    api.streams.logs(Number(id)).then(setLogs)
  }, [id])

  if (!stream) return <p>Loading...</p>

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <h2 className="text-2xl font-bold">{stream.name}</h2>
        <StatusBadge status={stream.status} />
      </div>
      <Card>
        <CardHeader><CardTitle>Info</CardTitle></CardHeader>
        <CardContent className="space-y-2">
          <div><span className="text-muted-foreground">URL:</span> {stream.url}</div>
          <div><span className="text-muted-foreground">Protocol:</span> {stream.protocol.toUpperCase()}</div>
          <div><span className="text-muted-foreground">Created:</span> {stream.created_at}</div>
        </CardContent>
      </Card>
      <div>
        <h3 className="text-lg font-semibold mb-2">Recording Logs</h3>
        <div className="space-y-2">
          {logs.map((log) => (
            <div key={log.id} className="flex justify-between items-center p-3 border rounded">
              <div>
                <div className="text-sm">{log.file_path}</div>
                <div className="text-xs text-muted-foreground">
                  {new Date(log.started_at).toLocaleString()} - {log.duration}s
                </div>
              </div>
              <div className="text-right">
                <div className="text-sm">{(log.file_size / 1024 / 1024).toFixed(2)} MB</div>
                <StatusBadge status={log.status} />
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
```

- [ ] **Step 14: Create Tasks page**

```tsx
import { useEffect, useState } from 'react'
import { api, RecordTask } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

export default function Tasks() {
  const [tasks, setTasks] = useState<RecordTask[]>([])
  const [editing, setEditing] = useState<Partial<RecordTask>>({})
  const [open, setOpen] = useState(false)
  const [streams, setStreams] = useState<{ id: number; name: string }[]>([])

  const load = () => {
    api.tasks.list().then(setTasks)
    api.streams.list().then((ss) => setStreams(ss.map((s) => ({ id: s.id, name: s.name }))))
  }

  useEffect(() => { load() }, [])

  const handleSave = async () => {
    if (editing.id) {
      await api.tasks.update(editing.id, editing)
    } else {
      await api.tasks.create(editing)
    }
    setOpen(false)
    setEditing({})
    load()
  }

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <h2 className="text-2xl font-bold">Record Tasks</h2>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button onClick={() => setEditing({ video_codec: 'copy', audio_codec: 'copy', storage_type: 'local' })}>
              Add Task
            </Button>
          </DialogTrigger>
          <DialogContent className="max-w-lg">
            <DialogHeader><DialogTitle>{editing.id ? 'Edit Task' : 'Add Task'}</DialogTitle></DialogHeader>
            <div className="grid grid-cols-2 gap-4">
              <div className="col-span-2">
                <Label>Stream</Label>
                <Select value={String(editing.stream_id || '')} onValueChange={(v) => setEditing({ ...editing, stream_id: Number(v) })}>
                  <SelectTrigger><SelectValue placeholder="Select stream" /></SelectTrigger>
                  <SelectContent>
                    {streams.map((s) => (
                      <SelectItem key={s.id} value={String(s.id)}>{s.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label>Output Template</Label>
                <Input value={editing.output_template || '{name}/{date}_{time}.mp4'} onChange={(e) => setEditing({ ...editing, output_template: e.target.value })} />
              </div>
              <div>
                <Label>Segment (sec, 0=off)</Label>
                <Input type="number" value={editing.segment_sec ?? 0} onChange={(e) => setEditing({ ...editing, segment_sec: Number(e.target.value) })} />
              </div>
              <div>
                <Label>Video Codec</Label>
                <Select value={editing.video_codec || 'copy'} onValueChange={(v) => setEditing({ ...editing, video_codec: v })}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="copy">Copy</SelectItem>
                    <SelectItem value="libx264">H.264</SelectItem>
                    <SelectItem value="h264_nvenc">H.264 (NVENC)</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label>Video Bitrate</Label>
                <Input value={editing.video_bitrate || ''} onChange={(e) => setEditing({ ...editing, video_bitrate: e.target.value })} placeholder="2000k" />
              </div>
              <div>
                <Label>Framerate (0=auto)</Label>
                <Input type="number" value={editing.framerate ?? 0} onChange={(e) => setEditing({ ...editing, framerate: Number(e.target.value) })} />
              </div>
              <div>
                <Label>Resolution</Label>
                <Input value={editing.resolution || ''} onChange={(e) => setEditing({ ...editing, resolution: e.target.value })} placeholder="1920x1080" />
              </div>
              <div>
                <Label>Audio Codec</Label>
                <Select value={editing.audio_codec || 'copy'} onValueChange={(v) => setEditing({ ...editing, audio_codec: v })}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="copy">Copy</SelectItem>
                    <SelectItem value="aac">AAC</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label>Audio Bitrate</Label>
                <Input value={editing.audio_bitrate || ''} onChange={(e) => setEditing({ ...editing, audio_bitrate: e.target.value })} placeholder="128k" />
              </div>
              <div>
                <Label>Schedule Cron</Label>
                <Input value={editing.schedule_cron || ''} onChange={(e) => setEditing({ ...editing, schedule_cron: e.target.value })} placeholder="0 0 * * * *" />
              </div>
              <div>
                <Label>Duration (sec, 0=unlimited)</Label>
                <Input type="number" value={editing.schedule_duration ?? 0} onChange={(e) => setEditing({ ...editing, schedule_duration: Number(e.target.value) })} />
              </div>
              <div className="col-span-2">
                <Label>Storage Type</Label>
                <Select value={editing.storage_type || 'local'} onValueChange={(v) => setEditing({ ...editing, storage_type: v })}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="local">Local</SelectItem>
                    <SelectItem value="s3">S3 / MinIO</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <Button onClick={handleSave} className="w-full">{editing.id ? 'Update' : 'Create'}</Button>
          </DialogContent>
        </Dialog>
      </div>
      <div className="space-y-3">
        {tasks.map((task) => (
          <Card key={task.id}>
            <CardHeader><CardTitle className="text-sm">
              {task.stream?.name || `Stream #${task.stream_id}`}
            </CardTitle></CardHeader>
            <CardContent className="text-sm text-muted-foreground space-y-1">
              <div>Codec: {task.video_codec} / {task.audio_codec}</div>
              <div>Segment: {task.segment_sec > 0 ? `${task.segment_sec}s` : 'off'}</div>
              <div>Cron: {task.schedule_cron || 'manual only'}</div>
              <div className="flex gap-2 pt-1">
                <Button size="sm" variant="outline" onClick={() => { setEditing(task); setOpen(true) }}>Edit</Button>
                <Button size="sm" variant="ghost" onClick={async () => { await api.tasks.delete(task.id); load() }}>Delete</Button>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  )
}
```

- [ ] **Step 15: Create Settings page**

```tsx
import { useEffect, useState } from 'react'
import { api } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

export default function Settings() {
  const [config, setConfig] = useState<Record<string, unknown>>({})
  const [dirty, setDirty] = useState<Record<string, unknown>>({})

  useEffect(() => { api.config.get().then(setConfig) }, [])

  const handleSave = async () => {
    await api.config.update(dirty)
    setDirty({})
  }

  const makeField = (key: string, label: string, type = 'text') => (
    <div key={key}>
      <Label>{label}</Label>
      <Input
        type={type}
        defaultValue={String(config[key] ?? '')}
        onChange={(e) => setDirty({ ...dirty, [key]: type === 'number' ? Number(e.target.value) : e.target.value })}
      />
    </div>
  )

  return (
    <div className="max-w-xl space-y-6">
      <h2 className="text-2xl font-bold">Settings</h2>
      <Card>
        <CardHeader><CardTitle>Global Config</CardTitle></CardHeader>
        <CardContent className="space-y-4">
          {makeField('ffmpeg_path', 'FFmpeg Path')}
          {makeField('max_parallel', 'Max Parallel Recordings', 'number')}
          {makeField('restart_on_failure', 'Restart on Failure (count)', 'number')}
          {makeField('health_check_interval', 'Health Check Interval (sec)', 'number')}
          <Button onClick={handleSave} disabled={Object.keys(dirty).length === 0}>Save</Button>
        </CardContent>
      </Card>
    </div>
  )
}
```

- [ ] **Step 16: Build frontend**

```bash
cd living-recorder/frontend && npx tsc --noEmit && npm run build
```

- [ ] **Step 17: Commit**

```bash
cd living-recorder && git add frontend/src && git commit -m "feat: add frontend pages and components"
```

---

### Task 12: Integration test

**Files:**
- Run: `backend` binary with frontend embedded

- [ ] **Step 1: Build and run**

```bash
cd living-recorder/frontend && npm run build && cp -r dist ../backend/embed/
cd ../backend && go build -o ../living-recorder ./
cd .. && ./living-recorder
```

- [ ] **Step 2: Quick API smoke test**

```bash
# Add stream
curl -s -X POST http://localhost:8080/api/streams \
  -H 'Content-Type: application/json' \
  -d '{"name":"test","url":"rtsp://example.com/stream","protocol":"rtsp"}' | jq .

# List streams
curl -s http://localhost:8080/api/streams | jq .

# Get status
curl -s http://localhost:8080/api/status | jq .
```

- [ ] **Step 3: Commit final build**

```bash
cd living-recorder && git add -A && git commit -m "feat: integrate frontend build and finalize"
```

---

### Task 13: Add .gitignore

**Files:**
- Create: `.gitignore`

- [ ] **Step 1: Create .gitignore**

```
# Binary
living-recorder
oh-my-video

# Temp
temp/
data/
recordings/

# Frontend
frontend/node_modules/
frontend/dist/
backend/embed/dist/

# IDE
.idea/
.vscode/
*.swp

# OS
.DS_Store
Thumbs.db

# Config
backend/config/config.yaml
```

- [ ] **Step 2: Commit**

```bash
cd living-recorder && git add .gitignore && git commit -m "chore: add gitignore"
```
