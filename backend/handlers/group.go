package handlers

import (
	"net/http"
	"strconv"

	"living-recorder/backend/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type GroupHandler struct {
	db *gorm.DB
}

func NewGroupHandler(db *gorm.DB) *GroupHandler {
	return &GroupHandler{db: db}
}

func (h *GroupHandler) List(c *gin.Context) {
	var groups []models.Group
	h.db.Order("sort_order asc").Find(&groups)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": groups})
}

func (h *GroupHandler) Create(c *gin.Context) {
	var input struct {
		Name     string `json:"name" binding:"required"`
		ParentID *uint  `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": err.Error()})
		return
	}

	var maxOrder int64
	query := h.db.Model(&models.Group{}).Select("COALESCE(MAX(sort_order), 0)")
	if input.ParentID != nil {
		query = query.Where("parent_id = ?", *input.ParentID)
	} else {
		query = query.Where("parent_id IS NULL")
	}
	query.Scan(&maxOrder)

	group := models.Group{
		Name:      input.Name,
		ParentID:  input.ParentID,
		SortOrder: int(maxOrder) + 1,
	}
	if err := h.db.Create(&group).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": 0, "data": group})
}

func (h *GroupHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var group models.Group
	if err := h.db.First(&group, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "message": "group not found"})
		return
	}

	var input struct {
		Name     *string `json:"name"`
		ParentID *int    `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if input.Name != nil {
		updates["name"] = *input.Name
	}
	if input.ParentID != nil {
		if *input.ParentID == 0 {
			var maxOrder int64
			h.db.Model(&models.Group{}).Select("COALESCE(MAX(sort_order), 0)").
				Where("parent_id IS NULL").Scan(&maxOrder)
			updates["parent_id"] = nil
			updates["sort_order"] = int(maxOrder) + 1
		} else {
			pid := uint(*input.ParentID)
			var maxOrder int64
			h.db.Model(&models.Group{}).Select("COALESCE(MAX(sort_order), 0)").
				Where("parent_id = ?", pid).Scan(&maxOrder)
			updates["parent_id"] = pid
			updates["sort_order"] = int(maxOrder) + 1
		}
	}

	if len(updates) > 0 {
		if err := h.db.Model(&group).Updates(updates).Error; err != nil {
			c.JSON(http.StatusConflict, gin.H{"code": 1, "message": err.Error()})
			return
		}
	}

	h.db.First(&group, id)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": group})
}

func (h *GroupHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var allIDs []uint
	h.collectDescendantIDs(uint(id), &allIDs)
	allIDs = append(allIDs, uint(id))

	h.db.Model(&models.Stream{}).Where("group_id IN ?", allIDs).Update("group_id", nil)
	h.db.Delete(&models.Group{}, allIDs)

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}

func (h *GroupHandler) collectDescendantIDs(parentID uint, ids *[]uint) {
	var children []models.Group
	h.db.Where("parent_id = ?", parentID).Find(&children)
	for _, child := range children {
		*ids = append(*ids, child.ID)
		h.collectDescendantIDs(child.ID, ids)
	}
}

func (h *GroupHandler) Reorder(c *gin.Context) {
	var input struct {
		ParentID *uint  `json:"parent_id"`
		Order    []uint `json:"order" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": err.Error()})
		return
	}

	for i, id := range input.Order {
		h.db.Model(&models.Group{}).Where("id = ?", id).Update("sort_order", i+1)
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "reordered"})
}
