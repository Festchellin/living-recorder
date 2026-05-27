package handlers

import (
	"living-recorder/backend/models"
	"living-recorder/backend/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TaskHandler struct {
	db        *gorm.DB
	scheduler *services.SchedulerService
}

func NewTaskHandler(db *gorm.DB, scheduler *services.SchedulerService) *TaskHandler {
	return &TaskHandler{db: db, scheduler: scheduler}
}

func (h *TaskHandler) List(c *gin.Context) {
	var tasks []models.RecordTask
	h.db.Preload("Stream").Find(&tasks)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": tasks})
}

func (h *TaskHandler) Create(c *gin.Context) {
	var task models.RecordTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": err.Error()})
		return
	}
	h.db.Create(&task)
	h.scheduler.AddTask(&task)
	c.JSON(http.StatusCreated, gin.H{"code": 0, "data": task})
}

func (h *TaskHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var task models.RecordTask
	if err := h.db.First(&task, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "message": "task not found"})
		return
	}
	var input models.RecordTask
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": err.Error()})
		return
	}
	h.db.Model(&task).Updates(input)
	h.db.First(&task, id)
	h.scheduler.ReloadTask(&task)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": task})
}

func (h *TaskHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	h.scheduler.RemoveTask(uint(id))
	h.db.Delete(&models.RecordTask{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}
