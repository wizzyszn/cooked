package discovery

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/api/middlewares"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
	"github.com/wizzyszn/cooked/pkg/models"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func filters(c *gin.Context) (Filters, error) {
	f := Filters{Query: c.Query("q"), Dietary: c.Query("dietary"), Difficulty: c.Query("difficulty"), Category: c.Query("category"), Region: c.Query("region"), Cursor: c.Query("cursor"), DishCursor: c.Query("dish_cursor"), RecipeCursor: c.Query("recipe_cursor")}
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return f, apperrors.ErrValidation
		}
		f.Limit = n
	}
	if raw := c.Query("max_total_time_seconds"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return f, apperrors.ErrValidation
		}
		f.MaxSeconds = &n
	}
	return f, nil
}
func recipeID(c *gin.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return uuid.Nil, apperrors.ErrValidation
	}
	return id, nil
}
func currentUserID(c *gin.Context) (uuid.UUID, error) {
	u, ok := middlewares.CurrentUserFromContext(c)
	if !ok {
		return uuid.Nil, apperrors.ErrUnauthorized
	}
	return u.ID, nil
}
func (h *Handler) Search(c *gin.Context) {
	f, e := filters(c)
	if e == nil {
		var out SearchResult
		out, e = h.service.Search(c, f)
		if e == nil {
			models.WriteOk(c, out)
			return
		}
	}
	models.WriteAppError(c, e)
}
func (h *Handler) Browse(c *gin.Context) {
	f, e := filters(c)
	if e == nil {
		var out DishPage
		out, e = h.service.Browse(c, f)
		if e == nil {
			models.WriteOk(c, out)
			return
		}
	}
	models.WriteAppError(c, e)
}
func (h *Handler) Recent(c *gin.Context) {
	f, e := filters(c)
	if e == nil {
		var out DishPage
		out, e = h.service.Recent(c, f)
		if e == nil {
			models.WriteOk(c, out)
			return
		}
	}
	models.WriteAppError(c, e)
}
func (h *Handler) Recommendations(c *gin.Context) {
	f, e := filters(c)
	u, ue := currentUserID(c)
	if e == nil {
		e = ue
	}
	if e == nil {
		var out RecipePage
		out, e = h.service.Recommendations(c, u, f)
		if e == nil {
			models.WriteOk(c, out)
			return
		}
	}
	models.WriteAppError(c, e)
}
func (h *Handler) Save(c *gin.Context) {
	id, e := recipeID(c)
	u, ue := currentUserID(c)
	if e == nil {
		e = ue
	}
	if e == nil {
		e = h.service.Save(c, u, id)
	}
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	c.Status(204)
}
func (h *Handler) Unsave(c *gin.Context) {
	id, e := recipeID(c)
	u, ue := currentUserID(c)
	if e == nil {
		e = ue
	}
	if e == nil {
		e = h.service.Unsave(c, u, id)
	}
	if e != nil {
		models.WriteAppError(c, e)
		return
	}
	c.Status(204)
}
func (h *Handler) Favorites(c *gin.Context) {
	f, e := filters(c)
	u, ue := currentUserID(c)
	if e == nil {
		e = ue
	}
	if e == nil {
		var out RecipePage
		out, e = h.service.Favorites(c, u, f)
		if e == nil {
			models.WriteOk(c, out)
			return
		}
	}
	models.WriteAppError(c, e)
}
