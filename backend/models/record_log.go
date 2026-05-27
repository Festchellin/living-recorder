package models

import "time"

type RecordLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	StreamID  uint      `gorm:"not null;index" json:"stream_id"`
	Stream    Stream    `gorm:"foreignKey:StreamID" json:"stream,omitempty"`
	FilePath  string    `gorm:"size:1024" json:"file_path"`
	FileSize  int64     `gorm:"default:0" json:"file_size"`
	Duration  int       `gorm:"default:0" json:"duration"`
	Status    string    `gorm:"size:32;not null" json:"status"`
	ErrorMsg  string    `gorm:"size:1024" json:"error_msg"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
}
