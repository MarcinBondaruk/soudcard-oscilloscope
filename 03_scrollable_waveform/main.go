package main

import (
	"errors"
	"image/color"
	"log/slog"
	"time"

	"github.com/gordonklaus/portaudio"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	windowWidth  = 1600
	windowHeight = 800
)

type Peak struct {
	min float32
	max float32
}

type Viewport struct {
	zoom           int
	scrollPosition int
	scrollSpeed    int
	peaks          []Peak
}

type Game struct {
	canvas   *ebiten.Image
	samples  []float32
	viewport *Viewport
}

func (g *Game) Update() error {
	zoomChanged := false
	scrollChanged := false

	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		if g.viewport.zoom < 32 {
			g.viewport.zoom *= 2
			g.viewport.scrollPosition *= 2
			zoomChanged = true
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		if g.viewport.zoom > 1 {
			g.viewport.zoom /= 2
			g.viewport.scrollPosition /= 2
			zoomChanged = true
		}
	}

	if ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
		maxScroll := (windowWidth * g.viewport.zoom) - windowWidth
		if g.viewport.scrollPosition < maxScroll {
			g.viewport.scrollPosition += g.viewport.scrollSpeed
			if g.viewport.scrollPosition > maxScroll {
				g.viewport.scrollPosition = maxScroll
			}

			scrollChanged = true
		}
	}

	if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
		if g.viewport.scrollPosition > 0 {
			g.viewport.scrollPosition -= g.viewport.scrollSpeed
			if g.viewport.scrollPosition < 0 {
				g.viewport.scrollPosition = 0
			}

			scrollChanged = true
		}
	}

	if zoomChanged {
		g.CalculatePeaks()
		g.CalculateCanvas()
	} else if scrollChanged {
		g.CalculateCanvas()
	}

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

func (g *Game) CalculatePeaks() {
	totalPixels := windowWidth * g.viewport.zoom
	g.viewport.peaks = make([]Peak, totalPixels)

	step := len(g.samples) / totalPixels
	for i := 0; i < totalPixels; i++ {
		start := i * step
		end := (i + 1) * step

		if end > len(g.samples) {
			end = len(g.samples)
		}

		if start >= end {
			break
		}

		min, max := MinMaxRange(start, end, g.samples)

		g.viewport.peaks[i] = Peak{min, max}
	}
}

func (g *Game) CalculateCanvas() {
	canvas := ebiten.NewImage(windowWidth, windowHeight)
	canvas.Fill(color.Black)

	centerY := float32(windowHeight / 2.0)

	for i := 0; i < windowWidth; i++ {
		position := g.viewport.scrollPosition + i

		if position >= len(g.viewport.peaks) {
			break
		}

		min := g.viewport.peaks[position].min*centerY + centerY
		max := g.viewport.peaks[position].max*centerY + centerY

		vector.StrokeLine(canvas, float32(i), min, float32(i), max, 1.0, color.RGBA{245, 40, 145, 255}, false)
	}

	g.canvas = canvas
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

	viewport := &Viewport{
		zoom:           1,
		scrollPosition: 0,
		scrollSpeed:    50,
		peaks:          nil,
	}
	game := &Game{
		canvas:   nil,
		samples:  samples,
		viewport: viewport,
	}

	game.CalculatePeaks()
	game.CalculateCanvas()

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
