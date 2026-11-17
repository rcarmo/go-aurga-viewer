package web

import (
	"aurga-web/internal/aurga"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"time"
)

const cookieName = "aurga_session"

type loginSession struct {
	Passcode string
	Expires  time.Time
	Device   string
}

func (s *Server) session(r *http.Request) (string, net.IP, bool) {
	c, e := r.Cookie(cookieName)
	if e != nil {
		return "", nil, false
	}
	s.mu.Lock()
	v, ok := s.sessions[c.Value]
	if ok && time.Now().After(v.Expires) {
		delete(s.sessions, c.Value)
		ok = false
	}
	s.mu.Unlock()
	if !ok {
		return "", nil, false
	}
	ip, e := s.target(r)
	if e != nil || ip.String() != v.Device {
		return "", nil, false
	}
	return v.Passcode, ip, true
}
func (s *Server) auth(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		if _, _, ok := s.session(r); !ok {
			http.Error(w, "Passcode required", 401)
			return
		}
		w.WriteHeader(204)
		return
	}
	if r.Method == "DELETE" {
		s.mu.Lock()
		if c, e := r.Cookie(cookieName); e == nil {
			delete(s.sessions, c.Value)
		}
		s.mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: cookieName, Path: "/", MaxAge: -1, HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode})
		w.WriteHeader(204)
		return
	}
	if !s.permitAttempt(r.RemoteAddr) {
		http.Error(w, "Too many attempts; wait one minute", 429)
		return
	}
	var in struct {
		Passcode string `json:"passcode"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	if json.NewDecoder(r.Body).Decode(&in) != nil || len(in.Passcode) == 0 || len(in.Passcode) > 16 {
		http.Error(w, "Passcode required (maximum 16 bytes)", 400)
		return
	}
	ip, e := s.target(r)
	if e != nil {
		http.Error(w, e.Error(), http.StatusBadRequest)
		return
	}
	valid := false
	if s.cfg.Passcode != "" {
		valid = subtle.ConstantTimeCompare([]byte(in.Passcode), []byte(s.cfg.Passcode)) == 1
	} else {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		cfg, e := aurga.DeviceConfiguration(ctx, ip, in.Passcode)
		valid = e == nil && cfg["rc"] == "0"
	}
	if !valid {
		http.Error(w, "Incorrect passcode or device unavailable", 401)
		return
	}
	var random [32]byte
	if _, e := rand.Read(random[:]); e != nil {
		http.Error(w, "Cannot create session", 500)
		return
	}
	token := base64.RawURLEncoding.EncodeToString(random[:])
	expires := time.Now().Add(24 * time.Hour)
	s.mu.Lock()
	for k, v := range s.sessions {
		if time.Now().After(v.Expires) {
			delete(s.sessions, k)
		}
	}
	if c, e := r.Cookie(cookieName); e == nil {
		delete(s.sessions, c.Value)
	}
	if len(s.sessions) >= 128 {
		s.mu.Unlock()
		http.Error(w, "Session capacity reached", 503)
		return
	}
	s.sessions[token] = loginSession{Passcode: in.Passcode, Expires: expires, Device: ip.String()}
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: token, Path: "/", MaxAge: 86400, Expires: expires, HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode})
	w.WriteHeader(204)
}
