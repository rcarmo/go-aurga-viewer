package web

import (
	"aurga-web/internal/aurga"
	"sync/atomic"
)

// Keep disposable media out of the reliable control/ACK queue. One producer
// (Session.Run) and one consumer (the WebSocket writer) own each delivery.
type delivery struct {
	control    chan aurga.Event
	video      chan aurga.Event
	audio      chan aurga.Event
	videoGap   atomic.Bool
	videoDrops atomic.Uint64
	audioDrops atomic.Uint64
}

func newDelivery() *delivery {
	return &delivery{control: make(chan aurga.Event, 32), video: make(chan aurga.Event, 4), audio: make(chan aurga.Event, 8)}
}

// offer only fails when reliable control overflows. Media overload is lossy,
// bounded, and recoverable; codec configuration remains reliable control.
func (d *delivery) offer(e aurga.Event) bool {
	if e.Type == "nal" && !e.Configuration {
		select {
		case d.video <- e:
			return true
		default:
		}
		dropped := uint64(0)
	drain:
		for {
			select {
			case <-d.video:
				dropped++
			default:
				break drain
			}
		}
		if dropped > 0 {
			d.videoDrops.Add(dropped)
			d.videoGap.Store(true)
		}
		select {
		case d.video <- e:
		default:
			d.videoDrops.Add(1)
			d.videoGap.Store(true)
		}
		return true
	}
	if e.Type == "pcm" {
		select {
		case d.audio <- e:
			return true
		default:
		}
		select {
		case <-d.audio:
			d.audioDrops.Add(1)
		default:
		}
		select {
		case d.audio <- e:
		default:
			d.audioDrops.Add(1)
		}
		return true
	}
	select {
	case d.control <- e:
		return true
	default:
		return false
	}
}

// Prioritise ACKs/configuration, then notify the decoder before more media.
func (d *delivery) priority() (aurga.Event, bool) {
	select {
	case e := <-d.control:
		return e, true
	default:
	}
	if d.videoGap.Swap(false) {
		return aurga.Event{Type: "video-reset"}, true
	}
	return aurga.Event{}, false
}
