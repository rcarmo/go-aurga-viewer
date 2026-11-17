package web

import (
	"aurga-web/internal/aurga"
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

func (s *Server) target(r *http.Request) (net.IP, error) {
	if raw := r.URL.Query().Get("ip"); raw != "" {
		ip := net.ParseIP(raw)
		if ip == nil || ip.To4() == nil || !ip.IsPrivate() {
			return nil, errors.New("ip must be a private IPv4 address")
		}
		return ip.To4(), nil
	}
	if s.cfg.DeviceIP != nil {
		return s.cfg.DeviceIP, nil
	}
	s.mu.Lock()
	cached, expires := s.discovered, s.discoveryExpires
	s.mu.Unlock()
	if cached != nil && time.Now().Before(expires) {
		return cached, nil
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	d, e := aurga.Discover(ctx, nil)
	if e != nil {
		return nil, errors.New("No AURGA discovered on the LAN")
	}
	ip := net.ParseIP(d.IP)
	if ip == nil || !ip.IsPrivate() {
		return nil, errors.New("Invalid discovered address")
	}
	s.mu.Lock()
	s.discovered = ip
	s.discoveryExpires = time.Now().Add(30 * time.Second)
	s.mu.Unlock()
	return ip, nil
}
