package services

import (
	"context"
	"fmt"
	"living-recorder/backend/config"
	"living-recorder/backend/models"
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
	db      *gorm.DB
	cfg     config.RecorderConfig
	ffmpeg  string
	store   StorageBackend
	mu      sync.RWMutex
	streams map[uint]*StreamProcess
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
