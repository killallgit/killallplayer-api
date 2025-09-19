package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid token")
	ErrTokenExpired       = errors.New("token expired")
	ErrUnauthorized       = errors.New("unauthorized - missing required permissions")
	ErrJWKSFetch          = errors.New("failed to fetch JWKS")
)

// Claims represents Supabase JWT claims with custom app_metadata
type Claims struct {
	// Standard Supabase claims
	Sub   string `json:"sub"`   // User ID
	Email string `json:"email"` // User email
	Phone string `json:"phone"` // User phone (optional)
	Role  string `json:"role"`  // Supabase role (authenticated, etc.)

	// Custom claims from app_metadata
	AppMetadata AppMetadata `json:"app_metadata"`

	// Standard JWT claims
	jwt.RegisteredClaims
}

// AppMetadata contains custom user metadata from Supabase
type AppMetadata struct {
	Permissions []string `json:"permissions"`
	Role        string   `json:"role"`
}

// HasPermission checks if the user has a specific permission
func (c *Claims) HasPermission(permission string) bool {
	for _, p := range c.AppMetadata.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// HasAnyPermission checks if the user has any of the specified permissions
func (c *Claims) HasAnyPermission(permissions ...string) bool {
	for _, permission := range permissions {
		if c.HasPermission(permission) {
			return true
		}
	}
	return false
}

// JWK represents a JSON Web Key
type JWK struct {
	Kty string `json:"kty"` // Key type
	Kid string `json:"kid"` // Key ID
	Use string `json:"use"` // Public key use
	Alg string `json:"alg"` // Algorithm
	Crv string `json:"crv"` // Curve (for EC keys)
	X   string `json:"x"`   // X coordinate (for EC keys)
	Y   string `json:"y"`   // Y coordinate (for EC keys)
}

// JWKS represents a JSON Web Key Set
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// Service handles authentication with Supabase JWTs using JWKS
type Service struct {
	jwksURL        string
	keys           map[string]*ecdsa.PublicKey
	keysMutex      sync.RWMutex
	lastFetch      time.Time
	cacheDuration  time.Duration
	devAuthEnabled bool
	devAuthToken   string
}

// NewService creates a new auth service for Supabase JWT validation using JWKS
func NewService(jwksURL string) (*Service, error) {
	if jwksURL == "" {
		return nil, fmt.Errorf("JWKS URL is required")
	}

	service := &Service{
		jwksURL:        jwksURL,
		keys:           make(map[string]*ecdsa.PublicKey),
		cacheDuration:  time.Hour, // Cache keys for 1 hour
		devAuthEnabled: false,
		devAuthToken:   "",
	}

	// Fetch keys on initialization
	if err := service.fetchJWKS(); err != nil {
		return nil, fmt.Errorf("failed to fetch initial JWKS: %w", err)
	}

	return service, nil
}

// SetDevAuth configures development authentication bypass
func (s *Service) SetDevAuth(enabled bool, token string) {
	s.devAuthEnabled = enabled
	s.devAuthToken = token
}

// fetchJWKS fetches and parses the JWKS from the URL
func (s *Service) fetchJWKS() error {
	resp, err := http.Get(s.jwksURL)
	if err != nil {
		return fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("failed to decode JWKS: %w", err)
	}

	s.keysMutex.Lock()
	defer s.keysMutex.Unlock()

	// Clear old keys
	s.keys = make(map[string]*ecdsa.PublicKey)

	// Parse and store new keys
	for _, jwk := range jwks.Keys {
		if jwk.Kty == "EC" && jwk.Alg == "ES256" {
			pubKey, err := s.parseECKey(jwk)
			if err != nil {
				continue // Skip invalid keys
			}
			s.keys[jwk.Kid] = pubKey
		}
	}

	s.lastFetch = time.Now()
	return nil
}

// parseECKey converts a JWK to an ECDSA public key
func (s *Service) parseECKey(jwk JWK) (*ecdsa.PublicKey, error) {
	// Decode X and Y coordinates
	xBytes, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil {
		return nil, fmt.Errorf("failed to decode X coordinate: %w", err)
	}

	yBytes, err := base64.RawURLEncoding.DecodeString(jwk.Y)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Y coordinate: %w", err)
	}

	// Create public key
	pubKey := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}

	return pubKey, nil
}

// getPublicKey retrieves a public key by kid, refreshing JWKS if necessary
func (s *Service) getPublicKey(kid string) (*ecdsa.PublicKey, error) {
	s.keysMutex.RLock()
	key, exists := s.keys[kid]
	shouldRefresh := time.Since(s.lastFetch) > s.cacheDuration
	s.keysMutex.RUnlock()

	// If key doesn't exist or cache is stale, refresh
	if !exists || shouldRefresh {
		if err := s.fetchJWKS(); err != nil {
			return nil, fmt.Errorf("failed to refresh JWKS: %w", err)
		}

		s.keysMutex.RLock()
		key, exists = s.keys[kid]
		s.keysMutex.RUnlock()
	}

	if !exists {
		return nil, fmt.Errorf("key with id %s not found", kid)
	}

	return key, nil
}

// ValidateToken validates a Supabase JWT and returns the claims
func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	// Check if it's the dev token first
	if s.devAuthEnabled && s.devAuthToken != "" &&
		subtle.ConstantTimeCompare([]byte(tokenString), []byte(s.devAuthToken)) == 1 {
		return s.GetDevClaims(), nil
	}

	// Parse token without verification to get the header
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify the signing method
		if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Get the key ID from the token header
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("no kid found in token header")
		}

		// Get the public key for this kid
		return s.getPublicKey(kid)
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		// Check if user has any API permissions
		if len(claims.AppMetadata.Permissions) == 0 {
			// If no permissions are set, deny access
			// This ensures only manually provisioned users can access the API
			return nil, ErrUnauthorized
		}

		// Check for at least one podcast permission
		if !claims.HasAnyPermission("podcasts:read", "podcasts:write", "podcasts:admin") {
			return nil, ErrUnauthorized
		}

		return claims, nil
	}

	return nil, ErrInvalidToken
}

// GetDevClaims returns fixed claims for development mode
func (s *Service) GetDevClaims() *Claims {
	return &Claims{
		Sub:   "dev-user-001",
		Email: "dev@killallplayer.local",
		Role:  "authenticated",
		AppMetadata: AppMetadata{
			Permissions: []string{"podcasts:read", "podcasts:write", "podcasts:admin"},
			Role:        "admin",
		},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(365 * 24 * time.Hour)), // 1 year
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}
}

// UserInfo represents public user information
type UserInfo struct {
	ID          string   `json:"id"`
	Email       string   `json:"email"`
	Permissions []string `json:"permissions"`
	Role        string   `json:"role"`
}

// GetUserInfo extracts user info from claims
func GetUserInfo(claims *Claims) *UserInfo {
	return &UserInfo{
		ID:          claims.Sub,
		Email:       claims.Email,
		Permissions: claims.AppMetadata.Permissions,
		Role:        claims.AppMetadata.Role,
	}
}
