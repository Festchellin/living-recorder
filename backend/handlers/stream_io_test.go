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
