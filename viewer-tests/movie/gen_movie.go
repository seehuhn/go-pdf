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

// gen_movie.go renders the test movie (1501 frames at 25 fps, 320x240,
// showing the playback timestamp as text on each frame) and encodes it
// to H.264/MP4 via ffmpeg.  Run via `go generate` in this directory.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"os/exec"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	width      = 320
	height     = 240
	fps        = 25
	totalFrame = 1501 // 0..1500 inclusive, t = 0.00 .. 60.00 s
	fontSize   = 64
	outputName = "movie.mp4"
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
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-preset", "medium",
		"-crf", "23",
		"-an",
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

		text := fmt.Sprintf("%.2f", float64(i)*0.04)
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
	fmt.Fprintf(os.Stderr, "wrote %s (%d bytes, %d frames)\n", outputName, fi.Size(), totalFrame)
	return nil
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
