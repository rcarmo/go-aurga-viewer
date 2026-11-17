package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCookieAuthentication(t *testing.T) {
	s := testServer()
	request := func(method, body string, c *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://localhost/api/auth", strings.NewReader(body))
		if c != nil {
			r.AddCookie(c)
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	if request("GET", "", nil).Code != 401 {
		t.Fatal("unauthenticated")
	}
	if request("POST", `{"passcode":"wrong"}`, nil).Code != 401 {
		t.Fatal("wrong passcode")
	}
	w := request("POST", `{"passcode":"test"}`, nil)
	if w.Code != 204 {
		t.Fatal(w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal(cookies)
	}
	c := cookies[0]
	if !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || len(c.Value) < 40 || strings.Contains(c.Value, "test") {
		t.Fatal("unsafe cookie", c)
	}
	if request("GET", "", c).Code != 204 {
		t.Fatal("cookie not accepted")
	}
	s.mu.Lock()
	v := s.sessions[c.Value]
	v.Expires = time.Now().Add(-time.Second)
	s.sessions[c.Value] = v
	s.mu.Unlock()
	if request("GET", "", c).Code != 401 {
		t.Fatal("expired cookie")
	}
	w = request("POST", `{"passcode":"test"}`, nil)
	c = w.Result().Cookies()[0]
	other := testServer()
	r := httptest.NewRequest("GET", "https://localhost/api/auth", nil)
	r.AddCookie(c)
	out := httptest.NewRecorder()
	other.ServeHTTP(out, r)
	if out.Code != 401 {
		t.Fatal("session survived new server")
	}
	if request("DELETE", "", c).Code != 204 || request("GET", "", c).Code != 401 {
		t.Fatal("logout failed")
	}
}
