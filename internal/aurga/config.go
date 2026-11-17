package aurga

import (
	"bufio"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"
)

// DeviceConfiguration reads metadata only. It never issues mutation commands.
func DeviceConfiguration(ctx context.Context, ip net.IP, passcode string) (map[string]string, error) {
	var challenge [16]byte
	if _, e := rand.Read(challenge[:]); e != nil {
		return nil, e
	}
	key := challenge
	key[0], key[4] = key[4], key[0]
	key[7], key[9] = key[9], key[7]
	key[11] = ^key[11]
	block, _ := aes.NewCipher(key[:])
	iv := []byte{0xa2, 0xb4, 0x00, 0xae, 0xa3, 0x91, 0xe6, 0x40, 0x03, 0xf0, 0xd4, 0xee, 0xa2, 0x9d, 0x76, 0x5b}
	enc := make([]byte, len(passcode))
	cipher.NewCTR(block, iv).XORKeyStream(enc, []byte(passcode))
	c, e := (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(ip.String(), "32465"))
	if e != nil {
		return nil, e
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(4 * time.Second))
	stop := context.AfterFunc(ctx, func() { c.Close() })
	defer stop()
	if _, e = fmt.Fprintf(c, "get device_info key=\"%s%s\"\n", hex.EncodeToString(challenge[:]), hex.EncodeToString(enc)); e != nil {
		return nil, e
	}
	r := bufio.NewReaderSize(c, 8192)
	line, e := r.ReadSlice('\n')
	if e != nil {
		return nil, e
	}
	m := map[string]string{}
	for _, x := range fieldPattern.FindAllStringSubmatch(strings.TrimSpace(string(line)), -1) {
		m[x[1]] = x[2]
	}
	return m, nil
}
