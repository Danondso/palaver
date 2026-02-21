//go:build ignore

// This program generates the default start and stop chime WAV files.
// Run with: go run generate_chimes.go
package main

import (
	"log"
	"math"
	"os"

	"github.com/Danondso/palaver/internal/recorder"
)

func main() {
	sampleRate := 44100
	duration := 0.15 // 150ms

	// Start chime: ascending tone (A4 440Hz -> C5 523Hz)
	startSamples := generateChime(sampleRate, duration, 440, 523)
	startWav, err := recorder.EncodeWAV(startSamples, sampleRate)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("assets/start.wav", startWav, 0644); err != nil {
		log.Fatal(err)
	}

	// Stop chime: descending tone (C5 523Hz -> A4 440Hz)
	stopSamples := generateChime(sampleRate, duration, 523, 440)
	stopWav, err := recorder.EncodeWAV(stopSamples, sampleRate)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("assets/stop.wav", stopWav, 0644); err != nil {
		log.Fatal(err)
	}
}

func generateChime(sampleRate int, duration, startFreq, endFreq float64) []int16 {
	numSamples := int(float64(sampleRate) * duration)
	samples := make([]int16, numSamples)
	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(sampleRate)
		progress := float64(i) / float64(numSamples)
		freq := startFreq + (endFreq-startFreq)*progress
		// Apply envelope (fade in/out)
		envelope := math.Sin(math.Pi * progress)
		val := math.Sin(2*math.Pi*freq*t) * envelope * 16000
		samples[i] = int16(val)
	}
	return samples
}
