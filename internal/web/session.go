package web

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sorin-work/status-page/internal/auth"
	"github.com/sorin-work/status-page/internal/store"
)

const sessionCookie = "sp_session"

type ctxKey int

const sessionKey ctxKey = iota

// currentSession returns the logged in admin, or nil.
func currentSession(r *http.Request) *store.Session {
	sess, _ := r.Context().Value(sessionKey).(*store.Session)
	return sess
}

// withSession loads the session named by the cookie, if any.
func (s *Server) withSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err == nil && c.Value != "" {
			sess, err := s.store.GetSession(r.Context(), auth.HashToken(c.Value))
			switch {
			case err == nil:
				r = r.WithContext(context.WithValue(r.Context(), sessionKey, &sess))
			case !errors.Is(err, store.ErrNotFound):
				s.serverError(w, r, err)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// admin wraps handlers of the admin area: it requires a session and, for
// anything but GET, a matching CSRF token in the form.
func (s *Server) admin(h http.HandlerFunc) http.Handler {
	return s.requireAdmin(s.checkCSRF(h))
}

// checkCSRF rejects state-changing requests that do not carry the CSRF
// token of the current session.
func (s *Server) checkCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		sess := currentSession(r)
		token := r.PostFormValue("csrf")
		if sess == nil || token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(sess.CSRFToken)) != 1 {
			http.Error(w, "The form has expired. Go back, reload the page and try again.", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireAdmin sends anonymous visitors to the login page.
func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if currentSession(r) == nil {
			target := "/admin/login?next=" + url.QueryEscape(r.URL.RequestURI())
			http.Redirect(w, r, target, http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type loginData struct {
	Username string
	Next     string
	Error    string
}

func (s *Server) handleLoginForm(w http.ResponseWriter, r *http.Request) {
	if currentSession(r) != nil {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	s.render(w, r, http.StatusOK, "login", page{Title: "Log in", Data: loginData{Next: r.URL.Query().Get("next")}})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.PostFormValue("username"))
	password := r.PostFormValue("password")
	next := r.PostFormValue("next")

	ip := clientIP(r)
	if blocked, wait := s.logins.Blocked(ip); blocked {
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		s.render(w, r, http.StatusTooManyRequests, "login", page{Title: "Log in", Data: loginData{
			Username: username, Next: next, Error: fmt.Sprintf("Too many failed attempts. Try again in %s.", formatDuration(wait.Round(time.Minute)+time.Minute)),
		}})
		return
	}

	fail := func() {
		s.logins.Fail(ip)
		s.logger.Warn("failed login", "username", username, "ip", ip)
		s.render(w, r, http.StatusUnauthorized, "login", page{Title: "Log in", Data: loginData{
			Username: username, Next: next, Error: "Wrong username or password.",
		}})
	}

	user, err := s.store.GetUserByUsername(r.Context(), username)
	if errors.Is(err, store.ErrNotFound) {
		fail()
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if err := auth.VerifyPassword(user.PasswordHash, password); err != nil {
		fail()
		return
	}

	token, err := auth.NewToken()
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	csrf, err := auth.NewToken()
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	now := s.now().UTC()
	sess := store.Session{
		TokenHash: auth.HashToken(token),
		UserID:    user.ID,
		CSRFToken: csrf,
		CreatedAt: now,
		ExpiresAt: now.Add(s.sessionTTL),
	}
	if err := s.store.CreateSession(r.Context(), sess); err != nil {
		s.serverError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  sess.ExpiresAt,
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	s.logins.Reset(ip)
	s.logger.Info("admin logged in", "username", user.Username)

	http.Redirect(w, r, safeNext(next), http.StatusSeeOther)
}

// safeNext returns next if it is a path on this site, or /admin. Values
// like "//evil.test" or "/\evil.test" start with a slash but browsers
// treat them as links to another host.
func safeNext(next string) string {
	const fallback = "/admin"
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.HasPrefix(next, "/\\") {
		return fallback
	}
	u, err := url.Parse(next)
	if err != nil || u.Scheme != "" || u.Host != "" || strings.ContainsAny(next, "\\\r\n") {
		return fallback
	}
	return next
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	// Clearing the cookie is not enough: a copied cookie would keep
	// working until the session expires.
	if sess := currentSession(r); sess != nil {
		if err := s.store.DeleteSession(r.Context(), sess.TokenHash); err != nil {
			s.serverError(w, r, err)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}
