package cook

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/api/middlewares"
	"github.com/wizzyszn/cooked/internal/domain"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionAndTimerHTTPStateSurvivesServiceReplacement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := cookDB(t)
	userID := seedCookUser(t, database, "cook-http", "UTC")
	recipe := seedCookRecipe(t, database, userID, newDishID())
	clock := &mutableClock{now: time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)}
	router := cookRouter(NewServiceWithClock(NewRepository(database), clock, DefaultRewards()), userID)
	start := performJSON(router, http.MethodPost, "/cook-sessions", map[string]any{"recipe_version_id": recipe.version})
	if start.Code != 200 {
		t.Fatalf("start status=%d body=%s", start.Code, start.Body.String())
	}
	var envelope struct {
		Data SessionView `json:"data"`
	}
	if e := json.Unmarshal(start.Body.Bytes(), &envelope); e != nil {
		t.Fatal(e)
	}
	visit := performJSON(router, http.MethodPost, "/cook-sessions/"+envelope.Data.ID.String()+"/steps/"+recipe.steps[0].String()+"/visit", nil)
	if visit.Code != 204 {
		t.Fatalf("visit=%d %s", visit.Code, visit.Body.String())
	}
	timer := performJSON(router, http.MethodPut, "/cook-sessions/"+envelope.Data.ID.String()+"/steps/"+recipe.steps[0].String()+"/timer", map[string]any{"action": "start", "duration_seconds": 90})
	if timer.Code != 200 {
		t.Fatalf("timer=%d %s", timer.Code, timer.Body.String())
	}
	clock.now = clock.now.Add(20 * time.Second)
	router = cookRouter(NewServiceWithClock(NewRepository(database), clock, DefaultRewards()), userID)
	get := performJSON(router, http.MethodGet, "/cook-sessions/"+envelope.Data.ID.String(), nil)
	if get.Code != 200 {
		t.Fatalf("get=%d %s", get.Code, get.Body.String())
	}
	var restored struct {
		Data SessionView `json:"data"`
	}
	json.Unmarshal(get.Body.Bytes(), &restored)
	if len(restored.Data.VisitedStepIDs) != 1 || len(restored.Data.Timers) != 1 || restored.Data.Timers[0].RemainingSeconds != 70 {
		t.Fatalf("restored=%#v", restored.Data)
	}
}
func cookRouter(service *Service, userID uuid.UUID) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middlewares.ContextUser, &domain.User{BaseModel: domain.BaseModel{ID: userID}})
		c.Next()
	})
	h := NewHandler(service)
	r.POST("/cook-sessions", h.Start)
	r.GET("/cook-sessions/:id", h.Get)
	r.POST("/cook-sessions/:id/steps/:stepId/visit", h.Visit)
	r.PUT("/cook-sessions/:id/steps/:stepId/timer", h.Timer)
	return r
}
func performJSON(r http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	var data []byte
	if body != nil {
		data, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
func newDishID() uuid.UUID { return uuid.New() }
