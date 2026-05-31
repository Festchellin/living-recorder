package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"living-recorder/backend/models"

	"github.com/gin-gonic/gin"
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

func TestExportHandler_JSON(t *testing.T) {
	db := setupTestDB(t)
	db.Create(&models.Stream{Name: "cam1", URL: "rtsp://example.com/1", Protocol: "rtsp"})

	h := &StreamHandler{db: db}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/export", strings.NewReader(`{"format":"json"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Export(c)

	var result []importStream
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result) != 1 || result[0].Name != "cam1" {
		t.Fatal("export failed")
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %q", ct)
	}
}

func TestExportHandler_CSV(t *testing.T) {
	db := setupTestDB(t)
	db.Create(&models.Stream{Name: "cam1", URL: "rtsp://example.com/1", Protocol: "rtsp"})

	h := &StreamHandler{db: db}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
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
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
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
	_ = result.Msg
}

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

	var child models.Group
	db.First(&child, g.ID)
	if child.Name != "东侧" {
		t.Fatalf("expected '东侧', got %q", child.Name)
	}

	g2, err := h.findOrCreateGroupPath("一楼/东侧")
	if err != nil {
		t.Fatal(err)
	}
	if g2.ID != g.ID {
		t.Fatal("expected same group ID on second call")
	}
	db.Model(&models.Group{}).Count(&count)
	if count != 2 {
		t.Fatalf("expected 2 groups after reuse, got %d", count)
	}
}
