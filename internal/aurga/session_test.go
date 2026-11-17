package aurga

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

func TestSessionSimulatedDevice(t *testing.T) {
	server, e := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if e != nil {
		t.Fatal(e)
	}
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	inputs := make(chan Input, 8)
	events := make(chan Event, 64)
	gotInput := make(chan []byte, 8)
	deviceDone := make(chan struct{})
	go func() {
		defer close(deviceDone)
		buf := make([]byte, 65536)
		for {
			server.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			n, src, e := server.ReadFromUDP(buf)
			if e != nil {
				if ctx.Err() != nil {
					return
				}
				continue
			}
			packets, e := ParsePackets(buf[:n])
			if e != nil {
				continue
			}
			for _, p := range packets {
				switch p.Type {
				case 4:
					p.Data = make([]byte, 66)
					server.WriteToUDP(p.Marshal(), src)
				case 5:
					server.WriteToUDP(p.Marshal(), src)
				case 8:
					op := p.Data[0]
					p.Data = nil
					server.WriteToUDP(p.Marshal(), src)
					if op == 5 {
						server.WriteToUDP((Packet{Version: 3, Type: 2, Sequence: 1, Total: 1, Data: []byte{0, 0, 0, 1, 0x65, 1}}).Marshal(), src)
					}
				case 3:
					select {
					case gotInput <- p.Data:
					default:
					}
					p.Data = nil
					server.WriteToUDP(p.Marshal(), src)
				}
			}
		}
	}()
	result := make(chan error, 1)
	go func() {
		result <- (Session{IP: net.ParseIP("127.0.0.1"), Port: server.LocalAddr().(*net.UDPAddr).Port}).Run(ctx, "test", inputs, func(e Event) { events <- e })
	}()
	seenVideo := false
	for !seenVideo {
		select {
		case e := <-events:
			seenVideo = e.Type == "nal"
		case err := <-result:
			t.Fatal(err)
		case <-ctx.Done():
			t.Fatal("no simulated video")
		}
	}
	inputs <- Input{Report: []byte{1, 0, 0, 4, 0, 0, 0, 0, 0}}
	select {
	case <-gotInput:
		t.Fatal("input sent without permission")
	case <-time.After(80 * time.Millisecond):
	}
	enabled := true
	inputs <- Input{Enable: &enabled}
	inputs <- Input{Report: []byte{1, 0, 0, 4, 0, 0, 0, 0, 0}}
	select {
	case b := <-gotInput:
		if len(b) != 12 || b[2] != 8 || b[6] != 4 {
			t.Fatalf("bad input framing %v", b)
		}
	case <-ctx.Done():
		t.Fatal("no input")
	}
	cancel()
	select {
	case <-result:
	case <-time.After(time.Second):
		t.Fatal("session did not cancel")
	}
	<-deviceDone
}
func TestSessionRejectsPasscode(t *testing.T) {
	server, e := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if e != nil {
		t.Fatal(e)
	}
	defer server.Close()
	go func() {
		b := make([]byte, 1024)
		server.SetReadDeadline(time.Now().Add(time.Second))
		n, src, e := server.ReadFromUDP(b)
		if e != nil {
			return
		}
		p, e := ParsePackets(b[:n])
		if e != nil {
			return
		}
		response := p[0]
		response.Data = make([]byte, 66)
		response.Data[1] = 2
		server.WriteToUDP(response.Marshal(), src)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	e = (Session{IP: net.ParseIP("127.0.0.1"), Port: server.LocalAddr().(*net.UDPAddr).Port}).Run(ctx, "bad", nil, func(Event) {})
	if e == nil || !strings.Contains(e.Error(), "rejected") {
		t.Fatal(e)
	}
}
