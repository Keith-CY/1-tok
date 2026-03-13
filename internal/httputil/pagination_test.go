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
	if p.Limit != DefaultPageLimit {
		t.Errorf("limit = %d, want %d (capped)", p.Limit, DefaultPageLimit)
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
