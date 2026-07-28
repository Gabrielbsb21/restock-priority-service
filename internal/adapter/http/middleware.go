package http

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

const MaxBodyBytes = 1048576 // 1MB

func MaxBodySizeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxBodyBytes)
		}
		c.Next()
	}
}

// BindJSONStrict reads the body, enforces DisallowUnknownFields, and disallows multiple JSON objects.
func BindJSONStrict(c *gin.Context, v interface{}) error {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		if err.Error() == "http: request body too large" {
			respondWithError(c, http.StatusRequestEntityTooLarge, "request_too_large", "request body exceeds limit", nil)
			return err
		}
		respondInvalidRequest(c, "unable to read request body")
		return err
	}

	decoder := json.NewDecoder(bytes.NewReader(bodyBytes))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(v); err != nil {
		respondInvalidRequest(c, "malformed or invalid JSON body: "+err.Error())
		return err
	}

	if decoder.More() {
		respondInvalidRequest(c, "request body must contain a single JSON object")
		return io.ErrUnexpectedEOF
	}

	return nil
}
