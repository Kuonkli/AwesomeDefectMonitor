package repository

import (
	"awesome-defect-tracker/user-service/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserRepository interface {
	Create(user *models.User) error
	FindByID(id uuid.UUID) (*models.User, error)
	FindByEmail(email string) (*models.User, error)
	Update(user *models.User) error
	Delete(id uuid.UUID) error
	List(role models.Role, limit, offset int) ([]models.User, error)
	ExistsByEmail(email string) (bool, error)
	Deactivate(id uuid.UUID) error
	Activate(id uuid.UUID) error
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(user *models.User) error {
	return r.db.Create(user).Error
}

func (r *userRepository) FindByID(id uuid.UUID) (*models.User, error) {
	var user models.User
	err := r.db.First(&user, "id = ?", id).Error
	return &user, err
}

func (r *userRepository) FindByEmail(email string) (*models.User, error) {
	var user models.User
	err := r.db.First(&user, "email = ?", email).Error
	return &user, err
}

func (r *userRepository) Update(user *models.User) error {
	return r.db.Save(user).Error
}

func (r *userRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.User{}, "id = ?", id).Error
}

func (r *userRepository) List(role models.Role, limit, offset int) ([]models.User, error) {
	var users []models.User
	query := r.db.Model(&models.User{})

	if role != "" {
		query = query.Where("role = ?", role)
	}

	query = query.Where("is_active = ?", true)

	err := query.Limit(limit).Offset(offset).Find(&users).Error
	return users, err
}

func (r *userRepository) ExistsByEmail(email string) (bool, error) {
	var count int64
	err := r.db.Model(&models.User{}).Where("email = ?", email).Count(&count).Error
	return count > 0, err
}

func (r *userRepository) Deactivate(id uuid.UUID) error {
	return r.db.Model(&models.User{}).Where("id = ?", id).Update("is_active", false).Error
}

func (r *userRepository) Activate(id uuid.UUID) error {
	return r.db.Model(&models.User{}).Where("id = ?", id).Update("is_active", true).Error
}
