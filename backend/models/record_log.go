package models

import "time"

type RecordLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	StreamID  *uint     `gorm:"index" json:"stream_id"`
	Stream    Stream    `gorm:"foreignKey:StreamID" json:"stream,omitempty"`
	FilePath  string    `gorm:"size:1024" json:"file_path"`
	FileSize  int64     `gorm:"default:0" json:"file_size"`
	Duration  int       `gorm:"default:0" json:"duration"`
	Status    string    `gorm:"size:32;not null" json:"status"`
	ErrorMsg  string    `gorm:"size:1024" json:"error_msg"`
	Level     string    `gorm:"size:16;default:info" json:"level"`
	Message   string    `gorm:"size:1024" json:"message"`
	EventType string    `gorm:"size:64" json:"event_type"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
}

const (
	LevelInfo    = "info"
	LevelWarning = "warning"
	LevelError   = "error"

	EventRecordingStarted = "recording_started"
	EventRecordingStopped = "recording_stopped"
	EventRecordingFailed  = "recording_failed"
	EventPreviewStarted   = "preview_started"
	EventPreviewStopped   = "preview_stopped"
	EventStreamCreated    = "stream_created"
	EventStreamUpdated    = "stream_updated"
	EventStreamDeleted    = "stream_deleted"
	EventStreamProbe      = "stream_probe"
	EventSystemStartup    = "system_startup"
	EventConfigChanged    = "config_changed"
)
