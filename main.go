package main

import (
	"fmt"
	"image/color"
	"math"

	"github.com/gordonklaus/portaudio"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	screenWidth  = 800
	screenHeight = 400
)

type Game struct {
	waveform []float32
}

func (g *Game) Update() error {
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{15, 20, 30, 255})
	centerY := float32(screenHeight / 2)
	maxWaveHeight := float32(screenHeight/2) * 0.9

	waveColor := color.RGBA{0, 210, 255, 255}
	xAxisColor := color.RGBA{40, 50, 70, 255}

	vector.StrokeLine(screen, 0, centerY, float32(screenWidth), centerY, 1, xAxisColor, false)

	for x := 0; x < len(g.waveform); x++ {
		amplitude := g.waveform[x]

		offsetY := amplitude * maxWaveHeight

		xPos := float32(x)
		y1 := centerY - offsetY
		y2 := centerY + offsetY
		vector.StrokeLine(screen, xPos, y1, xPos, y2, 1, waveColor, false)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func main() {
	samplingRate := 48 * 1000
	duration := 4
	// fps := 60
	// samplesPerFrame := samplingRate / fps
	bufferSize := samplingRate * duration

	inputBuffer := make([]float32, bufferSize)

	portaudio.Initialize()
	defer portaudio.Terminate()

	stream, err := portaudio.OpenDefaultStream(1, 0, float64(samplingRate), bufferSize, inputBuffer)
	if err != nil {
		fmt.Println("error: ", err)
		return
	}
	defer stream.Close()

	err = stream.Start()
	if err != nil {
		fmt.Println("error: ", err)
		return
	}

	err = stream.Read()
	if err != nil {
		fmt.Println("error: ", err)
		return
	}

	stream.Stop()
	if err != nil {
		fmt.Println("error: ", err)
		return
	}

	waveform := make([]float32, screenWidth)
	samplesPerPixel := bufferSize / screenWidth

	for x := 0; x < screenWidth; x++ {
		startIdx := x * samplesPerPixel
		endIdx := startIdx + samplesPerPixel

		var maxAmp float32 = 0.0
		for _, sample := range inputBuffer[startIdx:endIdx] {
			absSample := float32(math.Abs(float64(sample)))
			if absSample > maxAmp {
				maxAmp = absSample
			}
		}
		waveform[x] = maxAmp
	}
	game := &Game{
		waveform: waveform,
	}

	ebiten.SetWindowTitle("Statyczny Wykres Fali Audio (Waveform)")
	ebiten.SetWindowSize(screenWidth, screenHeight)
	if err := ebiten.RunGame(game); err != nil {
		return
	}
}
