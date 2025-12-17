package handlers

import (
	user_contracts "awesome-defect-tracker/shared/protogen/user"
	"net/http"

	"awesome-defect-tracker/gateway-service/internal/clients"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	userClient *clients.UserClient
}

func NewAuthHandler(userClient *clients.UserClient) *AuthHandler {
	return &AuthHandler{
		userClient: userClient,
	}
}

type RegisterRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=6"`
	FirstName string `json:"first_name" binding:"required"`
	LastName  string `json:"last_name" binding:"required"`
	Role      string `json:"role" binding:"required,oneof=engineer manager supervisor"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type AuthResponse struct {
	User struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Role      string `json:"role"`
		IsActive  bool   `json:"is_active"`
	} `json:"user"`
	Tokens struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresAt    int64  `json:"expires_at"`
	} `json:"tokens"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Вызываем User Service через gRPC
	resp, err := h.userClient.Register(c.Request.Context(),
		req.Email, req.Password, req.FirstName, req.LastName, req.Role)

	if err != nil {
		handleGRPCError(c, err)
		return
	}

	// Преобразуем gRPC ответ в HTTP ответ
	c.JSON(http.StatusCreated, transformAuthResponse(resp))
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Вызываем User Service через gRPC
	resp, err := h.userClient.Login(c.Request.Context(), req.Email, req.Password)

	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, transformAuthResponse(resp))
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.userClient.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  resp.Tokens.AccessToken,
		"refresh_token": resp.Tokens.RefreshToken,
		"expires_at":    resp.Tokens.ExpiresAt,
	})
}

func transformAuthResponse(resp *user_contracts.AuthResponse) AuthResponse {
	var authResp AuthResponse

	if resp.User != nil {
		authResp.User.ID = resp.User.Id
		authResp.User.Email = resp.User.Email
		authResp.User.FirstName = resp.User.FirstName
		authResp.User.LastName = resp.User.LastName
		authResp.User.Role = resp.User.Role
		authResp.User.IsActive = resp.User.IsActive
	}

	if resp.Tokens != nil {
		authResp.Tokens.AccessToken = resp.Tokens.AccessToken
		authResp.Tokens.RefreshToken = resp.Tokens.RefreshToken
		authResp.Tokens.ExpiresAt = resp.Tokens.ExpiresAt
	}

	return authResp
}
