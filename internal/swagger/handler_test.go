package swagger

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSwaggerUIAndEmbeddedContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	Register(router)

	ui := httptest.NewRecorder()
	router.ServeHTTP(ui, httptest.NewRequest(http.MethodGet, "/docs/", nil))
	if ui.Code != http.StatusOK || !strings.Contains(ui.Body.String(), "SwaggerUIBundle") || !strings.Contains(ui.Body.String(), "/docs/openapi.yaml") {
		t.Fatalf("swagger UI status=%d body=%s", ui.Code, ui.Body.String())
	}

	spec := httptest.NewRecorder()
	router.ServeHTTP(spec, httptest.NewRequest(http.MethodGet, "/docs/openapi.yaml", nil))
	if spec.Code != http.StatusOK || !strings.Contains(spec.Body.String(), "openapi: 3.1.0") || !strings.Contains(spec.Header().Get("Content-Type"), "application/yaml") {
		t.Fatalf("OpenAPI status=%d content-type=%s", spec.Code, spec.Header().Get("Content-Type"))
	}
}
