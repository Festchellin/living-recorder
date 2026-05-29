package services

import (
	"fmt"
	"living-recorder/backend/models"
	"time"

	"gorm.io/gorm"
)

type LogWriter struct {
	db *gorm.DB
}

func NewLogWriter(db *gorm.DB) *LogWriter {
	return &LogWriter{db: db}
}

func (w *LogWriter) write(streamID *uint, level, eventType, status, message, errMsg string) {
	now := time.Now()
	w.db.Create(&models.RecordLog{
		StreamID:  streamID,
		Status:    status,
		ErrorMsg:  errMsg,
		Level:     level,
		Message:   message,
		EventType: eventType,
		StartedAt: now,
		EndedAt:   now,
	})
}

func (w *LogWriter) Info(eventType, message string, args ...interface{}) {
	w.write(nil, models.LevelInfo, eventType, "success", fmtMessage(message, args...), "")
}

func (w *LogWriter) Warn(eventType, message string, args ...interface{}) {
	w.write(nil, models.LevelWarning, eventType, "success", fmtMessage(message, args...), "")
}

func (w *LogWriter) Error(eventType, message, errMsg string, args ...interface{}) {
	w.write(nil, models.LevelError, eventType, "failed", fmtMessage(message, args...), errMsg)
}

func (w *LogWriter) StreamInfo(streamID uint, eventType, message string, args ...interface{}) {
	w.write(&streamID, models.LevelInfo, eventType, "success", fmtMessage(message, args...), "")
}

func (w *LogWriter) StreamWarn(streamID uint, eventType, message string, args ...interface{}) {
	w.write(&streamID, models.LevelWarning, eventType, "success", fmtMessage(message, args...), "")
}

func (w *LogWriter) StreamError(streamID uint, eventType, message, errMsg string, args ...interface{}) {
	w.write(&streamID, models.LevelError, eventType, "failed", fmtMessage(message, args...), errMsg)
}

func (w *LogWriter) StreamRecordingLog(streamID uint, eventType, status, message, errMsg, filePath string, fileSize int64, duration int, startedAt, endedAt time.Time) {
	w.db.Create(&models.RecordLog{
		StreamID:  &streamID,
		FilePath:  filePath,
		FileSize:  fileSize,
		Duration:  duration,
		Status:    status,
		ErrorMsg:  errMsg,
		Level:     models.LevelInfo,
		Message:   message,
		EventType: eventType,
		StartedAt: startedAt,
		EndedAt:   endedAt,
	})
}

func fmtMessage(tpl string, args ...interface{}) string {
	if len(args) == 0 {
		return tpl
	}
	return fmt.Sprintf(tpl, args...)
}
