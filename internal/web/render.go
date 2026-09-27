package web

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/sorin-work/status-page/internal/store"
	"github.com/sorin-work/status-page/internal/version"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

// page is the data every template receives.
type page struct {
	Title string
	Site  string
	// Nav marks the active admin section.
	Nav   string
	User  *store.Session
	Flash string
	Data  any
}

// templates holds one parsed template set per page, each combined with the
// shared layout.
type templates struct {
	pages map[string]*template.Template
}

func loadTemplates(funcs template.FuncMap) (*templates, error) {
	layout, err := template.New("layout.html").Funcs(funcs).ParseFS(templateFS, "templates/layout.html", "templates/partials.html")
	if err != nil {
		return nil, err
	}
	names, err := fs.Glob(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	t := &templates{pages: map[string]*template.Template{}}
	for _, name := range names {
		base := strings.TrimSuffix(path.Base(name), ".html")
		if base == "layout" || base == "partials" {
			continue
		}
		clone, err := layout.Clone()
		if err != nil {
			return nil, err
		}
		if _, err := clone.ParseFS(templateFS, name); err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		t.pages[base] = clone
	}
	return t, nil
}

func staticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return http.FileServerFS(sub)
}

func (s *Server) funcs() template.FuncMap {
	return template.FuncMap{
		"duration": formatDuration,
		"now":      s.now,
		"version":  func() string { return version.Version },
		"nl2br":    nl2br,
		"ago": func(t time.Time) template.HTML {
			// The exact time is shown on hover.
			return template.HTML(fmt.Sprintf(`<time datetime="%s" title="%s">%s</time>`,
				t.UTC().Format(time.RFC3339), template.HTMLEscapeString(t.In(s.loc).Format("2006-01-02 15:04:05 MST")), formatAgo(s.now().Sub(t))))
		},
		"datetime": func(t time.Time) string {
			if t.IsZero() {
				return "never"
			}
			return t.In(s.loc).Format("2006-01-02 15:04 MST")
		},
	}
}

// render executes a page template into a buffer first, so a template error
// produces a clean 500 instead of half a page.
func (s *Server) render(w http.ResponseWriter, r *http.Request, status int, name string, p page) {
	t, ok := s.tmpl.pages[name]
	if !ok {
		s.serverError(w, r, fmt.Errorf("unknown template %q", name))
		return
	}
	p.Site = s.site
	if p.User == nil {
		p.User = currentSession(r)
	}
	if p.User != nil && p.Flash == "" {
		p.Flash = s.takeFlash(w, r)
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", p); err != nil {
		s.serverError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

func (s *Server) serverError(w http.ResponseWriter, r *http.Request, err error) {
	s.logger.Error("request failed", "method", r.Method, "path", r.URL.Path, "request_id", requestID(r), "err", err)
	http.Error(w, "Something went wrong. Please try again. (request "+requestID(r)+")", http.StatusInternalServerError)
}

// formatDuration prints short durations such as 30s, 5m or 1h30m.
func formatDuration(d time.Duration) string {
	switch {
	case d <= 0:
		return "0s"
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < time.Hour && d%time.Minute == 0:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return d.Truncate(time.Second).String()
}

// formatAgo prints how long ago something happened, e.g. "3m ago".
func formatAgo(d time.Duration) string {
	switch {
	case d < 10*time.Second:
		return "just now"
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// nl2br escapes text and keeps its line breaks. The escaping must happen
// first: messages are written by admins and API clients but shown on the
// public page.
func nl2br(text string) template.HTML {
	escaped := template.HTMLEscapeString(strings.ReplaceAll(text, "\r\n", "\n"))
	return template.HTML(strings.ReplaceAll(escaped, "\n", "<br>"))
}

// notFound renders the 404 page.
func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	s.render(w, r, http.StatusNotFound, "not_found", page{Title: "Page not found"})
}
