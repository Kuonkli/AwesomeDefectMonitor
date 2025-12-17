// internal/handlers/project_handler.go
package handlers

import (
	pb "awesome-defect-tracker/shared/protogen/project"
	"net/http"
	"strconv"
	"strings"
	"time"

	"awesome-defect-tracker/gateway-service/internal/clients"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ProjectHandler struct {
	projectClient *clients.ProjectClient
}

func NewProjectHandler(projectClient *clients.ProjectClient) *ProjectHandler {
	return &ProjectHandler{
		projectClient: projectClient,
	}
}

// Структуры запросов
type CreateProjectRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	ManagerID   string   `json:"manager_id" binding:"required,uuid"`
	EngineerIDs []string `json:"engineer_ids" binding:"dive,uuid"`
	StartDate   string   `json:"start_date"` // ISO 8601 format
	EndDate     string   `json:"end_date"`   // ISO 8601 format
}

type UpdateProjectRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
	StartDate   *string `json:"start_date"`
	EndDate     *string `json:"end_date"`
}

type AddEngineersRequest struct {
	EngineerIDs []string `json:"engineer_ids" binding:"required,min=1,dive,uuid"`
}

// Маршруты проектов
func (h *ProjectHandler) CreateProject(c *gin.Context) {
	var req CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Парсим даты
	var startDate, endDate *time.Time
	if req.StartDate != "" {
		if t, err := time.Parse(time.RFC3339, req.StartDate); err == nil {
			startDate = &t
		}
	}
	if req.EndDate != "" {
		if t, err := time.Parse(time.RFC3339, req.EndDate); err == nil {
			endDate = &t
		}
	}

	project, err := h.projectClient.CreateProject(c.Request.Context(),
		req.Name, req.Description, req.ManagerID, req.EngineerIDs,
		startDate, endDate)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusCreated, project)
}

func (h *ProjectHandler) GetProject(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}

	project, err := h.projectClient.GetProject(c.Request.Context(), id)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, project)
}

func (h *ProjectHandler) ListProjects(c *gin.Context) {
	managerID := c.Query("manager_id")
	statusStr := c.Query("status")
	engineerID := c.Query("engineer_id")
	search := c.Query("search")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	var status pb.ProjectStatus
	if statusStr != "" {
		status = stringToProjectStatus(statusStr)
	}

	response, err := h.projectClient.ListProjects(c.Request.Context(),
		managerID, engineerID, search, status, int32(page), int32(limit))
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *ProjectHandler) UpdateProject(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}

	var req UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	grpcReq := &pb.UpdateProjectRequest{
		ProjectId: id,
	}

	if req.Name != nil {
		grpcReq.Name = &pb.OptionalString{Value: *req.Name}
	}
	if req.Description != nil {
		grpcReq.Description = &pb.OptionalString{Value: *req.Description}
	}
	if req.Status != nil {
		status := stringToProjectStatus(*req.Status)
		grpcReq.Status = &pb.OptionalProjectStatus{Value: status}
	}
	if req.StartDate != nil {
		if t, err := time.Parse(time.RFC3339, *req.StartDate); err == nil {
			grpcReq.StartDate = &pb.OptionalTimestamp{Value: timestamppb.New(t)}
		}
	}
	if req.EndDate != nil {
		if t, err := time.Parse(time.RFC3339, *req.EndDate); err == nil {
			grpcReq.EndDate = &pb.OptionalTimestamp{Value: timestamppb.New(t)}
		}
	}

	project, err := h.projectClient.UpdateProject(c.Request.Context(), grpcReq)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, project)
}

func (h *ProjectHandler) DeleteProject(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}

	err := h.projectClient.DeleteProject(c.Request.Context(), id)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "project deleted"})
}

func (h *ProjectHandler) AddEngineers(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}

	var req AddEngineersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	project, err := h.projectClient.AddEngineers(c.Request.Context(), id, req.EngineerIDs)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, project)
}

func (h *ProjectHandler) RemoveEngineer(c *gin.Context) {
	projectID := c.Param("id")
	engineerID := c.Param("engineer_id")

	if _, err := uuid.Parse(projectID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	if _, err := uuid.Parse(engineerID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid engineer id"})
		return
	}

	project, err := h.projectClient.RemoveEngineer(c.Request.Context(), projectID, engineerID)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, project)
}

func (h *ProjectHandler) ValidateAccess(c *gin.Context) {
	projectID := c.Param("id")
	userID := c.GetString("user_id")

	if _, err := uuid.Parse(projectID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}

	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	response, err := h.projectClient.ValidateAccess(c.Request.Context(), projectID, userID)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// Вспомогательные функции
func stringToProjectStatus(s string) pb.ProjectStatus {
	switch strings.ToLower(s) {
	case "active":
		return pb.ProjectStatus_PROJECT_STATUS_ACTIVE
	case "planning":
		return pb.ProjectStatus_PROJECT_STATUS_PLANNING
	case "completed":
		return pb.ProjectStatus_PROJECT_STATUS_COMPLETED
	case "archived":
		return pb.ProjectStatus_PROJECT_STATUS_ARCHIVED
	default:
		return pb.ProjectStatus_PROJECT_STATUS_UNSPECIFIED
	}
}
