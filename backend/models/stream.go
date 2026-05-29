package models

import "time"

type Stream struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:255;not null" json:"name"`
	URL       string    `gorm:"size:1024;not null" json:"url"`
	Protocol  string    `gorm:"size:32;not null" json:"protocol"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	Status    string    `gorm:"size:32;default:idle" json:"status"`
	GroupID   *uint     `gorm:"index" json:"group_id"`
	Group     *Group    `gorm:"foreignKey:GroupID" json:"group,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
