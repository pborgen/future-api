package appointment

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
)

// newStrictJSONDecoder returns a json.Decoder bound to the request body that
// rejects unknown fields. gin's ShouldBindJSON is permissive by default; we
// want strict parsing so typos in the payload surface as 400s instead of
// being silently dropped.
func newStrictJSONDecoder(c *gin.Context) *json.Decoder {
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	return dec
}
