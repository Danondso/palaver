package recorder

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/gordonklaus/portaudio"
	resampling "github.com/tphakala/go-audio-resampling"
)

// Recorder captures audio from the default input device.
type Recorder struct {
	mu             sync.Mutex
	stream         *portaudio.Stream
	buf            []int16
	recording      bool
	done           chan struct{} // closed when readLoop should exit
	loopDone       chan struct{} // closed when readLoop has exited
	nativeSR       float64
	nativeChannels int
	targetSR       int
	maxDurationSec int
	startTime      time.Time
	truncated      bool
	audioLevel     uint64 // atomic float64 bits; RMS of last chunk (0.0–1.0)
}

// New creates a Recorder. Call portaudio.Initialize() before using this.
func New(targetSampleRate, maxDurationSec int) (*Recorder, error) {
	defIn, err := portaudio.DefaultInputDevice()
	if err != nil {
		return nil, fmt.Errorf("default input device: %w", err)
	}

	return &Recorder{
		nativeSR:       defIn.DefaultSampleRate,
		nativeChannels: defIn.MaxInputChannels,
		targetSR:       targetSampleRate,
		maxDurationSec: maxDurationSec,
	}, nil
}

// Start begins capturing audio. Returns an error if already recording.
func (r *Recorder) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.recording {
		return fmt.Errorf("already recording")
	}

	r.buf = nil
	r.truncated = false
	r.startTime = time.Now()

	channels := r.nativeChannels
	if channels > 2 {
		channels = 2
	}
	if channels < 1 {
		channels = 1
	}

	framesPerBuffer := int(r.nativeSR / 10) // ~100ms chunks
	inputBuf := make([]int16, framesPerBuffer*channels)

	stream, err := portaudio.OpenDefaultStream(channels, 0, r.nativeSR, framesPerBuffer, &inputBuf)
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}

	if err := stream.Start(); err != nil {
		stream.Close()
		return fmt.Errorf("start stream: %w", err)
	}

	r.stream = stream
	r.recording = true
	r.done = make(chan struct{})
	r.loopDone = make(chan struct{})

	go r.readLoop(stream, inputBuf, channels, r.done, r.loopDone)

	return nil
}

func (r *Recorder) readLoop(stream *portaudio.Stream, inputBuf []int16, channels int, done, loopDone chan struct{}) {
	defer close(loopDone)
	maxSamples := int(r.nativeSR) * r.maxDurationSec

	for {
		select {
		case <-done:
			return
		default:
		}

		err := stream.Read()
		if err != nil {
			return
		}

		r.mu.Lock()
		if !r.recording {
			r.mu.Unlock()
			return
		}

		if channels == 2 {
			for i := 0; i < len(inputBuf); i += 2 {
				mono := (int32(inputBuf[i]) + int32(inputBuf[i+1])) / 2
				r.buf = append(r.buf, int16(mono))
			}
		} else {
			r.buf = append(r.buf, inputBuf...)
		}

		atomic.StoreUint64(&r.audioLevel, math.Float64bits(computeRMS(inputBuf, channels)))

		if len(r.buf) >= maxSamples {
			r.truncated = true
			r.recording = false
			r.mu.Unlock()
			return
		}
		r.mu.Unlock()
	}
}

// Stop stops recording and returns the WAV-encoded audio data.
// The second return value indicates if recording was truncated due to max duration.
func (r *Recorder) Stop() ([]byte, bool, error) {
	r.mu.Lock()
	wasRecording := r.recording
	wasTruncated := r.truncated
	r.recording = false
	done := r.done
	loopDone := r.loopDone
	r.mu.Unlock()

	if !wasRecording && !wasTruncated {
		return nil, false, fmt.Errorf("not recording")
	}

	// Signal readLoop to stop, then wait for it to exit before closing the stream.
	// This prevents a segfault from stream.Read() racing with stream.Close().
	if done != nil {
		close(done)
	}
	if loopDone != nil {
		<-loopDone
	}

	if r.stream != nil {
		r.stream.Stop()
		r.stream.Close()
		r.stream = nil
	}

	atomic.StoreUint64(&r.audioLevel, math.Float64bits(0))

	r.mu.Lock()
	samples := make([]int16, len(r.buf))
	copy(samples, r.buf)
	truncated := r.truncated
	nativeSR := r.nativeSR
	targetSR := r.targetSR
	r.mu.Unlock()

	if len(samples) == 0 {
		return nil, truncated, fmt.Errorf("no audio captured")
	}

	// Resample using polyphase FIR if needed
	if int(nativeSR) != targetSR {
		resampled, err := Resample(samples, nativeSR, float64(targetSR))
		if err != nil {
			return nil, truncated, fmt.Errorf("resample: %w", err)
		}
		samples = resampled
	}

	wavData, err := EncodeWAV(samples, targetSR)
	if err != nil {
		return nil, truncated, fmt.Errorf("encode wav: %w", err)
	}

	return wavData, truncated, nil
}

// IsRecording returns whether the recorder is currently capturing.
func (r *Recorder) IsRecording() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.recording
}

// AudioLevel returns the RMS amplitude of the most recently captured chunk,
// in the range [0.0, 1.0]. Safe to call from any goroutine.
func (r *Recorder) AudioLevel() float64 {
	return math.Float64frombits(atomic.LoadUint64(&r.audioLevel))
}

// computeRMS computes the root-mean-square of int16 samples normalized to [0.0, 1.0].
// For stereo input, averages the two channels before computing.
func computeRMS(buf []int16, channels int) float64 {
	if len(buf) == 0 {
		return 0
	}
	var sum float64
	n := len(buf) / channels
	for i := 0; i < len(buf); i += channels {
		var v float64
		if channels == 2 {
			v = float64(int32(buf[i])+int32(buf[i+1])) / 2.0
		} else {
			v = float64(buf[i])
		}
		v /= 32768.0
		sum += v * v
	}
	return math.Sqrt(sum / float64(n))
}

// MicAvailable returns true if PortAudio can find a default input device.
// portaudio.Initialize() must have been called before using this.
func MicAvailable() bool {
	dev, err := portaudio.DefaultInputDevice()
	return err == nil && dev != nil && dev.MaxInputChannels > 0
}

// MicName returns the name of the default input device, or "" if unavailable.
func MicName() string {
	dev, err := portaudio.DefaultInputDevice()
	if err != nil || dev == nil {
		return ""
	}
	return dev.Name
}

// Resample converts PCM int16 samples from inputRate to outputRate using
// polyphase FIR filtering with Kaiser window (via go-audio-resampling).
// Uses QualityLow preset which provides 16-bit precision, suitable for speech.
func Resample(samples []int16, inputRate, outputRate float64) ([]int16, error) {
	if inputRate == outputRate || len(samples) == 0 {
		return samples, nil
	}

	// Convert int16 to float64 (normalized to -1.0..1.0)
	floats := make([]float64, len(samples))
	for i, s := range samples {
		floats[i] = float64(s) / 32768.0
	}

	resampled, err := resampling.ResampleMono(floats, inputRate, outputRate, resampling.QualityLow)
	if err != nil {
		return nil, fmt.Errorf("resample mono: %w", err)
	}

	// Convert back to int16
	out := make([]int16, len(resampled))
	for i, f := range resampled {
		v := f * 32768.0
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		out[i] = int16(math.Round(v))
	}

	return out, nil
}

// DownmixStereoToMono converts interleaved stereo int16 samples to mono
// by averaging left and right channels.
func DownmixStereoToMono(stereo []int16) []int16 {
	mono := make([]int16, len(stereo)/2)
	for i := 0; i < len(stereo); i += 2 {
		mono[i/2] = int16((int32(stereo[i]) + int32(stereo[i+1])) / 2)
	}
	return mono
}

// writeSeeker is an in-memory io.WriteSeeker for WAV encoding.
type writeSeeker struct {
	buf []byte
	pos int
}

func (ws *writeSeeker) Write(p []byte) (int, error) {
	end := ws.pos + len(p)
	if end > len(ws.buf) {
		ws.buf = append(ws.buf, make([]byte, end-len(ws.buf))...)
	}
	copy(ws.buf[ws.pos:], p)
	ws.pos = end
	return len(p), nil
}

func (ws *writeSeeker) Seek(offset int64, whence int) (int64, error) {
	var newPos int
	switch whence {
	case 0: // io.SeekStart
		newPos = int(offset)
	case 1: // io.SeekCurrent
		newPos = ws.pos + int(offset)
	case 2: // io.SeekEnd
		newPos = len(ws.buf) + int(offset)
	default:
		return 0, fmt.Errorf("invalid whence: %d", whence)
	}
	if newPos < 0 || newPos > len(ws.buf) {
		return 0, fmt.Errorf("seek position %d out of bounds [0, %d]", newPos, len(ws.buf))
	}
	ws.pos = newPos
	return int64(ws.pos), nil
}

// EncodeWAV encodes mono int16 PCM samples to WAV format in memory.
func EncodeWAV(samples []int16, sampleRate int) ([]byte, error) {
	ws := &writeSeeker{}

	intBuf := &audio.IntBuffer{
		Data: make([]int, len(samples)),
		Format: &audio.Format{
			SampleRate:  sampleRate,
			NumChannels: 1,
		},
		SourceBitDepth: 16,
	}
	for i, s := range samples {
		intBuf.Data[i] = int(s)
	}

	enc := wav.NewEncoder(ws, sampleRate, 16, 1, 1)
	if err := enc.Write(intBuf); err != nil {
		return nil, fmt.Errorf("write wav: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("close wav encoder: %w", err)
	}

	return ws.buf, nil
}

// DecodeWAV reads a WAV file from bytes and returns the samples and sample rate.
func DecodeWAV(data []byte) ([]int16, int, error) {
	reader := bytes.NewReader(data)
	dec := wav.NewDecoder(reader)
	if !dec.IsValidFile() {
		return nil, 0, fmt.Errorf("invalid WAV file")
	}

	pcmBuf, err := dec.FullPCMBuffer()
	if err != nil {
		return nil, 0, fmt.Errorf("decode wav: %w", err)
	}

	samples := make([]int16, len(pcmBuf.Data))
	for i, v := range pcmBuf.Data {
		samples[i] = int16(v)
	}

	return samples, int(dec.SampleRate), nil
}

// ValidateWAVHeader reads minimal WAV header info from data.
func ValidateWAVHeader(data []byte) (sampleRate int, channels int, bitDepth int, err error) {
	if len(data) < 44 {
		return 0, 0, 0, fmt.Errorf("data too short for WAV header")
	}

	r := bytes.NewReader(data)

	// read wraps binary.Read to capture the first error.
	var firstErr error
	read := func(v interface{}) {
		if firstErr != nil {
			return
		}
		firstErr = binary.Read(r, binary.LittleEndian, v)
	}

	var riffID [4]byte
	read(&riffID)
	if firstErr != nil {
		return 0, 0, 0, fmt.Errorf("read RIFF header: %w", firstErr)
	}
	if string(riffID[:]) != "RIFF" {
		return 0, 0, 0, fmt.Errorf("not a RIFF file")
	}

	var fileSize uint32
	read(&fileSize)

	var waveID [4]byte
	read(&waveID)
	if firstErr != nil {
		return 0, 0, 0, fmt.Errorf("read WAVE header: %w", firstErr)
	}
	if string(waveID[:]) != "WAVE" {
		return 0, 0, 0, fmt.Errorf("not a WAVE file")
	}

	var fmtID [4]byte
	read(&fmtID)

	var fmtSize uint32
	read(&fmtSize)

	var audioFormat uint16
	read(&audioFormat)

	var numChannels uint16
	read(&numChannels)

	var sr uint32
	read(&sr)

	var byteRate uint32
	var blockAlign uint16
	read(&byteRate)
	read(&blockAlign)

	var bitsPerSample uint16
	read(&bitsPerSample)

	if firstErr != nil {
		return 0, 0, 0, fmt.Errorf("read WAV format: %w", firstErr)
	}

	return int(sr), int(numChannels), int(bitsPerSample), nil
}
