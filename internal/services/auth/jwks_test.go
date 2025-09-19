package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test JWKS server for testing
func createTestJWKSServer(t *testing.T) (*httptest.Server, *ecdsa.PrivateKey, string) {
	// Generate test key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	// Create JWK from public key
	pubKey := privateKey.PublicKey
	x := pubKey.X.Bytes()
	y := pubKey.Y.Bytes()

	// Pad to 32 bytes for P-256
	if len(x) < 32 {
		padding := make([]byte, 32-len(x))
		x = append(padding, x...)
	}
	if len(y) < 32 {
		padding := make([]byte, 32-len(y))
		y = append(padding, y...)
	}

	jwk := JWK{
		Kty: "EC",
		Kid: "test-key-1",
		Use: "sig",
		Alg: "ES256",
		Crv: "P-256",
		X:   base64.RawURLEncoding.EncodeToString(x),
		Y:   base64.RawURLEncoding.EncodeToString(y),
	}

	jwks := JWKS{
		Keys: []JWK{jwk},
	}

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))

	return server, privateKey, jwk.Kid
}

func createTestToken(t *testing.T, privateKey *ecdsa.PrivateKey, kid string, claims *Claims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = kid

	tokenString, err := token.SignedString(privateKey)
	require.NoError(t, err)
	return tokenString
}

func TestNewService(t *testing.T) {
	server, _, _ := createTestJWKSServer(t)
	defer server.Close()

	t.Run("valid JWKS URL", func(t *testing.T) {
		service, err := NewService(server.URL)
		assert.NoError(t, err)
		assert.NotNil(t, service)
		assert.Equal(t, server.URL, service.jwksURL)
		assert.False(t, service.devAuthEnabled)
	})

	t.Run("empty JWKS URL", func(t *testing.T) {
		service, err := NewService("")
		assert.Error(t, err)
		assert.Nil(t, service)
		assert.Contains(t, err.Error(), "JWKS URL is required")
	})

	t.Run("invalid JWKS URL", func(t *testing.T) {
		service, err := NewService("http://invalid-url-that-does-not-exist.local:99999")
		assert.Error(t, err)
		assert.Nil(t, service)
		assert.Contains(t, err.Error(), "failed to fetch initial JWKS")
	})
}

func TestService_SetDevAuth(t *testing.T) {
	server, _, _ := createTestJWKSServer(t)
	defer server.Close()

	service, err := NewService(server.URL)
	require.NoError(t, err)

	// Test enabling dev auth
	service.SetDevAuth(true, "test-token")
	assert.True(t, service.devAuthEnabled)
	assert.Equal(t, "test-token", service.devAuthToken)

	// Test disabling dev auth
	service.SetDevAuth(false, "")
	assert.False(t, service.devAuthEnabled)
	assert.Equal(t, "", service.devAuthToken)
}

func TestService_ValidateToken_DevMode(t *testing.T) {
	server, _, _ := createTestJWKSServer(t)
	defer server.Close()

	service, err := NewService(server.URL)
	require.NoError(t, err)

	devToken := "dev-test-token"
	service.SetDevAuth(true, devToken)

	t.Run("valid dev token", func(t *testing.T) {
		claims, err := service.ValidateToken(devToken)
		assert.NoError(t, err)
		assert.NotNil(t, claims)
		assert.Equal(t, "dev-user-001", claims.Sub)
		assert.Equal(t, "dev@killallplayer.local", claims.Email)
		assert.Contains(t, claims.AppMetadata.Permissions, "podcasts:admin")
	})

	t.Run("invalid dev token", func(t *testing.T) {
		claims, err := service.ValidateToken("wrong-token")
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Equal(t, ErrInvalidToken, err)
	})
}

func TestService_ValidateToken_JWT(t *testing.T) {
	server, privateKey, kid := createTestJWKSServer(t)
	defer server.Close()

	service, err := NewService(server.URL)
	require.NoError(t, err)

	t.Run("valid token with permissions", func(t *testing.T) {
		claims := &Claims{
			Sub:   "user-123",
			Email: "test@example.com",
			Role:  "authenticated",
			AppMetadata: AppMetadata{
				Permissions: []string{"podcasts:read", "podcasts:write"},
				Role:        "user",
			},
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
		}

		token := createTestToken(t, privateKey, kid, claims)
		validatedClaims, err := service.ValidateToken(token)

		assert.NoError(t, err)
		assert.NotNil(t, validatedClaims)
		assert.Equal(t, "user-123", validatedClaims.Sub)
		assert.Equal(t, "test@example.com", validatedClaims.Email)
		assert.Contains(t, validatedClaims.AppMetadata.Permissions, "podcasts:read")
	})

	t.Run("token without permissions", func(t *testing.T) {
		claims := &Claims{
			Sub:   "user-456",
			Email: "noperm@example.com",
			Role:  "authenticated",
			AppMetadata: AppMetadata{
				Permissions: []string{}, // No permissions
				Role:        "user",
			},
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
		}

		token := createTestToken(t, privateKey, kid, claims)
		validatedClaims, err := service.ValidateToken(token)

		assert.Error(t, err)
		assert.Nil(t, validatedClaims)
		assert.Equal(t, ErrUnauthorized, err)
	})

	t.Run("token without podcast permissions", func(t *testing.T) {
		claims := &Claims{
			Sub:   "user-789",
			Email: "other@example.com",
			Role:  "authenticated",
			AppMetadata: AppMetadata{
				Permissions: []string{"other:read"}, // Wrong permissions
				Role:        "user",
			},
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
		}

		token := createTestToken(t, privateKey, kid, claims)
		validatedClaims, err := service.ValidateToken(token)

		assert.Error(t, err)
		assert.Nil(t, validatedClaims)
		assert.Equal(t, ErrUnauthorized, err)
	})

	t.Run("expired token", func(t *testing.T) {
		claims := &Claims{
			Sub:   "user-expired",
			Email: "expired@example.com",
			Role:  "authenticated",
			AppMetadata: AppMetadata{
				Permissions: []string{"podcasts:read"},
				Role:        "user",
			},
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)), // Expired
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			},
		}

		token := createTestToken(t, privateKey, kid, claims)
		validatedClaims, err := service.ValidateToken(token)

		assert.Error(t, err)
		assert.Nil(t, validatedClaims)
		assert.Equal(t, ErrTokenExpired, err)
	})

	t.Run("token with wrong key ID", func(t *testing.T) {
		claims := &Claims{
			Sub:   "user-wrong-kid",
			Email: "wrongkid@example.com",
			Role:  "authenticated",
			AppMetadata: AppMetadata{
				Permissions: []string{"podcasts:read"},
				Role:        "user",
			},
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
		}

		token := createTestToken(t, privateKey, "wrong-kid", claims)
		validatedClaims, err := service.ValidateToken(token)

		assert.Error(t, err)
		assert.Nil(t, validatedClaims)
		assert.Equal(t, ErrInvalidToken, err)
	})

	t.Run("malformed token", func(t *testing.T) {
		validatedClaims, err := service.ValidateToken("invalid.token.here")

		assert.Error(t, err)
		assert.Nil(t, validatedClaims)
		assert.Equal(t, ErrInvalidToken, err)
	})
}

func TestClaims_HasPermission(t *testing.T) {
	claims := &Claims{
		AppMetadata: AppMetadata{
			Permissions: []string{"podcasts:read", "podcasts:write"},
		},
	}

	assert.True(t, claims.HasPermission("podcasts:read"))
	assert.True(t, claims.HasPermission("podcasts:write"))
	assert.False(t, claims.HasPermission("podcasts:admin"))
	assert.False(t, claims.HasPermission("other:read"))
}

func TestClaims_HasAnyPermission(t *testing.T) {
	claims := &Claims{
		AppMetadata: AppMetadata{
			Permissions: []string{"podcasts:read", "podcasts:write"},
		},
	}

	assert.True(t, claims.HasAnyPermission("podcasts:read"))
	assert.True(t, claims.HasAnyPermission("podcasts:admin", "podcasts:read"))
	assert.True(t, claims.HasAnyPermission("other:read", "podcasts:write"))
	assert.False(t, claims.HasAnyPermission("podcasts:admin"))
	assert.False(t, claims.HasAnyPermission("other:read", "other:write"))
}

func TestGetUserInfo(t *testing.T) {
	claims := &Claims{
		Sub:   "user-123",
		Email: "test@example.com",
		AppMetadata: AppMetadata{
			Permissions: []string{"podcasts:read", "podcasts:write"},
			Role:        "user",
		},
	}

	userInfo := GetUserInfo(claims)

	assert.Equal(t, "user-123", userInfo.ID)
	assert.Equal(t, "test@example.com", userInfo.Email)
	assert.Equal(t, []string{"podcasts:read", "podcasts:write"}, userInfo.Permissions)
	assert.Equal(t, "user", userInfo.Role)
}

func TestService_GetDevClaims(t *testing.T) {
	server, _, _ := createTestJWKSServer(t)
	defer server.Close()

	service, err := NewService(server.URL)
	require.NoError(t, err)

	claims := service.GetDevClaims()

	assert.Equal(t, "dev-user-001", claims.Sub)
	assert.Equal(t, "dev@killallplayer.local", claims.Email)
	assert.Equal(t, "authenticated", claims.Role)
	assert.Contains(t, claims.AppMetadata.Permissions, "podcasts:read")
	assert.Contains(t, claims.AppMetadata.Permissions, "podcasts:write")
	assert.Contains(t, claims.AppMetadata.Permissions, "podcasts:admin")
	assert.Equal(t, "admin", claims.AppMetadata.Role)
}

// Test edge cases and error conditions
func TestService_ErrorConditions(t *testing.T) {
	t.Run("JWKS server returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		service, err := NewService(server.URL)
		assert.Error(t, err)
		assert.Nil(t, service)
	})

	t.Run("JWKS server returns invalid JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		service, err := NewService(server.URL)
		assert.Error(t, err)
		assert.Nil(t, service)
	})
}

func TestService_KeyCaching(t *testing.T) {
	server, privateKey, kid := createTestJWKSServer(t)
	defer server.Close()

	service, err := NewService(server.URL)
	require.NoError(t, err)

	// Create a valid token
	claims := &Claims{
		Sub:   "user-cache-test",
		Email: "cache@example.com",
		Role:  "authenticated",
		AppMetadata: AppMetadata{
			Permissions: []string{"podcasts:read"},
			Role:        "user",
		},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := createTestToken(t, privateKey, kid, claims)

	// First validation should work (loads from JWKS)
	validatedClaims, err := service.ValidateToken(token)
	assert.NoError(t, err)
	assert.NotNil(t, validatedClaims)

	// Second validation should work (uses cached key)
	validatedClaims, err = service.ValidateToken(token)
	assert.NoError(t, err)
	assert.NotNil(t, validatedClaims)
}
