// probe verifies discovery and configuration authentication. -stream additionally
// attempts a read-only video session; screen contents and credentials are never logged.
package main

import (
	"aurga-web/internal/aurga"
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	device := flag.String("device", "", "required private device IPv4")
	stream := flag.Bool("stream", false, "attempt a read-only video session for six seconds")
	port := flag.Int("stream-port", 0, "override UDP stream port")
	trace := flag.Bool("trace", false, "print packet header metadata only")
	flag.Parse()
	ip := net.ParseIP(*device)
	if ip == nil || ip.To4() == nil || !ip.IsPrivate() {
		fmt.Fprintln(os.Stderr, "an explicit private IPv4 is required")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	d, e := aurga.Discover(ctx, ip)
	if e != nil {
		fmt.Fprintln(os.Stderr, "Discovery failed:", e)
		os.Exit(1)
	}
	fmt.Printf("Device: %s firmware=%s protected=%t\n", d.Name, d.Firmware, d.Secured)
	passcode := os.Getenv("AURGA_PASSCODE")
	if passcode == "" {
		fmt.Println("No AURGA_PASSCODE set; authentication not attempted")
		return
	}
	cfg, e := aurga.DeviceConfiguration(ctx, ip, passcode)
	if e != nil || cfg["rc"] != "0" {
		fmt.Fprintln(os.Stderr, "Configuration authentication failed")
		os.Exit(1)
	}
	fmt.Println("Configuration authentication: accepted")
	if !*stream {
		return
	}
	streamCtx, stop := context.WithTimeout(ctx, 6*time.Second)
	defer stop()
	nals := 0
	var tracer func(string)
	if *trace {
		tracer = func(s string) { fmt.Println(s) }
	}
	e = (aurga.Session{IP: ip, Port: *port, Trace: tracer}).Run(streamCtx, passcode, nil, func(event aurga.Event) {
		if event.Type == "nal" {
			nals++
		} else {
			fmt.Printf("%s: %s %s\n", event.Type, event.Message, event.Codec)
		}
	})
	fmt.Printf("Video buffers received: %d\n", nals)
	if e != nil && streamCtx.Err() == nil {
		fmt.Fprintln(os.Stderr, "Stream failed:", e)
		os.Exit(1)
	}
}
