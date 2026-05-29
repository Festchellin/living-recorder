package models

import "time"

type Group struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:255;not null" json:"name"`
	ParentID  *uint     `gorm:"index" json:"parent_id"`
	Children  []*Group  `gorm:"-" json:"children,omitempty"`
	SortOrder int       `gorm:"default:0" json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
