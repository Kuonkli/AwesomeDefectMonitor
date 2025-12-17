package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type DefectStatus string

const (
	DefectStatusNew       DefectStatus = "new"
	DefectStatusInWork    DefectStatus = "in_work"
	DefectStatusInReview  DefectStatus = "in_review"
	DefectStatusClosed    DefectStatus = "closed"
	DefectStatusCancelled DefectStatus = "cancelled"
)

type DefectPriority string

const (
	DefectPriorityLow    DefectPriority = "low"
	DefectPriorityMedium DefectPriority = "medium"
	DefectPriorityHigh   DefectPriority = "high"
)

type Defect struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP;constraint:OnUpdate:CURRENT_TIMESTAMP"`

	Title       string         `gorm:"not null;index"`
	Description string         `gorm:"type:text"`
	Status      DefectStatus   `gorm:"type:varchar(20);default:'new';index"`
	Priority    DefectPriority `gorm:"type:varchar(10);default:'medium';index"`
	DueDate     time.Time      `gorm:"index"`

	// Храним только ID, User данные будем получать через User Service
	ProjectID  uuid.UUID `gorm:"type:uuid;not null;index"`
	EngineerID uuid.UUID `gorm:"type:uuid;not null;index"`
	ReporterID uuid.UUID `gorm:"type:uuid;not null;index"`

	Comments []Comment `gorm:"foreignKey:DefectID;constraint:OnDelete:CASCADE"`
}

type Comment struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP;constraint:OnUpdate:CURRENT_TIMESTAMP"`

	Text string `gorm:"type:text;not null"`

	UserID   uuid.UUID `gorm:"type:uuid;not null;index"`
	DefectID uuid.UUID `gorm:"type:uuid;not null;index"`
}

func (d *Defect) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}

func (c *Comment) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}
