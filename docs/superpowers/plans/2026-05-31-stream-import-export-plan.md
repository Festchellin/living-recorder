# Stream Import/Export Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add import/export functionality for streams in JSON, CSV, Excel (xlsx), and Text (JSON Lines) formats.

**Architecture:** Backend (Go + excelize) handles all file parsing/generation via two new endpoints. Frontend adds toolbar buttons, stream selection dialog for export, file upload for import, and a remark field to the create/edit form. Group paths are serialized as "/"-separated strings and auto-created on import.

**Tech Stack:** Go 1.25, Gin, GORM, excelize/v2, TypeScript, React, shadcn/ui

**Spec:** `docs/superpowers/specs/2026-05-31-stream-import-export-design.md`

---

### Task 1: Add Remark field to Stream model

**Files:**
- Modify: `backend/models/stream.go:5-16`

- [ ] **Step 1: Add Remark field**

```go
type Stream struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:255;not null" json:"name"`
	URL       string    `gorm:"size:1024;not null" json:"url"`
	Protocol  string    `gorm:"size:32;not null" json:"protocol"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	Status    string    `gorm:"size:32;default:idle" json:"status"`
	Remark    string    `gorm:"size:512" json:"remark"`
	GroupID   *uint     `gorm:"index" json:"group_id"`
	Group     *Group    `gorm:"foreignKey:GroupID" json:"group,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
```

- [ ] **Step 2: Add Remark field to frontend Stream interface**

Edit `frontend/src/lib/api.ts`, add `remark: string` to the `Stream` interface.

- [ ] **Step 3: Commit**

```bash
git add backend/models/stream.go frontend/src/lib/api.ts
git commit -m "feat: add remark field to Stream model"
```

### Task 2: Add excelize Go dependency

**Files:**
- Modify: `backend/go.mod`

- [ ] **Step 1: Install excelize**

Run: `cd backend && go get github.com/xuri/excelize/v2`

- [ ] **Step 2: Verify it compiles**

Run: `cd backend && go build ./...`

- [ ] **Step 3: Commit**

```bash
git add backend/go.mod backend/go.sum
git commit -m "chore: add excelize dependency for xlsx export/import"
```

### Task 3: Import/export types and group path helpers

**Files:**
- Create: `backend/handlers/stream_io.go` (shared types + group path helpers)

- [ ] **Step 1: Write the test file**

Create `backend/handlers/stream_io_test.go`:

```go
package handlers

import (
	"living-recorder/backend/models"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	db.AutoMigrate(&models.Stream{}, &models.Group{})
	return db
}

func TestGetGroupPath(t *testing.T) {
	db := setupTestDB(t)

	// Create groups: "一楼/东侧" → g1 (root) → g2 (child)
	g1 := models.Group{Name: "一楼"}
	db.Create(&g1)

	pid := g1.ID
	g2 := models.Group{Name: "东侧", ParentID: &pid}
	db.Create(&g2)

	h := &StreamHandler{db: db}
	path := h.getGroupPath(&g2)
	if path != "一楼/东侧" {
		t.Fatalf("expected '一楼/东侧', got %q", path)
	}
}

func TestGetGroupPath_RootOnly(t *testing.T) {
	db := setupTestDB(t)
	g := models.Group{Name: "一楼"}
	db.Create(&g)

	h := &StreamHandler{db: db}
	path := h.getGroupPath(&g)
	if path != "一楼" {
		t.Fatalf("expected '一楼', got %q", path)
	}
}

func TestFindOrCreateGroupPath(t *testing.T) {
	db := setupTestDB(t)
	h := &StreamHandler{db: db}

	g, err := h.findOrCreateGroupPath("一楼/东侧")
	if err != nil {
		t.Fatal(err)
	}

	var count int64
	db.Model(&models.Group{}).Count(&count)
	if count != 2 {
		t.Fatalf("expected 2 groups, got %d", count)
	}

	// Fetch full group with parent
	var child models.Group
	db.Preload("Children").First(&child, g.ID)
	if child.Name != "东侧" {
		t.Fatalf("expected '东侧', got %q", child.Name)
	}

	// Second call should reuse existing groups
	g2, err := h.findOrCreateGroupPath("一楼/东侧")
	if err != nil {
		t.Fatal(err)
	}
	if g2.ID != g.ID {
		t.Fatal("expected same group ID on second call")
	}
	// Count should still be 2
	db.Model(&models.Group{}).Count(&count)
	if count != 2 {
		t.Fatalf("expected 2 groups after reuse, got %d", count)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./handlers/ -run TestGetGroupPath -v`
Expected: FAIL (functions not defined yet)

- [ ] **Step 3: Add types and group path helpers to stream_io.go**

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./handlers/ -run TestGetGroupPath -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/handlers/stream_io.go backend/handlers/stream_io_test.go
git commit -m "feat: add group path helpers for import/export"
```

### Task 4: Export format writers

**Files:**
- Modify: `backend/handlers/stream_io.go`
- Modify: `backend/handlers/stream_io_test.go`

- [ ] **Step 1: Write tests for format writers**

Add to `backend/handlers/stream_io_test.go`:

```go
func TestExportJSON(t *testing.T) {
	db := setupTestDB(t)
	h := &StreamHandler{db: db}
	streams := []models.Stream{
		{Name: "cam1", URL: "rtsp://example.com/1", Protocol: "rtsp", Remark: "test"},
	}

	var buf strings.Builder
	err := h.writeJSON(&buf, streams)
	if err != nil {
		t.Fatal(err)
	}

	var decoded []importStream
	if err := json.Unmarshal([]byte(buf.String()), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 1 || decoded[0].Name != "cam1" {
		t.Fatal("JSON round-trip failed")
	}
}

func TestExportCSV(t *testing.T) {
	db := setupTestDB(t)
	h := &StreamHandler{db: db}
	streams := []models.Stream{
		{Name: "cam1", URL: "rtsp://example.com/1", Protocol: "rtsp"},
	}

	var buf strings.Builder
	err := h.writeCSV(&buf, streams)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (header + data), got %d", len(lines))
	}
	if !strings.HasPrefix(lines[0], "name") {
		t.Fatalf("expected CSV header, got %q", lines[0])
	}
}

func TestExportTXT(t *testing.T) {
	db := setupTestDB(t)
	h := &StreamHandler{db: db}
	streams := []models.Stream{
		{Name: "cam1", URL: "rtsp://example.com/1", Protocol: "rtsp"},
	}

	var buf strings.Builder
	err := h.writeTXT(&buf, streams)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	var decoded importStream
	if err := json.Unmarshal([]byte(lines[0]), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Name != "cam1" {
		t.Fatal("TXT round-trip failed")
	}
}
```

Add `"encoding/json"` and `"strings"` to the imports in `stream_io_test.go`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./handlers/ -run "TestExport" -v`
Expected: FAIL (writeJSON/writeCSV/writeTXT not defined)

- [ ] **Step 3: Add format writers to stream_io.go**

Add these functions:

```go
import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"living-recorder/backend/models"

	"github.com/xuri/excelize/v2"
)

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
	// Style header
	style, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	f.SetCellStyle(sheet, "A1", "F1", style)

	// Auto-width
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

type exportItem struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Protocol  string `json:"protocol"`
	Enabled   bool   `json:"enabled"`
	GroupPath string `json:"group_path"`
	Remark    string `json:"remark"`
}

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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./handlers/ -run "TestExport" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/handlers/stream_io.go backend/handlers/stream_io_test.go
git commit -m "feat: add export format writers (json, csv, xlsx, txt)"
```

### Task 5: Export handler

**Files:**
- Modify: `backend/handlers/stream_io.go`
- Modify: `backend/handlers/stream_io_test.go`

- [ ] **Step 1: Write test for Export handler**

Add to `stream_io_test.go`:

```go
func TestExportHandler_JSON(t *testing.T) {
	db := setupTestDB(t)
	db.Create(&models.Stream{Name: "cam1", URL: "rtsp://example.com/1", Protocol: "rtsp"})

	h := &StreamHandler{db: db}
	c, w := gin.CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("POST", "/export", strings.NewReader(`{"format":"json"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Export(c)

	var result struct {
		Code int            `json:"code"`
		Data []importStream `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &result)
	if result.Code != 0 {
		t.Fatalf("expected code 0, got %d", result.Code)
	}
	if len(result.Data) != 1 || result.Data[0].Name != "cam1" {
		t.Fatal("export failed")
	}
}

func TestExportHandler_CSV(t *testing.T) {
	db := setupTestDB(t)
	db.Create(&models.Stream{Name: "cam1", URL: "rtsp://example.com/1", Protocol: "rtsp"})

	h := &StreamHandler{db: db}
	c, w := gin.CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("POST", "/export", strings.NewReader(`{"format":"csv"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Export(c)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/csv" {
		t.Fatalf("expected text/csv, got %q", ct)
	}
}

func TestExportHandler_InvalidFormat(t *testing.T) {
	db := setupTestDB(t)
	h := &StreamHandler{db: db}
	c, w := gin.CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("POST", "/export", strings.NewReader(`{"format":"pdf"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Export(c)

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"message"`
	}
	json.Unmarshal(w.Body.Bytes(), &result)
	if result.Code != 1 {
		t.Fatalf("expected error code 1, got %d", result.Code)
	}
}
```

Add imports: `"net/http"`, `"net/http/httptest"`, `"github.com/gin-gonic/gin"`

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./handlers/ -run "TestExportHandler" -v`
Expected: FAIL (Export handler not defined)

- [ ] **Step 3: Add Export handler**

Add to `stream_io.go`:

```go
import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *StreamHandler) Export(c *gin.Context) {
	var req exportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "无效的请求: " + err.Error()})
		return
	}

	query := h.db.Model(&models.Stream{}).Preload("Group")
	if len(req.IDs) > 0 {
		query = query.Where("id IN ?", req.IDs)
	}
	var streams []models.Stream
	if err := query.Find(&streams).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "查询失败"})
		return
	}

	ts := time.Now().Unix()
	switch req.Format {
	case "json":
		c.Header("Content-Type", "application/json")
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="streams-json-%d.json"`, ts))
		h.writeJSON(c.Writer, streams)
	case "csv":
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="streams-csv-%d.csv"`, ts))
		h.writeCSV(c.Writer, streams)
	case "xlsx":
		data, err := h.writeXLSX(streams)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "生成Excel失败"})
			return
		}
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="streams-xlsx-%d.xlsx"`, ts))
		c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
	case "txt":
		c.Header("Content-Type", "text/plain")
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="streams-txt-%d.txt"`, ts))
		h.writeTXT(c.Writer, streams)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "不支持的格式: " + req.Format})
	}
}
```

Need to import `"fmt"` and `"time"` too (already should be imported).

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./handlers/ -run "TestExportHandler" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/handlers/stream_io.go backend/handlers/stream_io_test.go
git commit -m "feat: add export handler with format dispatch"
```

### Task 6: Import format parsers

**Files:**
- Modify: `backend/handlers/stream_io.go`
- Modify: `backend/handlers/stream_io_test.go`

- [ ] **Step 1: Write tests for format parsers**

Add to `stream_io_test.go`:

```go
func TestParseJSON(t *testing.T) {
	db := setupTestDB(t)
	h := &StreamHandler{db: db}
	input := `[{"name":"cam1","url":"rtsp://example.com/1","protocol":"rtsp","enabled":true,"group_path":"","remark":"test"}]`

	items, err := h.parseJSON(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "cam1" {
		t.Fatal("parseJSON failed")
	}
}

func TestParseJSON_Invalid(t *testing.T) {
	db := setupTestDB(t)
	h := &StreamHandler{db: db}

	_, err := h.parseJSON(strings.NewReader(`{invalid}`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseCSV(t *testing.T) {
	db := setupTestDB(t)
	h := &StreamHandler{db: db}
	input := "name,url,protocol,enabled,group_path,remark\ncam1,rtsp://example.com/1,rtsp,true,,test\n"

	items, err := h.parseCSV(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "cam1" || !items[0].Enabled {
		t.Fatal("parseCSV failed")
	}
}

func TestParseTXT(t *testing.T) {
	db := setupTestDB(t)
	h := &StreamHandler{db: db}
	input := `{"name":"cam1","url":"rtsp://example.com/1","protocol":"rtsp","enabled":true,"group_path":"","remark":"test"}` + "\n"

	items, err := h.parseTXT(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "cam1" {
		t.Fatal("parseTXT failed")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./handlers/ -run "TestParse" -v`
Expected: FAIL (parse functions not defined)

- [ ] **Step 3: Add format parsers**

Add to `stream_io.go`:

```go
import (
	"encoding/csv"
	"encoding/json"
	"bufio"
	"io"
	"strconv"
	"strings"
)

func (h *StreamHandler) parseJSON(r io.Reader) ([]importStream, error) {
	var items []importStream
	if err := json.NewDecoder(r).Decode(&items); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w", err)
	}
	return items, nil
}

func (h *StreamHandler) parseCSV(r io.Reader) ([]importStream, error) {
	reader := csv.NewReader(r)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV解析失败: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("CSV文件为空或只有表头")
	}
	headers := records[0]
	colMap := make(map[string]int)
	for i, h := range headers {
		colMap[strings.TrimSpace(h)] = i
	}

	var items []importStream
	for lineIdx, record := range records[1:] {
		item := importStream{Enabled: true}
		if idx, ok := colMap["name"]; ok && idx < len(record) {
			item.Name = strings.TrimSpace(record[idx])
		}
		if idx, ok := colMap["url"]; ok && idx < len(record) {
			item.URL = strings.TrimSpace(record[idx])
		}
		if idx, ok := colMap["protocol"]; ok && idx < len(record) {
			item.Protocol = strings.TrimSpace(record[idx])
		}
		if idx, ok := colMap["enabled"]; ok && idx < len(record) {
			if v, err := strconv.ParseBool(strings.TrimSpace(record[idx])); err == nil {
				item.Enabled = v
			}
		}
		if idx, ok := colMap["group_path"]; ok && idx < len(record) {
			item.GroupPath = strings.TrimSpace(record[idx])
		}
		if idx, ok := colMap["remark"]; ok && idx < len(record) {
			item.Remark = strings.TrimSpace(record[idx])
		}
		items = append(items, item)
		_ = lineIdx
	}
	return items, nil
}

func (h *StreamHandler) parseXLSX(data []byte) ([]importStream, error) {
	f, err := excelize.OpenReader(strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("Excel解析失败: %w", err)
	}
	defer f.Close()

	rows, err := f.GetRows("Streams")
	if err != nil {
		// Try default sheet name
		rows, err = f.GetRows("Sheet1")
		if err != nil {
			return nil, fmt.Errorf("Excel中未找到数据表: %w", err)
		}
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("Excel工作表为空")
	}

	headers := rows[0]
	colMap := make(map[string]int)
	for i, h := range headers {
		colMap[strings.TrimSpace(h)] = i
	}

	var items []importStream
	for _, row := range rows[1:] {
		item := importStream{Enabled: true}
		if idx, ok := colMap["name"]; ok && idx < len(row) {
			item.Name = strings.TrimSpace(row[idx])
		}
		if idx, ok := colMap["url"]; ok && idx < len(row) {
			item.URL = strings.TrimSpace(row[idx])
		}
		if idx, ok := colMap["protocol"]; ok && idx < len(row) {
			item.Protocol = strings.TrimSpace(row[idx])
		}
		if idx, ok := colMap["enabled"]; ok && idx < len(row) {
			if v, err := strconv.ParseBool(strings.TrimSpace(row[idx])); err == nil {
				item.Enabled = v
			}
		}
		if idx, ok := colMap["group_path"]; ok && idx < len(row) {
			item.GroupPath = strings.TrimSpace(row[idx])
		}
		if idx, ok := colMap["remark"]; ok && idx < len(row) {
			item.Remark = strings.TrimSpace(row[idx])
		}
		items = append(items, item)
	}
	return items, nil
}

func (h *StreamHandler) parseTXT(r io.Reader) ([]importStream, error) {
	var items []importStream
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var item importStream
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("JSON Lines解析失败: %w", err)
		}
		items = append(items, item)
	}
	return items, scanner.Err()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./handlers/ -run "TestParse" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/handlers/stream_io.go backend/handlers/stream_io_test.go
git commit -m "feat: add import format parsers (json, csv, xlsx, txt)"
```

### Task 7: Import handler

**Files:**
- Modify: `backend/handlers/stream_io.go`
- Modify: `backend/handlers/stream_io_test.go`

- [ ] **Step 1: Write test for Import handler**

Add to `stream_io_test.go`:

```go
func TestImportHandler_JSON(t *testing.T) {
	db := setupTestDB(t)
	db.Create(&models.Stream{Name: "existing", URL: "rtsp://existing/1", Protocol: "rtsp"})

	h := &StreamHandler{db: db}

	body := `[{"name":"newcam","url":"rtsp://new/1","protocol":"rtsp","group_path":"","remark":""}]`
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("POST", "/import", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	var req struct {
		Format string `json:"format"`
	}
	req.Format = "json"
	// We test via the internal import logic directly
	result := h.processImport([]importStream{
		{Name: "newcam", URL: "rtsp://new/1", Protocol: "rtsp", Enabled: true},
		{Name: "existing", URL: "rtsp://existing/1", Protocol: "rtsp", Enabled: true},
	})

	if result.Success != 1 {
		t.Fatalf("expected 1 success, got %d", result.Success)
	}
	if result.Skipped != 1 {
		t.Fatalf("expected 1 skipped, got %d", result.Skipped)
	}
}

func TestImportHandler_GroupPath(t *testing.T) {
	db := setupTestDB(t)
	h := &StreamHandler{db: db}

	result := h.processImport([]importStream{
		{Name: "cam1", URL: "rtsp://cam/1", Protocol: "rtsp", GroupPath: "一楼/东侧"},
	})

	if result.Success != 1 {
		t.Fatalf("expected 1 success, got %d", result.Success)
	}

	var count int64
	db.Model(&models.Group{}).Count(&count)
	if count != 2 {
		t.Fatalf("expected 2 groups created, got %d", count)
	}

	var stream models.Stream
	db.Preload("Group").First(&stream)
	if stream.Group == nil || stream.Group.Name != "东侧" {
		t.Fatal("stream not associated with correct group")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./handlers/ -run "TestImportHandler" -v`
Expected: FAIL (processImport not defined)

- [ ] **Step 3: Add Import handler and processImport**

Add to `stream_io.go`:

```go
import (
	"mime/multipart"
	"path/filepath"
)

func (h *StreamHandler) Import(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "请上传文件"})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "文件读取失败"})
		return
	}
	defer f.Close()

	var items []importStream
	ext := strings.ToLower(filepath.Ext(file.Filename))
	switch ext {
	case ".json":
		items, err = h.parseJSON(f)
	case ".csv":
		items, err = h.parseCSV(f)
	case ".xlsx":
		data, readErr := io.ReadAll(f)
		if readErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "文件读取失败"})
			return
		}
		items, err = h.parseXLSX(data)
	case ".txt":
		items, err = h.parseTXT(f)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "不支持的文件格式: " + ext})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": err.Error()})
		return
	}

	result := h.processImport(items)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

func (h *StreamHandler) processImport(items []importStream) importResult {
	var result importResult
	for lineIdx, item := range items {
		line := lineIdx + 1

		if item.Name == "" {
			result.Errors = append(result.Errors, importError{Line: line, Message: "名称为空"})
			continue
		}
		if item.URL == "" {
			result.Errors = append(result.Errors, importError{Line: line, Message: "URL为空"})
			continue
		}
		if !validProtocols[item.Protocol] {
			result.Errors = append(result.Errors, importError{Line: line, Message: "不支持的协议: " + item.Protocol})
			continue
		}

		// Check for duplicates
		var existing models.Stream
		if !h.db.Where("name = ? OR url = ?", item.Name, item.URL).First(&existing).Error.Is(h.db.Error) && existing.ID > 0 {
			// Name or URL already exists
			result.Skipped++
			continue
		}

		stream := models.Stream{
			Name:     item.Name,
			URL:      item.URL,
			Protocol: item.Protocol,
			Enabled:  item.Enabled,
			Status:   "idle",
			Remark:   item.Remark,
		}

		if item.GroupPath != "" {
			group, err := h.findOrCreateGroupPath(item.GroupPath)
			if err != nil {
				result.Errors = append(result.Errors, importError{Line: line, Message: "分组创建失败: " + err.Error()})
				continue
			}
			stream.GroupID = &group.ID
		}

		if err := h.db.Create(&stream).Error; err != nil {
			result.Errors = append(result.Errors, importError{Line: line, Message: "创建失败: " + err.Error()})
			continue
		}
		result.Success++
	}
	return result
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./handlers/ -run "TestImportHandler" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/handlers/stream_io.go backend/handlers/stream_io_test.go
git commit -m "feat: add import handler with conflict detection"
```

### Task 8: Register routes

**Files:**
- Modify: `backend/routes/routes.go:24-38`

- [ ] **Step 1: Add import/export routes**

Add after the stop-all route:
```go
api.POST("/streams/export", streamHandler.Export)
api.POST("/streams/import", streamHandler.Import)
```

- [ ] **Step 2: Verify it compiles**

Run: `cd backend && go build ./...`

- [ ] **Step 3: Commit**

```bash
git add backend/routes/routes.go
git commit -m "feat: register import/export API routes"
```

### Task 9: Frontend API client additions

**Files:**
- Modify: `frontend/src/lib/api.ts`

- [ ] **Step 1: Add ImportResult interface and import/export methods**

Add to `frontend/src/lib/api.ts`:

```typescript
export interface ImportResult {
  success: number
  skipped: number
  errors?: { line: number; message: string }[]
}
```

In the `streams` object, add:

```typescript
export: (format: string, ids?: number[]) =>
  fetch('/api/streams/export', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ format, ids }),
  }).then(async (r) => {
    const disp = r.headers.get('Content-Disposition') || ''
    const match = disp.match(/filename="?(.+?)"?$/)
    const filename = match?.[1] || `streams.${format}`
    const blob = await r.blob()
    return { blob, filename }
  }),

import: (file: File) => {
  const form = new FormData()
  form.append('file', file)
  return request<ImportResult>('/api/streams/import', {
    method: 'POST',
    body: form,
  })
},
```

Also add `remark: string` to the `Stream` interface if not already done.

- [ ] **Step 2: Commit**

```bash
git add frontend/src/lib/api.ts
git commit -m "feat: add import/export methods to API client"
```

### Task 10: Frontend Streams page UI

**Files:**
- Modify: `frontend/src/pages/Streams.tsx`

- [ ] **Step 1: Add import/export buttons and dialogs**

Changes needed:

1. Add icons to imports at top:
```typescript
import { Plus, Play, Square, FileUp, FileDown } from 'lucide-react'
```

2. Add state variables after existing state:
```typescript
const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set())
const [exportOpen, setExportOpen] = useState(false)
const [importOpen, setImportOpen] = useState(false)
const [importResult, setImportResult] = useState<ImportResult | null>(null)
const fileInputRef = useRef<HTMLInputElement>(null)
```

3. Add handler functions before the return statement:
```typescript
const handleExport = async (format: string) => {
  const ids = selectedIds.size > 0 ? Array.from(selectedIds) : undefined
  try {
    const { blob, filename } = await api.streams.export(format, ids)
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    a.click()
    URL.revokeObjectURL(url)
    addMsg(`导出成功: ${filename}`, 'success')
  } catch {
    addMsg('导出失败', 'error')
  }
  setExportOpen(false)
}

const handleImportFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
  const file = e.target.files?.[0]
  if (!file) return
  try {
    const result = await api.streams.import(file)
    setImportResult(result)
    if (result.success > 0) {
      addMsg(`导入成功: ${result.success}个`, 'success')
    }
    if (result.skipped > 0) {
      addMsg(`跳过: ${result.skipped}个（名称/URL重复）`, 'success')
    }
    loadStreams()
    loadGroups()
  } catch {
    addMsg('导入失败', 'error')
  }
  if (fileInputRef.current) fileInputRef.current.value = ''
}
```

4. Add toggle selection handler:
```typescript
const toggleSelect = (id: number) => {
  setSelectedIds((prev) => {
    const next = new Set(prev)
    if (next.has(id)) next.delete(id)
    else next.add(id)
    return next
  })
}
```

5. Add import/export buttons to the toolbar (after the "添加流媒体" button):
```tsx
<Button variant="outline" onClick={() => setExportOpen(true)}>
  <FileDown className="h-4 w-4 mr-1" />
  导出
</Button>
<Button variant="outline" onClick={() => fileInputRef.current?.click()}>
  <FileUp className="h-4 w-4 mr-1" />
  导入
</Button>
<input
  ref={fileInputRef}
  type="file"
  accept=".json,.csv,.xlsx,.txt"
  className="hidden"
  onChange={handleImportFile}
/>
```

6. Add Export dialog before the closing `</div>` of the page:
```tsx
<Dialog open={exportOpen} onOpenChange={setExportOpen}>
  <DialogContent>
    <DialogHeader>
      <DialogTitle>导出信号源</DialogTitle>
    </DialogHeader>
    <div className="space-y-3">
      <p className="text-sm text-white/60">
        {selectedIds.size > 0
          ? `已选择 ${selectedIds.size} 个信号源`
          : '将导出所有信号源'}
      </p>
      <div className="grid grid-cols-2 gap-2">
        {[
          { format: 'json', label: 'JSON' },
          { format: 'csv', label: 'CSV' },
          { format: 'xlsx', label: 'Excel' },
          { format: 'txt', label: 'TXT(JSON Lines)' },
        ].map(({ format, label }) => (
          <Button key={format} variant="outline" onClick={() => handleExport(format)}>
            {label}
          </Button>
        ))}
      </div>
    </div>
  </DialogContent>
</Dialog>
```

7. Import result dialog:
```tsx
<Dialog open={importResult !== null} onOpenChange={(o) => { if (!o) setImportResult(null) }}>
  <DialogContent>
    <DialogHeader>
      <DialogTitle>导入结果</DialogTitle>
    </DialogHeader>
    <div className="space-y-2">
      <p className="text-green-400">成功: {importResult?.success}</p>
      <p className="text-yellow-400">跳过: {importResult?.skipped}</p>
      {importResult?.errors && importResult.errors.length > 0 && (
        <div className="text-red-400 text-sm space-y-1">
          {importResult.errors.map((e, i) => (
            <p key={i}>第{e.line}行: {e.message}</p>
          ))}
        </div>
      )}
    </div>
  </DialogContent>
</Dialog>
```

8. Add checkbox to each StreamCard or refactor cards to show selection. Since StreamCard doesn't support selection, wrap each card in a selectable container or pass a `selected` prop:

Modify the StreamCard rendering section:
```tsx
{filteredStreams.map((s) => (
  <div key={s.id} className="relative group">
    <div
      className="absolute top-2 left-2 z-10 opacity-0 group-hover:opacity-100 transition-opacity"
      onClick={(e) => { e.stopPropagation(); toggleSelect(s.id) }}
    >
      <input
        type="checkbox"
        checked={selectedIds.has(s.id)}
        onChange={() => toggleSelect(s.id)}
        className="w-4 h-4 accent-blue-500"
      />
    </div>
    <StreamCard
      stream={s}
      groupPath={getGroupPath(groups, s.group_id)}
      onStart={...}
      onStop={...}
      onEdit={...}
      onDelete={...}
    />
  </div>
))}
```

- [ ] **Step 2: Verify frontend builds**

Run: `cd frontend && npm run build`

Expected: Compiled successfully

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/Streams.tsx
git commit -m "feat: add import/export UI to Streams page"
```
