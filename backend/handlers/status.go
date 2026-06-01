package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"living-recorder/backend/config"
	"living-recorder/backend/models"
	"living-recorder/backend/services"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
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
	s3Configured := h.cfg.Storage.S3.Endpoint != "" &&
		h.cfg.Storage.S3.AccessKey != "" &&
		h.cfg.Storage.S3.SecretKey != "" &&
		h.cfg.Storage.S3.Bucket != ""

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"ffmpeg_path":            h.cfg.FFmpeg.Path,
		"storage_default":        h.cfg.Storage.Default,
		"storage_local_path":     h.cfg.Storage.Local.Path,
		"storage_s3_endpoint":    h.cfg.Storage.S3.Endpoint,
		"storage_s3_bucket":      h.cfg.Storage.S3.Bucket,
		"storage_s3_region":      h.cfg.Storage.S3.Region,
		"storage_s3_configured":  s3Configured,
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
		StorageDefault      *string `json:"storage_default"`
		StorageS3Endpoint   *string `json:"storage_s3_endpoint"`
		StorageS3AccessKey  *string `json:"storage_s3_access_key"`
		StorageS3SecretKey  *string `json:"storage_s3_secret_key"`
		StorageS3Bucket     *string `json:"storage_s3_bucket"`
		StorageS3Region     *string `json:"storage_s3_region"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": err.Error()})
		return
	}

	storageChanged := false

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
	if input.StorageDefault != nil {
		h.cfg.Storage.Default = *input.StorageDefault
		storageChanged = true
	}
	if input.StorageS3Endpoint != nil {
		h.cfg.Storage.S3.Endpoint = *input.StorageS3Endpoint
		storageChanged = true
	}
	if input.StorageS3AccessKey != nil {
		h.cfg.Storage.S3.AccessKey = *input.StorageS3AccessKey
		storageChanged = true
	}
	if input.StorageS3SecretKey != nil {
		h.cfg.Storage.S3.SecretKey = *input.StorageS3SecretKey
		storageChanged = true
	}
	if input.StorageS3Bucket != nil {
		h.cfg.Storage.S3.Bucket = *input.StorageS3Bucket
		storageChanged = true
	}
	if input.StorageS3Region != nil {
		h.cfg.Storage.S3.Region = *input.StorageS3Region
		storageChanged = true
	}

	if storageChanged {
		var store services.StorageBackend
		switch h.cfg.Storage.Default {
		case "s3":
			s3store, err := services.NewS3Storage(h.cfg.Storage.S3)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "S3 配置无效: " + err.Error()})
				return
			}
			store = s3store
		default:
			store = services.NewLocalStorage(h.cfg.Storage.Local)
		}
		h.recorder.SetStore(h.cfg.Storage.Default, store)
	}

	if err := h.cfg.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "save config: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "config updated"})
}

func (h *StatusHandler) TestS3Connection(c *gin.Context) {
	var input struct {
		Endpoint  string `json:"endpoint"`
		AccessKey string `json:"access_key"`
		SecretKey string `json:"secret_key"`
		Bucket    string `json:"bucket"`
		Region    string `json:"region"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": err.Error()})
		return
	}

	endpoint := input.Endpoint
	secure := true
	if strings.Contains(endpoint, "://") {
		secure = strings.HasPrefix(endpoint, "https://")
		endpoint = strings.TrimPrefix(endpoint, "https://")
		endpoint = strings.TrimPrefix(endpoint, "http://")
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(input.AccessKey, input.SecretKey, ""),
		Secure: secure,
		Region: input.Region,
	})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "data": gin.H{"message": "创建连接失败: " + err.Error()}})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := client.BucketExists(ctx, input.Bucket)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "data": gin.H{"message": "连接失败: " + err.Error()}})
		return
	}
	if !exists {
		c.JSON(http.StatusOK, gin.H{"code": 1, "data": gin.H{"message": "Bucket 不存在"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"message": "连接成功"}})
}
