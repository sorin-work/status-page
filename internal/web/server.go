// Package web serves the admin interface, the public status page and the
// JSON API.
package web

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/sorin-work/status-page/internal/store"
)

// Options configure a Server.
type Options struct {
	Store     *store.Store
	Logger    *slog.Logger
	SiteTitle string
	Now       func() time.Time
}

// Server is the HTTP handler of the application.
type Server struct {
	store  *store.Store
	logger *slog.Logger
	site   string
	now    func() time.Time
	tmpl   *templates
	mux    *http.ServeMux
}

// New builds a Server and registers its routes.
func New(opts Options) (*Server, error) {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.SiteTitle == "" {
		opts.SiteTitle = "Status"
	}
	s := &Server{
		store:  opts.Store,
		logger: opts.Logger,
		site:   opts.SiteTitle,
		now:    opts.Now,
		mux:    http.NewServeMux(),
	}
	tmpl, err := loadTemplates(s.funcs())
	if err != nil {
		return nil, err
	}
	s.tmpl = tmpl
	s.routes()
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", staticHandler()))
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("GET /{$}", s.handleHome)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if err := s.store.Ping(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("database unavailable\n"))
		return
	}
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "home", page{})
}
