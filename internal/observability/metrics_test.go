package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHTTPMetricsUseRouteTemplatesAndCountErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	registry := NewRegistry()
	r := gin.New()
	r.Use(registry.Middleware())
	r.GET("/items/:id", func(c *gin.Context) { c.Status(http.StatusConflict) })
	r.GET("/metrics", registry.Handler(nil))
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/items/private-value", nil))
	out := httptest.NewRecorder()
	r.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := out.Body.String()
	if !strings.Contains(body, `cooked_http_errors_total{route="GET /items/:id"} 1`) || strings.Contains(body, "private-value") {
		t.Fatalf("metrics=%s", body)
	}
}
