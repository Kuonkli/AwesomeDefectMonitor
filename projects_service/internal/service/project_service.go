package service

import (
	"awesome-defect-tracker/projects-service/internal/models"
	"awesome-defect-tracker/projects-service/internal/repository"
	pb "awesome-defect-tracker/shared/protogen/project"
	pu "awesome-defect-tracker/shared/protogen/user"
	"context"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"log"
)

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrDefectNotFound  = errors.New("defect not found")
	ErrCommentNotFound = errors.New("comment not found")
	ErrNoAccess        = errors.New("no access to project")
)

// UserClient для получения данных пользователей
type UserClient interface {
	GetUser(ctx context.Context, userID string) (*pu.User, error)
	GetUsers(ctx context.Context, userIDs []string) (map[string]*pu.User, error)
}

type ProjectService struct {
	pb.UnimplementedProjectServiceServer
	repo       repository.Repository
	userClient UserClient
}

func NewProjectService(repo repository.Repository, userClient UserClient) *ProjectService {
	return &ProjectService{
		repo:       repo,
		userClient: userClient,
	}
}

// === Проекты ===
func (s *ProjectService) CreateProject(ctx context.Context, req *pb.CreateProjectRequest) (*pb.Project, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "project name is required")
	}

	managerID, err := uuid.Parse(req.ManagerId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid manager id")
	}

	project := &models.Project{
		Name:        req.Name,
		Description: req.Description,
		ManagerID:   managerID,
		Status:      models.ProjectStatusActive,
	}

	if req.StartDate != nil {
		project.StartDate = req.StartDate.AsTime()
	}
	if req.EndDate != nil {
		project.EndDate = req.EndDate.AsTime()
	}

	if err := s.repo.CreateProject(project); err != nil {
		log.Printf("Failed to create project: %v", err)
		return nil, status.Error(codes.Internal, "failed to create project")
	}

	// Добавляем инженеров
	for _, engIDStr := range req.EngineerIds {
		engineerID, err := uuid.Parse(engIDStr)
		if err != nil {
			continue
		}
		if err := s.repo.AddProjectEngineer(project.ID, engineerID); err != nil {
			log.Printf("Failed to add engineer %s to project %s: %v", engIDStr, project.ID, err)
		}
	}

	return s.getProjectWithUsers(ctx, project.ID)
}

func (s *ProjectService) GetProject(ctx context.Context, req *pb.GetProjectRequest) (*pb.Project, error) {
	projectID, err := uuid.Parse(req.ProjectId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid project id")
	}

	return s.getProjectWithUsers(ctx, projectID)
}

func (s *ProjectService) getProjectWithUsers(ctx context.Context, projectID uuid.UUID) (*pb.Project, error) {
	project, err := s.repo.FindProjectByID(projectID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "project not found")
	}

	// Собираем все UserID для запроса
	userIDs := []string{project.ManagerID.String()}
	engineerIDs, _ := s.repo.GetProjectEngineers(projectID)
	for _, engID := range engineerIDs {
		userIDs = append(userIDs, engID.String())
	}

	// Получаем пользователей
	users, err := s.userClient.GetUsers(ctx, userIDs)
	if err != nil {
		log.Printf("Failed to get users: %v", err)
		// Продолжаем без данных пользователей
	}

	// Конвертируем в protobuf
	return s.projectToProto(project, users, engineerIDs), nil
}

func (s *ProjectService) ListProjects(ctx context.Context, req *pb.ListProjectsRequest) (*pb.ListProjectsResponse, error) {
	filter := repository.ProjectFilter{}

	if req.ManagerId != "" {
		managerID, err := uuid.Parse(req.ManagerId)
		if err == nil {
			filter.ManagerID = managerID
		}
	}

	if req.EngineerId != "" {
		engineerID, err := uuid.Parse(req.EngineerId)
		if err == nil {
			filter.EngineerID = engineerID
		}
	}

	if req.Search != "" {
		filter.Search = req.Search
	}

	limit := 10
	if req.Limit > 0 && req.Limit <= 100 {
		limit = int(req.Limit)
	}

	page := 1
	if req.Page > 0 {
		page = int(req.Page)
	}
	offset := (page - 1) * limit

	projects, err := s.repo.ListProjects(filter, limit, offset)
	if err != nil {
		log.Printf("Failed to list projects: %v", err)
		return nil, status.Error(codes.Internal, "failed to list projects")
	}

	total, err := s.repo.CountProjects(filter)
	if err != nil {
		total = int64(len(projects))
	}

	// Собираем всех пользователей
	userIDs := make([]string, 0)
	for _, project := range projects {
		userIDs = append(userIDs, project.ManagerID.String())
		engineerIDs, _ := s.repo.GetProjectEngineers(project.ID)
		for _, engID := range engineerIDs {
			userIDs = append(userIDs, engID.String())
		}
	}

	users, _ := s.userClient.GetUsers(ctx, userIDs)

	// Конвертируем проекты
	protoProjects := make([]*pb.Project, len(projects))
	for i, project := range projects {
		engineerIDs, _ := s.repo.GetProjectEngineers(project.ID)
		protoProjects[i] = s.projectToProto(&project, users, engineerIDs)
	}

	return &pb.ListProjectsResponse{
		Projects: protoProjects,
		Total:    int32(total),
		Page:     req.Page,
		Limit:    req.Limit,
	}, nil
}

func (s *ProjectService) UpdateProject(ctx context.Context, req *pb.UpdateProjectRequest) (*pb.Project, error) {
	projectID, err := uuid.Parse(req.ProjectId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid project id")
	}

	project, err := s.repo.FindProjectByID(projectID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "project not found")
	}

	// Обновляем поля
	if req.Name != nil {
		project.Name = req.Name.String()
	}
	if req.Description != nil {
		project.Description = req.Description.String()
	}
	if req.Status != nil {
		project.Status = s.protoToProjectStatus(req.Status.Value)
	}
	if req.StartDate != nil {
		project.StartDate = req.StartDate.Value.AsTime()
	}
	if req.EndDate != nil {
		project.EndDate = req.EndDate.Value.AsTime()
	}

	if err := s.repo.UpdateProject(project); err != nil {
		log.Printf("Failed to update project: %v", err)
		return nil, status.Error(codes.Internal, "failed to update project")
	}

	return s.getProjectWithUsers(ctx, projectID)
}

func (s *ProjectService) DeleteProject(ctx context.Context, req *pb.GetProjectRequest) (*emptypb.Empty, error) {
	projectID, err := uuid.Parse(req.ProjectId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid project id")
	}

	if err := s.repo.DeleteProject(projectID); err != nil {
		log.Printf("Failed to delete project: %v", err)
		return nil, status.Error(codes.Internal, "failed to delete project")
	}

	return &emptypb.Empty{}, nil
}

func (s *ProjectService) AddEngineers(ctx context.Context, req *pb.AddEngineersRequest) (*pb.Project, error) {
	projectID, err := uuid.Parse(req.ProjectId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid project id")
	}

	// Проверяем существование проекта
	_, err = s.repo.FindProjectByID(projectID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "project not found")
	}

	// Добавляем инженеров
	for _, engIDStr := range req.EngineerIds {
		engineerID, err := uuid.Parse(engIDStr)
		if err != nil {
			continue
		}

		if err := s.repo.AddProjectEngineer(projectID, engineerID); err != nil {
			log.Printf("Failed to add engineer %s to project %s: %v", engIDStr, projectID, err)
		}
	}

	return s.getProjectWithUsers(ctx, projectID)
}

func (s *ProjectService) RemoveEngineer(ctx context.Context, req *pb.RemoveEngineerRequest) (*pb.Project, error) {
	projectID, err := uuid.Parse(req.ProjectId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid project id")
	}

	engineerID, err := uuid.Parse(req.EngineerId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid engineer id")
	}

	if err := s.repo.RemoveProjectEngineer(projectID, engineerID); err != nil {
		log.Printf("Failed to remove engineer: %v", err)
		return nil, status.Error(codes.Internal, "failed to remove engineer")
	}

	return s.getProjectWithUsers(ctx, projectID)
}

func (s *ProjectService) ValidateAccess(ctx context.Context, req *pb.ValidateAccessRequest) (*pb.ValidateAccessResponse, error) {
	projectID, err := uuid.Parse(req.ProjectId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid project id")
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user id")
	}

	role, err := s.repo.GetUserRoleInProject(projectID, userID)
	if err != nil {
		return &pb.ValidateAccessResponse{
			HasAccess:     false,
			RoleInProject: "none",
		}, nil
	}

	return &pb.ValidateAccessResponse{
		HasAccess:     role != "none",
		RoleInProject: role,
	}, nil
}

// === Дефекты ===
func (s *ProjectService) CreateDefect(ctx context.Context, req *pb.CreateDefectRequest) (*pb.Defect, error) {
	if req.Title == "" {
		return nil, status.Error(codes.InvalidArgument, "defect title is required")
	}

	projectID, err := uuid.Parse(req.ProjectId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid project id")
	}

	engineerID, err := uuid.Parse(req.EngineerId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid engineer id")
	}

	// Здесь нужно получить reporterID из токена (через middleware)
	// Пока используем заглушку - engineer как reporter
	reporterID := engineerID

	defect := &models.Defect{
		Title:       req.Title,
		Description: req.Description,
		Priority:    s.protoToDefectPriority(req.Priority),
		ProjectID:   projectID,
		EngineerID:  engineerID,
		ReporterID:  reporterID,
		Status:      models.DefectStatusNew,
	}

	if req.DueDate != nil {
		defect.DueDate = req.DueDate.AsTime()
	}

	if err := s.repo.CreateDefect(defect); err != nil {
		log.Printf("Failed to create defect: %v", err)
		return nil, status.Error(codes.Internal, "failed to create defect")
	}

	return s.getDefectWithUsers(ctx, defect.ID)
}

func (s *ProjectService) GetDefect(ctx context.Context, req *pb.GetDefectRequest) (*pb.Defect, error) {
	defectID, err := uuid.Parse(req.DefectId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid defect id")
	}

	return s.getDefectWithUsers(ctx, defectID)
}

func (s *ProjectService) getDefectWithUsers(ctx context.Context, defectID uuid.UUID) (*pb.Defect, error) {
	defect, err := s.repo.FindDefectByID(defectID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "defect not found")
	}

	// Получаем проект для defect count
	project, _ := s.repo.FindProjectByID(defect.ProjectID)

	// Получаем пользователей
	userIDs := []string{
		defect.EngineerID.String(),
		defect.ReporterID.String(),
	}
	if project != nil {
		userIDs = append(userIDs, project.ManagerID.String())
	}

	users, _ := s.userClient.GetUsers(ctx, userIDs)

	// Получаем комментарии с пользователями
	comments, _ := s.repo.ListComments(defectID, 50, 0, "created_at", "asc")
	var commentUsers []string
	for _, comment := range comments {
		commentUsers = append(commentUsers, comment.UserID.String())
	}

	commentUsersMap, _ := s.userClient.GetUsers(ctx, commentUsers)

	return s.defectToProto(defect, project, users, comments, commentUsersMap), nil
}

// service/project_service.go - продолжение

func (s *ProjectService) UpdateDefect(ctx context.Context, req *pb.UpdateDefectRequest) (*pb.Defect, error) {
	defectID, err := uuid.Parse(req.DefectId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid defect id")
	}

	defect, err := s.repo.FindDefectByID(defectID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "defect not found")
	}

	// Обновляем поля
	if req.Title != nil {
		defect.Title = req.Title.String()
	}
	if req.Description != nil {
		defect.Description = req.Description.String()
	}
	if req.DueDate != nil {
		defect.DueDate = req.DueDate.Value.AsTime()
	}

	if err := s.repo.UpdateDefect(defect); err != nil {
		log.Printf("Failed to update defect: %v", err)
		return nil, status.Error(codes.Internal, "failed to update defect")
	}

	return s.getDefectWithUsers(ctx, defectID)
}

func (s *ProjectService) UpdateDefectStatus(ctx context.Context, req *pb.UpdateDefectStatusRequest) (*pb.Defect, error) {
	defectID, err := uuid.Parse(req.DefectId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid defect id")
	}

	defect, err := s.repo.FindDefectByID(defectID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "defect not found")
	}

	defect.Status = s.protoToDefectStatus(req.Status)

	if err := s.repo.UpdateDefect(defect); err != nil {
		log.Printf("Failed to update defect status: %v", err)
		return nil, status.Error(codes.Internal, "failed to update defect status")
	}

	return s.getDefectWithUsers(ctx, defectID)
}

func (s *ProjectService) UpdateDefectPriority(ctx context.Context, req *pb.UpdateDefectPriorityRequest) (*pb.Defect, error) {
	defectID, err := uuid.Parse(req.DefectId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid defect id")
	}

	defect, err := s.repo.FindDefectByID(defectID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "defect not found")
	}

	defect.Priority = s.protoToDefectPriority(req.Priority)

	if err := s.repo.UpdateDefect(defect); err != nil {
		log.Printf("Failed to update defect priority: %v", err)
		return nil, status.Error(codes.Internal, "failed to update defect priority")
	}

	return s.getDefectWithUsers(ctx, defectID)
}

func (s *ProjectService) AssignEngineer(ctx context.Context, req *pb.AssignEngineerRequest) (*pb.Defect, error) {
	defectID, err := uuid.Parse(req.DefectId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid defect id")
	}

	engineerID, err := uuid.Parse(req.EngineerId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid engineer id")
	}

	defect, err := s.repo.FindDefectByID(defectID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "defect not found")
	}

	defect.EngineerID = engineerID
	defect.Status = models.DefectStatusInWork

	if err := s.repo.UpdateDefect(defect); err != nil {
		log.Printf("Failed to assign engineer: %v", err)
		return nil, status.Error(codes.Internal, "failed to assign engineer")
	}

	return s.getDefectWithUsers(ctx, defectID)
}

func (s *ProjectService) DeleteDefect(ctx context.Context, req *pb.GetDefectRequest) (*emptypb.Empty, error) {
	defectID, err := uuid.Parse(req.DefectId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid defect id")
	}

	if err := s.repo.DeleteDefect(defectID); err != nil {
		log.Printf("Failed to delete defect: %v", err)
		return nil, status.Error(codes.Internal, "failed to delete defect")
	}

	return &emptypb.Empty{}, nil
}

func (s *ProjectService) ListDefects(ctx context.Context, req *pb.ListDefectsRequest) (*pb.ListDefectsResponse, error) {
	filter := repository.DefectFilter{}

	// Обрабатываем фильтры
	if req.Filter != nil {
		// ProjectID
		if req.Filter.ProjectId != nil && req.Filter.ProjectId.Value != "" {
			projectID, err := uuid.Parse(req.Filter.ProjectId.Value)
			if err == nil {
				filter.ProjectID = projectID
			}
		}

		// EngineerID
		if req.Filter.EngineerId != nil && req.Filter.EngineerId.Value != "" {
			engineerID, err := uuid.Parse(req.Filter.EngineerId.Value)
			if err == nil {
				filter.EngineerID = engineerID
			}
		}

		// ReporterID
		if req.Filter.ReporterId != nil && req.Filter.ReporterId.Value != "" {
			reporterID, err := uuid.Parse(req.Filter.ReporterId.Value)
			if err == nil {
				filter.ReporterID = reporterID
			}
		}

		// Status
		if req.Filter.Status != nil && req.Filter.Status.Value != pb.DefectStatus_DEFECT_STATUS_UNSPECIFIED {
			filter.Status = s.protoToDefectStatus(req.Filter.Status.Value)
		}

		// Priority
		if req.Filter.Priority != nil && req.Filter.Priority.Value != pb.DefectPriority_DEFECT_PRIORITY_UNSPECIFIED {
			filter.Priority = s.protoToDefectPriority(req.Filter.Priority.Value)
		}

		// Search
		if req.Filter.Search != nil && req.Filter.Search.Value != "" {
			filter.Search = req.Filter.Search.Value
		}
	}

	// Обрабатываем пагинацию
	limit := 10
	page := 1
	var sortBy, sortOrder string

	if req.Pagination != nil {
		if req.Pagination.Limit > 0 && req.Pagination.Limit <= 100 {
			limit = int(req.Pagination.Limit)
		}
		if req.Pagination.Page > 0 {
			page = int(req.Pagination.Page)
		}
		if req.Pagination.SortBy != "" {
			sortBy = req.Pagination.SortBy
		}
		if req.Pagination.SortOrder != "" {
			sortOrder = req.Pagination.SortOrder
		}
	}

	offset := (page - 1) * limit

	defects, err := s.repo.ListDefects(filter, limit, offset, sortBy, sortOrder)
	if err != nil {
		log.Printf("Failed to list defects: %v", err)
		return nil, status.Error(codes.Internal, "failed to list defects")
	}

	total, err := s.repo.CountDefects(filter)
	if err != nil {
		total = int64(len(defects))
	}

	// Собираем все userID для запроса пользователей
	userIDs := make([]string, 0)
	projectIDs := make([]string, 0)

	for _, defect := range defects {
		userIDs = append(userIDs,
			defect.EngineerID.String(),
			defect.ReporterID.String(),
		)
		projectIDs = append(projectIDs, defect.ProjectID.String())
	}

	// Получаем проекты для дефектов
	projects := make(map[string]*models.Project)
	for _, projectIDStr := range projectIDs {
		projectID, _ := uuid.Parse(projectIDStr)
		project, _ := s.repo.FindProjectByID(projectID)
		if project != nil {
			projects[projectIDStr] = project
			userIDs = append(userIDs, project.ManagerID.String())
		}
	}

	users, _ := s.userClient.GetUsers(ctx, userIDs)

	// Получаем комментарии для всех дефектов
	var allComments []models.Comment
	commentUsers := make(map[string]bool)

	for _, defect := range defects {
		comments, _ := s.repo.ListComments(defect.ID, 10, 0, "created_at", "asc")
		for _, comment := range comments {
			allComments = append(allComments, comment)
			commentUsers[comment.UserID.String()] = true
		}
	}

	// Получаем пользователей для комментариев
	commentUserIDs := make([]string, 0, len(commentUsers))
	for userID := range commentUsers {
		commentUserIDs = append(commentUserIDs, userID)
	}
	commentUsersMap, _ := s.userClient.GetUsers(ctx, commentUserIDs)

	// Группируем комментарии по дефектам
	commentsByDefect := make(map[uuid.UUID][]models.Comment)
	for _, comment := range allComments {
		commentsByDefect[comment.DefectID] = append(commentsByDefect[comment.DefectID], comment)
	}

	// Конвертируем дефекты
	protoDefects := make([]*pb.Defect, len(defects))
	for i, defect := range defects {
		project := projects[defect.ProjectID.String()]
		defectComments := commentsByDefect[defect.ID]

		protoDefects[i] = s.defectToProto(&defect, project, users, defectComments, commentUsersMap)
	}

	// Рассчитываем общее количество страниц
	totalPages := int32(0)
	if total > 0 && limit > 0 {
		totalPages = int32((total + int64(limit) - 1) / int64(limit))
	}

	paginatedResponse := &pb.PaginatedResponse{
		Total:      int32(total),
		Page:       int32(page),
		Limit:      int32(limit),
		TotalPages: totalPages,
	}

	return &pb.ListDefectsResponse{
		Defects:    protoDefects,
		Pagination: paginatedResponse,
	}, nil
}

// Методы для комментариев
func (s *ProjectService) AddComment(ctx context.Context, req *pb.AddCommentRequest) (*pb.Comment, error) {
	defectID, err := uuid.Parse(req.DefectId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid defect id")
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user id")
	}

	// Проверяем существование дефекта
	_, err = s.repo.FindDefectByID(defectID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "defect not found")
	}

	comment := &models.Comment{
		Text:     req.Text,
		UserID:   userID,
		DefectID: defectID,
	}

	if err := s.repo.CreateComment(comment); err != nil {
		log.Printf("Failed to add comment: %v", err)
		return nil, status.Error(codes.Internal, "failed to add comment")
	}

	// Получаем пользователя для комментария
	users, _ := s.userClient.GetUsers(ctx, []string{userID.String()})
	user, _ := users[userID.String()]

	return s.commentToProto(comment, user), nil
}

func (s *ProjectService) UpdateComment(ctx context.Context, req *pb.UpdateCommentRequest) (*pb.Comment, error) {
	commentID, err := uuid.Parse(req.CommentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid comment id")
	}

	comment, err := s.repo.FindCommentByID(commentID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "comment not found")
	}

	comment.Text = req.Text

	if err := s.repo.UpdateComment(comment); err != nil {
		log.Printf("Failed to update comment: %v", err)
		return nil, status.Error(codes.Internal, "failed to update comment")
	}

	// Получаем пользователя
	users, _ := s.userClient.GetUsers(ctx, []string{comment.UserID.String()})
	user, _ := users[comment.UserID.String()]

	return s.commentToProto(comment, user), nil
}

func (s *ProjectService) DeleteComment(ctx context.Context, req *pb.DeleteCommentRequest) (*emptypb.Empty, error) {
	commentID, err := uuid.Parse(req.CommentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid comment id")
	}

	if err := s.repo.DeleteComment(commentID); err != nil {
		log.Printf("Failed to delete comment: %v", err)
		return nil, status.Error(codes.Internal, "failed to delete comment")
	}

	return &emptypb.Empty{}, nil
}

func (s *ProjectService) GetComments(ctx context.Context, req *pb.GetCommentsRequest) (*pb.GetCommentsResponse, error) {
	defectID, err := uuid.Parse(req.DefectId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid defect id")
	}

	limit := 10
	if req.Limit > 0 && req.Limit <= 100 {
		limit = int(req.Limit)
	}

	page := 1
	if req.Page > 0 {
		page = int(req.Page)
	}
	offset := (page - 1) * limit

	comments, err := s.repo.ListComments(defectID, limit, offset, "created_at", "asc")
	if err != nil {
		log.Printf("Failed to list comments: %v", err)
		return nil, status.Error(codes.Internal, "failed to list comments")
	}

	total, err := s.repo.CountComments(defectID)
	if err != nil {
		total = int64(len(comments))
	}

	// Собираем userID
	userIDs := make([]string, len(comments))
	for i, comment := range comments {
		userIDs[i] = comment.UserID.String()
	}

	users, _ := s.userClient.GetUsers(ctx, userIDs)

	// Конвертируем комментарии
	protoComments := make([]*pb.Comment, len(comments))
	for i, comment := range comments {
		user, _ := users[comment.UserID.String()]
		protoComments[i] = s.commentToProto(&comment, user)
	}

	return &pb.GetCommentsResponse{
		Comments: protoComments,
		Total:    int32(total),
		Page:     req.Page,
		Limit:    req.Limit,
	}, nil
}

// Методы конвертации моделей в protobuf
func (s *ProjectService) projectToProto(project *models.Project, users map[string]*pu.User, engineerIDs []uuid.UUID) *pb.Project {
	engineerIdsStr := make([]string, len(engineerIDs))
	for i, id := range engineerIDs {
		engineerIdsStr[i] = id.String()
	}

	var protoDefects []*pb.Defect
	if project.Defects != nil && len(project.Defects) > 0 {
		protoDefects = make([]*pb.Defect, len(project.Defects))
		for i, defect := range project.Defects {
			protoDefects[i] = &pb.Defect{
				Id:          defect.ID.String(),
				Title:       defect.Title,
				Description: defect.Description,
				Status:      s.defectStatusToProto(defect.Status),
				Priority:    s.defectPriorityToProto(defect.Priority),
				ProjectId:   defect.ProjectID.String(),
				EngineerId:  defect.EngineerID.String(),
				ReporterId:  defect.ReporterID.String(),
				DueDate:     timestamppb.New(defect.DueDate),
				CreatedAt:   timestamppb.New(defect.CreatedAt),
				UpdatedAt:   timestamppb.New(defect.UpdatedAt),
			}
		}
	}

	return &pb.Project{
		Id:          project.ID.String(),
		Name:        project.Name,
		Description: project.Description,
		Status:      s.projectStatusToProto(project.Status),
		StartDate:   timestamppb.New(project.StartDate),
		EndDate:     timestamppb.New(project.EndDate),
		ManagerId:   project.ManagerID.String(), // Только ID
		EngineerIds: engineerIdsStr,
		CreatedAt:   timestamppb.New(project.CreatedAt),
		UpdatedAt:   timestamppb.New(project.UpdatedAt),
		Defects:     protoDefects,
	}
}

func (s *ProjectService) defectToProto(defect *models.Defect, project *models.Project, users map[string]*pu.User,
	comments []models.Comment, commentUsers map[string]*pu.User) *pb.Defect {

	// Конвертируем комментарии
	protoComments := make([]*pb.Comment, len(comments))
	for i, comment := range comments {
		user, _ := commentUsers[comment.UserID.String()]
		protoComments[i] = s.commentToProto(&comment, user)
	}

	return &pb.Defect{
		Id:          defect.ID.String(),
		Title:       defect.Title,
		Description: defect.Description,
		Status:      s.defectStatusToProto(defect.Status),
		Priority:    s.defectPriorityToProto(defect.Priority),
		DueDate:     timestamppb.New(defect.DueDate),
		CreatedAt:   timestamppb.New(defect.CreatedAt),
		UpdatedAt:   timestamppb.New(defect.UpdatedAt),
		ProjectId:   defect.ProjectID.String(),
		EngineerId:  defect.EngineerID.String(),
		ReporterId:  defect.ReporterID.String(),
		Comments:    protoComments,
	}
}

func (s *ProjectService) commentToProto(comment *models.Comment, user *pu.User) *pb.Comment {
	return &pb.Comment{
		Id:        comment.ID.String(),
		Text:      comment.Text,
		UserId:    comment.UserID.String(),
		CreatedAt: timestamppb.New(comment.CreatedAt),
		UpdatedAt: timestamppb.New(comment.UpdatedAt),
	}
}

func (s *ProjectService) protoToProjectStatus(status pb.ProjectStatus) models.ProjectStatus {
	switch status {
	case pb.ProjectStatus_PROJECT_STATUS_ACTIVE:
		return models.ProjectStatusActive
	case pb.ProjectStatus_PROJECT_STATUS_PLANNING:
		return models.ProjectStatusPlanning
	case pb.ProjectStatus_PROJECT_STATUS_COMPLETED:
		return models.ProjectStatusCompleted
	case pb.ProjectStatus_PROJECT_STATUS_ARCHIVED:
		return models.ProjectStatusArchived
	default:
		return models.ProjectStatusActive
	}
}

func (s *ProjectService) protoToDefectStatus(status pb.DefectStatus) models.DefectStatus {
	switch status {
	case pb.DefectStatus_DEFECT_STATUS_NEW:
		return models.DefectStatusNew
	case pb.DefectStatus_DEFECT_STATUS_IN_WORK:
		return models.DefectStatusInWork
	case pb.DefectStatus_DEFECT_STATUS_IN_REVIEW:
		return models.DefectStatusInReview
	case pb.DefectStatus_DEFECT_STATUS_CLOSED:
		return models.DefectStatusClosed
	case pb.DefectStatus_DEFECT_STATUS_CANCELLED:
		return models.DefectStatusCancelled
	default:
		return models.DefectStatusNew
	}
}

func (s *ProjectService) protoToDefectPriority(priority pb.DefectPriority) models.DefectPriority {
	switch priority {
	case pb.DefectPriority_DEFECT_PRIORITY_LOW:
		return models.DefectPriorityLow
	case pb.DefectPriority_DEFECT_PRIORITY_MEDIUM:
		return models.DefectPriorityMedium
	case pb.DefectPriority_DEFECT_PRIORITY_HIGH:
		return models.DefectPriorityHigh
	default:
		return models.DefectPriorityMedium
	}
}

func (s *ProjectService) projectStatusToProto(status models.ProjectStatus) pb.ProjectStatus {
	switch status {
	case models.ProjectStatusActive:
		return pb.ProjectStatus_PROJECT_STATUS_ACTIVE
	case models.ProjectStatusPlanning:
		return pb.ProjectStatus_PROJECT_STATUS_PLANNING
	case models.ProjectStatusCompleted:
		return pb.ProjectStatus_PROJECT_STATUS_COMPLETED
	case models.ProjectStatusArchived:
		return pb.ProjectStatus_PROJECT_STATUS_ARCHIVED
	default:
		return pb.ProjectStatus_PROJECT_STATUS_ACTIVE
	}
}

func (s *ProjectService) defectStatusToProto(status models.DefectStatus) pb.DefectStatus {
	switch status {
	case models.DefectStatusNew:
		return pb.DefectStatus_DEFECT_STATUS_NEW
	case models.DefectStatusInWork:
		return pb.DefectStatus_DEFECT_STATUS_IN_WORK
	case models.DefectStatusInReview:
		return pb.DefectStatus_DEFECT_STATUS_IN_REVIEW
	case models.DefectStatusClosed:
		return pb.DefectStatus_DEFECT_STATUS_CLOSED
	case models.DefectStatusCancelled:
		return pb.DefectStatus_DEFECT_STATUS_CANCELLED
	default:
		return pb.DefectStatus_DEFECT_STATUS_NEW
	}
}

func (s *ProjectService) defectPriorityToProto(priority models.DefectPriority) pb.DefectPriority {
	switch priority {
	case models.DefectPriorityLow:
		return pb.DefectPriority_DEFECT_PRIORITY_LOW
	case models.DefectPriorityMedium:
		return pb.DefectPriority_DEFECT_PRIORITY_MEDIUM
	case models.DefectPriorityHigh:
		return pb.DefectPriority_DEFECT_PRIORITY_HIGH
	default:
		return pb.DefectPriority_DEFECT_PRIORITY_MEDIUM
	}
}
