package main

import (
	"errors"
	"fmt"
	"image/color"
	"log/slog"
	"time"

	"github.com/gordonklaus/portaudio"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
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
		g.FillCanvas()
	} else if scrollChanged {
		g.FillCanvas()
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

func (g *Game) generateHorizontalAxis() {
	totalSeconds := float64(len(g.samples)) / float64(48000)
	pixelsPerSecond := float64(windowWidth*g.viewport.zoom) / totalSeconds

	gridStep := 1.0
	if g.viewport.zoom > 4 {
		gridStep = 0.5
	}
	if g.viewport.zoom > 16 {
		gridStep = 0.1
	}

	for s := 0.0; s <= totalSeconds; s += gridStep {
		x := float32(s*pixelsPerSecond) - float32(g.viewport.scrollPosition)
		if x >= 0 && x <= windowWidth {
			vector.StrokeLine(g.canvas, x, 0, x, windowHeight, 1.0, color.RGBA{50, 50, 50, 255}, false)
			ebitenutil.DebugPrintAt(g.canvas, fmt.Sprintf("%.1fs", s), int(x)+5, 10)
		}
	}
}

func (g *Game) generateVertialAxis(midPosition float32) {
	magnitudes := []float32{-1.0, -0.5, 0.0, 0.5, 1.0}
	for _, mag := range magnitudes {
		y := midPosition - (mag * midPosition)

		vector.StrokeLine(g.canvas, 0, y, float32(windowWidth), y, 1.0, color.RGBA{50, 50, 50, 255}, false)

		ebitenutil.DebugPrintAt(g.canvas, fmt.Sprintf("%.1f", mag), 5, int(y)-15)
	}
}

func (g *Game) generateWaveform(midPosition float32) {
	for i := 0; i < windowWidth; i++ {
		position := g.viewport.scrollPosition + i

		if position >= len(g.viewport.peaks) {
			break
		}

		min := g.viewport.peaks[position].min*midPosition + midPosition
		max := g.viewport.peaks[position].max*midPosition + midPosition

		vector.StrokeLine(g.canvas, float32(i), min, float32(i), max, 1.0, color.RGBA{245, 40, 145, 255}, false)
	}
}

func (g *Game) FillCanvas() {
	centerY := float32(windowHeight / 2.0)

	g.canvas.Fill(color.Black)
	g.generateHorizontalAxis()
	g.generateVertialAxis(centerY)
	g.generateWaveform(centerY)
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

	viewport := &Viewport{
		zoom:           1,
		scrollPosition: 0,
		scrollSpeed:    50,
		peaks:          nil,
	}
	game := &Game{
		canvas:   canvas,
		samples:  samples,
		viewport: viewport,
	}

	game.CalculatePeaks()
	game.FillCanvas()

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
