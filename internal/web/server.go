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
	// SessionTTL is how long a login lasts.
	SessionTTL time.Duration
	// CookieSecure sets the Secure flag on the session cookie.
	CookieSecure bool
}

// Server is the HTTP handler of the application.
type Server struct {
	store  *store.Store
	logger *slog.Logger
	site   string
	now    func() time.Time
	tmpl   *templates
	mux    *http.ServeMux

	sessionTTL   time.Duration
	cookieSecure bool
}

// New builds a Server and registers its routes.
func New(opts Options) (*Server, error) {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.SessionTTL <= 0 {
		opts.SessionTTL = 7 * 24 * time.Hour
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

		sessionTTL:   opts.SessionTTL,
		cookieSecure: opts.CookieSecure,
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
	s.withSession(s.mux).ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", staticHandler()))
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("GET /{$}", s.handleStatusPage)

	s.mux.HandleFunc("GET /admin/login", s.handleLoginForm)
	s.mux.HandleFunc("POST /admin/login", s.handleLogin)
	s.mux.Handle("POST /admin/logout", s.admin(s.handleLogout))
	s.mux.Handle("GET /admin", s.admin(s.handleDashboard))
	s.mux.Handle("GET /admin/monitors/new", s.admin(s.handleNewMonitor))
	s.mux.Handle("POST /admin/monitors", s.admin(s.handleCreateMonitor))
	s.mux.Handle("GET /admin/monitors/{id}", s.admin(s.handleMonitorDetail))
	s.mux.Handle("GET /admin/monitors/{id}/edit", s.admin(s.handleEditMonitor))
	s.mux.Handle("POST /admin/monitors/{id}", s.admin(s.handleUpdateMonitor))
	s.mux.Handle("POST /admin/monitors/{id}/delete", s.admin(s.handleDeleteMonitor))
	s.mux.Handle("POST /admin/monitors/{id}/pause", s.admin(s.handlePauseMonitor))
	s.mux.Handle("POST /admin/monitors/{id}/resume", s.admin(s.handleResumeMonitor))

	s.mux.Handle("GET /admin/components", s.admin(s.handleComponents))
	s.mux.Handle("POST /admin/components", s.admin(s.handleCreateComponent))
	s.mux.Handle("POST /admin/components/{id}", s.admin(s.handleUpdateComponent))
	s.mux.Handle("POST /admin/components/{id}/delete", s.admin(s.handleDeleteComponent))

	s.mux.Handle("GET /admin/incidents", s.admin(s.handleIncidents))
	s.mux.Handle("GET /admin/incidents/new", s.admin(s.handleNewIncident))
	s.mux.Handle("POST /admin/incidents", s.admin(s.handleCreateIncident))
	s.mux.Handle("GET /admin/incidents/{id}", s.admin(s.handleIncidentDetail))
	s.mux.Handle("POST /admin/incidents/{id}/updates", s.admin(s.handleAddIncidentUpdate))
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
