package middlewares

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCheckStaticToken_ValidToken(t *testing.T) {
	os.Setenv("API_STATIC_TOKEN", "test_token_123")
	defer os.Unsetenv("API_STATIC_TOKEN")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/test", CheckStaticToken(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer test_token_123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCheckStaticToken_InvalidToken(t *testing.T) {
	os.Setenv("API_STATIC_TOKEN", "test_token_123")
	defer os.Unsetenv("API_STATIC_TOKEN")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/test", CheckStaticToken(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer wrong_token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCheckStaticToken_MissingHeader(t *testing.T) {
	os.Setenv("API_STATIC_TOKEN", "test_token_123")
	defer os.Unsetenv("API_STATIC_TOKEN")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/test", CheckStaticToken(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCheckStaticToken_EmptyEnv(t *testing.T) {
	os.Unsetenv("API_STATIC_TOKEN")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/test", CheckStaticToken(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer any_token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
