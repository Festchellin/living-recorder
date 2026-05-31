package handlers

import (
	"errors"
	"fmt"
	"strings"

	"living-recorder/backend/models"

	"gorm.io/gorm"
)

type exportRequest struct {
	Format string `json:"format" binding:"required"`
	IDs    []uint `json:"ids"`
}

type importStream struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Protocol  string `json:"protocol"`
	Enabled   bool   `json:"enabled"`
	GroupPath string `json:"group_path"`
	Remark    string `json:"remark"`
}

type importResult struct {
	Success int           `json:"success"`
	Skipped int           `json:"skipped"`
	Errors  []importError `json:"errors,omitempty"`
}

type importError struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

var validProtocols = map[string]bool{"rtsp": true, "rtmp": true, "flv": true, "hls": true}

func (h *StreamHandler) getGroupPath(group *models.Group) string {
	parts := []string{group.Name}
	current := group
	for current.ParentID != nil {
		var parent models.Group
		if err := h.db.First(&parent, *current.ParentID).Error; err != nil {
			break
		}
		parts = append([]string{parent.Name}, parts...)
		current = &parent
	}
	return strings.Join(parts, "/")
}

func (h *StreamHandler) findOrCreateGroupPath(path string) (*models.Group, error) {
	segments := strings.Split(path, "/")
	var parentID *uint
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		var group models.Group
		err := h.db.Where("name = ? AND (parent_id = ? OR parent_id IS NULL AND ? IS NULL)", seg, parentID, parentID).First(&group).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			group = models.Group{Name: seg, ParentID: parentID}
			if err := h.db.Create(&group).Error; err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}
		pid := group.ID
		parentID = &pid
	}
	if parentID == nil {
		return nil, fmt.Errorf("empty group path: %q", path)
	}
	var result models.Group
	if err := h.db.First(&result, *parentID).Error; err != nil {
		return nil, err
	}
	return &result, nil
}
