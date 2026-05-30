package database

import (
	"fmt"

	"living-recorder/backend/config"
	"living-recorder/backend/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func Init(cfg config.DatabaseConfig) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(cfg.Path), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// WAL mode + relaxed sync: cuts fsync overhead on slow disks (NFS/SMB/USB)
	db.Exec("PRAGMA journal_mode = WAL")
	db.Exec("PRAGMA synchronous = NORMAL")

	if err := db.AutoMigrate(
		&models.Group{},
		&models.Stream{},
		&models.RecordTask{},
		&models.RecordLog{},
	); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}

	return db, nil
}
