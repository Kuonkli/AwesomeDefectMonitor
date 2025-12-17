package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type ProjectStatus string

const (
	ProjectStatusActive    ProjectStatus = "active"
	ProjectStatusPlanning  ProjectStatus = "planning"
	ProjectStatusCompleted ProjectStatus = "completed"
	ProjectStatusArchived  ProjectStatus = "archived"
)

type Project struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP;constraint:OnUpdate:CURRENT_TIMESTAMP"`

	Name        string        `gorm:"not null;index"`
	Description string        `gorm:"type:text"`
	Status      ProjectStatus `gorm:"type:varchar(20);default:'active'"`
	StartDate   time.Time
	EndDate     time.Time

	ManagerID uuid.UUID `gorm:"type:uuid;not null;index"`

	Defects []Defect `gorm:"foreignKey:ProjectID"`
}

// ProjectEngineer связь проекты-инженеры (только ID)
type ProjectEngineer struct {
	ProjectID uuid.UUID `gorm:"type:uuid;primary_key"`
	UserID    uuid.UUID `gorm:"type:uuid;primary_key"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP"`
}

func (p *Project) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}
