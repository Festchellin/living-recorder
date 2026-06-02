package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type PreviewConfig struct {
	Width  int
	Height int
	FPS    int
	CRF    int
}

type PreviewManager struct {
	mu        sync.RWMutex
	streams   map[uint]*PreviewStream
	ffmpeg    string
	ffprobe   string
	hwEncoder string
	probeCache sync.Map
}

type cachedProbe struct {
	info streamInfo
	ts   time.Time
}

const (
	probeCacheTTL   = 60 * time.Second
	previewSubChanCap = 8
	previewReadBufSize = 65536
)

func NewPreviewManager(ffmpeg, ffprobe, hwEncoder string) *PreviewManager {
	return &PreviewManager{
		streams:   make(map[uint]*PreviewStream),
		ffmpeg:    ffmpeg,
		ffprobe:   ffprobe,
		hwEncoder: hwEncoder,
	}
}

func (pm *PreviewManager) Subscribe(streamID uint, url, protocol string, conn *websocket.Conn, cfg PreviewConfig) error {
	conn.SetReadLimit(512)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	pm.mu.Lock()
	ps, exists := pm.streams[streamID]
	if !exists {
		ps = &PreviewStream{
			streamID:    streamID,
			subscribers: make(map[*websocket.Conn]chan []byte),
			done:        make(chan struct{}),
			ready:       make(chan struct{}),
			cancel:      func() {},
		}
		pm.streams[streamID] = ps
	}
	pm.mu.Unlock()

	if !exists {
		needCancel := true
		defer func() {
			if needCancel {
				close(ps.ready)
			}
		}()

		info := pm.probeSource(url, protocol)
		if cfg.Width <= 0 {
			cfg.Width = 640
		}
		if cfg.Height <= 0 {
			cfg.Height = 360
		}
		if cfg.FPS <= 0 {
			cfg.FPS = 10
		}
		if cfg.CRF <= 0 {
			cfg.CRF = 35
		}
		if info.width > 0 {
			if cfg.Width > info.width || cfg.Height > info.height {
				cfg.Width = min(cfg.Width, info.width)
				cfg.Height = min(cfg.Height, info.height)
			}
			if cfg.FPS > 0 && info.fps > 0 && cfg.FPS > info.fps {
				cfg.FPS = info.fps
			}
		}

		args := buildFFmpegArgs(url, protocol, cfg, pm.hwEncoder)
		ctx, cancel := context.WithCancel(context.Background())
		cmd := exec.CommandContext(ctx, pm.ffmpeg, args...)

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			cancel()
			pm.mu.Lock()
			delete(pm.streams, streamID)
			pm.mu.Unlock()
			return fmt.Errorf("stdout pipe: %w", err)
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			cancel()
			pm.mu.Lock()
			delete(pm.streams, streamID)
			pm.mu.Unlock()
			return fmt.Errorf("stderr pipe: %w", err)
		}

		if err := cmd.Start(); err != nil {
			cancel()
			pm.mu.Lock()
			delete(pm.streams, streamID)
			pm.mu.Unlock()
			return fmt.Errorf("ffmpeg start: %w", err)
		}

		ps.cmd = cmd
		ps.cancel = cancel
		ps.url = url
		ps.protocol = protocol
		needCancel = false
		close(ps.ready)

		go func() {
			errBytes, _ := io.ReadAll(stderr)
			if len(errBytes) > 0 {
				log.Printf("[preview] stream %d ffmpeg stderr: %s", streamID, string(errBytes))
			}
		}()
		go ps.readLoop(stdout)
	} else {
		<-ps.ready
		pm.mu.RLock()
		_, stillExists := pm.streams[streamID]
		pm.mu.RUnlock()
		if !stillExists {
			return fmt.Errorf("stream %d: ffmpeg failed to start", streamID)
		}
	}

	ch := make(chan []byte, previewSubChanCap)
	ps.mu.Lock()
	ps.subscribers[conn] = ch
	atomic.AddInt32(&ps.refCount, 1)
	ps.mu.Unlock()

	go pm.writeLoop(conn, ch, ps.done)
	go func() {
		defer pm.Unsubscribe(streamID, conn)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	return nil
}

func (pm *PreviewManager) Unsubscribe(streamID uint, conn *websocket.Conn) {
	pm.mu.Lock()
	ps, exists := pm.streams[streamID]
	if !exists {
		pm.mu.Unlock()
		return
	}

	ps.mu.Lock()
	ch, ok := ps.subscribers[conn]
	if ok {
		delete(ps.subscribers, conn)
		close(ch)
	}
	ps.mu.Unlock()

	if ok {
		if atomic.AddInt32(&ps.refCount, -1) <= 0 {
			ps.cancel()
			delete(pm.streams, streamID)
			pm.mu.Unlock()

			<-ps.done
			ps.cmd.Wait()
			return
		}
	}

	pm.mu.Unlock()
}

func (pm *PreviewManager) writeLoop(conn *websocket.Conn, ch chan []byte, done chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	defer conn.Close()

	for {
		select {
		case data, ok := <-ch:
			if !ok {
				return
			}
			if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				return
			}
			if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
				return
			}
		case <-ticker.C:
			if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				return
			}
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}

func (pm *PreviewManager) StopAll() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for id, ps := range pm.streams {
		ps.mu.Lock()
		for conn, ch := range ps.subscribers {
			delete(ps.subscribers, conn)
			close(ch)
		}
		ps.mu.Unlock()
		ps.cancel()
		delete(pm.streams, id)
	}
}

type PreviewStream struct {
	streamID    uint
	url         string
	protocol    string
	cmd         *exec.Cmd
	cancel      context.CancelFunc
	refCount    int32
	subscribers map[*websocket.Conn]chan []byte
	mu          sync.Mutex
	done        chan struct{}
	ready       chan struct{}
}

func (ps *PreviewStream) readLoop(stdout io.Reader) {
	defer close(ps.done)
	buf := make([]byte, previewReadBufSize)
	for {
		n, err := stdout.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			ps.mu.Lock()
			for _, ch := range ps.subscribers {
				select {
				case ch <- data:
				default:
				}
			}
			ps.mu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

func buildFFmpegArgs(url, protocol string, cfg PreviewConfig, hwEncoder string) []string {
	args := []string{}
	if protocol == "rtsp" {
		args = append(args, "-rtsp_transport", "tcp")
	}
	if hwEncoder == "h264_vaapi" {
		args = append(args, "-vaapi_device", "/dev/dri/renderD128")
	}
	args = append(args, "-i", url)
	args = append(args, "-fflags", "nobuffer")
	args = append(args, "-flags", "low_delay")

	switch hwEncoder {
	case "h264_vaapi":
		args = append(args, "-c:v", "h264_vaapi")
		args = append(args, "-vf", fmt.Sprintf("scale=%d:%d,format=nv12,hwupload", cfg.Width, cfg.Height))
	case "h264_nvenc":
		args = append(args, "-c:v", "h264_nvenc")
		args = append(args, "-preset", "p1")
		args = append(args, "-vf", fmt.Sprintf("scale=%d:%d", cfg.Width, cfg.Height))
	case "h264_qsv":
		args = append(args, "-c:v", "h264_qsv")
		args = append(args, "-preset", "1")
		args = append(args, "-global_quality", fmt.Sprintf("%d", cfg.CRF))
		args = append(args, "-vf", fmt.Sprintf("scale=%d:%d", cfg.Width, cfg.Height))
	case "h264_amf":
		args = append(args, "-c:v", "h264_amf")
		args = append(args, "-quality", "speed")
		args = append(args, "-qp_i", fmt.Sprintf("%d", cfg.CRF))
		args = append(args, "-qp_p", fmt.Sprintf("%d", cfg.CRF))
		args = append(args, "-vf", fmt.Sprintf("scale=%d:%d", cfg.Width, cfg.Height))
	case "h264_videotoolbox":
		args = append(args, "-c:v", "h264_videotoolbox")
		args = append(args, "-encoder", "speed")
		args = append(args, "-vf", fmt.Sprintf("scale=%d:%d", cfg.Width, cfg.Height))
	default:
		args = append(args, "-c:v", "libx264")
		args = append(args, "-preset", "ultrafast")
		args = append(args, "-tune", "zerolatency")
		args = append(args, "-vf", fmt.Sprintf("scale=%d:%d", cfg.Width, cfg.Height))
	}
	args = append(args, "-r", fmt.Sprintf("%d", cfg.FPS))
	if hwEncoder != "h264_videotoolbox" && hwEncoder != "h264_amf" && hwEncoder != "h264_qsv" {
		args = append(args, "-crf", fmt.Sprintf("%d", cfg.CRF))
	}
	args = append(args, "-bsf:v", "dump_extra")
	args = append(args, "-c:a", "aac")
	args = append(args, "-f", "mpegts")
	args = append(args, "-flush_packets", "1")
	args = append(args, "-")
	return args
}

type streamInfo struct {
	width  int
	height int
	fps    int
}

func (pm *PreviewManager) probeSource(url, protocol string) streamInfo {
	if v, ok := pm.probeCache.Load(url); ok {
		cp := v.(cachedProbe)
		if time.Since(cp.ts) < probeCacheTTL {
			return cp.info
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	args := []string{"-v", "quiet", "-print_format", "json", "-show_streams"}
	if protocol == "rtsp" {
		args = append(args, "-rtsp_transport", "tcp")
	}
	args = append(args, "-i", url)

	cmd := exec.CommandContext(ctx, pm.ffprobe, args...)
	output, err := cmd.Output()
	if err != nil {
		return streamInfo{}
	}

	var probeResult struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			AvgFPS    string `json:"avg_frame_rate"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(output, &probeResult); err != nil {
		return streamInfo{}
	}

	for _, s := range probeResult.Streams {
		if s.CodecType == "video" {
			info := streamInfo{width: s.Width, height: s.Height, fps: parseFPS(s.AvgFPS)}
			pm.probeCache.Store(url, cachedProbe{info: info, ts: time.Now()})
			return info
		}
	}

	return streamInfo{}
}

func parseFPS(s string) int {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return 0
	}
	num, _ := strconv.Atoi(parts[0])
	den, _ := strconv.Atoi(parts[1])
	if den == 0 {
		return 0
	}
	return num / den
}
