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

type DefectHandler struct {
	projectClient *clients.ProjectClient
}

func NewDefectHandler(projectClient *clients.ProjectClient) *DefectHandler {
	return &DefectHandler{
		projectClient: projectClient,
	}
}

// Структуры запросов
type CreateDefectRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	Priority    string `json:"priority" binding:"required,oneof=low medium high"`
	ProjectID   string `json:"project_id" binding:"required,uuid"`
	EngineerID  string `json:"engineer_id" binding:"required,uuid"`
	DueDate     string `json:"due_date"` // ISO 8601 format
}

type UpdateDefectRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	DueDate     *string `json:"due_date"`
}

type UpdateStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=new in_work in_review closed cancelled"`
}

type UpdatePriorityRequest struct {
	Priority string `json:"priority" binding:"required,oneof=low medium high"`
}

type AssignEngineerRequest struct {
	EngineerID string `json:"engineer_id" binding:"required,uuid"`
}

type AddCommentRequest struct {
	Text string `json:"text" binding:"required,min=1,max=1000"`
}

type UpdateCommentRequest struct {
	Text string `json:"text" binding:"required,min=1,max=1000"`
}

// Маршруты дефектов
func (h *DefectHandler) CreateDefect(c *gin.Context) {
	var req CreateDefectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Получаем reporter_id из токена
	reporterID := c.GetString("user_id")
	if reporterID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	// Парсим дату
	var dueDatePB *timestamppb.Timestamp
	if req.DueDate != "" {
		if t, err := time.Parse(time.RFC3339, req.DueDate); err == nil {
			dueDatePB = timestamppb.New(t)
		}
	}

	priority := stringToDefectPriority(req.Priority)

	grpcReq := &pb.CreateDefectRequest{
		Title:       req.Title,
		Description: req.Description,
		Priority:    priority,
		ProjectId:   req.ProjectID,
		EngineerId:  req.EngineerID,
	}

	if dueDatePB != nil {
		grpcReq.DueDate = dueDatePB
	}

	defect, err := h.projectClient.CreateDefect(c.Request.Context(), grpcReq)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusCreated, defect)
}

func (h *DefectHandler) GetDefect(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid defect id"})
		return
	}

	defect, err := h.projectClient.GetDefect(c.Request.Context(), id)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, defect)
}

func (h *DefectHandler) ListDefects(c *gin.Context) {
	projectID := c.Query("project_id")
	engineerID := c.Query("engineer_id")
	reporterID := c.Query("reporter_id")
	statusStr := c.Query("status")
	priorityStr := c.Query("priority")
	search := c.Query("search")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	// Создаем фильтр
	filter := &pb.DefectFilter{}

	if projectID != "" {
		filter.ProjectId = &pb.OptionalString{Value: projectID}
	}
	if engineerID != "" {
		filter.EngineerId = &pb.OptionalString{Value: engineerID}
	}
	if reporterID != "" {
		filter.ReporterId = &pb.OptionalString{Value: reporterID}
	}
	if statusStr != "" {
		status := stringToDefectStatus(statusStr)
		filter.Status = &pb.OptionalDefectStatus{Value: status}
	}
	if priorityStr != "" {
		priority := stringToDefectPriority(priorityStr)
		filter.Priority = &pb.OptionalDefectPriority{Value: priority}
	}
	if search != "" {
		filter.Search = &pb.OptionalString{Value: search}
	}

	req := &pb.ListDefectsRequest{
		Filter: filter,
		Pagination: &pb.PaginationRequest{
			Page:  int32(page),
			Limit: int32(limit),
		},
	}

	response, err := h.projectClient.ListDefects(c.Request.Context(), req.Filter, req.Pagination.Page, req.Pagination.Limit)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *DefectHandler) UpdateDefect(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid defect id"})
		return
	}

	var req UpdateDefectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	grpcReq := &pb.UpdateDefectRequest{
		DefectId: id,
	}

	if req.Title != nil {
		grpcReq.Title = &pb.OptionalString{Value: *req.Title}
	}
	if req.Description != nil {
		grpcReq.Description = &pb.OptionalString{Value: *req.Description}
	}
	if req.DueDate != nil {
		if t, err := time.Parse(time.RFC3339, *req.DueDate); err == nil {
			grpcReq.DueDate = &pb.OptionalTimestamp{Value: timestamppb.New(t)}
		}
	}

	defect, err := h.projectClient.UpdateDefect(c.Request.Context(), grpcReq)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, defect)
}

func (h *DefectHandler) DeleteDefect(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid defect id"})
		return
	}

	err := h.projectClient.DeleteDefect(c.Request.Context(), id)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "defect deleted"})
}

func (h *DefectHandler) UpdateDefectStatus(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid defect id"})
		return
	}

	var req UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status := stringToDefectStatus(req.Status)

	defect, err := h.projectClient.UpdateDefectStatus(c.Request.Context(), id, status)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, defect)
}

func (h *DefectHandler) UpdateDefectPriority(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid defect id"})
		return
	}

	var req UpdatePriorityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	priority := stringToDefectPriority(req.Priority)

	defect, err := h.projectClient.UpdateDefectPriority(c.Request.Context(), id, priority)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, defect)
}

func (h *DefectHandler) AssignEngineer(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid defect id"})
		return
	}

	var req AssignEngineerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	defect, err := h.projectClient.AssignEngineer(c.Request.Context(), id, req.EngineerID)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, defect)
}

// Маршруты комментариев
func (h *DefectHandler) AddComment(c *gin.Context) {
	defectID := c.Param("id")
	if _, err := uuid.Parse(defectID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid defect id"})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	var req AddCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	comment, err := h.projectClient.AddComment(c.Request.Context(), defectID, req.Text, userID)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusCreated, comment)
}

func (h *DefectHandler) UpdateComment(c *gin.Context) {
	commentID := c.Param("comment_id")
	if _, err := uuid.Parse(commentID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid comment id"})
		return
	}

	var req UpdateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	comment, err := h.projectClient.UpdateComment(c.Request.Context(), commentID, req.Text)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, comment)
}

func (h *DefectHandler) DeleteComment(c *gin.Context) {
	commentID := c.Param("comment_id")
	if _, err := uuid.Parse(commentID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid comment id"})
		return
	}

	err := h.projectClient.DeleteComment(c.Request.Context(), commentID)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "comment deleted"})
}

func (h *DefectHandler) GetComments(c *gin.Context) {
	defectID := c.Param("id")
	if _, err := uuid.Parse(defectID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid defect id"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	response, err := h.projectClient.GetComments(c.Request.Context(), defectID, int32(page), int32(limit))
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// Вспомогательные функции
func stringToDefectStatus(s string) pb.DefectStatus {
	switch strings.ToLower(s) {
	case "new":
		return pb.DefectStatus_DEFECT_STATUS_NEW
	case "in_work":
		return pb.DefectStatus_DEFECT_STATUS_IN_WORK
	case "in_review":
		return pb.DefectStatus_DEFECT_STATUS_IN_REVIEW
	case "closed":
		return pb.DefectStatus_DEFECT_STATUS_CLOSED
	case "cancelled":
		return pb.DefectStatus_DEFECT_STATUS_CANCELLED
	default:
		return pb.DefectStatus_DEFECT_STATUS_NEW
	}
}

func stringToDefectPriority(s string) pb.DefectPriority {
	switch strings.ToLower(s) {
	case "low":
		return pb.DefectPriority_DEFECT_PRIORITY_LOW
	case "medium":
		return pb.DefectPriority_DEFECT_PRIORITY_MEDIUM
	case "high":
		return pb.DefectPriority_DEFECT_PRIORITY_HIGH
	default:
		return pb.DefectPriority_DEFECT_PRIORITY_MEDIUM
	}
}
