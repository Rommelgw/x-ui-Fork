package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

func TestExternalAPIAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Prepare settings key
	settingSvc := service.SettingService{}
	const testKey = "TEST_EXTERNAL_KEY"
	if err := settingSvc.SetExternalAPIKey(testKey); err != nil {
		t.Fatalf("failed to set external api key: %v", err)
	}

	r := gin.New()
	r.Use(ExternalAPIKeyMiddleware())
	r.GET("/api/external/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	// No header -> 401
	req1 := httptest.NewRequest(http.MethodGet, "/api/external/ping", nil)
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without key, got %d", rec1.Code)
	}

	// Wrong header -> 401
	req2 := httptest.NewRequest(http.MethodGet, "/api/external/ping", nil)
	req2.Header.Set("X-API-Key", "WRONG")
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong key, got %d", rec2.Code)
	}

	// Correct header -> 200
	req3 := httptest.NewRequest(http.MethodGet, "/api/external/ping", nil)
	req3.Header.Set("X-API-Key", testKey)
	rec3 := httptest.NewRecorder()
	r.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("expected 200 with correct key, got %d", rec3.Code)
	}
}
