package services

import (
	"fmt"
	"living-recorder/backend/models"
	"log"
	"time"

	"gorm.io/gorm"
)

const (
	maxBatchSize = 64
	flushInterval = 2 * time.Second
)

type LogWriter struct {
	db      *gorm.DB
	entries chan *models.RecordLog
	done    chan struct{}
}

func NewLogWriter(db *gorm.DB) *LogWriter {
	w := &LogWriter{
		db:      db,
		entries: make(chan *models.RecordLog, 512),
		done:    make(chan struct{}),
	}
	go w.flushLoop()
	return w
}

func (w *LogWriter) Stop() {
	close(w.entries)
	<-w.done
}

func (w *LogWriter) flushLoop() {
	defer close(w.done)
	batch := make([]*models.RecordLog, 0, maxBatchSize)
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case entry, ok := <-w.entries:
			if !ok {
				w.flush(batch)
				return
			}
			batch = append(batch, entry)
			if len(batch) >= maxBatchSize {
				w.flush(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				w.flush(batch)
				batch = batch[:0]
			}
		}
	}
}

func (w *LogWriter) flush(batch []*models.RecordLog) {
	if len(batch) == 0 {
		return
	}
	if err := w.db.CreateInBatches(batch, len(batch)).Error; err != nil {
		log.Printf("[logger] batch insert failed (%d entries): %v", len(batch), err)
	}
}

func (w *LogWriter) enqueue(streamID *uint, level, eventType, status, message, errMsg string) {
	now := time.Now()
	w.entries <- &models.RecordLog{
		StreamID:  streamID,
		Status:    status,
		ErrorMsg:  errMsg,
		Level:     level,
		Message:   message,
		EventType: eventType,
		StartedAt: now,
		EndedAt:   now,
	}
}

func (w *LogWriter) Info(eventType, message string, args ...interface{}) {
	w.enqueue(nil, models.LevelInfo, eventType, "success", fmtMessage(message, args...), "")
}

func (w *LogWriter) Warn(eventType, message string, args ...interface{}) {
	w.enqueue(nil, models.LevelWarning, eventType, "success", fmtMessage(message, args...), "")
}

func (w *LogWriter) Error(eventType, message, errMsg string, args ...interface{}) {
	w.enqueue(nil, models.LevelError, eventType, "failed", fmtMessage(message, args...), errMsg)
}

func (w *LogWriter) StreamInfo(streamID uint, eventType, message string, args ...interface{}) {
	w.enqueue(&streamID, models.LevelInfo, eventType, "success", fmtMessage(message, args...), "")
}

func (w *LogWriter) StreamWarn(streamID uint, eventType, message string, args ...interface{}) {
	w.enqueue(&streamID, models.LevelWarning, eventType, "success", fmtMessage(message, args...), "")
}

func (w *LogWriter) StreamError(streamID uint, eventType, message, errMsg string, args ...interface{}) {
	w.enqueue(&streamID, models.LevelError, eventType, "failed", fmtMessage(message, args...), errMsg)
}

func (w *LogWriter) StreamRecordingLog(streamID uint, eventType, status, message, errMsg, filePath string, fileSize int64, duration int, startedAt, endedAt time.Time) {
	w.entries <- &models.RecordLog{
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
	}
}

func fmtMessage(tpl string, args ...interface{}) string {
	if len(args) == 0 {
		return tpl
	}
	return fmt.Sprintf(tpl, args...)
}
