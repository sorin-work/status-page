package web

import (
	"net/http"

	"github.com/sorin-work/status-page/internal/auth"
)

type accountData struct {
	Error string
}

func (s *Server) handleAccount(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "account", page{Title: "Account", Nav: "account", Data: accountData{}})
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	sess := currentSession(r)
	current := r.PostFormValue("current_password")
	next := r.PostFormValue("new_password")
	confirm := r.PostFormValue("confirm_password")

	fail := func(msg string) {
		s.render(w, r, http.StatusUnprocessableEntity, "account", page{Title: "Account", Nav: "account", Data: accountData{Error: msg}})
	}
	user, err := s.store.GetUser(r.Context(), sess.UserID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	switch {
	case auth.VerifyPassword(user.PasswordHash, current) != nil:
		fail("The current password is wrong.")
		return
	case len(next) < auth.MinPasswordLength:
		fail("The new password must be at least 8 characters.")
		return
	case next != confirm:
		fail("The new passwords do not match.")
		return
	case next == current:
		fail("The new password must be different from the current one.")
		return
	}
	hash, err := auth.HashPassword(next)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if err := s.store.UpdatePassword(r.Context(), user.ID, hash, sess.TokenHash); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.logger.Info("password changed", "username", user.Username)
	s.redirectWithFlash(w, r, "/admin/account", "Password changed. Other sessions were logged out.")
}
