package aurga

import (
	"bytes"
	"context"
	"encoding/hex"
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
		firstInput := true
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
					if p.Sequence == 0 || p.Sequence > 10000 {
						t.Errorf("initial command sequence outside vendor range: %d", p.Sequence)
					}
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
					if p.Sequence == 1 {
						if firstInput {
							firstInput = false
							continue
						} // Drop the first ACK.
						if buf[5] != 0x83 {
							t.Errorf("first retry must use vendor complement flag, got %x", buf[5])
						}
					}
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
	inputs <- Input{Report: []byte{10, 2, 0, 11, 0, 0, 0, 0, 0}}
	select {
	case b := <-gotInput:
		if len(b) != 12 || b[2] != 8 || b[3] != 10 || b[4] != 2 || b[6] != 11 {
			t.Fatalf("bad vendor software-keyboard framing %v", b)
		}
	case <-ctx.Done():
		t.Fatal("no software key tap")
	}
	cancel()
	select {
	case <-result:
	case <-time.After(time.Second):
		t.Fatal("session did not cancel")
	}
	<-deviceDone
	var finalStats *InputStats
	for len(events) > 0 {
		ev := <-events
		if ev.Input != nil {
			finalStats = ev.Input
		}
	}
	if finalStats == nil || finalStats.Received != 2 || finalStats.Sent != 2 || finalStats.Retries < 1 || finalStats.Acknowledged < 1 {
		t.Fatalf("inaccurate input counters: %+v", finalStats)
	}
	close(gotInput)
	relativeRelease := false
	for b := range gotInput {
		if len(b) == 10 && b[3] == 4 {
			t.Fatal("absolute pointer release can jump to origin")
		}
		if len(b) == 8 && b[3] == 2 && b[4] == 0 && b[5] == 0 && b[6] == 0 && b[7] == 0 {
			relativeRelease = true
		}
	}
	if !relativeRelease {
		t.Fatal("missing zero-motion relative release")
	}
}
func TestVendorInputTransmissionGolden(t *testing.T) {
	p := Packet{Version: 3, Type: 3, Sequence: 1, Data: []byte{12, 0, 8, 1, 0, 0, 4, 0, 0, 0, 0, 0}}
	// Native ASClientEnqueueInput + SendThread, independently transcribed bytes.
	plain, _ := hex.DecodeString("52485641030300000000010000000c000c0008010000040000000000")
	complemented, _ := hex.DecodeString("52485641038300000000010000000c00f3fff7fefffffbffffffffff")
	for attempt := 0; attempt < 4; attempt++ {
		want := plain
		if attempt%2 == 1 {
			want = complemented
		}
		wire := inputTransmission(p, attempt).Marshal()
		if !bytes.Equal(wire, want) {
			t.Fatalf("attempt %d: %x != %x", attempt, wire, want)
		}
		decoded, e := ParsePackets(wire)
		if e != nil || len(decoded) != 1 || !bytes.Equal(decoded[0].Data, p.Data) {
			t.Fatalf("retry did not decode to original report: %v", e)
		}
	}
	if !bytes.Equal(p.Marshal(), plain) {
		t.Fatal("retry mutated retained input packet")
	}
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

func TestAudioPCMConfigurationAndSequence(t *testing.T) {
	peer, e := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			peer.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
			n, addr, e := peer.ReadFromUDP(buf)
			if e != nil {
				if ctx.Err() != nil {
					return
				}
				continue
			}
			ps, e := ParsePackets(buf[:n])
			if e != nil {
				continue
			}
			for _, p := range ps {
				switch p.Type {
				case 4:
					p.Data = make([]byte, 66)
					p.Data[2] = 1
					p.Data[3] = 2
					le.PutUint16(p.Data[4:], 48000)
					peer.WriteToUDP(p.Marshal(), addr)
				case 5:
					peer.WriteToUDP(p.Marshal(), addr)
				case 8:
					op, value := p.Data[0], p.Data[1]
					p.Data = nil
					peer.WriteToUDP(p.Marshal(), addr)
					if op == 1 && value == 1 {
						for _, seq := range []uint32{1, 1, 0, 2, 3} {
							peer.WriteToUDP((Packet{Type: 1, Sequence: seq, Data: []byte{0, 128, 255, 127}}).Marshal(), addr)
						}
					}
				}
			}
		}
	}()
	formats, frames := 0, 0
	var finalAudio *AudioStats
	err := (Session{IP: net.ParseIP("127.0.0.1"), Port: peer.LocalAddr().(*net.UDPAddr).Port, Audio: true}).Run(ctx, "test", nil, func(e Event) {
		if e.Type == "audio" {
			if e.Channels != 2 || e.SampleRate != 48000 {
				t.Error(e)
			}
			formats++
		}
		if e.Audio != nil {
			finalAudio = e.Audio
		}
		if e.Type == "pcm" {
			frames++
			if len(e.Data) != 4 || e.Data[1] != 128 {
				t.Error("PCM changed")
			}
			if frames == 3 {
				cancel()
			}
		}
	})
	cancel()
	<-done
	if finalAudio == nil || finalAudio.Packets != 5 || finalAudio.Accepted != 3 || finalAudio.Bytes != 20 || finalAudio.SampleRate != 48000 || finalAudio.Channels != 2 {
		t.Fatalf("inaccurate audio counters: %+v", finalAudio)
	}
	if formats != 1 || frames != 3 {
		t.Fatalf("formats=%d frames=%d error=%v", formats, frames, err)
	}
}
