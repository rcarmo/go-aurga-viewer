package web

import (
	"aurga-web/internal/aurga"
	"sync"
	"testing"
)

func TestMediaOverloadPreservesControlAndConfiguration(t *testing.T) {
	d := newDelivery()
	// A paused writer used to fill the shared queue and cancel the session.
	for i := 0; i < 2000; i++ {
		if !d.offer(aurga.Event{Type: "pcm", Data: []byte{byte(i)}}) || !d.offer(aurga.Event{Type: "nal", Data: []byte{byte(i)}}) {
			t.Fatal("media overflow terminated control")
		}
	}
	config := aurga.Event{Type: "nal", Configuration: true, Data: []byte{7, 8}}
	if !d.offer(config) || !d.offer(aurga.Event{Type: "input-ack"}) {
		t.Fatal("media starved control")
	}
	for _, kind := range []string{"nal", "input-ack", "video-reset"} {
		e, ok := d.priority()
		if !ok || e.Type != kind {
			t.Fatalf("priority got %s, want %s", e.Type, kind)
		}
		if kind == "nal" && !e.Configuration {
			t.Fatal("codec configuration was dropped")
		}
	}
	if len(d.video) > 4 || len(d.audio) > 8 || d.videoDrops.Load() == 0 || d.audioDrops.Load() == 0 {
		t.Fatal("unbounded media or missing drop counters")
	}
	if _, ok := d.priority(); ok {
		t.Fatal("gap notification was not coalesced")
	}
}

func TestReliableControlOverflowFailsClosed(t *testing.T) {
	d := newDelivery()
	for i := 0; i < 32; i++ {
		if !d.offer(aurga.Event{Type: "input-ack"}) {
			t.Fatal("control capacity changed")
		}
	}
	if d.offer(aurga.Event{Type: "input-ack"}) {
		t.Fatal("reliable control must not silently drop")
	}
}

func TestConcurrentMediaRecovery(t *testing.T) {
	d := newDelivery()
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10000; i++ {
			if !d.offer(aurga.Event{Type: "nal"}) || !d.offer(aurga.Event{Type: "pcm"}) {
				t.Error("media caused failure")
				break
			}
		}
		close(done)
	}()
consume:
	for {
		if _, ok := d.priority(); ok {
			continue
		}
		select {
		case <-d.video:
		case <-d.audio:
		case <-done:
			break consume
		}
	}
	wg.Wait()
}
