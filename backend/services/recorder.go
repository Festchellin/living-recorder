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

	"golang.org/x/sys/unix"
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
	stderrDone chan struct{}
	done       chan struct{}
}

type StatusChangeCallback func(streamID uint)

type RecorderService struct {
	db             *gorm.DB
	cfg            *config.RecorderConfig
	ffmpeg         string
	store          StorageBackend
	storageType    string
	log            *LogWriter
	mu             sync.RWMutex
	streams        map[uint]*StreamProcess
	onStatusChange StatusChangeCallback
}

func NewRecorderService(db *gorm.DB, cfg *config.RecorderConfig, ffmpegPath string, store StorageBackend) *RecorderService {
	return &RecorderService{
		db:          db,
		cfg:         cfg,
		ffmpeg:      ffmpegPath,
		store:       store,
		storageType: "local",
		log:         NewLogWriter(db),
		streams:     make(map[uint]*StreamProcess),
	}
}

func (s *RecorderService) getDiskFreeGB(path string) float64 {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return -1
	}
	return float64(stat.Bavail*uint64(stat.Bsize)) / (1 << 30)
}

func (s *RecorderService) getRetryCount(streamID uint) int {
	var stream models.Stream
	if err := s.db.Select("retry_count").First(&stream, streamID).Error; err != nil {
		return 0
	}
	return stream.RetryCount
}

func (s *RecorderService) setRetryCount(streamID uint, count int) {
	s.db.Model(&models.Stream{}).Where("id = ?", streamID).Update("retry_count", count)
}

func (s *RecorderService) streamRetryCount(streamID uint) int {
	return s.getRetryCount(streamID)
}

func (s *RecorderService) OnStatusChange(cb StatusChangeCallback) {
	s.onStatusChange = cb
}

func (s *RecorderService) SetStore(storageType string, store StorageBackend) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.storageType = storageType
	s.store = store
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

	free := s.getDiskFreeGB(s.cfg.StorageLocalPath)
	if free >= 0 && free < 1.0 {
		return fmt.Errorf("磁盘空间不足: %.1fGB 可用，需至少 1GB", free)
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

	if task.AudioCodec == "copy" {
		codec, err := s.probeAudioCodec(&stream)
		if err == nil && codec != "" && !isAudioCodecMP4Compatible(codec) {
			log.Printf("[recorder] stream %d: audio codec %q not compatible with mp4, falling back to aac", streamID, codec)
			task.AudioCodec = "aac"
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
		stderrDone: make(chan struct{}),
		done:       make(chan struct{}),
	}
	s.streams[streamID] = sp

	s.db.Model(&stream).Update("status", "recording")

	s.log.StreamInfo(streamID, models.EventRecordingStarted, "开始录制: %s", stream.Name)

	if err := cmd.Start(); err != nil {
		delete(s.streams, streamID)
		s.db.Model(&stream).Update("status", "idle")
		return fmt.Errorf("start ffmpeg: %w", err)
	}

	go func() {
		io.Copy(&sp.stderrBuf, stderr)
		close(sp.stderrDone)
	}()
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

	if sp.stdin != nil {
		sp.stdin.Write([]byte("q\n"))
		sp.stdin.Close()
	}

	select {
	case <-sp.done:
	case <-time.After(5 * time.Second):
		sp.Cmd.Process.Kill()
		<-sp.done
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

func (s *RecorderService) FFprobePath() string {
	dir := filepath.Dir(s.ffmpeg)
	base := filepath.Base(s.ffmpeg)
	ffprobeBase := strings.Replace(base, "ffmpeg", "ffprobe", 1)
	if dir == "." {
		return ffprobeBase
	}
	return filepath.Join(dir, ffprobeBase)
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

	cmd := exec.CommandContext(ctx, s.FFprobePath(), args...)
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

	limit := s.cfg.MaxParallel
	if limit < 1 {
		limit = 1
	}

	result := BatchResult{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, limit)

	for i := range streams {
		if s.IsRecording(streams[i].ID) {
			continue
		}
		wg.Add(1)
		go func(stream models.Stream) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if !s.IsReachable(&stream) {
				errMsg := "信号源不可达，已跳过"
				mu.Lock()
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", stream.Name, errMsg))
				mu.Unlock()
				s.log.StreamWarn(stream.ID, models.EventStreamProbe, "全部启动跳过 — %s: %s", stream.Name, errMsg)
				return
			}
			if err := s.Start(stream.ID, nil); err != nil {
				mu.Lock()
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", stream.Name, err))
				mu.Unlock()
				return
			}
			mu.Lock()
			result.Success++
			mu.Unlock()
		}(streams[i])
	}

	wg.Wait()
	return result
}

func (s *RecorderService) StopAll() BatchResult {
	s.mu.RLock()
	active := make([]uint, 0, len(s.streams))
	for id := range s.streams {
		active = append(active, id)
	}
	s.mu.RUnlock()

	limit := s.cfg.MaxParallel
	if limit < 1 {
		limit = 1
	}

	result := BatchResult{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, limit)

	for _, id := range active {
		wg.Add(1)
		go func(id uint) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := s.Stop(id); err != nil {
				var stream models.Stream
				mu.Lock()
				if s.db.First(&stream, id).Error == nil {
					result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", stream.Name, err))
				} else {
					result.Errors = append(result.Errors, fmt.Sprintf("stream %d: %v", id, err))
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			result.Success++
			mu.Unlock()
		}(id)
	}

	wg.Wait()
	return result
}

func (s *RecorderService) buildFFmpegArgs(stream *models.Stream, task *models.RecordTask) []string {
	args := []string{}

	switch stream.Protocol {
	case "rtsp":
		args = append(args, "-rtsp_transport", "tcp")
		args = append(args, "-rtsp_flags", "prefer_tcp")
	case "rtmp", "flv":
		args = append(args, "-fflags", "+nobuffer")
	case "hls":
	default:
	}

	args = append(args, "-analyzeduration", "100M")
	args = append(args, "-probesize", "100M")
	args = append(args, "-i", stream.URL)
	if task.VideoCodec != "copy" {
		args = append(args, "-vf", "setparams=color_primaries=bt709:color_trc=bt709:colorspace=bt709")
		args = append(args, "-pix_fmt", "yuv420p")
	}
	args = append(args, "-c:v", task.VideoCodec)
	if task.VideoCodec == "libx264" {
		args = append(args, "-preset", "ultrafast")
		args = append(args, "-tune", "zerolatency")
	}

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

	segSec := task.SegmentSec
	if segSec <= 0 {
		segSec = s.cfg.SegmentDuration
	}
	if segSec > 0 {
		args = append(args, "-f", "segment")
		args = append(args, "-segment_time", fmt.Sprintf("%d", segSec))
		args = append(args, "-reset_timestamps", "1")
	} else {
		args = append(args, "-movflags", "+frag_keyframe+empty_moov")
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

	select {
	case <-sp.stderrDone:
	case <-time.After(3 * time.Second):
		log.Printf("[recorder] stream %d: stderr timeout, continuing", sp.StreamID)
	}

	s.mu.Lock()
	delete(s.streams, sp.StreamID)
	s.mu.Unlock()

	s.db.Model(&models.Stream{}).Where("id = ?", sp.StreamID).Update("status", "idle")

	fileSize := int64(0)
	if fi, statErr := os.Stat(sp.OutputPath); statErr == nil {
		fileSize = fi.Size()
	}
	duration := int(endedAt.Sub(sp.StartedAt).Seconds())

	if sp.stderrBuf.Len() > 0 {
		log.Printf("[recorder] ffmpeg stderr for stream %d (%s):\n%s", sp.StreamID, sp.StreamName, sp.stderrBuf.String())
	}

	status := "failed"
	errMsg := ""
	eventType := models.EventRecordingFailed
	msg := fmt.Sprintf("录制结束: %s (时长 %d秒)", sp.StreamName, duration)

	if fileSize > 0 {
		msg += fmt.Sprintf(", 大小 %.1fMB", float64(fileSize)/1024/1024)
	}

	if err == nil {
		status = "success"
		eventType = models.EventRecordingStopped
		msg = fmt.Sprintf("录制结束: %s (时长 %d秒)", sp.StreamName, duration)
	} else if sp.userStopped.Load() {
		status = "success"
		eventType = models.EventRecordingStopped
		msg = fmt.Sprintf("录制已停止: %s (时长 %d秒)", sp.StreamName, duration)
	} else if isProcessKilled(err) {
		status = "failed"
		sig := exitSignal(err)
		if name, ok := signalNames[sig]; ok {
			errMsg = fmt.Sprintf("进程崩溃 (signal %d %s)", sig, name)
		} else {
			errMsg = fmt.Sprintf("进程崩溃 (signal %d)", sig)
		}
		eventType = models.EventRecordingFailed
		msg = fmt.Sprintf("录制失败: %s — %s", sp.StreamName, errMsg)
		log.Printf("[recorder] stream %d (%s) 进程异常退出, sig=%d", sp.StreamID, sp.StreamName, sig)
	} else {
		status = "failed"
		if exitErr, ok := err.(*exec.ExitError); ok {
			code := exitErr.ExitCode()
			sig := exitSignal(err)
			if sig != 0 {
				if name, ok := signalNames[sig]; ok {
					errMsg = fmt.Sprintf("进程崩溃 (signal %d %s)", sig, name)
				} else {
					errMsg = fmt.Sprintf("进程崩溃 (signal %d)", sig)
				}
			} else {
				errMsg = fmt.Sprintf("exit code %d", code)
			}
		} else {
			errMsg = err.Error()
		}
		eventType = models.EventRecordingFailed
		msg = fmt.Sprintf("录制失败: %s — %s", sp.StreamName, errMsg)
	}

	if status == "success" && s.storageType == "s3" && fileSize > 0 {
		destPath := strings.TrimPrefix(sp.OutputPath, s.cfg.StorageLocalPath)
		destPath = strings.TrimPrefix(destPath, "/")
		destPath = strings.TrimPrefix(destPath, "\\")
		go func(fp string) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			if err := s.store.Save(ctx, fp, destPath); err != nil {
				log.Printf("[recorder] stream %d: s3 upload failed: %v", sp.StreamID, err)
				return
			}
			os.Remove(fp)
			log.Printf("[recorder] stream %d: s3 upload completed, removed local file", sp.StreamID)
		}(sp.OutputPath)
	}

	s.log.StreamRecordingLog(sp.StreamID, eventType, status, msg, errMsg, sp.OutputPath, fileSize, duration, sp.StartedAt, endedAt)

	if s.onStatusChange != nil {
		go s.onStatusChange(sp.StreamID)
	}

	close(sp.done)

	if status == "success" {
		s.setRetryCount(sp.StreamID, 0)
	}

	if status == "failed" && s.cfg.RestartOnFailure > 0 && !sp.userStopped.Load() {
		attempt := s.streamRetryCount(sp.StreamID) + 1
		if attempt > s.cfg.RestartOnFailure {
			log.Printf("[recorder] stream %d (%s): 已达最大重试次数 %d，放弃重连",
				sp.StreamID, sp.StreamName, s.cfg.RestartOnFailure)
			s.setRetryCount(sp.StreamID, 0)
			return
		}

		videoCodec := s.cfg.DefaultVideoCodec
		audioCodec := s.cfg.DefaultAudioCodec
		resolution := ""
		framerate := 0
		extra := ""

		if s.cfg.RetryWithReEncode && attempt >= 2 {
			if attempt == 2 {
				extra = "ultrafast 预设"
				videoCodec = "libx264"
				audioCodec = "aac"
			} else if attempt == 3 {
				extra = "ultrafast + 720p"
				videoCodec = "libx264"
				audioCodec = "aac"
				resolution = "1280x720"
			} else {
				extra = "ultrafast + 720p + 15fps"
				videoCodec = "libx264"
				audioCodec = "aac"
				resolution = "1280x720"
				framerate = 15
			}
			log.Printf("[recorder] stream %d (%s): 第 %d 次重试，降级为重新编码 (%s)",
				sp.StreamID, sp.StreamName, attempt, extra)
		}

		s.setRetryCount(sp.StreamID, attempt)

		log.Printf("[recorder] stream %d (%s): 将在 3 秒后自动重连 (第 %d 次)",
			sp.StreamID, sp.StreamName, attempt)
		time.Sleep(3 * time.Second)

		_ = s.Start(sp.StreamID, &models.RecordTask{
			StreamID:       sp.StreamID,
			OutputTemplate: s.cfg.DefaultOutputTemplate,
			VideoCodec:     videoCodec,
			AudioCodec:     audioCodec,
			Resolution:     resolution,
			Framerate:      framerate,
		})
	}
}

func (s *RecorderService) checkHealth() {
	s.mu.RLock()
	processes := make([]*StreamProcess, 0, len(s.streams))
	for _, sp := range s.streams {
		processes = append(processes, sp)
	}
	s.mu.RUnlock()

	timeout := time.Duration(s.cfg.HealthCheckTimeout) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	for _, sp := range processes {
		if sp.userStopped.Load() {
			continue
		}
		fi, err := os.Stat(sp.OutputPath)
		if err != nil {
			continue
		}
		if time.Since(fi.ModTime()) > timeout {
			log.Printf("[health] stream %d (%s): 文件 %s 超过 %v 无写入，强制重启",
				sp.StreamID, sp.StreamName, sp.OutputPath, timeout)
			if sp.Cmd != nil && sp.Cmd.Process != nil {
				sp.Cmd.Process.Kill()
			}
		}
	}
}

func (s *RecorderService) StartHealthCheck(ctx context.Context) {
	interval := s.cfg.HealthCheckInterval
	if interval <= 0 {
		interval = 60
	}
	go func() {
		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.checkHealth()
			}
		}
	}()
	log.Printf("[health] 录制健康检测已启动 (间隔=%ds, 超时=%ds)",
		interval, s.cfg.HealthCheckTimeout)
}

var signalNames = map[int]string{
	1: "SIGHUP", 2: "SIGINT", 3: "SIGQUIT", 4: "SIGILL", 6: "SIGABRT",
	7: "SIGBUS", 8: "SIGFPE", 9: "SIGKILL", 11: "SIGSEGV", 13: "SIGPIPE",
	14: "SIGALRM", 15: "SIGTERM",
}

func exitSignal(err error) int {
	if exitErr, ok := err.(*exec.ExitError); ok {
		if ws, ok := exitErr.Sys().(interface{ Signal() int }); ok {
			return ws.Signal()
		}
	}
	return 0
}

var (
	hwEncoderOnce sync.Once
	hwEncoder     string

	audioCodecCache      sync.Map
	audioCodecCacheTTL   = 60 * time.Second

	incompatibleAudioCodecs = map[string]bool{
		"pcm_mulaw": true,
		"pcm_alaw":  true,
		"pcm_s16le": true,
		"pcm_s16be": true,
		"pcm_u8":    true,
		"pcm_u16le": true,
		"pcm_u16be": true,
		"pcm_f32le": true,
		"pcm_f32be": true,
		"pcm_s24le": true,
		"pcm_s32le": true,
		"adpcm_g726": true,
	}
)

func isAudioCodecMP4Compatible(codec string) bool {
	return !incompatibleAudioCodecs[codec]
}

func (s *RecorderService) probeAudioCodec(stream *models.Stream) (string, error) {
	if v, ok := audioCodecCache.Load(stream.URL); ok {
		entry := v.(struct {
			codec string
			ts    time.Time
		})
		if time.Since(entry.ts) < audioCodecCacheTTL {
			return entry.codec, nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := []string{"-v", "quiet", "-print_format", "json", "-show_streams"}
	if stream.Protocol == "rtsp" {
		args = append(args, "-rtsp_transport", "tcp")
	}
	args = append(args, "-i", stream.URL)

	cmd := exec.CommandContext(ctx, s.FFprobePath(), args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	var result struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return "", err
	}

	codec := ""
	for _, s := range result.Streams {
		if s.CodecType == "audio" {
			codec = s.CodecName
			break
		}
	}

	audioCodecCache.Store(stream.URL, struct {
		codec string
		ts    time.Time
	}{codec, time.Now()})

	return codec, nil
}

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
		return exitErr.ExitCode() == -1
	}
	return false
}
