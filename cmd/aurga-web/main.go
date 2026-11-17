package main

import (
	"aurga-web/internal/web"
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:9443", "HTTPS listen address (one explicit IP)")
	device := flag.String("device", "192.168.1.94", "fixed device IPv4 address")
	streamPort := flag.Int("stream-port", 0, "override the protocol UDP stream port")
	state := flag.String("state-dir", "", "persistent TLS certificate directory")
	plain := flag.Bool("http", false, "HTTP for loopback testing only")
	flag.Parse()
	ip := net.ParseIP(*device)
	if ip == nil || ip.To4() == nil || !ip.IsPrivate() {
		log.Fatal("device must be an explicit private IPv4 address")
	}
	host, _, err := net.SplitHostPort(*listen)
	if err != nil {
		log.Fatal(err)
	}
	bind := net.ParseIP(host)
	if bind == nil || bind.IsUnspecified() {
		log.Fatal("listen must name one explicit IP; wildcard binding is disabled")
	}
	if *plain && !bind.IsLoopback() {
		log.Fatal("HTTP is restricted to loopback; use HTTPS for LAN access")
	}
	if *streamPort < 0 || *streamPort > 65535 {
		log.Fatal("invalid stream port")
	}
	if *state == "" {
		base, e := os.UserConfigDir()
		if e != nil {
			log.Fatal(e)
		}
		*state = filepath.Join(base, "aurga-web")
	}
	handler := web.New(web.Config{DeviceIP: ip, StreamPort: *streamPort, Passcode: os.Getenv("AURGA_PASSCODE"), Hosts: []string{host, "localhost"}})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	server := &http.Server{Addr: *listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12}, BaseContext: func(net.Listener) context.Context { return ctx }}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		server.Shutdown(shutdown)
	}()
	scheme := "https"
	if *plain {
		scheme = "http"
	}
	fmt.Printf("AURGA viewer: %s://%s/\nDevice: %s (control disabled until explicitly enabled)\n", scheme, *listen, ip)
	if *plain {
		err = server.ListenAndServe()
	} else {
		cert, key, e := web.Certificate(*state, host)
		if e != nil {
			log.Fatal(e)
		}
		err = server.ListenAndServeTLS(cert, key)
	}
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
