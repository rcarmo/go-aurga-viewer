package aurga

import (
	"bytes"
	"crypto/md5"
	"testing"
)

func TestAssemblyOutOfOrderDuplicateAndAck(t *testing.T) {
	var a Assembler
	p := Packet{Type: 2, Sequence: 7, Total: 2, Index: 1, Data: []byte{2, 3}}
	frame, ack, e := a.Put(p)
	if e != nil || frame != nil || le.Uint32(ack.Data) != 2 {
		t.Fatal(frame, ack, e)
	}
	_, ack, e = a.Put(p)
	if e != nil || le.Uint32(ack.Data) != 2 {
		t.Fatal(ack, e)
	}
	p.Index = 0
	p.Data = bytes.Repeat([]byte{1}, FragmentSize)
	frame, ack, e = a.Put(p)
	if e != nil || len(frame) != 1202 || frame[1201] != 3 || le.Uint32(ack.Data) != 3 {
		t.Fatal(len(frame), ack, e)
	}
	frame, _, e = a.Put(p)
	if e != nil || frame != nil {
		t.Fatal("duplicate frame emitted", e)
	}
	p.Sequence = 6
	frame, ack, e = a.Put(p)
	if e != nil || frame != nil || ack.Type != 0 {
		t.Fatal("old frame accepted")
	}
}
func TestAssemblyBounds(t *testing.T) {
	for _, p := range []Packet{{Total: 0}, {Total: MaxFragments + 1}, {Total: 1, Index: 1, Data: []byte{1}}, {Total: 2, Index: 0, Data: []byte{1}}, {Total: 1, Data: make([]byte, 1201)}} {
		var a Assembler
		if _, _, e := a.Put(p); e == nil {
			t.Fatalf("accepted %+v", p)
		}
	}
}
func TestFrameChecksum(t *testing.T) {
	body := []byte{0, 0, 0, 1, 0x65, 1, 2, 3}
	sum := md5.Sum(body)
	data := append(bytes.Clone(body), sum[:]...)
	var a Assembler
	frame, _, e := a.Put(Packet{Total: 0x8001, Sequence: 1, Data: data})
	if e != nil || !bytes.Equal(frame, body) {
		t.Fatal(frame, e)
	}
	data[len(data)-1] ^= 1
	_, _, e = a.Put(Packet{Total: 0x8001, Sequence: 2, Data: data})
	if e == nil {
		t.Fatal("bad checksum accepted")
	}
}
func TestConflictingFragments(t *testing.T) {
	var a Assembler
	a.Put(Packet{Total: 2, Sequence: 1, Index: 1, Data: []byte{1}})
	if _, _, e := a.Put(Packet{Total: 2, Sequence: 1, Index: 1, Data: []byte{2}}); e == nil {
		t.Fatal("conflicting duplicate accepted")
	}
}
func FuzzAssembly(f *testing.F) {
	f.Add(uint16(1), uint16(0), uint32(1), []byte{1})
	f.Fuzz(func(t *testing.T, total, index uint16, seq uint32, data []byte) {
		var a Assembler
		a.Put(Packet{Total: total, Index: index, Sequence: seq, Data: data})
	})
}
