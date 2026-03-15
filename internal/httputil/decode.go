package httputil

import (
	"encoding/json"
	"net/http"
)

// DecodeJSON reads and decodes JSON from the request body into v.
// Returns an error response on failure (400 Bad Request).
func DecodeJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return &DecodeError{Message: "empty request body"}
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return &DecodeError{Message: "invalid json: " + err.Error()}
	}
	return nil
}

// DecodeError represents a JSON decode error.
type DecodeError struct {
	Message string
}

func (e *DecodeError) Error() string { return e.Message }

// HandleDecodeError writes a 400 response for decode errors.
func HandleDecodeError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if de, ok := err.(*DecodeError); ok {
		WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, de.Message)
		return true
	}
	WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
	return true
}
