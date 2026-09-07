package main

import (
	"errors"
	"fmt"
	"image/color"
	"log/slog"
	"sync"

	"github.com/gordonklaus/portaudio"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	windowWidth    = 1600
	windowHeight   = 800
	sampleRate     = 48000
	framesPerRead  = 1024
	ringBufferSize = 1 << 17 // 131072 samples, ~2.7s at 48kHz
	ringBufferMask = ringBufferSize - 1
)

type Peak struct {
	min float32
	max float32
}

type Viewport struct {
	timeBase int
	peaks    []Peak
}

type TriggerConfig struct {
	level   float32
	enabled bool
}

type Game struct {
	canvas  *ebiten.Image
	viewport *Viewport
	trigger  *TriggerConfig

	mu          sync.Mutex
	ringBuffer  [ringBufferSize]float32
	writePos    int
	displayBuffer []float32

	running bool
	wg      sync.WaitGroup
}

func (g *Game) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}

	timeBaseChanged := false

	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		if g.viewport.timeBase > 800 {
			g.viewport.timeBase /= 2
			timeBaseChanged = true
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		if g.viewport.timeBase < 48000 {
			g.viewport.timeBase *= 2
			timeBaseChanged = true
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyT) {
		g.trigger.enabled = !g.trigger.enabled
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyW) {
		g.trigger.level += 0.05
		if g.trigger.level > 1.0 {
			g.trigger.level = 1.0
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyS) {
		g.trigger.level -= 0.05
		if g.trigger.level < -1.0 {
			g.trigger.level = -1.0
		}
	}

	_ = timeBaseChanged

	g.SnapshotBuffer()

	triggerOffset := 0
	if g.trigger.enabled {
		triggerOffset = FindTrigger(g.displayBuffer, g.trigger.level)
	}

	window := g.displayBuffer[triggerOffset:]
	if len(window) > g.viewport.timeBase {
		window = window[:g.viewport.timeBase]
	}

	g.CalculatePeaks(window)
	g.FillCanvas()

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	opts := &ebiten.DrawImageOptions{}
	screen.DrawImage(g.canvas, opts)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return windowWidth, windowHeight
}

func (g *Game) SnapshotBuffer() {
	needed := g.viewport.timeBase * 2
	if needed > ringBufferSize {
		needed = ringBufferSize
	}
	g.displayBuffer = make([]float32, needed)

	g.mu.Lock()
	start := g.writePos - needed
	for i := 0; i < needed; i++ {
		g.displayBuffer[i] = g.ringBuffer[(start+i)&ringBufferMask]
	}
	g.mu.Unlock()
}

func FindTrigger(samples []float32, level float32) int {
	for i := 1; i < len(samples); i++ {
		if samples[i-1] <= level && samples[i] > level {
			return i
		}
	}
	return 0
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

func (g *Game) CalculatePeaks(samples []float32) {
	g.viewport.peaks = make([]Peak, windowWidth)

	if len(samples) == 0 {
		return
	}

	if len(samples) <= windowWidth {
		for i := 0; i < len(samples); i++ {
			pixelIndex := i * windowWidth / len(samples)
			if pixelIndex >= windowWidth {
				pixelIndex = windowWidth - 1
			}
			s := samples[i]
			p := g.viewport.peaks[pixelIndex]
			if s < p.min {
				p.min = s
			}
			if s > p.max {
				p.max = s
			}
			g.viewport.peaks[pixelIndex] = p
		}
		return
	}

	step := len(samples) / windowWidth
	for i := 0; i < windowWidth; i++ {
		start := i * step
		end := (i + 1) * step

		if end > len(samples) {
			end = len(samples)
		}

		if start >= end {
			break
		}

		min, max := MinMaxRange(start, end, samples)
		g.viewport.peaks[i] = Peak{min, max}
	}
}

func (g *Game) generateHorizontalAxis() {
	totalSeconds := float64(g.viewport.timeBase) / float64(sampleRate)
	pixelsPerSecond := float64(windowWidth) / totalSeconds

	gridStep := selectGridStep(totalSeconds)

	for s := 0.0; s <= totalSeconds; s += gridStep {
		x := float32(s * pixelsPerSecond)
		if x >= 0 && x <= windowWidth {
			vector.StrokeLine(g.canvas, x, 0, x, windowHeight, 1.0, color.RGBA{50, 50, 50, 255}, false)

			label := formatTimeLabel(s)
			ebitenutil.DebugPrintAt(g.canvas, label, int(x)+5, 10)
		}
	}
}

func selectGridStep(totalSeconds float64) float64 {
	steps := []float64{0.0001, 0.0002, 0.0005, 0.001, 0.002, 0.005, 0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1.0}
	for _, step := range steps {
		if totalSeconds/step <= 10 {
			return step
		}
	}
	return 1.0
}

func formatTimeLabel(s float64) string {
	if s == 0 {
		return "0"
	}
	if s < 0.001 {
		return fmt.Sprintf("%.0fµs", s*1_000_000)
	}
	if s < 1.0 {
		return fmt.Sprintf("%.1fms", s*1000)
	}
	return fmt.Sprintf("%.1fs", s)
}

func (g *Game) generateVerticalAxis(midPosition float32) {
	magnitudes := []float32{-1.0, -0.5, 0.0, 0.5, 1.0}
	for _, mag := range magnitudes {
		y := midPosition - (mag * midPosition)

		vector.StrokeLine(g.canvas, 0, y, float32(windowWidth), y, 1.0, color.RGBA{50, 50, 50, 255}, false)

		ebitenutil.DebugPrintAt(g.canvas, fmt.Sprintf("%.1f", mag), 5, int(y)-15)
	}
}

func (g *Game) generateWaveform(midPosition float32) {
	for i := 0; i < windowWidth; i++ {
		if i >= len(g.viewport.peaks) {
			break
		}

		min := g.viewport.peaks[i].min*midPosition + midPosition
		max := g.viewport.peaks[i].max*midPosition + midPosition

		vector.StrokeLine(g.canvas, float32(i), min, float32(i), max, 1.0, color.RGBA{245, 40, 145, 255}, false)
	}
}

func (g *Game) generateTriggerLine(midPosition float32) {
	if !g.trigger.enabled {
		return
	}
	y := midPosition - (g.trigger.level * midPosition)
	vector.StrokeLine(g.canvas, 0, y, float32(windowWidth), y, 1.0, color.RGBA{255, 255, 0, 255}, false)
	ebitenutil.DebugPrintAt(g.canvas, fmt.Sprintf("T:%.2f", g.trigger.level), windowWidth-70, int(y)-15)
}

func (g *Game) drawStatus() {
	totalMs := float64(g.viewport.timeBase) / float64(sampleRate) * 1000
	status := fmt.Sprintf("Time/div: %.1fms", totalMs)
	ebitenutil.DebugPrintAt(g.canvas, status, windowWidth-200, windowHeight-30)

	triggerStatus := "Trigger: OFF"
	if g.trigger.enabled {
		triggerStatus = fmt.Sprintf("Trigger: %.2f", g.trigger.level)
	}
	ebitenutil.DebugPrintAt(g.canvas, triggerStatus, windowWidth-200, windowHeight-50)
}

func (g *Game) FillCanvas() {
	centerY := float32(windowHeight / 2.0)

	g.canvas.Fill(color.Black)
	g.generateHorizontalAxis()
	g.generateVerticalAxis(centerY)
	g.generateWaveform(centerY)
	g.generateTriggerLine(centerY)
	g.drawStatus()
}

func StartAudioCapture(g *Game) (*portaudio.Stream, error) {
	err := portaudio.Initialize()
	if err != nil {
		return nil, errors.Join(errors.New("failed to initialize portaudio"), err)
	}

	buffer := make([]float32, framesPerRead)
	stream, err := portaudio.OpenDefaultStream(1, 0, float64(sampleRate), framesPerRead, buffer)
	if err != nil {
		terminateErr := portaudio.Terminate()
		if terminateErr != nil {
			slog.Error("portaudio terminate failed", "err", terminateErr)
		}
		return nil, errors.Join(errors.New("failed to open the stream"), err)
	}

	err = stream.Start()
	if err != nil {
		closeErr := stream.Close()
		if closeErr != nil {
			slog.Error("stream close failed", "err", closeErr)
		}
		terminateErr := portaudio.Terminate()
		if terminateErr != nil {
			slog.Error("portaudio terminate failed", "err", terminateErr)
		}
		return nil, errors.Join(errors.New("failed to start the stream"), err)
	}

	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		for g.running {
			err := stream.Read()
			if err != nil {
				slog.Error("stream read error", "err", err)
				continue
			}
			g.mu.Lock()
			for _, s := range buffer {
				g.ringBuffer[g.writePos&ringBufferMask] = s
				g.writePos++
			}
			g.mu.Unlock()
		}
	}()

	return stream, nil
}

func NewGame() *Game {
	canvas := ebiten.NewImage(windowWidth, windowHeight)

	viewport := &Viewport{
		timeBase: 4800, // 100ms at 48kHz
		peaks:    nil,
	}

	trigger := &TriggerConfig{
		level:   0.0,
		enabled: true,
	}

	return &Game{
		canvas:   canvas,
		viewport: viewport,
		trigger:  trigger,
		running:  true,
	}
}

func main() {
	game := NewGame()

	stream, err := StartAudioCapture(game)
	if err != nil {
		slog.Error("failed to start audio capture", "err", err)
		return
	}

	ebiten.SetWindowSize(windowWidth, windowHeight)
	ebiten.SetWindowTitle("live scope")

	err = ebiten.RunGame(game)
	if err != nil {
		slog.Error("error running ebitengine", "err", err)
	}

	game.running = false
	game.wg.Wait()

	if stopErr := stream.Stop(); stopErr != nil {
		slog.Error("stream stop failed", "err", stopErr)
	}
	if closeErr := stream.Close(); closeErr != nil {
		slog.Error("stream close failed", "err", closeErr)
	}
	if termErr := portaudio.Terminate(); termErr != nil {
		slog.Error("portaudio terminate failed", "err", termErr)
	}
}
