package web

import (
	"net/http"
	"net/url"
)

const flashCookie = "sp_flash"

// setFlash stores a one-time message shown on the next page.
func (s *Server) setFlash(w http.ResponseWriter, msg string) {
	http.SetCookie(w, &http.Cookie{
		Name:     flashCookie,
		Value:    url.QueryEscape(msg),
		Path:     "/admin",
		MaxAge:   60,
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// redirectWithFlash sets a flash message and redirects.
func (s *Server) redirectWithFlash(w http.ResponseWriter, r *http.Request, target, msg string) {
	s.setFlash(w, msg)
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// takeFlash returns the pending flash message, if any, and clears it.
func (s *Server) takeFlash(w http.ResponseWriter, r *http.Request) string {
	c, err := r.Cookie(flashCookie)
	if err != nil || c.Value == "" {
		return ""
	}
	http.SetCookie(w, &http.Cookie{Name: flashCookie, Value: "", Path: "/admin", MaxAge: -1, HttpOnly: true, Secure: s.cookieSecure, SameSite: http.SameSiteLaxMode})
	msg, err := url.QueryUnescape(c.Value)
	if err != nil {
		return ""
	}
	return msg
}
