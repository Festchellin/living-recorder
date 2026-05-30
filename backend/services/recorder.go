package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"living-recorder/backend/config"
	"living-recorder/backend/models"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
)

type StreamProcess struct {
	StreamID   uint
	TaskID     uint
	Cmd        *exec.Cmd
	stdin      io.WriteCloser
	StartedAt  time.Time
	Status     string
	OutputPath string
	StreamName string
	userStopped atomic.Bool
	stderrBuf  bytes.Buffer
}

type StatusChangeCallback func(streamID uint)

type RecorderService struct {
	db             *gorm.DB
	cfg            *config.RecorderConfig
	ffmpeg         string
	store          StorageBackend
	log            *LogWriter
	mu             sync.RWMutex
	streams        map[uint]*StreamProcess
	onStatusChange StatusChangeCallback
}

func NewRecorderService(db *gorm.DB, cfg *config.RecorderConfig, ffmpegPath string, store StorageBackend) *RecorderService {
	return &RecorderService{
		db:      db,
		cfg:     cfg,
		ffmpeg:  ffmpegPath,
		store:   store,
		log:     NewLogWriter(db),
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
	if err := s.db.Preload("Group").First(&stream, streamID).Error; err != nil {
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

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	sp := &StreamProcess{
		StreamID:   streamID,
		TaskID:     task.ID,
		Cmd:        cmd,
		stdin:      stdin,
		StartedAt:  time.Now(),
		Status:     "recording",
		OutputPath: outputPath,
		StreamName: stream.Name,
	}
	s.streams[streamID] = sp

	s.db.Model(&stream).Update("status", "recording")

	s.log.StreamInfo(streamID, models.EventRecordingStarted, "开始录制: %s", stream.Name)

	if err := cmd.Start(); err != nil {
		delete(s.streams, streamID)
		s.db.Model(&stream).Update("status", "idle")
		return fmt.Errorf("start ffmpeg: %w", err)
	}

	go io.Copy(&sp.stderrBuf, stderr)
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
	sp.userStopped.Store(true)
	s.mu.Unlock()

	s.log.StreamInfo(streamID, models.EventRecordingStopped, "停止录制: %s", sp.StreamName)

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

func (s *RecorderService) FFmpegPath() string {
	return s.ffmpeg
}

type BatchResult struct {
	Success int      `json:"success"`
	Errors  []string `json:"errors,omitempty"`
}

func (s *RecorderService) IsReachable(stream *models.Stream) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := []string{"-v", "quiet", "-print_format", "json", "-show_streams"}
	if stream.Protocol == "rtsp" {
		args = append(args, "-rtsp_transport", "tcp")
	}
	args = append(args, "-i", stream.URL)

	cmd := exec.CommandContext(ctx, "ffprobe", args...)
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	var info struct {
		Streams []interface{} `json:"streams"`
	}
	return json.Unmarshal(output, &info) == nil && len(info.Streams) > 0
}

func (s *RecorderService) StartAll() BatchResult {
	var streams []models.Stream
	s.db.Where("enabled = ?", true).Find(&streams)

	result := BatchResult{}
	for _, stream := range streams {
		if s.IsRecording(stream.ID) {
			continue
		}
		if !s.IsReachable(&stream) {
			errMsg := "信号源不可达，已跳过"
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", stream.Name, errMsg))
			s.log.StreamWarn(stream.ID, models.EventStreamProbe, "全部启动跳过 — %s: %s", stream.Name, errMsg)
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

	outputPath := filepath.Join(s.cfg.StorageLocalPath, tpl)

	if stream.Group != nil && stream.Group.Name != "" {
		groupPath := s.buildGroupPath(stream.Group)
		outputPath = filepath.Join(s.cfg.StorageLocalPath, groupPath, tpl)
	}

	return outputPath
}

func (s *RecorderService) buildGroupPath(group *models.Group) string {
	names := []string{group.Name}
	currentID := group.ParentID
	for currentID != nil {
		var parent models.Group
		if err := s.db.First(&parent, *currentID).Error; err != nil {
			break
		}
		names = append([]string{parent.Name}, names...)
		currentID = parent.ParentID
	}
	return filepath.Join(names...)
}

func (s *RecorderService) watchProcess(sp *StreamProcess) {
	err := sp.Cmd.Wait()
	endedAt := time.Now()

	s.mu.Lock()
	delete(s.streams, sp.StreamID)
	s.mu.Unlock()

	s.db.Model(&models.Stream{}).Where("id = ?", sp.StreamID).Update("status", "idle")

	fileSize := int64(0)
	if fi, statErr := os.Stat(sp.OutputPath); statErr == nil {
		fileSize = fi.Size()
	}
	duration := int(endedAt.Sub(sp.StartedAt).Seconds())

	status := "success"
	errMsg := ""
	eventType := models.EventRecordingStopped
	msg := fmt.Sprintf("录制结束: %s (时长 %d秒)", sp.StreamName, duration)

	if fileSize > 0 {
		msg += fmt.Sprintf(", 大小 %.1fMB", float64(fileSize)/1024/1024)
	}

	if sp.stderrBuf.Len() > 0 {
		log.Printf("[recorder] ffmpeg stderr for stream %d (%s):\n%s", sp.StreamID, sp.StreamName, sp.stderrBuf.String())
	}

	if err != nil {
		if sp.userStopped.Load() {
			status = "success"
			eventType = models.EventRecordingStopped
			msg = fmt.Sprintf("录制已停止: %s (时长 %d秒)", sp.StreamName, duration)
		} else if isProcessKilled(err) {
			status = "success"
			eventType = models.EventRecordingStopped
			if exitErr, ok := err.(*exec.ExitError); ok {
				errMsg = fmt.Sprintf("exit code %d (%#x)", exitErr.ExitCode(), uint32(exitErr.ExitCode()))
			}
			msg = fmt.Sprintf("录制意外终止: %s (时长 %d秒, %s)", sp.StreamName, duration, errMsg)
			log.Printf("[recorder] stream %d (%s) 进程意外退出, code=%s", sp.StreamID, sp.StreamName, errMsg)
		} else {
			status = "failed"
			if exitErr, ok := err.(*exec.ExitError); ok {
				errMsg = fmt.Sprintf("exit code %d (%#x)", exitErr.ExitCode(), uint32(exitErr.ExitCode()))
			} else {
				errMsg = err.Error()
			}
			eventType = models.EventRecordingFailed
			msg = fmt.Sprintf("录制失败: %s — %s", sp.StreamName, errMsg)
		}
	}

	s.log.StreamRecordingLog(sp.StreamID, eventType, status, msg, errMsg, sp.OutputPath, fileSize, duration, sp.StartedAt, endedAt)

	if s.onStatusChange != nil {
		go s.onStatusChange(sp.StreamID)
	}
}

var (
	hwEncoderOnce sync.Once
	hwEncoder     string
)

func findVAAPIDevice() string {
	entries, err := os.ReadDir("/dev/dri")
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "renderD") {
			return "/dev/dri/" + e.Name()
		}
	}
	return ""
}

func (s *RecorderService) validateEncoder(name string) bool {
	var args []string
	switch name {
	case "h264_vaapi":
		device := findVAAPIDevice()
		if device == "" {
			return false
		}
		args = []string{"-hide_banner", "-vaapi_device", device, "-f", "lavfi", "-i", "color=s=64x64", "-c:v", name, "-f", "null", "-", "-loglevel", "error"}
	case "h264_nvenc":
		args = []string{"-hide_banner", "-f", "lavfi", "-i", "color=s=64x64", "-c:v", name, "-preset", "p1", "-f", "null", "-", "-loglevel", "error"}
	case "h264_qsv":
		args = []string{"-hide_banner", "-f", "lavfi", "-i", "color=s=64x64", "-c:v", name, "-preset", "1", "-f", "null", "-", "-loglevel", "error"}
	case "h264_amf":
		args = []string{"-hide_banner", "-f", "lavfi", "-i", "color=s=64x64", "-c:v", name, "-quality", "speed", "-f", "null", "-", "-loglevel", "error"}
	case "h264_videotoolbox":
		args = []string{"-hide_banner", "-f", "lavfi", "-i", "color=s=64x64", "-c:v", name, "-encoder", "speed", "-f", "null", "-", "-loglevel", "error"}
	default:
		return false
	}
	cmd := exec.Command(s.ffmpeg, args...)
	return cmd.Run() == nil
}

func (s *RecorderService) detectHardwareEncoder() {
	hwEncoderOnce.Do(func() {
		cmd := exec.Command(s.ffmpeg, "-hide_banner", "-encoders")
		out, err := cmd.Output()
		if err != nil {
			log.Printf("detect hardware encoder: %v", err)
			return
		}
		output := string(out)
		for _, enc := range []string{"h264_vaapi", "h264_nvenc", "h264_qsv", "h264_amf", "h264_videotoolbox"} {
			if strings.Contains(output, enc) && s.validateEncoder(enc) {
				hwEncoder = enc
				log.Printf("detected and validated hardware encoder: %s", enc)
				return
			}
		}
		log.Printf("no usable hardware encoder found, using software encoding")
	})
}

func (s *RecorderService) GetHardwareEncoder() string {
	s.detectHardwareEncoder()
	return hwEncoder
}

func isProcessKilled(err error) bool {
	if exitErr, ok := err.(*exec.ExitError); ok {
		code := exitErr.ExitCode()
		// -1 = killed by signal (Unix)
		// 1 = killed by Process.Kill() / TerminateProcess  (Windows)
		// uint32(code) == 0xFFFFFFEA = forced termination (Windows, works on both 32/64-bit)
		return code == -1 || code == 1 || uint32(code) == 0xFFFFFFEA
	}
	return false
}
