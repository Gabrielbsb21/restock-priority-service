package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const MaxBodyBytes = 1048576 // 1MB

const (
	requestIDHeader     = "X-Request-Id"
	requestIDContextKey = "requestID"
)

func MaxBodySizeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxBodyBytes)
		}
		c.Next()
	}
}

// RequestIDMiddleware assigns every request a correlation ID and echoes it back.
//
// An inbound ID is reused only when it is a valid UUID, so an arbitrary client
// string cannot end up in the logs as a correlation key.
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(requestIDHeader)
		if _, err := uuid.Parse(id); err != nil {
			id = uuid.NewString()
		}

		c.Set(requestIDContextKey, id)
		c.Writer.Header().Set(requestIDHeader, id)
		c.Next()
	}
}

// RequestLogMiddleware emits one structured event per request.
func RequestLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		slog.InfoContext(c.Request.Context(), "handled request",
			"requestId", requestID(c),
			"method", c.Request.Method,
			"route", c.FullPath(),
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"durationMs", time.Since(start).Milliseconds(),
		)
	}
}

func requestID(c *gin.Context) string {
	id, ok := c.Get(requestIDContextKey)
	if !ok {
		return ""
	}

	value, ok := id.(string)
	if !ok {
		return ""
	}

	return value
}

// BindJSONStrict reads the body, enforces DisallowUnknownFields, and disallows
// multiple JSON objects. On failure it has already written the response.
func BindJSONStrict(c *gin.Context, v any) error {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			respondWithError(c, http.StatusRequestEntityTooLarge, "request_too_large", "request body exceeds limit", nil)
			return err
		}
		respondInvalidRequest(c, "unable to read request body")
		return err
	}

	decoder := json.NewDecoder(bytes.NewReader(bodyBytes))
	decoder.DisallowUnknownFields()

	// The decoder's own message names Go types and struct fields, so it is logged
	// rather than returned.
	if err := decoder.Decode(v); err != nil {
		slog.WarnContext(c.Request.Context(), "rejected request body",
			"requestId", requestID(c),
			"error", err,
		)
		respondInvalidRequest(c, "request body is malformed, or contains an unknown field")
		return err
	}

	if decoder.More() {
		respondInvalidRequest(c, "request body must contain a single JSON object")
		return io.ErrUnexpectedEOF
	}

	return nil
}
