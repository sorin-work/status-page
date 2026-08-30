package web

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var unsafeFilename = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// handleExportChecks downloads the raw check history of a monitor as CSV.
func (s *Server) handleExportChecks(w http.ResponseWriter, r *http.Request) {
	m, ok := s.loadMonitor(w, r)
	if !ok {
		return
	}
	total, err := s.store.CountChecks(r.Context(), m.ID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	checks, err := s.store.ListChecks(r.Context(), m.ID, total, 0)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	name := strings.Trim(unsafeFilename.ReplaceAllString(strings.ToLower(m.Name), "-"), "-")
	if name == "" {
		name = "monitor"
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-checks-%s.csv"`, name, s.now().UTC().Format("20060102")))
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"checked_at", "up", "latency_ms", "status_code", "error"})
	for _, c := range checks {
		_ = cw.Write([]string{
			c.CheckedAt.UTC().Format(time.RFC3339),
			strconv.FormatBool(c.Up),
			strconv.FormatInt(c.Latency.Milliseconds(), 10),
			strconv.Itoa(c.StatusCode),
			csvSafe(c.Error),
		})
	}
	cw.Flush()
}

// csvSafe stops spreadsheet programs from treating a cell as a formula.
func csvSafe(v string) string {
	if v != "" && strings.ContainsAny(v[:1], "=+-@") {
		return "'" + v
	}
	return v
}
