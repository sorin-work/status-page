package web

import (
	"net/http"
	"strconv"
	"time"

	"github.com/sorin-work/status-page/internal/monitor"
)

type apiCheck struct {
	CheckedAt  time.Time `json:"checked_at"`
	Up         bool      `json:"up"`
	LatencyMS  int64     `json:"latency_ms"`
	StatusCode int       `json:"status_code,omitempty"`
	Error      string    `json:"error,omitempty"`
}

func toAPICheck(r monitor.Result) apiCheck {
	return apiCheck{CheckedAt: r.CheckedAt, Up: r.Up, LatencyMS: r.Latency.Milliseconds(), StatusCode: r.StatusCode, Error: r.Error}
}

// queryInt reads a non-negative integer query parameter.
func queryInt(r *http.Request, name string, def, maxVal int) (int, bool) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return def, true
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 || n > maxVal {
		return 0, false
	}
	return n, true
}

func (s *Server) apiListChecks(w http.ResponseWriter, r *http.Request) {
	m, ok := s.apiLoadMonitor(w, r)
	if !ok {
		return
	}
	limit, ok := queryInt(r, "limit", 100, 1000)
	if !ok || limit == 0 {
		writeError(w, http.StatusBadRequest, "limit must be between 1 and 1000")
		return
	}
	offset, ok := queryInt(r, "offset", 0, 1<<30)
	if !ok {
		writeError(w, http.StatusBadRequest, "offset must be a non-negative number")
		return
	}
	checks, err := s.store.ListChecks(r.Context(), m.ID, limit, offset)
	if err != nil {
		s.apiServerError(w, r, err)
		return
	}
	total, err := s.store.CountChecks(r.Context(), m.ID)
	if err != nil {
		s.apiServerError(w, r, err)
		return
	}
	out := make([]apiCheck, 0, len(checks))
	for _, c := range checks {
		out = append(out, toAPICheck(c))
	}
	writeJSON(w, http.StatusOK, map[string]any{"checks": out, "total": total, "limit": limit, "offset": offset})
}
