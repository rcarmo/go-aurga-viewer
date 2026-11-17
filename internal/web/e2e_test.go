package web

import (
	"aurga-web/internal/aurga"
	"context"
	"net"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"
)

// Optional: requires make browser-test's generated synthetic H.264 fixture.
func TestBrowserEndToEnd(t *testing.T) {
	if os.Getenv("AURGA_BROWSER_E2E") != "1" {
		t.Skip("set AURGA_BROWSER_E2E=1 for optional browser acceptance")
	}
	frame, e := os.ReadFile(os.Getenv("AURGA_TEST_FRAME"))
	if e != nil {
		t.Fatal(e)
	}
	peer, e := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	received := make(chan []byte, 128)
	peerDone := make(chan struct{})
	go func() {
		defer close(peerDone)
		buf := make([]byte, 65536)
		for {
			peer.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			n, addr, e := peer.ReadFromUDP(buf)
			if e != nil {
				if ctx.Err() != nil {
					return
				}
				continue
			}
			packets, e := aurga.ParsePackets(buf[:n])
			if e != nil {
				continue
			}
			for _, p := range packets {
				switch p.Type {
				case 4:
					p.Data = make([]byte, 66)
					p.Data[6] = 1
					peer.WriteToUDP(p.Marshal(), addr)
				case 5:
					peer.WriteToUDP(p.Marshal(), addr)
				case 8:
					op := p.Data[0]
					p.Data = nil
					peer.WriteToUDP(p.Marshal(), addr)
					if op == 5 {
						total := (len(frame) + 1199) / 1200
						for i := 0; i < total; i++ {
							end := (i + 1) * 1200
							if end > len(frame) {
								end = len(frame)
							}
							peer.WriteToUDP((aurga.Packet{Version: 3, Type: 2, Sequence: 1, Total: uint16(total), Index: uint16(i), Data: frame[i*1200 : end]}).Marshal(), addr)
						}
					}
				case 3:
					select {
					case received <- p.Data:
					default:
					}
					p.Data = nil
					peer.WriteToUDP(p.Marshal(), addr)
				case 30:
					peer.WriteToUDP(p.Marshal(), addr)
				}
			}
		}
	}()
	server := httptest.NewTLSServer(New(Config{DeviceIP: net.ParseIP("127.0.0.1"), StreamPort: peer.LocalAddr().(*net.UDPAddr).Port, Passcode: "test", Hosts: []string{"127.0.0.1"}}))
	defer server.Close()
	cmd := exec.CommandContext(ctx, "bun", "../../tests/e2e.ts")
	cmd.Env = append(os.Environ(), "AURGA_TEST_URL="+server.URL)
	out, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("browser failed: %v\n%s", e, out)
	}
	t.Log(string(out))
	cancel()
	<-peerDone
	close(received)
	key, mouse, release := false, false, false
	touch, drag, mouseRelease := false, false, false
	right, wheel := false, false
	backspace, enter := false, false
	for report := range received {
		if len(report) == 12 && report[3] == 1 {
			if report[6] == 42 {
				backspace = true
			}
			if report[6] == 40 {
				enter = true
			}
			if report[6] == 4 {
				key = true
			}
			if report[4] == 0 && report[6] == 0 {
				release = true
			}
		}
		if len(report) == 10 && report[3] == 4 {
			if report[4] == 2 {
				right = true
			}
			if report[9] == 255 {
				wheel = true
			}
			x := int(report[5]) | int(report[6])<<8
			y := int(report[7]) | int(report[8])<<8
			if report[4] == 1 {
				mouse = true
				if x > 22500 && x < 23400 && y > 19000 && y < 20400 {
					touch = true
				}
			}
			if report[4] == 0 {
				mouseRelease = true
				if x > 12500 && x < 13700 && y > 15700 && y < 17000 {
					drag = true
				}
			}
		}
	}
	if !key || !mouse || !release || !touch || !drag || !mouseRelease || !right || !wheel || !backspace || !enter {
		t.Fatalf("missing HID: key=%t mouse=%t release=%t touch=%t drag=%t mouseRelease=%t right=%t wheel=%t backspace=%t enter=%t", key, mouse, release, touch, drag, mouseRelease, right, wheel, backspace, enter)
	}
}
