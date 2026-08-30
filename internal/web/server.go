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
	// Location is the time zone used to show dates. Defaults to UTC.
	Location *time.Location
	// APIToken protects the JSON API. Empty disables it.
	APIToken string
	// Notifier sends webhook events. Optional.
	Notifier Notifier
	// StatusCacheTTL caches the public status data. Zero disables it.
	StatusCacheTTL time.Duration
}

// Server is the HTTP handler of the application.
type Server struct {
	store   *store.Store
	logger  *slog.Logger
	site    string
	now     func() time.Time
	tmpl    *templates
	mux     *http.ServeMux
	handler http.Handler

	sessionTTL   time.Duration
	cookieSecure bool
	loc          *time.Location
	apiToken     string
	notifier     Notifier
	logins       *loginLimiter
	statusCache  *statusCache
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
	if opts.Location == nil {
		opts.Location = time.UTC
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
		loc:          opts.Location,
		apiToken:     opts.APIToken,
		notifier:     opts.Notifier,
	}
	s.logins = newLoginLimiter(5, 15*time.Minute, s.now)
	s.statusCache = &statusCache{ttl: opts.StatusCacheTTL, now: s.now}
	tmpl, err := loadTemplates(s.funcs())
	if err != nil {
		return nil, err
	}
	s.tmpl = tmpl
	s.routes()
	s.handler = withRequestID(s.logRequests(securityHeaders(s.withSession(s.mux))))
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		// Anything that may change data makes the next status page fresh.
		defer s.statusCache.invalidate()
	}
	s.handler.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", staticHandler()))
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("/", s.notFound)
	s.mux.HandleFunc("GET /{$}", s.handleStatusPage)
	s.mux.HandleFunc("GET /incidents/{id}", s.handlePublicIncident)
	s.mux.HandleFunc("GET /history", s.handleHistory)

	s.mux.HandleFunc("GET /api/status", s.apiStatus)
	s.mux.Handle("GET /api/monitors", s.apiAuth(s.apiListMonitors))
	s.mux.Handle("POST /api/monitors", s.apiAuth(s.apiCreateMonitor))
	s.mux.Handle("GET /api/monitors/{id}", s.apiAuth(s.apiGetMonitor))
	s.mux.Handle("PUT /api/monitors/{id}", s.apiAuth(s.apiUpdateMonitor))
	s.mux.Handle("DELETE /api/monitors/{id}", s.apiAuth(s.apiDeleteMonitor))
	s.mux.Handle("POST /api/monitors/{id}/pause", s.apiAuth(s.apiPauseMonitor))
	s.mux.Handle("POST /api/monitors/{id}/resume", s.apiAuth(s.apiResumeMonitor))
	s.mux.Handle("GET /api/monitors/{id}/checks", s.apiAuth(s.apiListChecks))
	s.mux.Handle("GET /api/monitors/{id}/uptime", s.apiAuth(s.apiUptime))

	s.mux.Handle("GET /api/incidents", s.apiAuth(s.apiListIncidents))
	s.mux.Handle("POST /api/incidents", s.apiAuth(s.apiCreateIncident))
	s.mux.Handle("GET /api/incidents/{id}", s.apiAuth(s.apiGetIncident))
	s.mux.Handle("POST /api/incidents/{id}/updates", s.apiAuth(s.apiAddIncidentUpdate))

	s.mux.HandleFunc("GET /admin/login", s.handleLoginForm)
	s.mux.HandleFunc("POST /admin/login", s.handleLogin)
	s.mux.Handle("POST /admin/logout", s.admin(s.handleLogout))
	s.mux.Handle("GET /admin", s.admin(s.handleDashboard))
	s.mux.Handle("GET /admin/monitors/new", s.admin(s.handleNewMonitor))
	s.mux.Handle("POST /admin/monitors", s.admin(s.handleCreateMonitor))
	s.mux.Handle("GET /admin/monitors/{id}", s.admin(s.handleMonitorDetail))
	s.mux.Handle("GET /admin/monitors/{id}/edit", s.admin(s.handleEditMonitor))
	s.mux.Handle("GET /admin/monitors/{id}/duplicate", s.admin(s.handleDuplicateMonitor))
	s.mux.Handle("GET /admin/monitors/{id}/checks.csv", s.admin(s.handleExportChecks))
	s.mux.Handle("POST /admin/monitors/{id}", s.admin(s.handleUpdateMonitor))
	s.mux.Handle("POST /admin/monitors/{id}/delete", s.admin(s.handleDeleteMonitor))
	s.mux.Handle("POST /admin/monitors/{id}/pause", s.admin(s.handlePauseMonitor))
	s.mux.Handle("POST /admin/monitors/{id}/resume", s.admin(s.handleResumeMonitor))

	s.mux.Handle("GET /admin/components", s.admin(s.handleComponents))
	s.mux.Handle("POST /admin/components", s.admin(s.handleCreateComponent))
	s.mux.Handle("POST /admin/components/{id}", s.admin(s.handleUpdateComponent))
	s.mux.Handle("POST /admin/components/{id}/delete", s.admin(s.handleDeleteComponent))

	s.mux.Handle("GET /admin/maintenance", s.admin(s.handleMaintenance))
	s.mux.Handle("POST /admin/maintenance", s.admin(s.handleCreateMaintenance))
	s.mux.Handle("POST /admin/maintenance/{id}/end", s.admin(s.handleEndMaintenance))
	s.mux.Handle("POST /admin/maintenance/{id}/delete", s.admin(s.handleDeleteMaintenance))

	s.mux.Handle("GET /admin/account", s.admin(s.handleAccount))
	s.mux.Handle("POST /admin/account/password", s.admin(s.handleChangePassword))

	s.mux.Handle("GET /admin/webhooks", s.admin(s.handleWebhooks))
	s.mux.Handle("POST /admin/webhooks", s.admin(s.handleCreateWebhook))
	s.mux.Handle("GET /admin/webhooks/{id}", s.admin(s.handleWebhookDeliveries))
	s.mux.Handle("POST /admin/webhooks/{id}/toggle", s.admin(s.handleToggleWebhook))
	s.mux.Handle("POST /admin/webhooks/{id}/delete", s.admin(s.handleDeleteWebhook))
	s.mux.Handle("POST /admin/webhooks/{id}/test", s.admin(s.handleTestWebhook))

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
