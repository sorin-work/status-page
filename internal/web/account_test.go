package web

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/sorin-work/status-page/internal/auth"
)

func TestChangePassword(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	c, csrf := login(t, srv)
	other, _ := login(t, srv)

	cases := []struct {
		current, next, confirm, want string
	}{
		{"wrong", "new-password-1", "new-password-1", "current password is wrong"},
		{"demo-password", "short", "short", "at least 8 characters"},
		{"demo-password", "new-password-1", "new-password-2", "do not match"},
		{"demo-password", "demo-password", "demo-password", "must be different"},
	}
	for _, tc := range cases {
		rec := postForm(t, srv, "/admin/account/password", url.Values{"csrf": {csrf}, "current_password": {tc.current}, "new_password": {tc.next}, "confirm_password": {tc.confirm}}, c)
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), tc.want) {
			t.Errorf("%+v: %d", tc, rec.Code)
		}
	}

	rec := postForm(t, srv, "/admin/account/password", url.Values{"csrf": {csrf}, "current_password": {"demo-password"}, "new_password": {"new-password-1"}, "confirm_password": {"new-password-1"}}, c)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("change: %d", rec.Code)
	}
	u, _ := st.GetUserByUsername(context.Background(), "admin")
	if auth.VerifyPassword(u.PasswordHash, "new-password-1") != nil {
		t.Error("password not changed")
	}
	if rec := getWith(t, srv, "/admin", c); rec.Code != http.StatusOK {
		t.Errorf("current session was logged out: %d", rec.Code)
	}
	if rec := getWith(t, srv, "/admin", other); rec.Code != http.StatusSeeOther {
		t.Errorf("other session still valid: %d", rec.Code)
	}
}
