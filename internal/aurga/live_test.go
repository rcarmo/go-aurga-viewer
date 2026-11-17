package aurga

import (
	"context"
	"net"
	"os"
	"testing"
	"time"
)

// Explicit opt-in: sends only the all-keys-released keyboard report. No key is
// pressed, no pointer moved, and no screen contents retained.
func TestLiveNeutralKeyboardAcknowledgement(t *testing.T) {
	if os.Getenv("AURGA_LIVE_INPUT_ACK") != "1" {
		t.Skip("explicit real-device neutral-report test")
	}
	ip := net.ParseIP(os.Getenv("AURGA_LIVE_DEVICE"))
	pass := os.Getenv("AURGA_PASSCODE")
	if ip == nil || !ip.IsPrivate() || pass == "" {
		t.Fatal("explicit private target and passcode required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	inputs := make(chan Input, 2)
	sent, acked := false, false
	err := (Session{IP: ip, InitialInput: true, Trace: func(s string) { t.Log(s) }}).Run(ctx, pass, inputs, func(e Event) {
		if e.Type == "control" && e.Message == "true" && !sent {
			sent = true
			inputs <- Input{Report: []byte{1, 0, 0, 0, 0, 0, 0, 0, 0}}
		}
		if e.Type == "input-ack" {
			acked = true
			cancel()
		}
	})
	if !sent || !acked {
		t.Fatalf("neutral HID: negotiated=%t acknowledged=%t error=%v", sent, acked, err)
	}
	t.Log("PASS: real device acknowledged neutral keyboard report; no keys pressed")
}
