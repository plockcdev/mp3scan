package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const silenceThreshold = 0.00005623413 // -85 dBFS on normalized PCM.
const maxFFmpegErrorBytes = 64 * 1024

type silenceResult struct {
	start       string
	status      string
	duration    string
	lowestLevel string
	title       string
}

type audioStream struct {
	SampleRate string `json:"sample_rate"`
	Channels   int    `json:"channels"`
}

type probeResult struct {
	Streams []audioStream `json:"streams"`
	Format  struct {
		Tags struct {
			Title string `json:"title"`
		} `json:"tags"`
	} `json:"format"`
}

type boundedBuffer struct {
	bytes.Buffer
	truncated bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	inputLength := len(p)
	remaining := maxFFmpegErrorBytes - b.Len()
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		if _, err := b.Buffer.Write(p); err != nil {
			return 0, err
		}
	}
	if inputLength > remaining {
		b.truncated = true
	}
	return inputLength, nil
}

func trailingSilence(path, ffprobePath, ffmpegPath string) (silenceResult, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return silenceResult{}, fmt.Errorf("resolve audio path: %w", err)
	}
	sampleRate, channels, title, err := probeAudio(absolutePath, ffprobePath)
	if err != nil {
		return silenceResult{}, err
	}

	cmd := exec.Command(ffmpegPath,
		"-nostdin", "-hide_banner", "-loglevel", "error",
		"-i", absolutePath, "-map", "0:a:0", "-vn", "-sn", "-dn",
		"-ar", strconv.Itoa(sampleRate), "-ac", strconv.Itoa(channels),
		"-c:a", "pcm_f32le", "-f", "f32le", "pipe:1",
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return silenceResult{}, fmt.Errorf("prepare ffmpeg output: %w", err)
	}
	var stderr boundedBuffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return silenceResult{}, fmt.Errorf("start ffmpeg: %w", err)
	}
	silence, readErr := scanPCM(stdout, sampleRate, channels)
	waitErr := cmd.Wait()
	if waitErr != nil {
		message := strings.TrimSpace(stderr.String())
		if stderr.truncated {
			message += "\n[ffmpeg error output truncated]"
		}
		if message != "" {
			return silenceResult{}, fmt.Errorf("ffmpeg decode: %w: %s", waitErr, message)
		}
		return silenceResult{}, fmt.Errorf("ffmpeg decode: %w", waitErr)
	}
	if readErr != nil {
		return silenceResult{}, fmt.Errorf("read decoded audio: %w", readErr)
	}
	silence.title = title
	return silence, nil
}

func probeAudio(path, ffprobePath string) (sampleRate, channels int, title string, err error) {
	output, err := exec.Command(ffprobePath,
		"-v", "error", "-select_streams", "a:0", "-show_streams",
		"-show_entries", "stream=sample_rate,channels:format_tags=title", "-show_format", "-of", "json", path,
	).CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message != "" {
			return 0, 0, "", fmt.Errorf("ffprobe: %w: %s", err, message)
		}
		return 0, 0, "", fmt.Errorf("ffprobe: %w", err)
	}
	var result probeResult
	if err := json.Unmarshal(output, &result); err != nil {
		return 0, 0, "", fmt.Errorf("parse ffprobe output: %w", err)
	}
	if len(result.Streams) != 1 {
		return 0, 0, "", errors.New("first audio stream not found")
	}
	sampleRate, err = strconv.Atoi(result.Streams[0].SampleRate)
	if err != nil {
		return 0, 0, "", fmt.Errorf("invalid audio sample rate %q: %w", result.Streams[0].SampleRate, err)
	}
	channels = result.Streams[0].Channels
	if sampleRate <= 0 || sampleRate > 1_000_000 || channels <= 0 || channels > 1024 {
		return 0, 0, "", fmt.Errorf("invalid audio stream properties: %d Hz, %d channels", sampleRate, channels)
	}
	return sampleRate, channels, result.Format.Tags.Title, nil
}

func scanPCM(r io.Reader, sampleRate, channels int) (silenceResult, error) {
	frameBytes := channels * 4
	buffer := make([]byte, frameBytes*4096)
	var totalFrames int64
	trailingFrames := int64(0)
	lowestAmplitude := math.Inf(1)
	for {
		n, readErr := io.ReadFull(r, buffer)
		if n%frameBytes != 0 {
			return silenceResult{}, errors.New("decoded PCM ended within a sample frame")
		}
		for offset := 0; offset < n; offset += frameBytes {
			silent := true
			frameLowestAmplitude := math.Inf(1)
			for channel := range channels {
				sampleOffset := offset + channel*4
				bits := binary.LittleEndian.Uint32(buffer[sampleOffset : sampleOffset+4])
				value := float64(math.Float32frombits(bits))
				amplitude := math.Abs(value)
				if math.IsNaN(value) || math.IsInf(value, 0) || amplitude > silenceThreshold {
					silent = false
				} else if amplitude > 0 && amplitude < frameLowestAmplitude {
					frameLowestAmplitude = amplitude
				}
			}
			if silent {
				trailingFrames++
				if frameLowestAmplitude < lowestAmplitude {
					lowestAmplitude = frameLowestAmplitude
				}
			} else {
				trailingFrames = 0
				lowestAmplitude = math.Inf(1)
			}
			totalFrames++
		}
		if readErr == nil {
			continue
		}
		if errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrUnexpectedEOF) {
			break
		}
		return silenceResult{}, readErr
	}
	if totalFrames == 0 {
		return silenceResult{}, errors.New("audio stream contains no decoded samples")
	}
	startFrame := totalFrames - trailingFrames
	if trailingFrames == 0 {
		return silenceResult{
			start:       "-",
			status:      "OK",
			duration:    "-",
			lowestLevel: "-",
		}, nil
	}
	status := "OK"
	duration := fmt.Sprintf("%.1f", math.Round(float64(trailingFrames)*10/float64(sampleRate))/10)
	if startFrame == 0 {
		status = "EMPTY"
		duration = "-"
	} else if trailingFrames > 3*int64(sampleRate) {
		status = "BROKEN"
	}
	wholeSeconds := startFrame / int64(sampleRate)
	hours := wholeSeconds / 3600
	minutes := (wholeSeconds / 60) % 60
	seconds := wholeSeconds % 60
	lowestLevel := "-"
	if !math.IsInf(lowestAmplitude, 1) {
		lowestLevel = fmt.Sprintf("%.2f", 20*math.Log10(lowestAmplitude))
	}
	return silenceResult{
		start:       fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds),
		status:      status,
		duration:    duration,
		lowestLevel: lowestLevel,
	}, nil
}
