package main

import (
	"errors"
	"fmt"
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
	samples []float32
}

func (g *Game) Update() error {
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.Black)

	centerY := float32(windowHeight / 2.0)
	stepX := windowWidth / float32(len(g.samples)-1)

	for i := range len(g.samples) - 1 {
		x0 := float32(i) * stepX
		y0 := g.samples[i]*centerY + centerY

		x1 := float32(i) * stepX
		y1 := g.samples[i+1]*centerY + centerY
		vector.StrokeLine(screen, x0, y0, x1, y1, 1.0, color.RGBA{90, 252, 3, 1}, true)
	}

	fmt.Println("dupa")
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return windowWidth, windowHeight
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

func DisplayWaveform(samples []float32) error {
	game := &Game{
		samples: samples,
	}
	ebiten.SetWindowSize(windowWidth, windowHeight)
	ebiten.SetWindowTitle("waveform")

	err := ebiten.RunGame(game)
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
