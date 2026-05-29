package handlers

import (
	"context"
	"encoding/json"
	"io"
	"living-recorder/backend/models"
	"living-recorder/backend/services"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

type StreamHandler struct {
	db       *gorm.DB
	recorder *services.RecorderService
	log      *services.LogWriter
}

func NewStreamHandler(db *gorm.DB, recorder *services.RecorderService) *StreamHandler {
	return &StreamHandler{db: db, recorder: recorder, log: services.NewLogWriter(db)}
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

	cmd := exec.CommandContext(ctx, "ffprobe", args...)
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
	defer conn.Close()

	hwEnc := h.recorder.GetHardwareEncoder()
	args := []string{}
	if stream.Protocol == "rtsp" {
		args = append(args, "-rtsp_transport", "tcp")
	}
	if hwEnc == "h264_vaapi" {
		args = append(args, "-vaapi_device", "/dev/dri/renderD128")
	}
	args = append(args, "-i", stream.URL)
	args = append(args, "-fflags", "nobuffer")
	args = append(args, "-flags", "low_delay")

	switch hwEnc {
	case "h264_vaapi":
		args = append(args, "-c:v", "h264_vaapi")
		args = append(args, "-vf", "scale=-2:360,format=nv12,hwupload")
	case "h264_nvenc":
		args = append(args, "-c:v", "h264_nvenc")
		args = append(args, "-preset", "p1")
		args = append(args, "-vf", "scale=-2:360")
	case "h264_qsv":
		args = append(args, "-c:v", "h264_qsv")
		args = append(args, "-preset", "1")
		args = append(args, "-global_quality", "35")
		args = append(args, "-vf", "scale=-2:360")
	case "h264_amf":
		args = append(args, "-c:v", "h264_amf")
		args = append(args, "-quality", "speed")
		args = append(args, "-vf", "scale=-2:360")
	case "h264_videotoolbox":
		args = append(args, "-c:v", "h264_videotoolbox")
		args = append(args, "-encoder", "speed")
		args = append(args, "-vf", "scale=-2:360")
	default:
		args = append(args, "-c:v", "libx264")
		args = append(args, "-preset", "ultrafast")
		args = append(args, "-tune", "zerolatency")
		args = append(args, "-vf", "scale=-2:360")
	}
	args = append(args, "-r", "10")
	args = append(args, "-crf", "35")
	args = append(args, "-c:a", "aac")
	args = append(args, "-f", "mpegts")
	args = append(args, "-flush_packets", "1")
	args = append(args, "-")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, h.recorder.FFmpegPath(), args...)
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		log.Printf("preview stream %d ffmpeg start failed: %v", stream.ID, err)
		return
	}

	go func() {
		errBytes, _ := io.ReadAll(stderr)
		if len(errBytes) > 0 {
			log.Printf("preview stream %d ffmpeg stderr: %s", stream.ID, string(errBytes))
		}
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 65536)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				if wsErr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); wsErr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// Block until client disconnects or ffmpeg exits
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}

	cancel()
	cmd.Process.Kill()
	<-done
	cmd.Wait()
}
