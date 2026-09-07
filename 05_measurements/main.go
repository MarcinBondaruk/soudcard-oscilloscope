package main

import (
	"errors"
	"fmt"
	"image/color"
	"log/slog"
	"math"
	"math/cmplx"
	"sync"

	"github.com/gordonklaus/portaudio"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	windowWidth      = 1600
	windowHeight     = 800
	sampleRate       = 48000
	framesPerRead    = 1024
	waveformHeight   = 480
	spectrumTop      = 500
	spectrumHeight   = 250
)

type RecordingState int

const (
	Stopped RecordingState = iota
	Recording
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

type Measurements struct {
	rms        float64
	peakToPeak float64
	dcOffset   float64
	frequency  float64
}

type Spectrum struct {
	magnitudes []float64
	fftSize    int
}

type Game struct {
	canvas   *ebiten.Image
	viewport *Viewport

	mu      sync.Mutex
	samples []float32
	state   RecordingState

	stream  *portaudio.Stream
	wg      sync.WaitGroup
	running bool

	showMeasurements bool
	measurements     *Measurements
	spectrum         *Spectrum
}

func (g *Game) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}

	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		switch g.state {
		case Stopped:
			g.startRecording()
		case Recording:
			g.stopRecording()
		}
	}

	if g.state == Recording {
		g.updateLiveView()
		return nil
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyM) {
		g.showMeasurements = !g.showMeasurements
	}

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
		g.computeMeasurements()
		g.FillCanvas()
	} else if scrollChanged {
		g.computeMeasurements()
		g.FillCanvas()
	}

	return nil
}

func (g *Game) startRecording() {
	g.mu.Lock()
	g.samples = g.samples[:0]
	g.state = Recording
	g.mu.Unlock()

	g.viewport.zoom = 1
	g.viewport.scrollPosition = 0
	g.viewport.peaks = nil
	g.measurements = nil
}

func (g *Game) stopRecording() {
	g.mu.Lock()
	g.state = Stopped
	g.mu.Unlock()

	if len(g.samples) >= 2 {
		g.CalculatePeaks()
		g.computeMeasurements()
		g.FillCanvas()
	}
}

func (g *Game) updateLiveView() {
	g.mu.Lock()
	n := len(g.samples)
	var liveSamples []float32
	if n > 0 {
		start := 0
		if n > windowWidth {
			start = n - windowWidth
		}
		liveSamples = make([]float32, n-start)
		copy(liveSamples, g.samples[start:n])
	}
	g.mu.Unlock()

	g.viewport.peaks = make([]Peak, windowWidth)
	for i, s := range liveSamples {
		pixelIndex := i
		if len(liveSamples) > windowWidth {
			pixelIndex = i * windowWidth / len(liveSamples)
		} else {
			pixelIndex = windowWidth - len(liveSamples) + i
		}
		if pixelIndex < 0 || pixelIndex >= windowWidth {
			continue
		}
		p := g.viewport.peaks[pixelIndex]
		if s < p.min {
			p.min = s
		}
		if s > p.max {
			p.max = s
		}
		g.viewport.peaks[pixelIndex] = p
	}

	g.FillCanvas()
}

func (g *Game) Draw(screen *ebiten.Image) {
	opts := &ebiten.DrawImageOptions{}
	screen.DrawImage(g.canvas, opts)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
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
	if step < 1 {
		step = 1
	}

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

func (g *Game) visibleSampleRange() (int, int) {
	totalPixels := windowWidth * g.viewport.zoom
	step := len(g.samples) / totalPixels
	if step < 1 {
		step = 1
	}

	startSample := g.viewport.scrollPosition * step
	endSample := (g.viewport.scrollPosition + windowWidth) * step
	if startSample < 0 {
		startSample = 0
	}
	if endSample > len(g.samples) {
		endSample = len(g.samples)
	}

	return startSample, endSample
}

func (g *Game) computeMeasurements() {
	if len(g.samples) < 2 {
		g.measurements = nil
		g.spectrum = nil
		return
	}

	startSample, endSample := g.visibleSampleRange()
	if endSample-startSample < 2 {
		g.measurements = nil
		g.spectrum = nil
		return
	}

	window := g.samples[startSample:endSample]
	n := len(window)

	var sum float64
	var sumSq float64
	minVal := float64(window[0])
	maxVal := float64(window[0])

	for _, s := range window {
		v := float64(s)
		sum += v
		sumSq += v * v
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	dc := sum / float64(n)
	rms := math.Sqrt(sumSq / float64(n))
	vpp := maxVal - minVal

	spectrum, freq := ComputeSpectrum(window, sampleRate, dc)
	g.spectrum = spectrum

	g.measurements = &Measurements{
		rms:        rms,
		peakToPeak: vpp,
		dcOffset:   dc,
		frequency:  freq,
	}
}

// --- FFT (Cooley-Tukey radix-2 DIT) ---

func nextPowerOf2(n int) int {
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}

func fft(x []complex128) []complex128 {
	n := len(x)
	if n <= 1 {
		return x
	}

	even := make([]complex128, n/2)
	odd := make([]complex128, n/2)
	for i := 0; i < n/2; i++ {
		even[i] = x[2*i]
		odd[i] = x[2*i+1]
	}

	even = fft(even)
	odd = fft(odd)

	result := make([]complex128, n)
	for k := 0; k < n/2; k++ {
		t := cmplx.Rect(1, -2*math.Pi*float64(k)/float64(n)) * odd[k]
		result[k] = even[k] + t
		result[k+n/2] = even[k] - t
	}
	return result
}

func ComputeSpectrum(samples []float32, sr int, dcOffset float64) (*Spectrum, float64) {
	n := nextPowerOf2(len(samples))

	input := make([]complex128, n)
	for i, s := range samples {
		input[i] = complex(float64(s)-dcOffset, 0)
	}

	result := fft(input)

	half := n / 2
	magnitudes := make([]float64, half)
	for k := 0; k < half; k++ {
		magnitudes[k] = cmplx.Abs(result[k])
	}

	minBin := n * 20 / sr
	maxBin := n * 20000 / sr
	if maxBin > half {
		maxBin = half
	}
	if minBin < 1 {
		minBin = 1
	}

	bestBin := minBin
	bestMag := 0.0

	for k := minBin; k < maxBin; k++ {
		if magnitudes[k] > bestMag {
			bestMag = magnitudes[k]
			bestBin = k
		}
	}

	freq := 0.0
	if bestMag > 1e-10 {
		refinedBin := float64(bestBin)
		if bestBin > minBin && bestBin < maxBin-1 {
			magPrev := magnitudes[bestBin-1]
			magNext := magnitudes[bestBin+1]
			denom := magPrev - 2*bestMag + magNext
			if math.Abs(denom) > 1e-10 {
				refinedBin += 0.5 * (magPrev - magNext) / denom
			}
		}
		freq = refinedBin * float64(sr) / float64(n)
	}

	return &Spectrum{magnitudes: magnitudes, fftSize: n}, freq
}

// --- Drawing ---

func (g *Game) generateHorizontalAxis() {
	if len(g.samples) == 0 {
		return
	}

	totalSeconds := float64(len(g.samples)) / float64(sampleRate)
	pixelsPerSecond := float64(windowWidth*g.viewport.zoom) / totalSeconds

	gridStep := selectGridStep(totalSeconds / float64(g.viewport.zoom))

	for s := 0.0; s <= totalSeconds; s += gridStep {
		x := float32(s*pixelsPerSecond) - float32(g.viewport.scrollPosition)
		if x >= 0 && x <= windowWidth {
			vector.StrokeLine(g.canvas, x, 0, x, float32(waveformHeight), 1.0, color.RGBA{50, 50, 50, 255}, false)

			label := formatTimeLabel(s)
			ebitenutil.DebugPrintAt(g.canvas, label, int(x)+5, 10)
		}
	}
}

func selectGridStep(visibleSeconds float64) float64 {
	steps := []float64{0.0001, 0.0002, 0.0005, 0.001, 0.002, 0.005, 0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1.0}
	for _, step := range steps {
		if visibleSeconds/step <= 10 {
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
		return fmt.Sprintf("%.0fus", s*1_000_000)
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
		if y > float32(waveformHeight) {
			continue
		}

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

		if min > float32(waveformHeight) {
			min = float32(waveformHeight)
		}
		if max > float32(waveformHeight) {
			max = float32(waveformHeight)
		}

		vector.StrokeLine(g.canvas, float32(i), min, float32(i), max, 1.0, color.RGBA{245, 40, 145, 255}, false)
	}
}

func (g *Game) drawSpectrum() {
	if g.spectrum == nil || len(g.spectrum.magnitudes) == 0 {
		return
	}

	vector.StrokeLine(g.canvas, 0, float32(spectrumTop), float32(windowWidth), float32(spectrumTop), 1.0, color.RGBA{50, 50, 50, 255}, false)

	mags := g.spectrum.magnitudes
	n := g.spectrum.fftSize

	minFreq := 20.0
	maxFreq := 20000.0
	logMin := math.Log10(minFreq)
	logMax := math.Log10(maxFreq)

	maxMag := 0.0
	minBin := n * 20 / sampleRate
	if minBin < 1 {
		minBin = 1
	}
	maxBin := n * 20000 / sampleRate
	if maxBin > len(mags) {
		maxBin = len(mags)
	}
	for k := minBin; k < maxBin; k++ {
		if mags[k] > maxMag {
			maxMag = mags[k]
		}
	}

	if maxMag < 1e-10 {
		ebitenutil.DebugPrintAt(g.canvas, "Spectrum: no signal", 10, spectrumTop+10)
		return
	}

	bottom := float32(spectrumTop + spectrumHeight)
	dbRange := 80.0

	prevX := float32(-1)
	prevY := bottom

	for px := 0; px < windowWidth; px++ {
		logFreq := logMin + (logMax-logMin)*float64(px)/float64(windowWidth)
		freq := math.Pow(10, logFreq)
		bin := freq * float64(n) / float64(sampleRate)

		binIdx := int(bin)
		if binIdx < 1 || binIdx >= len(mags) {
			continue
		}

		mag := mags[binIdx]
		if mag < 1e-10 {
			mag = 1e-10
		}
		db := 20 * math.Log10(mag/maxMag)
		if db < -dbRange {
			db = -dbRange
		}

		normalized := (db + dbRange) / dbRange
		y := bottom - float32(normalized)*float32(spectrumHeight)

		if prevX >= 0 {
			vector.StrokeLine(g.canvas, prevX, prevY, float32(px), y, 1.0, color.RGBA{100, 200, 255, 255}, false)
		}
		prevX = float32(px)
		prevY = y
	}

	freqLabels := []float64{50, 100, 200, 500, 1000, 2000, 5000, 10000, 20000}
	for _, f := range freqLabels {
		logF := math.Log10(f)
		x := float32((logF - logMin) / (logMax - logMin) * float64(windowWidth))
		if x >= 0 && x <= windowWidth {
			vector.StrokeLine(g.canvas, x, float32(spectrumTop), x, bottom, 1.0, color.RGBA{40, 40, 40, 255}, false)

			label := fmt.Sprintf("%.0f", f)
			if f >= 1000 {
				label = fmt.Sprintf("%.0fk", f/1000)
			}
			ebitenutil.DebugPrintAt(g.canvas, label, int(x)+2, spectrumTop+5)
		}
	}

	dbLabels := []float64{0, -20, -40, -60}
	for _, db := range dbLabels {
		normalized := (db + dbRange) / dbRange
		y := bottom - float32(normalized)*float32(spectrumHeight)
		vector.StrokeLine(g.canvas, 0, y, float32(windowWidth), y, 1.0, color.RGBA{40, 40, 40, 255}, false)
		ebitenutil.DebugPrintAt(g.canvas, fmt.Sprintf("%.0fdB", db), 5, int(y)-15)
	}
}

func (g *Game) drawMeasurements() {
	if !g.showMeasurements || g.measurements == nil {
		return
	}

	x := windowWidth - 250
	y := 30
	lineHeight := 20

	ebitenutil.DebugPrintAt(g.canvas, fmt.Sprintf("RMS:   %.4f", g.measurements.rms), x, y)
	ebitenutil.DebugPrintAt(g.canvas, fmt.Sprintf("Vpp:   %.4f", g.measurements.peakToPeak), x, y+lineHeight)
	ebitenutil.DebugPrintAt(g.canvas, fmt.Sprintf("DC:    %.4f", g.measurements.dcOffset), x, y+2*lineHeight)

	if g.measurements.frequency > 0 {
		ebitenutil.DebugPrintAt(g.canvas, fmt.Sprintf("Freq:  %.1f Hz", g.measurements.frequency), x, y+3*lineHeight)
	} else {
		ebitenutil.DebugPrintAt(g.canvas, "Freq:  ---", x, y+3*lineHeight)
	}
}

func (g *Game) drawStatus() {
	y := windowHeight - 30

	switch g.state {
	case Recording:
		g.mu.Lock()
		n := len(g.samples)
		g.mu.Unlock()
		dur := float64(n) / float64(sampleRate)
		ebitenutil.DebugPrintAt(g.canvas, fmt.Sprintf("RECORDING  %.1fs  [Space to stop]", dur), 10, y)
	case Stopped:
		if len(g.samples) == 0 {
			ebitenutil.DebugPrintAt(g.canvas, "[Space to start recording]", 10, y)
		} else {
			dur := float64(len(g.samples)) / float64(sampleRate)
			ebitenutil.DebugPrintAt(g.canvas, fmt.Sprintf("STOPPED  %.1fs  zoom:%dx  [Space to record again]  [M toggle measurements]", dur, g.viewport.zoom), 10, y)
		}
	}
}

func (g *Game) FillCanvas() {
	centerY := float32(waveformHeight / 2.0)

	g.canvas.Fill(color.Black)
	g.generateHorizontalAxis()
	g.generateVerticalAxis(centerY)
	g.generateWaveform(centerY)
	g.drawSpectrum()
	g.drawMeasurements()
	g.drawStatus()
}

// --- Audio ---

func InitAudio(g *Game) error {
	err := portaudio.Initialize()
	if err != nil {
		return errors.Join(errors.New("failed to initialize portaudio"), err)
	}

	buffer := make([]float32, framesPerRead)
	stream, err := portaudio.OpenDefaultStream(1, 0, float64(sampleRate), framesPerRead, buffer)
	if err != nil {
		terminateErr := portaudio.Terminate()
		if terminateErr != nil {
			slog.Error("portaudio terminate failed", "err", terminateErr)
		}
		return errors.Join(errors.New("failed to open the stream"), err)
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
		return errors.Join(errors.New("failed to start the stream"), err)
	}

	g.stream = stream

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
			if g.state == Recording {
				g.samples = append(g.samples, buffer...)
			}
			g.mu.Unlock()
		}
	}()

	return nil
}

func NewGame() *Game {
	canvas := ebiten.NewImage(windowWidth, windowHeight)

	viewport := &Viewport{
		zoom:           1,
		scrollPosition: 0,
		scrollSpeed:    50,
		peaks:          nil,
	}

	game := &Game{
		canvas:           canvas,
		viewport:         viewport,
		samples:          make([]float32, 0, sampleRate*10),
		state:            Stopped,
		running:          true,
		showMeasurements: true,
	}

	game.FillCanvas()
	return game
}

func main() {
	game := NewGame()

	err := InitAudio(game)
	if err != nil {
		slog.Error("failed to init audio", "err", err)
		return
	}

	ebiten.SetWindowSize(windowWidth, windowHeight)
	ebiten.SetWindowTitle("measurements")

	err = ebiten.RunGame(game)
	if err != nil {
		slog.Error("error running ebitengine", "err", err)
	}

	game.running = false
	game.wg.Wait()

	if stopErr := game.stream.Stop(); stopErr != nil {
		slog.Error("stream stop failed", "err", stopErr)
	}
	if closeErr := game.stream.Close(); closeErr != nil {
		slog.Error("stream close failed", "err", closeErr)
	}
	if termErr := portaudio.Terminate(); termErr != nil {
		slog.Error("portaudio terminate failed", "err", termErr)
	}
}
