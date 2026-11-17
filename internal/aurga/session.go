package aurga

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"
)

type Event struct {
	Type       string `json:"type"`
	Message    string `json:"message,omitempty"`
	Codec      string `json:"codec,omitempty"`
	Data       []byte `json:"-"`
	Channels   int    `json:"channels,omitempty"`
	SampleRate int    `json:"sampleRate,omitempty"`
}
type Input struct {
	Report  []byte
	Enable  *bool
	Refresh bool
}
type Session struct {
	IP           net.IP
	Port         int
	Trace        func(string)
	InitialInput bool
	Audio        bool
}

// Run owns all protocol state and serialises writes. emit must not retain mutable buffers.
func (s Session) Run(ctx context.Context, passcode string, inputs <-chan Input, emit func(Event)) error {
	port := s.Port
	if port == 0 {
		port = StreamPort
	}
	c, e := net.DialUDP("udp4", nil, &net.UDPAddr{IP: s.IP, Port: port})
	if e != nil {
		return e
	}
	defer c.Close()
	stop := context.AfterFunc(ctx, func() { c.SetReadDeadline(time.Now()) })
	defer stop()
	c.SetReadBuffer(4 << 20)
	seq := uint32(time.Now().UnixMilli())
	next := func() uint32 { seq++; return seq }
	handshake, e := Handshake(passcode, next())
	if e != nil {
		return e
	}
	queue := []Packet{handshake}
	var lastCommand time.Time
	tries := 0
	lastSeen := time.Now()
	lastPing := time.Now()
	authenticated := false
	confirmed := false
	hdmi := false
	var completedFrames uint16
	inputEnabled := s.InitialInput
	var audioSequence uint32
	audioSeen := false
	audioChannels := 0
	audioConfig := func(channels, rate int) {
		if channels < 1 || channels > 8 || rate < 8000 || rate > 192000 {
			audioChannels = 0
			return
		}
		audioChannels = channels
		emit(Event{Type: "audio", Channels: channels, SampleRate: rate})
	}
	inputReady := false
	var inputSeq uint32
	var pendingInput *Packet
	var inputAt time.Time
	inputTries := 0
	var inputQueue [][]byte
	// Relative zero-motion release avoids a pointer jump on disconnect.
	releaseReports := [][]byte{{1, 0, 0, 0, 0, 0, 0, 0, 0}, {2, 0, 0, 0, 0}}
	defer func() {
		if inputEnabled {
			c.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
			for _, report := range releaseReports {
				inputSeq++
				b := make([]byte, len(report)+3)
				le.PutUint16(b, uint16(len(b)))
				b[2] = 8
				copy(b[3:], report)
				c.Write((Packet{Version: 3, Type: 3, Sequence: inputSeq, Data: b}).Marshal())
			}
			c.Write(Management(2, 0, next()).Marshal())
		}
	}()
	codec := "avc"
	var assembly Assembler
	buf := make([]byte, 65536)
	send := func(p Packet) error {
		if s.Trace != nil {
			s.Trace(fmt.Sprintf("send type=%d seq=%d bytes=%d", p.Type, p.Sequence, len(p.Data)))
		}
		_, err := c.Write(p.Marshal())
		return err
	}
	enqueue := func(op, value byte) { queue = append(queue, Management(op, value, next())) }
	emit(Event{Type: "status", Message: "Authenticating stream"})
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		now := time.Now()
		if len(queue) > 0 && (lastCommand.IsZero() || now.Sub(lastCommand) >= 500*time.Millisecond) {
			if tries >= 6 {
				return errors.New("stream command timed out")
			}
			if err := send(queue[0]); err != nil {
				return fmt.Errorf("stream service unavailable: %w", err)
			}
			tries++
			lastCommand = now
		}
		if now.Sub(lastSeen) > 5*time.Second {
			return errors.New("no response from device stream service")
		}
		if confirmed && now.Sub(lastPing) > time.Second {
			send(Packet{Version: 3, Type: 30, Sequence: uint32(now.UnixMilli())})
			lastPing = now
		}
		if inputReady && pendingInput == nil && len(inputQueue) > 0 {
			report := inputQueue[0]
			inputQueue = inputQueue[1:]
			inputSeq++
			data := make([]byte, 3+len(report))
			le.PutUint16(data, uint16(len(data)))
			data[2] = 8
			copy(data[3:], report)
			p := Packet{Version: 3, Type: 3, Sequence: inputSeq, Data: data}
			pendingInput = &p
			inputAt = time.Time{}
			inputTries = 0
		}
		if pendingInput != nil && now.Sub(inputAt) >= 20*time.Millisecond {
			if inputTries >= 10 {
				return errors.New("input acknowledgement timed out; control stopped")
			}
			if err := send(*pendingInput); err != nil {
				return err
			}
			inputTries++
			inputAt = now
		}
		select {
		case in, ok := <-inputs:
			if !ok {
				inputs = nil
				break
			}
			if !confirmed {
				break
			}
			if in.Enable != nil {
				if !*in.Enable && inputEnabled {
					for _, report := range releaseReports {
						inputSeq++
						b := make([]byte, 3+len(report))
						le.PutUint16(b, uint16(len(b)))
						b[2] = 8
						copy(b[3:], report)
						send(Packet{Version: 3, Type: 3, Sequence: inputSeq, Data: b})
					}
					inputQueue = nil
					pendingInput = nil
				}
				inputEnabled = *in.Enable
				if !inputEnabled {
					inputReady = false
				}
				value := byte(0)
				if inputEnabled {
					value = 1
				}
				enqueue(2, value)
				emit(Event{Type: "control", Message: fmt.Sprintf("%t", inputEnabled)})
			}
			if in.Refresh {
				enqueue(5, 0)
			}
			if len(in.Report) > 0 && inputEnabled {
				if err := ValidateReport(in.Report); err != nil {
					return err
				}
				if len(inputQueue) >= 64 {
					return errors.New("input queue full; control stopped")
				}
				inputQueue = append(inputQueue, append([]byte(nil), in.Report...))
			}
		default:
		}
		c.SetReadDeadline(time.Now().Add(20 * time.Millisecond))
		n, err := c.Read(buf)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return fmt.Errorf("device stream service unavailable: %w", err)
		}
		packets, err := ParsePackets(buf[:n])
		if err != nil {
			continue
		}
		lastSeen = time.Now()
		for _, p := range packets {
			if s.Trace != nil && p.Type != 2 && p.Type != 22 {
				s.Trace(fmt.Sprintf("recv type=%d seq=%d bytes=%d index=%d total=%d", p.Type, p.Sequence, len(p.Data), p.Index, p.Total))
			}
			if len(queue) > 0 && p.Type == queue[0].Type && p.Sequence == queue[0].Sequence {
				acknowledged := queue[0]
				if acknowledged.Type == 8 && len(acknowledged.Data) > 1 && acknowledged.Data[0] == 2 {
					inputReady = acknowledged.Data[1] != 0 && inputEnabled
					emit(Event{Type: "control", Message: fmt.Sprintf("%t", inputReady)})
				}
				queue = queue[1:]
				lastCommand = time.Time{}
				tries = 0
			}
			switch p.Type {
			case 4:
				if len(p.Data) < 8 {
					return errors.New("short handshake response")
				}
				if p.Data[1] != 0 {
					return fmt.Errorf("stream authentication rejected (status %d)", p.Data[1])
				}
				if authenticated {
					continue
				}
				authenticated = true
				if s.Audio && p.Data[2] != 0 {
					audioConfig(int(p.Data[3]), int(le.Uint16(p.Data[4:6])))
				}
				hdmi = p.Data[6] != 0
				if len(p.Data) > 58 && p.Data[58] == 1 {
					codec = "hevc"
				}
				emit(Event{Type: "codec", Codec: codec})
				size := int(p.Data[7])
				if size > len(p.Data)-8 {
					return errors.New("invalid codec configuration length")
				}
				if size > 0 {
					emit(Event{Type: "nal", Data: append([]byte(nil), p.Data[8:8+size]...)})
				}
				queue = append(queue, Packet{Version: 3, Type: 5, Sequence: next()})
				emit(Event{Type: "status", Message: fmt.Sprintf("Stream authenticated (HDMI state %d, codec configuration %d bytes)", p.Data[6], size)})
			case 5:
				if !authenticated {
					return errors.New("unexpected session confirmation")
				}
				if confirmed {
					continue
				}
				confirmed = true
				enqueue(7, 0) // ASClientWakeUp: session activation, separate from Wake-on-LAN.
				audioValue, inputValue := byte(0), byte(0)
				if s.Audio {
					audioValue = 1
				}
				if inputEnabled {
					inputValue = 1
				}
				enqueue(1, audioValue)
				enqueue(2, inputValue)
				enqueue(0, 1)
				enqueue(5, 0)
				message := "Connected — no HDMI signal"
				if hdmi {
					message = "Connected — waiting for HDMI video"
				}
				emit(Event{Type: "status", Message: message})
			case 2:
				if !confirmed {
					continue
				}
				frame, ack, err := assembly.Put(p)
				if err != nil {
					emit(Event{Type: "warning", Message: err.Error()})
					continue
				}
				if ack.Type != 0 {
					ack.Total = completedFrames
					if err = send(ack); err != nil {
						return err
					}
				}
				if len(frame) > 0 {
					completedFrames++
					if !hdmi {
						hdmi = true
						emit(Event{Type: "status", Message: "Connected — receiving video"})
					}
					emit(Event{Type: "nal", Data: frame})
				}
			case 1:
				if confirmed && s.Audio && audioChannels > 0 && len(p.Data) > 0 && len(p.Data) <= 16384 && len(p.Data)%(2*audioChannels) == 0 && (!audioSeen || int32(p.Sequence-audioSequence) > 0) {
					audioSeen = true
					audioSequence = p.Sequence
					emit(Event{Type: "pcm", Data: p.Data})
				}
			case 3:
				if pendingInput != nil && p.Sequence == pendingInput.Sequence {
					pendingInput = nil
					emit(Event{Type: "input-ack"})
				}
			case 6:
				p.Data = nil
				send(p)
			case 20:
				if !confirmed {
					continue
				}
				ack := p
				ack.Data = nil
				send(ack)
				if p.Index == 1 {
					hdmi = true
					emit(Event{Type: "status", Message: "Connected — waiting for HDMI video"})
					emit(Event{Type: "nal", Data: p.Data})
				} else {
					hdmi = false
					emit(Event{Type: "status", Message: "Connected — no HDMI signal"})
				}
			case 21:
				if s.Audio && len(p.Data) >= 8 {
					audioConfig(int(le.Uint32(p.Data)), int(le.Uint32(p.Data[4:])))
				}
				ack := p
				ack.Data = nil
				send(ack)
			case 30:
				// Connectivity response. No echo: the device is the responder.
			}
		}
	}
}
func ValidateReport(b []byte) error {
	if len(b) == 9 && b[0] == 1 {
		return nil
	}
	if len(b) == 7 && b[0] == 4 && b[1]&^byte(7) == 0 && binary.LittleEndian.Uint16(b[2:]) <= 32767 && binary.LittleEndian.Uint16(b[4:]) <= 32767 {
		return nil
	}
	if len(b) == 5 && b[0] == 2 && b[1]&^byte(7) == 0 {
		return nil
	}
	return errors.New("invalid HID report")
}
