package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSON(t *testing.T) {
	cases := []struct {
		body string
		code int
	}{
		{``, http.StatusBadRequest},
		{`{"name":`, http.StatusBadRequest},
		{`{"unknown":1}`, http.StatusBadRequest},
		{`{"name":"` + strings.Repeat("x", maxBodySize) + `"}`, http.StatusRequestEntityTooLarge},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(tc.body))
		var v struct {
			Name string `json:"name"`
		}
		if decodeJSON(rec, req, &v) {
			t.Errorf("%.20q: accepted", tc.body)
			continue
		}
		var body apiError
		if rec.Code != tc.code || json.Unmarshal(rec.Body.Bytes(), &body) != nil || body.Error == "" {
			t.Errorf("%.20q: %d %s", tc.body, rec.Code, rec.Body.String())
		}
	}
}
