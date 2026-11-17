package aurga

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"time"
)

type Device struct {
	Name     string `json:"name"`
	ID       string `json:"id"`
	IP       string `json:"ip"`
	Firmware string `json:"firmware"`
	Secured  bool   `json:"secured"`
}

var fieldPattern = regexp.MustCompile(`([a-z_]+)="([^"\r\n]*)"`)

func ParseDiscovery(data []byte) (Device, error) {
	m := map[string]string{}
	for _, x := range fieldPattern.FindAllSubmatch(data, -1) {
		m[string(x[1])] = string(x[2])
	}
	if m["mds"] == "" || m["id"] == "" {
		return Device{}, errors.New("invalid discovery response")
	}
	return Device{Name: m["mds"], ID: m["id"], IP: m["ip"], Firmware: m["version"], Secured: m["secured"] == "1"}, nil
}
func Discover(ctx context.Context, ip net.IP) (Device, error) {
	c, e := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero})
	if e != nil {
		return Device{}, e
	}
	defer c.Close()
	deadline := time.Now().Add(3 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	c.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { c.Close() })
	defer stop()
	port := c.LocalAddr().(*net.UDPAddr).Port
	_, e = c.WriteToUDP([]byte(fmt.Sprintf("mdc[%d]=%d|0", port, time.Now().UnixMilli())), &net.UDPAddr{IP: ip, Port: DiscoveryPort})
	if e != nil {
		return Device{}, e
	}
	buf := make([]byte, 4096)
	for {
		n, src, e := c.ReadFromUDP(buf)
		if e != nil {
			return Device{}, e
		}
		if !src.IP.Equal(ip) || src.Port != DiscoveryPort {
			continue
		}
		d, e := ParseDiscovery(buf[:n])
		if e != nil {
			continue
		}
		d.IP = ip.String()
		return d, nil
	}
}
