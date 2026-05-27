package database

import (
	"living-recorder/backend/config"
	"living-recorder/backend/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func Init(cfg config.DatabaseConfig) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(cfg.Path), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	db.AutoMigrate(
		&models.Stream{},
		&models.RecordTask{},
		&models.RecordLog{},
	)

	return db, nil
}
