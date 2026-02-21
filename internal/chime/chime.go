package chime

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/speaker"
	"github.com/gopxl/beep/wav"
)

//go:embed assets/start.wav
var defaultStartWav []byte

//go:embed assets/stop.wav
var defaultStopWav []byte

// Player manages audio chime playback.
type Player struct {
	startData []byte
	stopData  []byte
	enabled   bool
	initOnce  sync.Once
	initErr   error
}

// New creates a Player. If startPath/stopPath are empty, embedded defaults are used.
// If enabled is false, PlayStart/PlayStop are no-ops.
func New(startPath, stopPath string, enabled bool) (*Player, error) {
	p := &Player{
		startData: defaultStartWav,
		stopData:  defaultStopWav,
		enabled:   enabled,
	}

	if startPath != "" {
		data, err := os.ReadFile(startPath)
		if err != nil {
			return nil, fmt.Errorf("read start chime %s: %w", startPath, err)
		}
		p.startData = data
	}

	if stopPath != "" {
		data, err := os.ReadFile(stopPath)
		if err != nil {
			return nil, fmt.Errorf("read stop chime %s: %w", stopPath, err)
		}
		p.stopData = data
	}

	return p, nil
}

func (p *Player) initSpeaker(format beep.Format) {
	p.initOnce.Do(func() {
		p.initErr = speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
	})
}

func (p *Player) play(data []byte) {
	if !p.enabled || len(data) == 0 {
		return
	}

	go func() {
		reader := bytes.NewReader(data)
		streamer, format, err := wav.Decode(reader)
		if err != nil {
			return
		}
		defer streamer.Close()

		p.initSpeaker(format)
		if p.initErr != nil {
			return
		}

		done := make(chan struct{})
		speaker.Play(beep.Seq(streamer, beep.Callback(func() {
			close(done)
		})))
		<-done
	}()
}

// PlayStart plays the start recording chime (non-blocking).
func (p *Player) PlayStart() {
	p.play(p.startData)
}

// PlayStop plays the stop recording chime (non-blocking).
func (p *Player) PlayStop() {
	p.play(p.stopData)
}
