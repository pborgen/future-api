// Package httputil contains transport-layer primitives shared across every
// gin handler in the project: the JSON error envelope and request parsing
// helpers. Lives at the project root so any aggregate package can import it
// without depending on another aggregate.
package httputil

import (
	"errors"
	"time"

	"github.com/gin-gonic/gin"
)

// ErrorResponse is the JSON shape returned for any non-2xx response.
type ErrorResponse struct {
	Error string `json:"error" example:"invalid trainer_id"`
}

// WriteError writes a JSON ErrorResponse with the given status code.
func WriteError(c *gin.Context, status int, msg string) {
	c.JSON(status, ErrorResponse{Error: msg})
}

// ParseTimeQuery extracts an RFC3339 timestamp from c's query string.
func ParseTimeQuery(c *gin.Context, key string) (time.Time, error) {
	raw := c.Query(key)
	if raw == "" {
		return time.Time{}, errors.New(key + " is required")
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, errors.New(key + " must be RFC3339")
	}
	return t, nil
}
