package repository

import (
	"awesome-defect-tracker/projects-service/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ProjectFilter struct {
	ManagerID  uuid.UUID
	Status     models.ProjectStatus
	EngineerID uuid.UUID
	Search     string
}

type DefectFilter struct {
	ProjectID  uuid.UUID
	EngineerID uuid.UUID
	ReporterID uuid.UUID
	Status     models.DefectStatus
	Priority   models.DefectPriority
	Search     string
}

type Repository interface {
	// Проекты
	CreateProject(project *models.Project) error
	FindProjectByID(id uuid.UUID) (*models.Project, error)
	UpdateProject(project *models.Project) error
	DeleteProject(id uuid.UUID) error
	ListProjects(filter ProjectFilter, limit, offset int) ([]models.Project, error)
	CountProjects(filter ProjectFilter) (int64, error)

	// Инженеры проектов
	AddProjectEngineer(projectID, engineerID uuid.UUID) error
	RemoveProjectEngineer(projectID, engineerID uuid.UUID) error
	GetProjectEngineers(projectID uuid.UUID) ([]uuid.UUID, error)
	IsUserProjectEngineer(projectID, userID uuid.UUID) (bool, error)
	GetUserRoleInProject(projectID, userID uuid.UUID) (string, error)

	// Дефекты
	CreateDefect(defect *models.Defect) error
	FindDefectByID(id uuid.UUID) (*models.Defect, error)
	UpdateDefect(defect *models.Defect) error
	DeleteDefect(id uuid.UUID) error
	ListDefects(filter DefectFilter, limit, offset int, sortBy, sortOrder string) ([]models.Defect, error)
	CountDefects(filter DefectFilter) (int64, error)

	// Комментарии
	CreateComment(comment *models.Comment) error
	FindCommentByID(id uuid.UUID) (*models.Comment, error)
	UpdateComment(comment *models.Comment) error
	DeleteComment(id uuid.UUID) error
	ListComments(defectID uuid.UUID, limit, offset int, sortBy, sortOrder string) ([]models.Comment, error)
	CountComments(defectID uuid.UUID) (int64, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) CreateProject(project *models.Project) error {
	return r.db.Create(project).Error
}

func (r *repository) FindProjectByID(id uuid.UUID) (*models.Project, error) {
	var project models.Project
	err := r.db.First(&project, "id = ?", id).Error
	return &project, err
}

func (r *repository) UpdateProject(project *models.Project) error {
	return r.db.Save(project).Error
}

func (r *repository) DeleteProject(id uuid.UUID) error {
	return r.db.Delete(&models.Project{}, "id = ?", id).Error
}

func (r *repository) ListProjects(filter ProjectFilter, limit, offset int) ([]models.Project, error) {
	var projects []models.Project

	query := r.db.Model(&models.Project{})

	if filter.ManagerID != uuid.Nil {
		query = query.Where("manager_id = ?", filter.ManagerID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.EngineerID != uuid.Nil {
		query = query.Joins("JOIN project_engineers ON projects.id = project_engineers.project_id").
			Where("project_engineers.user_id = ?", filter.EngineerID)
	}
	if filter.Search != "" {
		query = query.Where("name ILIKE ? OR description ILIKE ?",
			"%"+filter.Search+"%", "%"+filter.Search+"%")
	}

	err := query.Limit(limit).Offset(offset).Order("created_at DESC").Find(&projects).Error
	return projects, err
}

func (r *repository) CountProjects(filter ProjectFilter) (int64, error) {
	var count int64

	query := r.db.Model(&models.Project{})

	if filter.ManagerID != uuid.Nil {
		query = query.Where("manager_id = ?", filter.ManagerID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.EngineerID != uuid.Nil {
		query = query.Joins("JOIN project_engineers ON projects.id = project_engineers.project_id").
			Where("project_engineers.user_id = ?", filter.EngineerID)
	}
	if filter.Search != "" {
		query = query.Where("name ILIKE ? OR description ILIKE ?",
			"%"+filter.Search+"%", "%"+filter.Search+"%")
	}

	err := query.Count(&count).Error
	return count, err
}

func (r *repository) AddProjectEngineer(projectID, engineerID uuid.UUID) error {
	var count int64
	r.db.Table("project_engineers").
		Where("project_id = ? AND user_id = ?", projectID, engineerID).
		Count(&count)

	if count > 0 {
		return nil // Уже добавлен
	}

	return r.db.Table("project_engineers").Create(&models.ProjectEngineer{
		ProjectID: projectID,
		UserID:    engineerID,
	}).Error
}

func (r *repository) RemoveProjectEngineer(projectID, engineerID uuid.UUID) error {
	return r.db.Table("project_engineers").
		Where("project_id = ? AND user_id = ?", projectID, engineerID).
		Delete(&models.ProjectEngineer{}).Error
}

func (r *repository) GetProjectEngineers(projectID uuid.UUID) ([]uuid.UUID, error) {
	var userIDs []uuid.UUID
	err := r.db.Table("project_engineers").
		Where("project_id = ?", projectID).
		Pluck("user_id", &userIDs).Error
	return userIDs, err
}

func (r *repository) IsUserProjectEngineer(projectID, userID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.Table("project_engineers").
		Where("project_id = ? AND user_id = ?", projectID, userID).
		Count(&count).Error
	return count > 0, err
}

func (r *repository) GetUserRoleInProject(projectID, userID uuid.UUID) (string, error) {
	// Сначала проверяем менеджера
	var project models.Project
	err := r.db.Select("manager_id").First(&project, "id = ?", projectID).Error
	if err != nil {
		return "", err
	}

	if project.ManagerID == userID {
		return "manager", nil
	}

	// Проверяем инженера
	isEngineer, err := r.IsUserProjectEngineer(projectID, userID)
	if err != nil {
		return "", err
	}

	if isEngineer {
		return "engineer", nil
	}

	return "none", nil
}

// === Дефекты ===
func (r *repository) CreateDefect(defect *models.Defect) error {
	return r.db.Create(defect).Error
}

func (r *repository) FindDefectByID(id uuid.UUID) (*models.Defect, error) {
	var defect models.Defect
	err := r.db.Preload("Comments").First(&defect, "id = ?", id).Error
	return &defect, err
}

func (r *repository) UpdateDefect(defect *models.Defect) error {
	return r.db.Save(defect).Error
}

func (r *repository) DeleteDefect(id uuid.UUID) error {
	return r.db.Delete(&models.Defect{}, "id = ?", id).Error
}

func (r *repository) ListDefects(filter DefectFilter, limit, offset int, sortBy, sortOrder string) ([]models.Defect, error) {
	var defects []models.Defect

	query := r.db.Model(&models.Defect{}).Preload("Comments")

	if filter.ProjectID != uuid.Nil {
		query = query.Where("project_id = ?", filter.ProjectID)
	}
	if filter.EngineerID != uuid.Nil {
		query = query.Where("engineer_id = ?", filter.EngineerID)
	}
	if filter.ReporterID != uuid.Nil {
		query = query.Where("reporter_id = ?", filter.ReporterID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Priority != "" {
		query = query.Where("priority = ?", filter.Priority)
	}
	if filter.Search != "" {
		query = query.Where("title ILIKE ? OR description ILIKE ?",
			"%"+filter.Search+"%", "%"+filter.Search+"%")
	}

	// Сортировка
	if sortBy == "" {
		sortBy = "created_at"
	}
	if sortOrder == "" {
		sortOrder = "DESC"
	}
	query = query.Order(sortBy + " " + sortOrder)

	err := query.Limit(limit).Offset(offset).Find(&defects).Error
	return defects, err
}

func (r *repository) CountDefects(filter DefectFilter) (int64, error) {
	var count int64

	query := r.db.Model(&models.Defect{})

	if filter.ProjectID != uuid.Nil {
		query = query.Where("project_id = ?", filter.ProjectID)
	}
	if filter.EngineerID != uuid.Nil {
		query = query.Where("engineer_id = ?", filter.EngineerID)
	}
	if filter.ReporterID != uuid.Nil {
		query = query.Where("reporter_id = ?", filter.ReporterID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Priority != "" {
		query = query.Where("priority = ?", filter.Priority)
	}
	if filter.Search != "" {
		query = query.Where("title ILIKE ? OR description ILIKE ?",
			"%"+filter.Search+"%", "%"+filter.Search+"%")
	}

	err := query.Count(&count).Error
	return count, err
}

// === Комментарии ===
func (r *repository) CreateComment(comment *models.Comment) error {
	return r.db.Create(comment).Error
}

func (r *repository) FindCommentByID(id uuid.UUID) (*models.Comment, error) {
	var comment models.Comment
	err := r.db.First(&comment, "id = ?", id).Error
	return &comment, err
}

func (r *repository) UpdateComment(comment *models.Comment) error {
	return r.db.Save(comment).Error
}

func (r *repository) DeleteComment(id uuid.UUID) error {
	return r.db.Delete(&models.Comment{}, "id = ?", id).Error
}

func (r *repository) ListComments(defectID uuid.UUID, limit, offset int, sortBy, sortOrder string) ([]models.Comment, error) {
	var comments []models.Comment

	query := r.db.Where("defect_id = ?", defectID)

	if sortBy == "" {
		sortBy = "created_at"
	}
	if sortOrder == "" {
		sortOrder = "ASC"
	}

	err := query.Order(sortBy + " " + sortOrder).
		Limit(limit).
		Offset(offset).
		Find(&comments).Error

	return comments, err
}

func (r *repository) CountComments(defectID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&models.Comment{}).
		Where("defect_id = ?", defectID).
		Count(&count).Error
	return count, err
}
