package web

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"github.com/coder/websocket"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func testServer() *Server {
	return New(Config{DeviceIP: net.ParseIP("127.0.0.1"), Passcode: "test", Hosts: []string{"127.0.0.1", "localhost"}})
}
func TestHostAndOrigin(t *testing.T) {
	s := testServer()
	for _, tc := range []struct {
		host, origin string
		want         int
	}{{"127.0.0.1:9443", "", 200}, {"attacker.example", "", 403}, {"127.0.0.1:9443", "https://attacker.example", 403}, {"127.0.0.1:9443", "https://127.0.0.1:9443", 200}} {
		r := httptest.NewRequest("GET", "https://"+tc.host+"/", nil)
		r.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("%+v got %d", tc, w.Code)
		}
		if w.Code == 200 && !strings.Contains(w.Body.String(), "LAN viewer") {
			t.Fatal("missing UI")
		}
	}
}
func TestEmbeddedAssets(t *testing.T) {
	for _, path := range []string{"/", "/app.js", "/style.css"} {
		r := httptest.NewRequest("GET", "https://localhost"+path, nil)
		w := httptest.NewRecorder()
		testServer().ServeHTTP(w, r)
		if w.Code != 200 || w.Body.Len() == 0 {
			t.Fatal(path, w.Code)
		}
		if w.Header().Get("Content-Security-Policy") == "" || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("missing security headers")
		}
	}
}
func TestRateLimit(t *testing.T) {
	s := testServer()
	for i := 0; i < 5; i++ {
		if !s.permitAttempt("127.0.0.1:12") {
			t.Fatal(i)
		}
	}
	if s.permitAttempt("127.0.0.1:99") {
		t.Fatal("sixth attempt accepted")
	}
	if !s.permitAttempt("127.0.0.2:99") {
		t.Fatal("another client blocked")
	}
}
func TestWebsocketWrongPasscode(t *testing.T) {
	server := httptest.NewServer(testServer())
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ws, _, e := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/ws", nil)
	if e != nil {
		t.Fatal(e)
	}
	defer ws.CloseNow()
	if e = ws.Write(ctx, websocket.MessageText, []byte(`{"type":"connect","passcode":"wrong"}`)); e != nil {
		t.Fatal(e)
	}
	_, _, e = ws.Read(ctx)
	if websocket.CloseStatus(e) != websocket.StatusPolicyViolation {
		t.Fatal(e)
	}
}
func TestWebsocketUntrustedOrigin(t *testing.T) {
	server := httptest.NewServer(testServer())
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	headers := http.Header{}
	headers.Set("Origin", "http://attacker.example")
	ws, r, e := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/ws", &websocket.DialOptions{HTTPHeader: headers})
	if ws != nil {
		ws.CloseNow()
	}
	if e == nil || r.StatusCode != 403 {
		t.Fatal("accepted cross-origin socket")
	}
}
func TestCertificatePersistenceAndPermissions(t *testing.T) {
	dir := t.TempDir()
	cp, kp, e := Certificate(dir, "192.168.1.167")
	if e != nil {
		t.Fatal(e)
	}
	cert, e := os.ReadFile(cp)
	if e != nil {
		t.Fatal(e)
	}
	block, _ := pem.Decode(cert)
	parsed, e := x509.ParseCertificate(block.Bytes)
	if e != nil || parsed.VerifyHostname("192.168.1.167") != nil {
		t.Fatal(e)
	}
	if _, e = tls.LoadX509KeyPair(cp, kp); e != nil {
		t.Fatal(e)
	}
	info, _ := os.Stat(kp)
	if info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
	Certificate(dir, "192.168.1.167")
	again, _ := os.ReadFile(cp)
	if string(again) != string(cert) {
		t.Fatal("certificate unnecessarily replaced")
	}
}
