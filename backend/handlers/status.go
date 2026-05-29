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
	var totalLogs int64
	var totalSize int64

	h.db.Model(&models.Stream{}).Count(&totalStreams)
	h.db.Model(&models.RecordLog{}).Count(&totalLogs)
	h.db.Model(&models.RecordLog{}).Select("COALESCE(SUM(file_size), 0)").Scan(&totalSize)
	activeRecordings := int64(len(h.recorder.GetActiveStreams()))

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
	if err := h.cfg.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "save config: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "config updated"})
}
