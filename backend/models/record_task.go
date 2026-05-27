package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

type JSON json.RawMessage

func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return string(j), nil
}

func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		*j = nil
		return nil
	}
	*j = append((*j)[:0], bytes...)
	return nil
}

type RecordTask struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	StreamID         uint      `gorm:"not null;index" json:"stream_id"`
	Stream           Stream    `gorm:"foreignKey:StreamID" json:"stream,omitempty"`
	OutputTemplate   string    `gorm:"size:512;not null;default:'{name}/{date}_{time}.mp4'" json:"output_template"`
	SegmentSec       int       `gorm:"default:0" json:"segment_sec"`
	VideoCodec       string    `gorm:"size:64;default:copy" json:"video_codec"`
	VideoBitrate     string    `gorm:"size:32" json:"video_bitrate"`
	Framerate        int       `gorm:"default:0" json:"framerate"`
	Resolution       string    `gorm:"size:32" json:"resolution"`
	AudioCodec       string    `gorm:"size:64;default:copy" json:"audio_codec"`
	AudioBitrate     string    `gorm:"size:32" json:"audio_bitrate"`
	StorageType      string    `gorm:"size:32;default:local" json:"storage_type"`
	StorageConfig    JSON      `gorm:"type:text" json:"storage_config,omitempty"`
	ScheduleCron     string    `gorm:"size:128" json:"schedule_cron"`
	ScheduleDuration int       `gorm:"default:0" json:"schedule_duration"`
	LoopRecord       bool      `gorm:"default:false" json:"loop_record"`
	Enabled          bool      `gorm:"default:true" json:"enabled"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
