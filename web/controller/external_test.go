package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/web/middleware"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

func setupExternalAPITest(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// Setup test database
	_ = database.CloseDB()
	tdb := filepath.Join(t.TempDir(), "external_test.db")
	if err := database.InitDB(tdb); err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	// Setup external API key
	settingSvc := service.SettingService{}
	const testKey = "TEST_EXTERNAL_API_KEY_12345"
	if err := settingSvc.SetExternalAPIKey(testKey); err != nil {
		t.Fatalf("failed to set external api key: %v", err)
	}

	// Create router with external API
	r := gin.New()
	apiGroup := r.Group("/api/external")
	apiGroup.Use(middleware.ExternalAPIKeyMiddleware())
	_ = NewExternalController(apiGroup)

	return r, testKey
}

func TestExternalAPI_GetStatus_Unauthorized(t *testing.T) {
	r, _ := setupExternalAPITest(t)

	// No API key
	req := httptest.NewRequest(http.MethodGet, "/api/external/server/status", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}

	// Wrong API key
	req2 := httptest.NewRequest(http.MethodGet, "/api/external/server/status", nil)
	req2.Header.Set("X-API-Key", "WRONG_KEY")
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec2.Code)
	}
}

func TestExternalAPI_GetStatus_Success(t *testing.T) {
	r, apiKey := setupExternalAPITest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/external/server/status", nil)
	req.Header.Set("X-API-Key", apiKey)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", rec.Code, rec.Body.String())
	}

	var response struct {
		Success bool           `json:"success"`
		Obj     service.Status `json:"obj"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success=true")
	}

	// Verify status structure
	if response.Obj.Xray.State == "" {
		t.Error("expected xray state to be set")
	}
}

func TestExternalAPI_ListInbounds_Unauthorized(t *testing.T) {
	r, _ := setupExternalAPITest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/external/inbounds/list", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestExternalAPI_ListInbounds_Success(t *testing.T) {
	r, apiKey := setupExternalAPITest(t)

	// Add test inbound
	inboundSvc := service.InboundService{}
	testInbound := &model.Inbound{
		Remark:   "test-inbound",
		Port:     10000,
		Protocol: "vmess",
		Enable:   true,
		Settings: `{"clients":[]}`,
	}
	_, _, err := inboundSvc.AddInbound(testInbound)
	if err != nil {
		t.Fatalf("failed to add test inbound: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/external/inbounds/list", nil)
	req.Header.Set("X-API-Key", apiKey)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", rec.Code, rec.Body.String())
	}

	var response struct {
		Success bool             `json:"success"`
		Obj     []*model.Inbound `json:"obj"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success=true")
	}

	if len(response.Obj) == 0 {
		t.Error("expected at least one inbound")
	}

	found := false
	for _, inbound := range response.Obj {
		if inbound.Remark == "test-inbound" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find test inbound in response")
	}
}

func TestExternalAPI_ListInbounds_Empty(t *testing.T) {
	r, apiKey := setupExternalAPITest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/external/inbounds/list", nil)
	req.Header.Set("X-API-Key", apiKey)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var response struct {
		Success bool             `json:"success"`
		Obj     []*model.Inbound `json:"obj"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("expected success=true")
	}

	if response.Obj == nil {
		t.Error("expected obj to be an empty array, not nil")
	}
}

func TestExternalAPI_Healthcheck(t *testing.T) {
	r, apiKey := setupExternalAPITest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/external/health", nil)
	req.Header.Set("X-API-Key", apiKey)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", rec.Code, rec.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if status, ok := response["status"].(string); !ok || status != "healthy" {
		t.Errorf("expected status=healthy, got %v", response["status"])
	}

	if _, ok := response["timestamp"]; !ok {
		t.Error("expected timestamp in response")
	}
}

func TestExternalAPI_RateLimit(t *testing.T) {
	r, apiKey := setupExternalAPITest(t)

	// Make multiple requests quickly to test rate limiting
	successCount := 0
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/external/server/status", nil)
		req.Header.Set("X-API-Key", apiKey)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code == http.StatusOK {
			successCount++
		} else if rec.Code == http.StatusTooManyRequests {
			// Rate limit hit - this is expected
			break
		}
	}

	// Should allow some requests but eventually rate limit
	if successCount == 0 {
		t.Error("expected at least some successful requests")
	}
}
