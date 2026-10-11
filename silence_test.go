package main

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

func TestScanPCM(t *testing.T) {
	tests := []struct {
		name       string
		channels   int
		loudFrames int
		silentTail int
		wantStart  string
		wantStatus string
	}{
		{name: "trailing silence", channels: 2, loudFrames: 250, silentTail: 750, wantStart: "00:00:00.250", wantStatus: "OK"},
		{name: "exactly two seconds before end", channels: 1, loudFrames: 1000, silentTail: 2000, wantStart: "00:00:01.000", wantStatus: "OK"},
		{name: "short trailing silence qualifies", channels: 1, loudFrames: 501, silentTail: 499, wantStart: "00:00:00", wantStatus: "OK"},
		{name: "entire recording silent", channels: 2, silentTail: 600, wantStart: "00:00:00.000", wantStatus: "EMPTY"},
		{name: "within two seconds before end", channels: 1, loudFrames: 1, silentTail: 1500, wantStart: "00:00:00.001", wantStatus: "OK"},
		{name: "silence starts early", channels: 1, loudFrames: 1, silentTail: 3001, wantStart: "00:00:00.001", wantStatus: "BROKEN"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pcm bytes.Buffer
			writeFrames(&pcm, tt.loudFrames, tt.channels, 0.1)
			writeFrames(&pcm, tt.silentTail, tt.channels, 0)
			got, err := scanPCM(&pcm, 1000, tt.channels)
			if err != nil {
				t.Fatal(err)
			}
			if got.start != tt.wantStart || got.status != tt.wantStatus {
				t.Fatalf("got {%q, %q}, want {%q, %q}", got.start, got.status, tt.wantStart, tt.wantStatus)
			}
		})
	}
}

func TestScanPCMRequiresEveryChannelToBeSilent(t *testing.T) {
	var pcm bytes.Buffer
	writeFrames(&pcm, 250, 2, 0.1)
	for range 750 {
		writeSample(&pcm, 0)
		writeSample(&pcm, 0.1)
	}
	got, err := scanPCM(&pcm, 1000, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got.start != "-" || got.status != "OK" {
		t.Fatalf("got {%q, %q}, want no trailing silence with OK status", got.start, got.status)
	}
}

func TestScanPCMRejectsIncompleteFrame(t *testing.T) {
	_, err := scanPCM(bytes.NewReader([]byte{0, 0, 0}), 1000, 1)
	if err == nil {
		t.Fatal("expected an incomplete frame error")
	}
}

func writeFrames(pcm *bytes.Buffer, frames, channels int, value float32) {
	for range frames {
		for range channels {
			writeSample(pcm, value)
		}
	}
}

func writeSample(pcm *bytes.Buffer, value float32) {
	var data [4]byte
	binary.LittleEndian.PutUint32(data[:], math.Float32bits(value))
	_, _ = pcm.Write(data[:])
}
