package middleware

import (
	"awesome-defect-tracker/gateway-service/internal/clients"
	"crypto/rsa"
	"github.com/gin-gonic/gin"
)

type AuthMiddleware struct {
	publicKey  *rsa.PublicKey
	userClient *clients.UserClient
}

func NewAuthMiddleware(publicKey *rsa.PublicKey, userClient *clients.UserClient) *AuthMiddleware {
	return &AuthMiddleware{
		publicKey:  publicKey,
		userClient: userClient,
	}
}

func (m *AuthMiddleware) Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Пропускаем публичные роуты
		if m.isPublicRoute(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Получаем access token
		accessToken := m.getAccessToken(c)
		if accessToken == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "missing access token"})
			return
		}

		// Валидируем токен через User Service
		resp, err := m.userClient.ValidateToken(c.Request.Context(), accessToken)
		if err == nil && resp.Valid {
			// Токен валиден
			c.Set("user_id", resp.UserId)
			c.Set("user_role", resp.Role)
			c.Next()
			return
		}

		// Получаем refresh token
		refreshToken := m.getRefreshToken(c)
		if refreshToken == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "missing refresh token"})
			return
		}

		// Токен не валиден, пытаемся обновить
		newTokens, err := m.userClient.RefreshToken(c.Request.Context(), refreshToken)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "token refresh failed"})
			return
		}

		// Устанавливаем новые токены
		m.setTokensInResponse(c, newTokens.Tokens.AccessToken, newTokens.Tokens.RefreshToken)

		// Валидируем новый токен
		resp, err = m.userClient.ValidateToken(c.Request.Context(), newTokens.Tokens.AccessToken)
		if err != nil || !resp.Valid {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid refreshed token"})
			return
		}

		// Устанавливаем контекст
		c.Set("user_id", resp.UserId)
		c.Set("user_role", resp.Role)
		c.Next()
	}
}

func (m *AuthMiddleware) getAccessToken(c *gin.Context) string {
	// Из заголовка
	authHeader := c.GetHeader("Authorization")
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		return authHeader[7:]
	}

	// Из cookie
	if token, err := c.Cookie("access_token"); err == nil {
		return token
	}

	// Из query
	return c.Query("access_token")
}

func (m *AuthMiddleware) getRefreshToken(c *gin.Context) string {
	// Из cookie (основной способ)
	if token, err := c.Cookie("refresh_token"); err == nil {
		return token
	}

	// Из заголовка
	if token := c.GetHeader("X-Refresh-Token"); token != "" {
		return token
	}

	return ""
}

func (m *AuthMiddleware) setTokensInResponse(c *gin.Context, accessToken, refreshToken string) {
	// Устанавливаем в заголовки
	c.Header("X-New-Access-Token", accessToken)
	c.Header("X-New-Refresh-Token", refreshToken)

	// Для веб-приложений можно также установить в cookie
	c.SetCookie("access_token", accessToken, 900, "/", "", false, true)
	c.SetCookie("refresh_token", refreshToken, 604800, "/", "", false, true)
}

func (m *AuthMiddleware) isPublicRoute(path string) bool {
	publicRoutes := map[string]bool{
		"/api/auth/register": true,
		"/api/auth/login":    true,
		"/api/auth/refresh":  true,
		"/health":            true,
	}
	return publicRoutes[path]
}

func (m *AuthMiddleware) Role(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole := c.GetString("user_role")
		if userRole == "" {
			c.AbortWithStatusJSON(403, gin.H{"error": "role not found"})
			return
		}

		allowed := false
		for _, role := range allowedRoles {
			if userRole == role {
				allowed = true
				break
			}
		}

		if !allowed {
			c.AbortWithStatusJSON(403, gin.H{"error": "insufficient permissions"})
			return
		}

		c.Next()
	}
}
