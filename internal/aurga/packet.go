// Package aurga implements the LAN protocol recovered from AURGA Viewer Android 2.3.1.45.
package aurga

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
)

const DiscoveryPort = 32469
const StreamPort = 36459 // ASClientHandleScanning: sockaddr port bytes 8e 6b.
const HeaderSize = 16
const Magic uint32 = 0x41564852

var le = binary.LittleEndian

type Packet struct {
	Version, Type byte
	Total, Index  uint16
	Sequence      uint32
	Data          []byte
}

func (p Packet) Marshal() []byte {
	b := make([]byte, HeaderSize+len(p.Data))
	le.PutUint32(b, Magic)
	b[4] = p.Version
	if b[4] == 0 {
		b[4] = 3
	}
	b[5] = p.Type
	le.PutUint16(b[6:], p.Total)
	le.PutUint16(b[8:], p.Index)
	le.PutUint32(b[10:], p.Sequence)
	le.PutUint16(b[14:], uint16(len(p.Data)))
	copy(b[16:], p.Data)
	return b
}
func ParsePackets(b []byte) ([]Packet, error) {
	var packets []Packet
	for len(b) > 0 {
		if len(b) < 16 || le.Uint32(b) != Magic {
			return nil, errors.New("invalid AURGA packet header")
		}
		n := int(le.Uint16(b[14:]))
		if n > len(b)-16 {
			return nil, errors.New("truncated AURGA payload")
		}
		p := Packet{Version: b[4], Type: b[5], Total: le.Uint16(b[6:]), Index: le.Uint16(b[8:]), Sequence: le.Uint32(b[10:]), Data: append([]byte(nil), b[16:16+n]...)}
		if p.Version < 2 || p.Version > 3 {
			return nil, fmt.Errorf("unsupported protocol version %d", p.Version)
		}
		if p.Type&128 != 0 {
			p.Type &= 127
			for i := range p.Data {
				p.Data[i] = ^p.Data[i]
			}
		}
		packets = append(packets, p)
		b = b[16+n:]
	}
	return packets, nil
}

// cryptKey4 mirrors as_get_data_by_key4: transformed challenge as AES-128 key,
// fixed IV, AES CTR. Used to encrypt a zero-padded passcode in the handshake.
func cryptKey4(data []byte, challenge [16]byte) []byte {
	k := challenge
	k[2], k[11] = k[11], k[2]
	k[5], k[6] = k[6], k[5]
	k[9] = ^k[9]
	iv := []byte{0xa2, 0xb4, 0x00, 0xae, 0xa3, 0x91, 0xe6, 0x40, 0x03, 0xf0, 0xd4, 0xee, 0xa2, 0x9d, 0x76, 0x5b}
	block, _ := aes.NewCipher(k[:])
	out := make([]byte, len(data))
	cipher.NewCTR(block, iv).XORKeyStream(out, data)
	return out
}
func Handshake(passcode string, sequence uint32) (Packet, error) {
	if len(passcode) > 16 {
		return Packet{}, errors.New("passcode exceeds 16 UTF-8 bytes")
	}
	var challenge [16]byte
	if _, err := rand.Read(challenge[:]); err != nil {
		return Packet{}, err
	}
	return handshakeWithChallenge(passcode, sequence, challenge), nil
}
func handshakeWithChallenge(passcode string, sequence uint32, challenge [16]byte) Packet {
	data := make([]byte, 49)
	copy(data[1:17], challenge[:])
	if passcode != "" {
		data[0] = 1
		plain := make([]byte, 16)
		copy(plain, passcode)
		copy(data[17:33], cryptKey4(plain, challenge))
	}
	return Packet{Version: 3, Type: 4, Sequence: sequence, Data: data}
}

// Management uses a 65-byte payload: operation followed by its data union.
func Management(operation byte, value byte, sequence uint32) Packet {
	b := make([]byte, 65)
	b[0] = operation
	b[1] = value
	return Packet{Version: 3, Type: 8, Sequence: sequence, Data: b}
}
