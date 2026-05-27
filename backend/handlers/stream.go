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
