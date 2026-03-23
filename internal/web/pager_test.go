package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPager(t *testing.T) {
	cases := []struct {
		query        string
		total        int
		page, pages  int
		offset       int
		prev, hasNxt bool
	}{
		{"", 0, 1, 1, 0, false, false},
		{"", 100, 1, 2, 0, false, true},
		{"?page=2", 100, 2, 2, 50, true, false},
		{"?page=9", 100, 2, 2, 50, true, false},
		{"?page=-3", 100, 1, 2, 0, false, true},
		{"?page=abc", 150, 1, 3, 0, false, true},
		{"?page=2", 150, 2, 3, 50, true, true},
	}
	for _, tc := range cases {
		p := newPager(httptest.NewRequest(http.MethodGet, "/x"+tc.query, nil), tc.total, 50)
		if p.Page != tc.page || p.Pages != tc.pages || p.Offset() != tc.offset || p.HasPrev() != tc.prev || p.HasNext() != tc.hasNxt {
			t.Errorf("%q total %d: got %+v offset %d", tc.query, tc.total, p, p.Offset())
		}
	}
}
