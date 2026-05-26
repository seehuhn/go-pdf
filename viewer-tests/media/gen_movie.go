// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

//go:build ignore

// gen_movie.go renders the test movie (751 frames at 25 fps, 1280x720,
// 16:9, showing the playback timestamp on each frame) together with an
// audio track that emits a short tick at the start of every second (30 s
// total), and muxes them to H.264 + AAC / MP4 via ffmpeg.  Run via
// `go generate` in this directory.
package main

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"os"
	"os/exec"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	width      = 1280
	height     = 720
	fps        = 25
	totalFrame = 751 // 0..750 inclusive, t = 0.00 .. 30.00 s
	fontSize   = 200
	outputName = "movie.mp4"

	audioRate = 44100
	// One second longer than the video (30.04 s); -shortest then trims the
	// muxed output to the video length, so the final 30.00 frame is kept and
	// there is a tick at every whole second through 30.
	durationSec = 31
	tickFreq    = 1000.0 // Hz
	tickLen     = 0.04   // seconds
)

func main() {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		fmt.Fprintln(os.Stderr, "ffmpeg not found on PATH; install it (e.g. `brew install ffmpeg`) and retry")
		os.Exit(1)
	}

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	audioPath, cleanup, err := writeTickWAV()
	if err != nil {
		return fmt.Errorf("audio: %w", err)
	}
	defer cleanup()

	face, err := makeFace()
	if err != nil {
		return fmt.Errorf("font: %w", err)
	}
	defer face.Close()

	cmd := exec.Command("ffmpeg",
		"-y",
		"-loglevel", "error",
		"-f", "rawvideo",
		"-pixel_format", "rgba",
		"-video_size", fmt.Sprintf("%dx%d", width, height),
		"-framerate", fmt.Sprintf("%d", fps),
		"-i", "-",
		"-i", audioPath,
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-preset", "medium",
		"-crf", "23",
		"-c:a", "aac",
		"-b:a", "96k",
		"-map", "0:v:0",
		"-map", "1:a:0",
		"-shortest",
		"-movflags", "+faststart",
		outputName,
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start: %w", err)
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	black := image.NewUniform(color.RGBA{R: 0, G: 0, B: 0, A: 255})
	drawer := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.RGBA{R: 255, G: 255, B: 255, A: 255}),
		Face: face,
	}
	metrics := face.Metrics()
	baselineY := (fixed.I(height) + metrics.Ascent - metrics.Descent) / 2

	for i := range totalFrame {
		draw.Draw(img, img.Bounds(), black, image.Point{}, draw.Src)

		text := fmt.Sprintf("%.2f", float64(i)/fps)
		advance := drawer.MeasureString(text)
		drawer.Dot = fixed.Point26_6{
			X: (fixed.I(width) - advance) / 2,
			Y: baselineY,
		}
		drawer.DrawString(text)

		if _, err := stdin.Write(img.Pix); err != nil {
			_ = stdin.Close()
			_ = cmd.Wait()
			return fmt.Errorf("frame %d: %w", i, err)
		}
	}

	if err := stdin.Close(); err != nil {
		_ = cmd.Wait()
		return fmt.Errorf("close stdin: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("ffmpeg: %w", err)
	}

	fi, err := os.Stat(outputName)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%d bytes, %d frames @ %d fps = %.2f s)\n",
		outputName, fi.Size(), totalFrame, fps, float64(totalFrame)/fps)
	return nil
}

// writeTickWAV synthesises a mono 16-bit PCM WAV: a short decaying 1 kHz
// tone at the start of each second.  It returns the temp-file path and a
// cleanup function.
func writeTickWAV() (string, func(), error) {
	f, err := os.CreateTemp("", "tick-*.wav")
	if err != nil {
		return "", func() {}, err
	}
	path := f.Name()
	cleanup := func() { _ = os.Remove(path) }

	total := audioRate * durationSec
	samples := make([]int16, total)
	tickSamples := int(tickLen * audioRate)
	for sec := range durationSec {
		start := sec * audioRate
		for i := 0; i < tickSamples && start+i < total; i++ {
			t := float64(i) / audioRate
			env := math.Exp(-t * 40) // fast decay
			v := 0.5 * env * math.Sin(2*math.Pi*tickFreq*t)
			samples[start+i] = int16(v * 32767)
		}
	}

	if err := writeWAVHeader(f, len(samples)); err != nil {
		f.Close()
		cleanup()
		return "", func() {}, err
	}
	if err := binary.Write(f, binary.LittleEndian, samples); err != nil {
		f.Close()
		cleanup()
		return "", func() {}, err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return path, cleanup, nil
}

// writeWAVHeader writes a canonical 44-byte RIFF/WAVE header for mono
// 16-bit PCM at audioRate, for sampleCount samples.
func writeWAVHeader(f *os.File, sampleCount int) error {
	const (
		channels      = 1
		bitsPerSample = 16
	)
	byteRate := audioRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8
	dataSize := sampleCount * bitsPerSample / 8

	w := func(v any) error { return binary.Write(f, binary.LittleEndian, v) }
	if _, err := f.WriteString("RIFF"); err != nil {
		return err
	}
	if err := w(uint32(36 + dataSize)); err != nil {
		return err
	}
	if _, err := f.WriteString("WAVEfmt "); err != nil {
		return err
	}
	for _, v := range []any{
		uint32(16),            // fmt chunk size
		uint16(1),             // PCM
		uint16(channels),      //
		uint32(audioRate),     //
		uint32(byteRate),      //
		uint16(blockAlign),    //
		uint16(bitsPerSample), //
	} {
		if err := w(v); err != nil {
			return err
		}
	}
	if _, err := f.WriteString("data"); err != nil {
		return err
	}
	return w(uint32(dataSize))
}

func makeFace() (font.Face, error) {
	fnt, err := opentype.Parse(gomonobold.TTF)
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(fnt, &opentype.FaceOptions{
		Size:    fontSize,
		DPI:     72,
		Hinting: font.HintingFull,
	})
}
