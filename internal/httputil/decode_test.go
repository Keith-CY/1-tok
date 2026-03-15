package httputil

import (
	"net/http/httptest"
	"strings"
	"fmt"
	"testing"
)

func TestDecodeJSON_Success(t *testing.T) {
	body := strings.NewReader(`{"name":"test","value":42}`)
	req := httptest.NewRequest("POST", "/", body)

	var v struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	if err := DecodeJSON(req, &v); err != nil {
		t.Fatal(err)
	}
	if v.Name != "test" || v.Value != 42 {
		t.Errorf("got %+v", v)
	}
}

func TestDecodeJSON_InvalidJSON(t *testing.T) {
	body := strings.NewReader(`not json`)
	req := httptest.NewRequest("POST", "/", body)

	var v struct{}
	err := DecodeJSON(req, &v)
	if err == nil {
		t.Error("expected error")
	}
}

func TestDecodeJSON_NilBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/", nil)
	req.Body = nil

	var v struct{}
	err := DecodeJSON(req, &v)
	if err == nil {
		t.Error("expected error for nil body")
	}
}

func TestHandleDecodeError_Nil(t *testing.T) {
	w := httptest.NewRecorder()
	if HandleDecodeError(w, nil) {
		t.Error("nil error should return false")
	}
}

func TestHandleDecodeError_DecodeError(t *testing.T) {
	w := httptest.NewRecorder()
	handled := HandleDecodeError(w, &DecodeError{Message: "bad"})
	if !handled {
		t.Error("should handle DecodeError")
	}
	if w.Code != 400 {
		t.Errorf("status = %d", w.Code)
	}
}

func TestHandleDecodeError_OtherError(t *testing.T) {
	w := httptest.NewRecorder()
	handled := HandleDecodeError(w, fmt.Errorf("some error"))
	if !handled {
		t.Error("should handle generic error")
	}
	if w.Code != 400 {
		t.Errorf("status = %d", w.Code)
	}
}
