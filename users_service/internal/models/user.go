package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Role string

const (
	RoleEngineer   Role = "engineer"
	RoleManager    Role = "manager"
	RoleSupervisor Role = "supervisor"
)

type User struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP;constraint:OnUpdate:CURRENT_TIMESTAMP" json:"updated_at"`

	Email     string `gorm:"uniqueIndex;not null" json:"email"`
	Password  string `gorm:"not null" json:"-"`
	FirstName string `gorm:"not null" json:"first_name"`
	LastName  string `gorm:"not null" json:"last_name"`
	Role      Role   `gorm:"type:varchar(20);default:'engineer'" json:"role"`
	IsActive  bool   `gorm:"default:true" json:"is_active"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

func (u *User) FullName() string {
	return u.FirstName + " " + u.LastName
}
