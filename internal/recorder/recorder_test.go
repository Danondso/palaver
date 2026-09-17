package recorder

import (
	"math"
	"testing"
)

func TestResampleOutputLength(t *testing.T) {
	// 1 second of 48kHz audio = 48000 samples
	// Resampled to 16kHz should be ~16000 samples
	input := make([]int16, 48000)
	// Fill with a 440Hz sine wave
	for i := range input {
		input[i] = int16(10000 * math.Sin(2*math.Pi*440*float64(i)/48000))
	}

	output, err := Resample(input, 48000, 16000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Allow 1% tolerance on output length
	expectedLen := 16000
	tolerance := expectedLen / 100
	if len(output) < expectedLen-tolerance || len(output) > expectedLen+tolerance {
		t.Errorf("expected ~%d samples, got %d", expectedLen, len(output))
	}
}

func TestResampleSameRate(t *testing.T) {
	input := []int16{100, 200, 300, 400, 500}
	output, err := Resample(input, 16000, 16000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(output) != len(input) {
		t.Errorf("expected %d samples, got %d", len(input), len(output))
	}
}

func TestResampleEmpty(t *testing.T) {
	output, err := Resample([]int16{}, 48000, 16000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(output) != 0 {
		t.Errorf("expected 0 samples, got %d", len(output))
	}
}

func TestResample44100to16000(t *testing.T) {
	// 1 second at 44100 Hz
	input := make([]int16, 44100)
	for i := range input {
		input[i] = int16(10000 * math.Sin(2*math.Pi*440*float64(i)/44100))
	}

	output, err := Resample(input, 44100, 16000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedLen := 16000
	tolerance := expectedLen / 100
	if len(output) < expectedLen-tolerance || len(output) > expectedLen+tolerance {
		t.Errorf("expected ~%d samples, got %d", expectedLen, len(output))
	}
}

func TestDownmixStereoToMono(t *testing.T) {
	stereo := []int16{100, 200, 300, 400, 500, 600}
	mono := DownmixStereoToMono(stereo)

	if len(mono) != 3 {
		t.Fatalf("expected 3 mono samples, got %d", len(mono))
	}
	// (100+200)/2 = 150, (300+400)/2 = 350, (500+600)/2 = 550
	expected := []int16{150, 350, 550}
	for i, v := range mono {
		if v != expected[i] {
			t.Errorf("sample %d: expected %d, got %d", i, expected[i], v)
		}
	}
}

func TestEncodeDecodeWAV(t *testing.T) {
	// Create a short sine wave
	sampleRate := 16000
	samples := make([]int16, sampleRate) // 1 second
	for i := range samples {
		samples[i] = int16(10000 * math.Sin(2*math.Pi*440*float64(i)/float64(sampleRate)))
	}

	wavData, err := EncodeWAV(samples, sampleRate)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	if len(wavData) == 0 {
		t.Fatal("encoded WAV is empty")
	}

	// Validate WAV header
	sr, ch, bd, err := ValidateWAVHeader(wavData)
	if err != nil {
		t.Fatalf("validate header error: %v", err)
	}
	if sr != sampleRate {
		t.Errorf("expected sample rate %d, got %d", sampleRate, sr)
	}
	if ch != 1 {
		t.Errorf("expected 1 channel, got %d", ch)
	}
	if bd != 16 {
		t.Errorf("expected 16-bit, got %d", bd)
	}

	// Decode and verify
	decoded, decodedSR, err := DecodeWAV(wavData)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decodedSR != sampleRate {
		t.Errorf("decoded sample rate: expected %d, got %d", sampleRate, decodedSR)
	}
	if len(decoded) != len(samples) {
		t.Errorf("decoded length: expected %d, got %d", len(samples), len(decoded))
	}

	// Verify samples match (exact for 16-bit PCM)
	for i := 0; i < len(samples) && i < len(decoded); i++ {
		if samples[i] != decoded[i] {
			t.Errorf("sample %d: expected %d, got %d", i, samples[i], decoded[i])
			break
		}
	}
}

func TestValidateWAVHeaderTooShort(t *testing.T) {
	_, _, _, err := ValidateWAVHeader([]byte{1, 2, 3})
	if err == nil {
		t.Error("expected error for short data")
	}
}

func TestDurationCapSamples(t *testing.T) {
	if got := durationCapSamples(16000, 60); got != 16000*60 {
		t.Errorf("expected %d, got %d", 16000*60, got)
	}
	if got := durationCapSamples(48000, 0); got != 0 {
		t.Errorf("expected 0 for unlimited, got %d", got)
	}
	if got := durationCapSamples(48000, -1); got != 0 {
		t.Errorf("expected 0 for negative duration, got %d", got)
	}
}

func TestTakeWAVEncodesAndClearsBuffer(t *testing.T) {
	sampleRate := 16000
	samples := make([]int16, sampleRate/10) // 100ms
	for i := range samples {
		samples[i] = int16(10000 * math.Sin(2*math.Pi*440*float64(i)/float64(sampleRate)))
	}

	r := &Recorder{
		recording: true,
		buf:       samples,
		nativeSR:  float64(sampleRate),
		targetSR:  sampleRate,
	}

	wavData, err := r.TakeWAV()
	if err != nil {
		t.Fatalf("TakeWAV: %v", err)
	}
	if len(r.buf) != 0 {
		t.Errorf("expected buffer cleared, got %d samples", len(r.buf))
	}
	if !r.recording {
		t.Error("expected recording to stay true")
	}

	sr, ch, bd, err := ValidateWAVHeader(wavData)
	if err != nil {
		t.Fatalf("validate header: %v", err)
	}
	if sr != sampleRate || ch != 1 || bd != 16 {
		t.Errorf("unexpected wav header sr=%d ch=%d bd=%d", sr, ch, bd)
	}

	// Second take with empty buffer should error.
	if _, err := r.TakeWAV(); err == nil {
		t.Error("expected error on empty buffer")
	}
}

func TestTakeWAVNotRecording(t *testing.T) {
	r := &Recorder{buf: []int16{1, 2, 3}, nativeSR: 16000, targetSR: 16000}
	if _, err := r.TakeWAV(); err == nil {
		t.Error("expected error when not recording")
	}
}

func TestEncodeSamplesEmpty(t *testing.T) {
	if _, err := encodeSamples(nil, 16000, 16000); err == nil {
		t.Error("expected error for empty samples")
	}
}
