package web

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTargetSelectionAndCookieBinding(t *testing.T) {
	s := testServer()
	for _, raw := range []string{"8.8.8.8", "127.0.0.1", "localhost", "::1", "0.0.0.0", "192.168.10.50:123"} {
		r := httptest.NewRequest("GET", "https://localhost/?ip="+raw, nil)
		if _, e := s.target(r); e == nil {
			t.Fatalf("accepted target %q", raw)
		}
	}
	r := httptest.NewRequest("POST", "https://localhost/api/auth?ip=192.168.10.50", strings.NewReader(`{"passcode":"test"}`))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	cookie := w.Result().Cookies()[0]
	for _, tc := range []struct {
		ip   string
		want int
	}{{"192.168.10.50", 204}, {"192.168.10.51", 401}, {"8.8.8.8", 401}, {"", 401}} {
		q := ""
		if tc.ip != "" {
			q = "?ip=" + tc.ip
		}
		r := httptest.NewRequest("GET", "https://localhost/api/auth"+q, nil)
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("target %s got %d", tc.ip, w.Code)
		}
	}
}
