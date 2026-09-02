package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestIDPreservesOrCreatesHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		name     string
		provided string
	}{
		{name: "creates id"},
		{name: "preserves id", provided: "caller-request-id"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(RequestId())
			router.GET("/test", func(ctx *gin.Context) { ctx.Status(http.StatusNoContent) })
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.provided != "" {
				request.Header.Set(RequestHeaderKey, tt.provided)
			}
			router.ServeHTTP(recorder, request)
			got := recorder.Header().Get(RequestHeaderKey)
			if got == "" {
				t.Fatal("response is missing request id")
			}
			if tt.provided != "" && got != tt.provided {
				t.Fatalf("request id = %q, want %q", got, tt.provided)
			}
		})
	}
}
