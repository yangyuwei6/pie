package service

import (
	"net/http"

	"pie/internal/biz"

	"github.com/gin-gonic/gin"
)

const (
	CodeOK           = 0
	CodeBadRequest   = 40000
	CodeUnauthorized = 40100
	CodeInternal     = 50000
)

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{
		Code:    CodeOK,
		Message: "ok",
		Data:    data,
	})
}

func BadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, Response{
		Code:    CodeBadRequest,
		Message: message,
	})
}

func Unauthorized(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, Response{
		Code:    CodeUnauthorized,
		Message: message,
	})
}

func Fail(c *gin.Context, err error) {
	if e, ok := err.(*biz.Error); ok {
		c.JSON(http.StatusOK, Response{
			Code:    e.Code,
			Message: e.Message,
		})
		return
	}

	c.JSON(http.StatusInternalServerError, Response{
		Code:    CodeInternal,
		Message: "internal server error",
	})
}
