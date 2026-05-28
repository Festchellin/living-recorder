package services

import (
	"fmt"
	"io"
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
	stdin     io.WriteCloser
	StartedAt time.Time
	Status    string
}

type StatusChangeCallback func(streamID uint)

type RecorderService struct {
	db             *gorm.DB
	cfg            config.RecorderConfig
	ffmpeg         string
	store          StorageBackend
	mu             sync.RWMutex
	streams        map[uint]*StreamProcess
	onStatusChange StatusChangeCallback
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

func (s *RecorderService) OnStatusChange(cb StatusChangeCallback) {
	s.onStatusChange = cb
}

func (s *RecorderService) ResetStaleStatuses() {
	s.db.Model(&models.Stream{}).Where("status = ?", "recording").Update("status", "idle")
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
	outputPath := args[len(args)-1]
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	cmd := exec.Command(s.ffmpeg, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	sp := &StreamProcess{
		StreamID:  streamID,
		TaskID:    task.ID,
		Cmd:       cmd,
		stdin:     stdin,
		StartedAt: time.Now(),
		Status:    "recording",
	}
	s.streams[streamID] = sp

	s.db.Model(&stream).Update("status", "recording")

	if err := cmd.Start(); err != nil {
		delete(s.streams, streamID)
		s.db.Model(&stream).Update("status", "idle")
		return fmt.Errorf("start ffmpeg: %w", err)
	}

	go s.watchProcess(sp)

	if s.onStatusChange != nil {
		go s.onStatusChange(streamID)
	}

	return nil
}

func (s *RecorderService) Stop(streamID uint) error {
	s.mu.Lock()

	sp, exists := s.streams[streamID]
	if !exists {
		s.mu.Unlock()
		return fmt.Errorf("stream %d is not recording", streamID)
	}
	sp.Status = "stopping"
	s.mu.Unlock()

	sp.stdin.Write([]byte("q\n"))
	sp.stdin.Close()

	done := make(chan struct{})
	go func() {
		sp.Cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		sp.Cmd.Process.Kill()
		<-done
	}

	if s.onStatusChange != nil {
		go s.onStatusChange(streamID)
	}

	return nil
}

func (s *RecorderService) EffectiveStreamStatus(stream *models.Stream) string {
	s.mu.RLock()
	sp, exists := s.streams[stream.ID]
	s.mu.RUnlock()
	if exists {
		return sp.Status
	}
	return stream.Status
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
	if task.VideoCodec != "copy" {
		args = append(args, "-vf", "setparams=color_primaries=bt709:color_trc=bt709:colorspace=bt709")
		args = append(args, "-pix_fmt", "yuv420p")
	}
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

	args = append(args, "-movflags", "+faststart")
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
	wasStopping := sp.Status == "stopping"
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
		if wasStopping {
			log.Status = "success"
		} else {
			log.Status = "failed"
			log.ErrorMsg = err.Error()
		}
	}
	s.db.Create(&log)

	if s.onStatusChange != nil {
		go s.onStatusChange(sp.StreamID)
	}
}
