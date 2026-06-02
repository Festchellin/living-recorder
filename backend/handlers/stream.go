package handlers

import (
	"context"
	"encoding/json"
	"living-recorder/backend/models"
	"living-recorder/backend/services"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

type StreamHandler struct {
	db         *gorm.DB
	recorder   *services.RecorderService
	previewMgr *services.PreviewManager
	log        *services.LogWriter
}

func NewStreamHandler(db *gorm.DB, recorder *services.RecorderService, previewMgr *services.PreviewManager) *StreamHandler {
	return &StreamHandler{db: db, recorder: recorder, previewMgr: previewMgr, log: services.NewLogWriter(db)}
}

func (h *StreamHandler) List(c *gin.Context) {
	var streams []models.Stream
	query := h.db.Preload("Group")
	if groupID := c.Query("group_id"); groupID != "" {
		query = query.Where("group_id = ?", groupID)
	}
	query.Find(&streams)
	for i := range streams {
		streams[i].Status = h.recorder.EffectiveStreamStatus(&streams[i])
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": streams})
}

func (h *StreamHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var stream models.Stream
	if err := h.db.Preload("Group").First(&stream, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "message": "stream not found"})
		return
	}
	stream.Status = h.recorder.EffectiveStreamStatus(&stream)
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
	h.db.Preload("Group").First(&stream, stream.ID)
	h.log.StreamInfo(stream.ID, models.EventStreamCreated, "创建流媒体: %s (%s)", stream.Name, stream.URL)
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
	h.log.StreamInfo(stream.ID, models.EventStreamUpdated, "更新流媒体: %s", stream.Name)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": stream})
}

func (h *StreamHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var stream models.Stream
	if err := h.db.First(&stream, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "message": "stream not found"})
		return
	}
	if h.recorder.IsRecording(uint(id)) {
		c.JSON(http.StatusConflict, gin.H{"code": 1, "message": "stream is recording, stop first"})
		return
	}
	h.db.Delete(&models.Stream{}, id)
	h.log.StreamInfo(stream.ID, models.EventStreamDeleted, "删除流媒体: %s", stream.Name)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}

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

func (h *StreamHandler) StartAll(c *gin.Context) {
	result := h.recorder.StartAll()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

func (h *StreamHandler) StopAll(c *gin.Context) {
	result := h.recorder.StopAll()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
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

func (h *StreamHandler) RecentLogs(c *gin.Context) {
	var logs []models.RecordLog
	h.db.Preload("Stream").Order("started_at desc").Limit(100).Find(&logs)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": logs})
}

func (h *StreamHandler) Probe(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var stream models.Stream
	if err := h.db.First(&stream, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "message": "stream not found"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := []string{"-v", "quiet", "-print_format", "json", "-show_streams"}
	if stream.Protocol == "rtsp" {
		args = append(args, "-rtsp_transport", "tcp")
	}
	args = append(args, "-i", stream.URL)

	cmd := exec.CommandContext(ctx, h.recorder.FFprobePath(), args...)
	output, err := cmd.Output()

	if err != nil {
		h.log.StreamWarn(stream.ID, models.EventStreamProbe, "信号源不可达: %s (%s)", stream.Name, stream.URL)
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"reachable": false}})
		return
	}

	var info struct {
		Streams []interface{} `json:"streams"`
	}
	if json.Unmarshal(output, &info) == nil && len(info.Streams) > 0 {
		h.log.StreamInfo(stream.ID, models.EventStreamProbe, "信号源可达: %s (%s)", stream.Name, stream.URL)
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"reachable": true}})
	} else {
		h.log.StreamWarn(stream.ID, models.EventStreamProbe, "信号源无流数据: %s", stream.Name)
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"reachable": false}})
	}
}

func (h *StreamHandler) ProbeInfo(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var stream models.Stream
	if err := h.db.First(&stream, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "message": "stream not found"})
		return
	}

	info := probeSource(h.recorder.FFprobePath(), stream.URL, stream.Protocol)
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"width":  info.width,
			"height": info.height,
			"fps":    info.fps,
		},
	})
}

func (h *StreamHandler) PreviewWS(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var stream models.Stream
	if err := h.db.First(&stream, id).Error; err != nil {
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("preview ws upgrade failed: %v", err)
		return
	}

	q := c.Request.URL.Query()
	cfg := services.PreviewConfig{
		Width:  defaultInt(q.Get("width"), 640),
		Height: defaultInt(q.Get("height"), 360),
		FPS:    defaultInt(q.Get("fps"), 10),
		CRF:    defaultInt(q.Get("crf"), 35),
	}

	// WS 超时保护
	conn.SetReadLimit(512)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}()

	if err := h.previewMgr.Subscribe(uint(id), stream.URL, stream.Protocol, conn, cfg); err != nil {
		log.Printf("preview subscribe failed: %v", err)
		return
	}
}

type streamInfo struct {
	width  int
	height int
	fps    int
}

type cachedProbe struct {
	info streamInfo
	ts   time.Time
}

var (
	probeCache    sync.Map
	probeCacheTTL = 60 * time.Second
)

func probeSource(ffprobePath, url, protocol string) streamInfo {
	if v, ok := probeCache.Load(url); ok {
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

	cmd := exec.CommandContext(ctx, ffprobePath, args...)
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
			probeCache.Store(url, cachedProbe{info: info, ts: time.Now()})
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

func defaultInt(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return def
	}
	return v
}
