package services

import (
	"encoding/json"
	"log"
	"living-recorder/backend/config"
	"living-recorder/backend/models"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

type MonitorService struct {
	db       *gorm.DB
	recorder *RecorderService
	cfg      config.RecorderConfig
	clients  map[*websocket.Conn]bool
	mu       sync.RWMutex
}

type StatusMessage struct {
	Type          string            `json:"type"`
	ActiveStreams []*StreamProcess  `json:"active_streams,omitempty"`
	StreamStatus  *StreamStatusInfo `json:"stream_status,omitempty"`
	StorageUsed   int64             `json:"storage_used,omitempty"`
}

type StreamStatusInfo struct {
	StreamID uint   `json:"stream_id"`
	Status   string `json:"status"`
	Duration int    `json:"duration_sec"`
	FileSize int64  `json:"file_size"`
}

func NewMonitorService(db *gorm.DB, recorder *RecorderService, cfg config.RecorderConfig) *MonitorService {
	return &MonitorService{
		db:       db,
		recorder: recorder,
		cfg:      cfg,
		clients:  make(map[*websocket.Conn]bool),
	}
}

func (s *MonitorService) AddClient(conn *websocket.Conn) {
	s.mu.Lock()
	s.clients[conn] = true
	s.mu.Unlock()
}

func (s *MonitorService) RemoveClient(conn *websocket.Conn) {
	s.mu.Lock()
	delete(s.clients, conn)
	s.mu.Unlock()
	conn.Close()
}

func (s *MonitorService) broadcast(msg StatusMessage) {
	data, _ := json.Marshal(msg)
	s.mu.RLock()
	defer s.mu.RUnlock()
	for client := range s.clients {
		if err := client.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("ws write error: %v", err)
			client.Close()
			delete(s.clients, client)
		}
	}
}

func (s *MonitorService) Start() {
	go func() {
		ticker := time.NewTicker(time.Duration(s.cfg.HealthCheckInterval) * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			s.collectAndBroadcast()
		}
	}()
}

func (s *MonitorService) collectAndBroadcast() {
	active := s.recorder.GetActiveStreams()

	var totalSize int64
	s.db.Model(&models.RecordLog{}).Select("COALESCE(SUM(file_size), 0)").Scan(&totalSize)

	msg := StatusMessage{
		Type:          "status",
		ActiveStreams: active,
		StorageUsed:   totalSize,
	}
	s.broadcast(msg)
}

func (s *MonitorService) NotifyStreamChange(streamID uint) {
	sp := s.recorder.GetProcess(streamID)
	info := &StreamStatusInfo{StreamID: streamID}
	if sp != nil {
		info.Status = sp.Status
		info.Duration = int(time.Since(sp.StartedAt).Seconds())
	}
	s.broadcast(StatusMessage{
		Type:         "stream_update",
		StreamStatus: info,
	})
}
