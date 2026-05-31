package handlers

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"living-recorder/backend/models"

	"github.com/xuri/excelize/v2"
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

type exportItem struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Protocol  string `json:"protocol"`
	Enabled   bool   `json:"enabled"`
	GroupPath string `json:"group_path"`
	Remark    string `json:"remark"`
}

var validProtocols = map[string]bool{"rtsp": true, "rtmp": true, "flv": true, "hls": true}

func (h *StreamHandler) toExportItems(streams []models.Stream) []exportItem {
	items := make([]exportItem, len(streams))
	for i, s := range streams {
		groupPath := ""
		if s.Group != nil {
			groupPath = h.getGroupPath(s.Group)
		}
		items[i] = exportItem{
			Name:      s.Name,
			URL:       s.URL,
			Protocol:  s.Protocol,
			Enabled:   s.Enabled,
			GroupPath: groupPath,
			Remark:    s.Remark,
		}
	}
	return items
}

func (h *StreamHandler) writeJSON(w io.Writer, streams []models.Stream) error {
	items := h.toExportItems(streams)
	return json.NewEncoder(w).Encode(items)
}

func (h *StreamHandler) writeCSV(w io.Writer, streams []models.Stream) error {
	items := h.toExportItems(streams)
	writer := csv.NewWriter(w)
	writer.Write([]string{"name", "url", "protocol", "enabled", "group_path", "remark"})
	for _, item := range items {
		writer.Write([]string{item.Name, item.URL, item.Protocol, fmt.Sprintf("%t", item.Enabled), item.GroupPath, item.Remark})
	}
	writer.Flush()
	return writer.Error()
}

func (h *StreamHandler) writeXLSX(streams []models.Stream) ([]byte, error) {
	items := h.toExportItems(streams)
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Streams"
	f.SetSheetName("Sheet1", sheet)

	headers := []string{"name", "url", "protocol", "enabled", "group_path", "remark"}
	for i, hdr := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, hdr)
	}
	for rowIdx, item := range items {
		vals := []interface{}{item.Name, item.URL, item.Protocol, item.Enabled, item.GroupPath, item.Remark}
		for colIdx, val := range vals {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			f.SetCellValue(sheet, cell, val)
		}
	}
	style, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	f.SetCellStyle(sheet, "A1", "F1", style)

	colWidths := []float64{20, 50, 10, 10, 30, 30}
	for i, w := range colWidths {
		col, _ := excelize.ColumnNumberToName(i + 1)
		f.SetColWidth(sheet, col, col, w)
	}

	var buf strings.Builder
	if _, err := f.WriteTo(&buf); err != nil {
		return nil, err
	}
	return []byte(buf.String()), nil
}

func (h *StreamHandler) writeTXT(w io.Writer, streams []models.Stream) error {
	items := h.toExportItems(streams)
	enc := json.NewEncoder(w)
	for _, item := range items {
		if err := enc.Encode(item); err != nil {
			return err
		}
	}
	return nil
}

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
