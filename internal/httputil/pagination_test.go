package httputil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParsePagination_Defaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/items", nil)
	p := ParsePagination(r)
	if p.Limit != DefaultPageLimit {
		t.Errorf("limit = %d, want %d", p.Limit, DefaultPageLimit)
	}
	if p.Offset != 0 {
		t.Errorf("offset = %d, want 0", p.Offset)
	}
}

func TestParsePagination_CustomValues(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/items?limit=10&offset=20", nil)
	p := ParsePagination(r)
	if p.Limit != 10 {
		t.Errorf("limit = %d, want 10", p.Limit)
	}
	if p.Offset != 20 {
		t.Errorf("offset = %d, want 20", p.Offset)
	}
}

func TestParsePagination_CapsAtMax(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/items?limit=999", nil)
	p := ParsePagination(r)
	if p.Limit != MaxPageLimit {
		t.Errorf("limit = %d, want %d (capped)", p.Limit, MaxPageLimit)
	}
}

func TestParsePagination_NegativeOffset(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/items?offset=-5", nil)
	p := ParsePagination(r)
	if p.Offset != 0 {
		t.Errorf("offset = %d, want 0", p.Offset)
	}
}

func TestApply_BasicSlice(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	result := Apply(items, Pagination{Limit: 3, Offset: 2})
	if len(result) != 3 || result[0] != 3 {
		t.Errorf("got %v, want [3 4 5]", result)
	}
}

func TestApply_OffsetBeyondLength(t *testing.T) {
	items := []int{1, 2, 3}
	result := Apply(items, Pagination{Limit: 10, Offset: 100})
	if len(result) != 0 {
		t.Errorf("got %v, want empty", result)
	}
}

func TestApply_EmptySlice(t *testing.T) {
	result := Apply([]string{}, Pagination{Limit: 10, Offset: 0})
	if len(result) != 0 {
		t.Errorf("expected 0, got %d", len(result))
	}
}

func TestIntParam_InvalidValue(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/items?limit=abc", nil)
	p := ParsePagination(r)
	if p.Limit != DefaultPageLimit {
		t.Errorf("limit = %d, want %d (default)", p.Limit, DefaultPageLimit)
	}
}

func TestIntParam_NegativeValue(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/items?limit=-5", nil)
	p := ParsePagination(r)
	if p.Limit != DefaultPageLimit {
		t.Errorf("limit = %d, want %d (default for negative)", p.Limit, DefaultPageLimit)
	}
}

func TestIntParam_NegativeOffset(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/items?offset=-1", nil)
	p := ParsePagination(r)
	if p.Offset != 0 {
		t.Errorf("offset = %d, want 0", p.Offset)
	}
}

func TestNewPaginatedResponse(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e"}
	page := Pagination{Limit: 2, Offset: 1}
	resp := NewPaginatedResponse(items, page)

	if len(resp.Data) != 2 {
		t.Errorf("data = %d", len(resp.Data))
	}
	if resp.Pagination.Total != 5 {
		t.Errorf("total = %d", resp.Pagination.Total)
	}
	if resp.Pagination.Limit != 2 {
		t.Errorf("limit = %d", resp.Pagination.Limit)
	}
	if resp.Data[0] != "b" {
		t.Errorf("first = %s", resp.Data[0])
	}
}

func TestNewPaginatedResponse_Empty(t *testing.T) {
	resp := NewPaginatedResponse([]string{}, Pagination{Limit: 10})
	if len(resp.Data) != 0 {
		t.Errorf("data = %d", len(resp.Data))
	}
	if resp.Pagination.Total != 0 {
		t.Errorf("total = %d", resp.Pagination.Total)
	}
}
