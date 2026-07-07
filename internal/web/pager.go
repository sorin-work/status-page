package web

import (
	"net/http"
	"strconv"
)

// pager splits a list into numbered pages.
type pager struct {
	Page    int
	PerPage int
	Total   int
	Pages   int
}

// newPager reads ?page= from the request. Out of range values are clamped.
func newPager(r *http.Request, total, perPage int) pager {
	p := pager{Page: 1, PerPage: perPage, Total: total}
	p.Pages = (total + perPage - 1) / perPage
	if p.Pages < 1 {
		p.Pages = 1
	}
	if n, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && n > 1 {
		p.Page = n
	}
	if p.Page > p.Pages {
		p.Page = p.Pages
	}
	return p
}

// Offset is the index of the first item on the current page.
func (p pager) Offset() int { return (p.Page - 1) * p.PerPage }

func (p pager) HasPrev() bool { return p.Page > 1 }
func (p pager) HasNext() bool { return p.Page < p.Pages }
func (p pager) Prev() int     { return p.Page - 1 }
func (p pager) Next() int     { return p.Page + 1 }
