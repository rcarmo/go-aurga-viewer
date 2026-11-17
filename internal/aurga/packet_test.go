package aurga

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestPacketWire(t *testing.T) {
	p := Packet{Version: 3, Type: 4, Total: 2, Index: 1, Sequence: 0x12345678, Data: []byte{9, 8}}
	wire := p.Marshal()
	if hex.EncodeToString(wire) != "524856410304020001007856341202000908" {
		t.Fatalf("wire %x", wire)
	}
	got, e := ParsePackets(append(wire, wire...))
	if e != nil || len(got) != 2 || got[1].Sequence != p.Sequence || !bytes.Equal(got[1].Data, p.Data) {
		t.Fatalf("%+v %v", got, e)
	}
}
func TestInvertedPayload(t *testing.T) {
	wire := (Packet{Version: 3, Type: 0x82, Data: []byte{0xff, 0x55}}).Marshal()
	p, e := ParsePackets(wire)
	if e != nil || p[0].Type != 2 || !bytes.Equal(p[0].Data, []byte{0, 0xaa}) {
		t.Fatal(p, e)
	}
	if wire[16] != 0xff {
		t.Fatal("mutated input")
	}
}
func TestMalformedPackets(t *testing.T) {
	for _, b := range [][]byte{nil, {1}, make([]byte, 16), (Packet{Version: 8}).Marshal(), (Packet{Data: []byte{1}}).Marshal()[:16]} {
		if len(b) == 0 {
			continue
		}
		if _, e := ParsePackets(b); e == nil {
			t.Fatalf("accepted %x", b)
		}
	}
}
func TestHandshakeVector(t *testing.T) {
	p := handshakeWithChallenge("test", 1, [16]byte{})
	if len(p.Data) != 49 || p.Data[0] != 1 || hex.EncodeToString(p.Data[17:33]) != "a075bbf8387a505b6a3343946f2b9a35" {
		t.Fatalf("handshake mismatch %x", p.Data)
	}
	if !bytes.Equal(cryptKey4(p.Data[17:33], [16]byte{}), append([]byte("test"), make([]byte, 12)...)) {
		t.Fatal("CTR roundtrip")
	}
	if _, e := Handshake("01234567890123456", 1); e == nil {
		t.Fatal("long passcode accepted")
	}
}
func TestDiscovery(t *testing.T) {
	d, e := ParseDiscovery([]byte(`mds="test" id="123" ip="192.168.1.1" secured="1" version="4.0.0.0"`))
	if e != nil || !d.Secured || d.Name != "test" {
		t.Fatal(d, e)
	}
	if _, e := ParseDiscovery([]byte("garbage")); e == nil {
		t.Fatal("invalid discovery accepted")
	}
}
func TestReportValidation(t *testing.T) {
	for _, b := range [][]byte{{1, 0, 0, 4, 0, 0, 0, 0, 0}, {4, 1, 0xff, 0x7f, 0, 0, 0}, {2, 0, 0, 0, 0}, {20, 1, 0, 0, 0}, {2, 1, 129, 127, 0}} {
		if e := ValidateReport(b); e != nil {
			t.Fatal(e)
		}
	}
	for _, b := range [][]byte{{}, {1}, {4, 8, 0, 0, 0, 0, 0}, {4, 0, 0, 128, 0, 0, 0}, {20, 8, 0, 0, 0}, {20, 1, 0, 0}} {
		if ValidateReport(b) == nil {
			t.Fatalf("accepted %v", b)
		}
	}
}
func FuzzPackets(f *testing.F) {
	f.Add((Packet{Type: 4, Data: make([]byte, 49)}).Marshal())
	f.Add([]byte("garbage"))
	f.Fuzz(func(t *testing.T, b []byte) {
		ps, e := ParsePackets(b)
		if e != nil {
			return
		}
		for _, p := range ps {
			wire := p.Marshal()
			got, e := ParsePackets(wire)
			if e != nil || len(got) != 1 || !bytes.Equal(got[0].Data, p.Data) {
				t.Fatal("roundtrip")
			}
		}
	})
}
