package web

import "net/http"

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "dashboard", page{Title: "Monitors", Nav: "monitors"})
}
