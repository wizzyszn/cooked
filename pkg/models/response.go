package models

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
)

type APIResponse struct {
	Status string              `json:"status"`
	Meta   *APIMeta            `json:"meta,omitempty"`
	Data   interface{}         `json:"data,omitempty"`
	Error  *apperrors.AppError `json:"error,omitempty"`
}

type APIMeta struct {
	RequestId string `json:"request_id,omitempty"`
	TimeStamp string `json:"timestamp"`
}

func newMeta(c *gin.Context) *APIMeta {

	meta := &APIMeta{
		TimeStamp: time.Now().Format(time.RFC3339),
	}
	if c != nil {
		if reqID, exists := c.Get("requestId"); exists {
			if value, ok := reqID.(string); ok {
				meta.RequestId = value
			}
		}
	}
	return meta
}

func SuccessResponse(c *gin.Context, data interface{}) *APIResponse {
	return &APIResponse{
		Status: "success",
		Meta:   newMeta(c),
		Data:   data,
	}
}

func ErrorResponse(c *gin.Context, code, message string, httpStatus int) *APIResponse {
	return &APIResponse{
		Status: "error",
		Meta:   newMeta(c),
		Error:  apperrors.New(code, message, httpStatus),
	}
}

func ErrorResponseWithDetails(c *gin.Context, message, code string, details interface{}) *APIResponse {
	return &APIResponse{
		Status: "error",
		Meta:   newMeta(c),
		Error: &apperrors.AppError{
			Details: details,
			Message: message,
			Code:    code,
		},
	}
}

func WriteAppError(c *gin.Context, err error) {
	var appError *apperrors.AppError
	if errors.As(err, &appError) {
		c.AbortWithStatusJSON(appError.HTTPStatus, ErrorResponse(c, appError.Code, appError.Message, appError.HTTPStatus))
		return
	}
	c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse(c, apperrors.ErrInternalServerError.Code, apperrors.ErrInternalServerError.Message, apperrors.ErrInternalServerError.HTTPStatus))

}
func WriteOk(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, SuccessResponse(c, data))
}

func WriteCreated(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, SuccessResponse(c, data))
}
