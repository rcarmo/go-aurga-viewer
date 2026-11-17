package aurga

import (
	"bytes"
	"crypto/md5"
	"errors"
)

const FragmentSize = 1200
const MaxFragments = 4096

// Assembler keeps one bounded frame. Older frames and duplicate fragments are ignored.
type Assembler struct {
	sequence    uint32
	total       int
	have        bool
	chunks      [][]byte
	received    int
	checksummed bool
	done        bool
}

func (a *Assembler) Put(p Packet) (frame []byte, ack Packet, err error) {
	total := int(p.Total & 0x7fff)
	index := int(p.Index)
	if total < 1 || total > MaxFragments || index >= total || len(p.Data) > FragmentSize || len(p.Data) == 0 {
		return nil, Packet{}, errors.New("invalid video fragment bounds")
	}
	if index < total-1 && len(p.Data) != FragmentSize {
		return nil, Packet{}, errors.New("short non-final video fragment")
	}
	if a.have && p.Sequence != a.sequence && int32(p.Sequence-a.sequence) < 0 {
		return nil, Packet{}, nil
	}
	if !a.have || p.Sequence != a.sequence {
		a.sequence = p.Sequence
		a.total = total
		a.chunks = make([][]byte, total)
		a.received = 0
		a.have = true
		a.done = false
		a.checksummed = p.Total&0x8000 != 0
	}
	if a.total != total || a.checksummed != (p.Total&0x8000 != 0) {
		return nil, Packet{}, errors.New("inconsistent fragment metadata")
	}
	if a.chunks[index] == nil {
		a.chunks[index] = bytes.Clone(p.Data)
		a.received++
	} else if !bytes.Equal(a.chunks[index], p.Data) {
		return nil, Packet{}, errors.New("conflicting duplicate fragment")
	}
	var mask uint32
	group := index / 32
	for i := group * 32; i < total && i < (group+1)*32; i++ {
		if a.chunks[i] != nil {
			mask |= 1 << uint(i%32)
		}
	}
	data := make([]byte, 4)
	le.PutUint32(data, mask)
	ack = Packet{Version: 3, Type: 2, Index: p.Index, Sequence: p.Sequence, Data: data}
	if a.received == a.total && !a.done {
		frame = bytes.Join(a.chunks, nil)
		a.done = true
		if a.checksummed {
			if len(frame) < 16 {
				return nil, ack, errors.New("missing frame checksum")
			}
			body := frame[:len(frame)-16]
			sum := md5.Sum(body)
			if !bytes.Equal(sum[:], frame[len(frame)-16:]) {
				return nil, ack, errors.New("frame checksum mismatch")
			}
			frame = body
		}
	}
	return
}
