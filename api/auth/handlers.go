package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/internal/services/auth"
)

// Handler manages auth endpoints
type Handler struct {
	authService    *auth.Service
	devAuthToken   string
	devAuthEnabled bool
}

// NewHandler creates a new auth handler
func NewHandler(authService *auth.Service) *Handler {
	return &Handler{
		authService: authService,
	}
}

// SetDevAuth configures dev auth settings for the handler
func (h *Handler) SetDevAuth(enabled bool, token string) {
	h.devAuthEnabled = enabled
	h.devAuthToken = token
	h.authService.SetDevAuth(enabled, token)
}

// Me returns current user info from JWT
// @Summary Get current user
// @Description Get current user information from Supabase JWT token
// @Tags auth
// @Security BearerAuth
// @Produce json
// @Success 200 {object} auth.UserInfo
// @Failure 401 {object} map[string]string
// @Router /api/v1/me [get]
func (h *Handler) Me(c *gin.Context) {
	// Get claims from context (set by auth middleware)
	claims, exists := c.Get("claims")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	authClaims := claims.(*auth.Claims)
	userInfo := auth.GetUserInfo(authClaims)
	c.JSON(http.StatusOK, userInfo)
}

// AuthMiddleware validates Supabase JWT tokens
func (h *Handler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth entirely in development mode if configured
		if h.devAuthEnabled && h.devAuthToken == "SKIP_AUTH" {
			// Set mock dev user claims
			c.Set("claims", &auth.Claims{
				Sub:   "dev-user",
				Email: "dev@localhost",
				AppMetadata: auth.AppMetadata{
					Permissions: []string{"podcasts:read", "podcasts:write", "podcasts:admin"},
					Role:        "admin",
				},
			})
			c.Set("user_id", "dev-user")
			c.Set("email", "dev@localhost")
			c.Set("permissions", []string{"podcasts:read", "podcasts:write", "podcasts:admin"})
			c.Set("role", "admin")
			c.Next()
			return
		}

		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// Check Bearer prefix
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		// Validate token
		claims, err := h.authService.ValidateToken(parts[1])
		if err != nil {
			if err == auth.ErrUnauthorized {
				c.JSON(http.StatusForbidden, gin.H{"error": "Access denied - insufficient permissions"})
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			}
			c.Abort()
			return
		}

		// Store claims in context
		c.Set("claims", claims)
		c.Set("user_id", claims.Sub)
		c.Set("email", claims.Email)
		c.Set("permissions", claims.AppMetadata.Permissions)
		c.Set("role", claims.AppMetadata.Role)

		c.Next()
	}
}

// OptionalAuthMiddleware validates JWT if present but doesn't require it
func (h *Handler) OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}

		claims, err := h.authService.ValidateToken(parts[1])
		if err == nil {
			c.Set("claims", claims)
			c.Set("user_id", claims.Sub)
			c.Set("email", claims.Email)
			c.Set("permissions", claims.AppMetadata.Permissions)
			c.Set("role", claims.AppMetadata.Role)
		}

		c.Next()
	}
}

// RequirePermission creates middleware that requires specific permissions
func (h *Handler) RequirePermission(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, exists := c.Get("claims")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		authClaims := claims.(*auth.Claims)
		if !authClaims.HasAnyPermission(permissions...) {
			c.JSON(http.StatusForbidden, gin.H{
				"error":                "Insufficient permissions",
				"required_permissions": permissions,
				"user_permissions":     authClaims.AppMetadata.Permissions,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
