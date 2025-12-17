package handlers

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http"
	"strconv"

	"awesome-defect-tracker/gateway-service/internal/clients"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UserHandler struct {
	userClient *clients.UserClient
}

func NewUserHandler(userClient *clients.UserClient) *UserHandler {
	return &UserHandler{
		userClient: userClient,
	}
}

type UpdateProfileRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

type UpdateRoleRequest struct {
	Role string `json:"role" binding:"required,oneof=engineer manager supervisor"`
}

func (h *UserHandler) GetProfile(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	user, err := h.userClient.GetProfile(c.Request.Context(), userID)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.userClient.UpdateProfile(c.Request.Context(), userID, req.FirstName, req.LastName)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) ListUsers(c *gin.Context) {
	role := c.Query("role")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	resp, err := h.userClient.ListUsers(c.Request.Context(), role, int32(page), int32(limit))
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// UpdateUserRole обновляет роль пользователя
func (h *UserHandler) UpdateUserRole(c *gin.Context) {
	userID := c.Param("id")

	if _, err := uuid.Parse(userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id format"})
		return
	}

	var req UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Проверяем валидность роли
	validRoles := map[string]bool{
		"engineer":   true,
		"manager":    true,
		"supervisor": true,
	}
	if !validRoles[req.Role] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role. Allowed: engineer, manager, supervisor"})
		return
	}

	// Вызываем User Service
	user, err := h.userClient.UpdateUserRole(c.Request.Context(), userID, req.Role)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"user":    user,
			"message": "user role updated successfully",
		},
	})
}

// DeactivateUser деактивирует пользователя
func (h *UserHandler) DeactivateUser(c *gin.Context) {
	userID := c.Param("id")

	if _, err := uuid.Parse(userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id format"})
		return
	}

	// Вызываем User Service
	_, err := h.userClient.DeactivateUser(c.Request.Context(), userID)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "user deactivated successfully",
	})
}

// ActivateUser активирует пользователя
func (h *UserHandler) ActivateUser(c *gin.Context) {
	userID := c.Param("id")

	if _, err := uuid.Parse(userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id format"})
		return
	}

	// Вызываем User Service
	_, err := h.userClient.ActivateUser(c.Request.Context(), userID)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "user activated successfully",
	})
}

func (h *UserHandler) GetUser(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	user, err := h.userClient.GetProfile(c.Request.Context(), id)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) ChangePassword(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Note: Нужно добавить метод ChangePassword в UserClient
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented yet"})
}

// Вспомогательная функция для обработки ошибок
func handleGRPCError(c *gin.Context, err error) {
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.AlreadyExists:
			c.JSON(http.StatusConflict, gin.H{"error": st.Message()})
		case codes.NotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": st.Message()})
		case codes.Unauthenticated:
			c.JSON(http.StatusUnauthorized, gin.H{"error": st.Message()})
		case codes.PermissionDenied:
			c.JSON(http.StatusForbidden, gin.H{"error": st.Message()})
		case codes.InvalidArgument:
			c.JSON(http.StatusBadRequest, gin.H{"error": st.Message()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
