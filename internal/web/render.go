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
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

// page is the data every template receives.
type page struct {
	Title string
	Site  string
	Data  any
}

// templates holds one parsed template set per page, each combined with the
// shared layout.
type templates struct {
	pages map[string]*template.Template
}

func loadTemplates(funcs template.FuncMap) (*templates, error) {
	layout, err := template.New("layout.html").Funcs(funcs).ParseFS(templateFS, "templates/layout.html")
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
		if base == "layout" {
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
		"datetime": func(t time.Time) string {
			if t.IsZero() {
				return "never"
			}
			return t.UTC().Format("2006-01-02 15:04 UTC")
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
	s.logger.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	http.Error(w, "Something went wrong. Please try again.", http.StatusInternalServerError)
}
