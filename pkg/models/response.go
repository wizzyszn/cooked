package models

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wizzyszn/cooked/pkg/errors"
)

type APIResponse struct {
	Status string           `json:"status"`
	Meta   *APIMeta         `json:"meta,omitempty"`
	Data   interface{}      `json:"data,omitempty"`
	Error  *errors.AppError `json:"error,omitempty"`
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
			meta.RequestId = reqID.(string)
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
		Error:  errors.New(code, message, httpStatus),
	}
}

func ErrorResponseWithDetails(c *gin.Context, message, code string, details interface{}) *APIResponse {
	return &APIResponse{
		Status: "error",
		Meta:   newMeta(c),
		Error: &errors.AppError{
			Details: details,
			Message: message,
			Code:    code,
		},
	}
}
