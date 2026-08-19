package main

import (
	"errors"
	"image/color"
	"log/slog"
	"time"

	"github.com/gordonklaus/portaudio"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	windowWidth  = 1600
	windowHeight = 800
)

type Game struct {
	canvas         *ebiten.Image
	samples        []float32
	zoom           uint
	scrollPosition int
}

func (g *Game) Update() error {
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	opts := &ebiten.DrawImageOptions{}
	screen.DrawImage(g.canvas, opts)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return windowWidth, windowHeight
}

func MinMaxRange(start, end int, samples []float32) (float32, float32) {
	var min, max float32

	for i := start; i < end; i++ {
		if min > samples[i] {
			min = samples[i]
		}

		if max < samples[i] {
			max = samples[i]
		}
	}

	return min, max
}

func RecordWaveform(sampleRate int, recordingDuration time.Duration) ([]float32, error) {
	inputBufferSize := int(recordingDuration.Seconds()) * sampleRate
	inputBuffer := make([]float32, inputBufferSize)

	err := portaudio.Initialize()
	if err != nil {
		return nil, errors.Join(errors.New("failed to initialize portaudio"), err)
	}
	defer func() {
		err := portaudio.Terminate()
		if err != nil {
			slog.Error("portaudio terminate failed", "err", err)
		}
	}()

	stream, err := portaudio.OpenDefaultStream(1, 0, float64(sampleRate), inputBufferSize, inputBuffer)
	if err != nil {
		return nil, errors.Join(errors.New("failed to open the stream"), err)
	}
	defer func() {
		err := stream.Close()
		if err != nil {
			slog.Error("portaudio stream close failed", "err", err)
		}
	}()

	err = stream.Start()
	if err != nil {
		return nil, errors.Join(errors.New("failed to start the stream"), err)
	}
	defer func() {
		err := stream.Stop()
		if err != nil {
			slog.Error("portaudio stream stop failed", "err", err)
		}
	}()

	err = stream.Read()
	if err != nil {
		return nil, errors.Join(errors.New("failed to read stream"), err)
	}

	return inputBuffer, nil
}

func NewGame(samples []float32) (*Game, error) {
	if len(samples) < 2 {
		return nil, errors.New("there must be at least 2 samples")
	}

	canvas := ebiten.NewImage(windowWidth, windowHeight)

	canvas.Fill(color.Black)

	centerY := float32(windowHeight / 2.0)
	step := len(samples) / windowWidth

	for i := 0; i < windowWidth; i++ {
		startX := i * step
		endX := startX + step

		if endX > len(samples) {
			endX = len(samples)
		}

		minY, maxY := MinMaxRange(startX, endX, samples)

		minY = minY*centerY + centerY
		maxY = maxY*centerY + centerY

		vector.StrokeLine(canvas, float32(i), minY, float32(i), maxY, 2.0, color.RGBA{245, 40, 145, 255}, false)
	}

	game := &Game{
		canvas:         canvas,
		samples:        samples,
		zoom:           0,
		scrollPosition: 0,
	}

	return game, nil
}

func DisplayWaveform(samples []float32) error {
	game, err := NewGame(samples)
	if err != nil {
		return err
	}
	ebiten.SetWindowSize(windowWidth, windowHeight)
	ebiten.SetWindowTitle("waveform")

	err = ebiten.RunGame(game)
	if err != nil {
		return errors.Join(errors.New("error running ebitengine"), err)
	}

	return nil
}

func main() {
	samples, err := RecordWaveform(48*1000, time.Second*5)
	if err != nil {
		slog.Error("error during recording", "err", err)
		return
	}

	err = DisplayWaveform(samples)
	if err != nil {
		slog.Error("error displaying", "err", err)
		return
	}
}
