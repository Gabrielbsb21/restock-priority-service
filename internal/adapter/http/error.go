package http

import (
	"log/slog"
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
	c.AbortWithStatusJSON(status, ErrorResponseEnvelope{
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
	fields := make(map[string]string, len(fieldErrs))
	for name, message := range fieldErrs {
		fields[name] = message
	}
	respondWithError(c, http.StatusBadRequest, "validation_error", "the request contains invalid fields", fields)
}

// respondNotFound reports a missing part. Routing misses use respondRouteNotFound
// instead, so "part_not_found" always means a part was actually looked up.
func respondNotFound(c *gin.Context, message string) {
	respondWithError(c, http.StatusNotFound, "part_not_found", message, nil)
}

func respondRouteNotFound(c *gin.Context) {
	respondWithError(c, http.StatusNotFound, "not_found", "the requested resource does not exist", nil)
}

// respondInternalError returns a generic failure to the client and logs the cause
// once, here at the boundary that has the request context. The cause never reaches
// the client.
func respondInternalError(c *gin.Context, err error) {
	slog.ErrorContext(c.Request.Context(), "unhandled request failure",
		"requestId", requestID(c),
		"method", c.Request.Method,
		"route", c.FullPath(),
		"error", err,
	)
	respondWithError(c, http.StatusInternalServerError, "internal_error", "an unexpected error occurred", nil)
}

func respondServiceUnavailable(c *gin.Context, message string) {
	respondWithError(c, http.StatusServiceUnavailable, "service_unavailable", message, nil)
}
