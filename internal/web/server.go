package web

import (
	"aurga-web/internal/aurga"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"github.com/coder/websocket"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

//go:embed ui/*
var assets embed.FS

type Config struct {
	DeviceIP   net.IP
	StreamPort int
	Passcode   string
	Hosts      []string
}
type Server struct {
	cfg              Config
	handler          http.Handler
	slot             chan struct{}
	mu               sync.Mutex
	attempts         map[string][]time.Time
	sessions         map[string]loginSession
	discovered       net.IP
	discoveryExpires time.Time
}

func New(cfg Config) *Server {
	s := &Server{cfg: cfg, slot: make(chan struct{}, 1), attempts: map[string][]time.Time{}, sessions: map[string]loginSession{}}
	mux := http.NewServeMux()
	root, _ := fs.Sub(assets, "ui")
	mux.Handle("GET /", http.FileServer(http.FS(root)))
	mux.HandleFunc("GET /api/device", s.device)
	mux.HandleFunc("GET /ws", s.connect)
	mux.HandleFunc("GET /api/auth", s.auth)
	mux.HandleFunc("POST /api/auth", s.auth)
	mux.HandleFunc("DELETE /api/auth", s.auth)
	s.handler = mux
	return s
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if h, _, e := net.SplitHostPort(host); e == nil {
		host = h
	}
	allowed := false
	for _, h := range s.cfg.Hosts {
		if strings.EqualFold(h, host) {
			allowed = true
			break
		}
	}
	if !allowed {
		http.Error(w, "Host not allowed", http.StatusForbidden)
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, e := url.Parse(origin)
		if e != nil || u.Host != r.Host || u.Scheme != "https" && !(u.Scheme == "http" && r.TLS == nil) {
			http.Error(w, "Origin not allowed", http.StatusForbidden)
			return
		}
	}
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	s.handler.ServeHTTP(w, r)
}
func (s *Server) device(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	ip, e := s.target(r)
	if e != nil {
		http.Error(w, e.Error(), 400)
		return
	}
	d, e := aurga.Discover(ctx, ip)
	if e != nil {
		http.Error(w, "Device discovery timed out", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(d)
}
func (s *Server) permitAttempt(remote string) bool {
	ip, _, _ := net.SplitHostPort(remote)
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	recent := s.attempts[ip][:0]
	for _, t := range s.attempts[ip] {
		if now.Sub(t) < time.Minute {
			recent = append(recent, t)
		}
	}
	if len(recent) >= 5 {
		return false
	}
	if len(s.attempts) >= 128 {
		for k, v := range s.attempts {
			if len(v) == 0 || now.Sub(v[len(v)-1]) >= time.Minute {
				delete(s.attempts, k)
			}
		}
		if len(s.attempts) >= 128 && s.attempts[ip] == nil {
			return false
		}
	}
	s.attempts[ip] = append(recent, now)
	return true
}

type command struct {
	Type     string `json:"type"`
	ReadOnly bool   `json:"readOnly,omitempty"`
	Enabled  bool   `json:"enabled,omitempty"`
	Report   []byte `json:"report,omitempty"`
}

func (s *Server) connect(w http.ResponseWriter, r *http.Request) {
	passcode, deviceIP, authorized := s.session(r)
	if !authorized {
		http.Error(w, "Passcode required", http.StatusUnauthorized)
		return
	}
	ws, e := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
	if e != nil {
		return
	}
	defer ws.CloseNow()
	ws.SetReadLimit(4096)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	firstCtx, stop := context.WithTimeout(ctx, 10*time.Second)
	typ, data, e := ws.Read(firstCtx)
	stop()
	if e != nil {
		return
	}
	var first command
	if typ != websocket.MessageText || json.Unmarshal(data, &first) != nil || first.Type != "connect" {
		ws.Close(websocket.StatusPolicyViolation, "Passcode required")
		return
	}
	select {
	case s.slot <- struct{}{}:
		defer func() { <-s.slot }()
	case <-ctx.Done():
		return
	case <-time.After(time.Second):
		ws.Close(websocket.StatusTryAgainLater, "Viewer already in use")
		return
	}
	events := make(chan aurga.Event, 32)
	inputs := make(chan aurga.Input, 64)
	result := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		err := (aurga.Session{IP: deviceIP, Port: s.cfg.StreamPort, InitialInput: !first.ReadOnly, Audio: true}).Run(ctx, passcode, inputs, func(ev aurga.Event) {
			select {
			case events <- ev:
			case <-ctx.Done():
			default:
				if ev.Type != "pcm" {
					cancel()
				}
			}
		})
		result <- err
	}()
	go func() {
		defer cancel()
		for {
			typ, data, e := ws.Read(ctx)
			if e != nil {
				return
			}
			var cmd command
			if typ != websocket.MessageText || json.Unmarshal(data, &cmd) != nil {
				return
			}
			var in aurga.Input
			switch cmd.Type {
			case "control":
				b := cmd.Enabled
				in.Enable = &b
			case "input":
				if aurga.ValidateReport(cmd.Report) != nil {
					return
				}
				in.Report = cmd.Report
			case "refresh":
				in.Refresh = true
			case "disconnect":
				return
			default:
				return
			}
			select {
			case inputs <- in:
			case <-ctx.Done():
				return
			default:
				return
			}
		}
	}()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	}()
	for {
		select {
		case ev := <-events:
			var message []byte
			typ := websocket.MessageText
			if ev.Type == "nal" || ev.Type == "pcm" {
				typ = websocket.MessageBinary
				if ev.Type == "pcm" {
					message = append([]byte{1}, ev.Data...)
				} else {
					message = append([]byte{0}, ev.Data...)
				}
			} else {
				message, _ = json.Marshal(ev)
			}
			writeCtx, stop := context.WithTimeout(ctx, 2*time.Second)
			err := ws.Write(writeCtx, typ, message)
			stop()
			if err != nil {
				return
			}
		case err := <-result:
			if err != nil && !errors.Is(err, context.Canceled) {
				msg, _ := json.Marshal(aurga.Event{Type: "error", Message: err.Error()})
				writeCtx, stop := context.WithTimeout(ctx, time.Second)
				ws.Write(writeCtx, websocket.MessageText, msg)
				stop()
			}
			ws.Close(websocket.StatusNormalClosure, "Session ended")
			return
		case <-ctx.Done():
			return
		}
	}
}
