package main

import (
	"fmt"

	"github.com/gordonklaus/portaudio"
)

func main() {
	samplingRate := 48 * 1000
	duration := 4
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

	err = stream.Stop()
	if err != nil {
		fmt.Println("error: ", err)
		return
	}
}
