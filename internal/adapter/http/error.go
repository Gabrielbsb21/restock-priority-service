package http

import (
	"net/http"

	"github.com/Gabrielbsb21/restock-priority-service/internal/domain"
	"github.com/gin-gonic/gin"
)

type ErrorResponseBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

type ErrorResponseEnvelope struct {
	Error ErrorResponseBody `json:"error"`
}

func respondWithError(c *gin.Context, status int, code, message string, fields map[string]string) {
	c.JSON(status, ErrorResponseEnvelope{
		Error: ErrorResponseBody{
			Code:    code,
			Message: message,
			Fields:  fields,
		},
	})
}

func respondInvalidRequest(c *gin.Context, message string) {
	respondWithError(c, http.StatusBadRequest, "invalid_request", message, nil)
}

func respondValidationError(c *gin.Context, fieldErrs domain.FieldErrors) {
	fields := make(map[string]string)
	for k, v := range fieldErrs {
		fields[k] = v
	}
	respondWithError(c, http.StatusBadRequest, "validation_error", "the request contains invalid fields", fields)
}

func respondNotFound(c *gin.Context, message string) {
	respondWithError(c, http.StatusNotFound, "part_not_found", message, nil)
}

func respondInternalError(c *gin.Context) {
	respondWithError(c, http.StatusInternalServerError, "internal_error", "an unexpected error occurred", nil)
}

func respondServiceUnavailable(c *gin.Context, message string) {
	respondWithError(c, http.StatusServiceUnavailable, "service_unavailable", message, nil)
}
