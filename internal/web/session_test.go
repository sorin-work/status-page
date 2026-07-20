package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/sorin-work/status-page/internal/auth"
	"github.com/sorin-work/status-page/internal/store"
)

func TestMain(m *testing.M) {
	auth.Iterations = 1000
	os.Exit(m.Run())
}

func createAdmin(t *testing.T, st *store.Store) {
	t.Helper()
	hash, err := auth.HashPassword("demo-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser(context.Background(), "admin", hash); err != nil {
		t.Fatal(err)
	}
}

func postForm(t *testing.T, h http.Handler, path string, form url.Values, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func getWith(t *testing.T, h http.Handler, path string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func sessionCookieFrom(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	return nil
}

var csrfPattern = regexp.MustCompile(`name="csrf" value="([^"]+)"`)

// login signs in as the demo admin and returns the session cookie and the
// CSRF token of the session.
func login(t *testing.T, h http.Handler) (*http.Cookie, string) {
	t.Helper()
	rec := postForm(t, h, "/admin/login", url.Values{"username": {"admin"}, "password": {"demo-password"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("login: status %d", rec.Code)
	}
	c := sessionCookieFrom(rec)
	if c == nil {
		t.Fatal("login: no session cookie")
	}
	page := getWith(t, h, "/admin", c)
	m := csrfPattern.FindStringSubmatch(page.Body.String())
	if m == nil {
		t.Fatal("no CSRF token on the admin page")
	}
	return c, m[1]
}

func TestAdminRequiresLogin(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := get(t, srv, "/admin")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/login?next=%2Fadmin" {
		t.Fatalf("got %d to %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestLoginPage(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := get(t, srv, "/admin/login?next=/admin/monitors")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `value="/admin/monitors"`) {
		t.Fatalf("login page: %d", rec.Code)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	for _, form := range []url.Values{
		{"username": {"admin"}, "password": {"nope"}},
		{"username": {"ghost"}, "password": {"demo-password"}},
	} {
		rec := postForm(t, srv, "/admin/login", form)
		if rec.Code != http.StatusUnauthorized || sessionCookieFrom(rec) != nil {
			t.Errorf("%v: got %d", form, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Wrong username or password") {
			t.Errorf("%v: missing error message", form)
		}
	}
}

func TestLoginAndDashboard(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	c, _ := login(t, srv)
	if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie flags: %+v", c)
	}
	rec := getWith(t, srv, "/admin", c)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "admin") {
		t.Fatalf("dashboard: %d", rec.Code)
	}
	// Already logged in: the login page goes straight to the dashboard.
	if rec := getWith(t, srv, "/admin/login", c); rec.Code != http.StatusSeeOther {
		t.Errorf("login page while logged in: %d", rec.Code)
	}
}

func TestLoginRedirectsToNext(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	rec := postForm(t, srv, "/admin/login", url.Values{
		"username": {"admin"}, "password": {"demo-password"}, "next": {"/admin?x=1"},
	})
	if rec.Header().Get("Location") != "/admin?x=1" {
		t.Errorf("redirected to %q", rec.Header().Get("Location"))
	}
}

func TestLoginRefusesExternalRedirects(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	for _, next := range []string{"//evil.example/phish", "/\\evil.example", "https://evil.example", "/admin\\@evil.example", "javascript:alert(1)"} {
		rec := postForm(t, srv, "/admin/login", url.Values{"username": {"admin"}, "password": {"demo-password"}, "next": {next}})
		if got := rec.Header().Get("Location"); got != "/admin" {
			t.Errorf("next=%q redirected to %q", next, got)
		}
	}
}

func TestSafeNext(t *testing.T) {
	for in, want := range map[string]string{
		"":                      "/admin",
		"/admin/monitors/3":     "/admin/monitors/3",
		"/admin?range=7d":       "/admin?range=7d",
		"//evil.example":        "/admin",
		"/\\evil.example":       "/admin",
		"http://evil.example/x": "/admin",
	} {
		if got := safeNext(in); got != want {
			t.Errorf("safeNext(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLogoutNeedsCSRF(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	c, csrf := login(t, srv)

	if rec := postForm(t, srv, "/admin/logout", url.Values{}, c); rec.Code != http.StatusForbidden {
		t.Errorf("logout without token: %d", rec.Code)
	}
	if rec := postForm(t, srv, "/admin/logout", url.Values{"csrf": {"forged"}}, c); rec.Code != http.StatusForbidden {
		t.Errorf("logout with a wrong token: %d", rec.Code)
	}
	rec := postForm(t, srv, "/admin/logout", url.Values{"csrf": {csrf}}, c)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("logout: %d", rec.Code)
	}
	if cleared := sessionCookieFrom(rec); cleared == nil || cleared.MaxAge >= 0 {
		t.Errorf("session cookie not cleared: %+v", cleared)
	}
}

func TestLogoutEndsSessionOnServer(t *testing.T) {
	srv, st := newTestServer(t)
	createAdmin(t, st)
	c, csrf := login(t, srv)
	if rec := postForm(t, srv, "/admin/logout", url.Values{"csrf": {csrf}}, c); rec.Code != http.StatusSeeOther {
		t.Fatalf("logout: %d", rec.Code)
	}
	// Replaying the old cookie must not get back in.
	rec := getWith(t, srv, "/admin", c)
	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), "/admin/login") {
		t.Errorf("old session cookie still works after logout: %d", rec.Code)
	}
}
