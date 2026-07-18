package main

import (
	"encoding/binary"
	"log/slog"
	"os"

	"github.com/gordonklaus/portaudio"
)

func main() {
	sampleRate := 48 * 1000
	recordingDuration := 5
	inputBufferSize := recordingDuration * sampleRate
	inputBuffer := make([]float32, inputBufferSize)

	err := portaudio.Initialize()
	if err != nil {
		slog.Error("failed to initialize portaudio", "err", err)
		return
	}
	defer portaudio.Terminate()

	stream, err := portaudio.OpenDefaultStream(1, 0, float64(sampleRate), inputBufferSize, inputBuffer)
	if err != nil {
		slog.Error("failed to open the stream", "err", err)
		return
	}
	defer stream.Close()

	err = stream.Start()
	if err != nil {
		slog.Error("failed to start the stream", "err", err)
		return
	}
	defer stream.Stop()

	err = stream.Read()
	if err != nil {
		slog.Error("failed to read stream", "err", err)
		return
	}

	// float 32 = 4 Bytes
	dataSize := inputBufferSize * 4
	fileSize := 36 + dataSize

	//at this point inputBuffer is full
	f, err := os.Create("output.wav")
	if err != nil {
		slog.Error("failed to read stream", "err", err)
		return
	}
	defer f.Close()

	// RIFF
	binary.Write(f, binary.LittleEndian, [4]byte{0x52, 0x49, 0x46, 0x46})
	binary.Write(f, binary.LittleEndian, uint32(fileSize))
	// WAVE
	binary.Write(f, binary.LittleEndian, [4]byte{0x57, 0x41, 0x56, 0x45})
	// fmt_
	binary.Write(f, binary.LittleEndian, [4]byte{0x66, 0x6D, 0x74, 0x20})
	binary.Write(f, binary.LittleEndian, uint32(16))
	binary.Write(f, binary.LittleEndian, uint16(3))
	binary.Write(f, binary.LittleEndian, uint16(1))
	binary.Write(f, binary.LittleEndian, uint32(sampleRate))
	binary.Write(f, binary.LittleEndian, uint32(sampleRate*4))
	binary.Write(f, binary.LittleEndian, uint16(4))
	binary.Write(f, binary.LittleEndian, uint16(32))

	// data
	binary.Write(f, binary.LittleEndian, [4]byte{0x64, 0x61, 0x74, 0x61})
	binary.Write(f, binary.LittleEndian, int32(dataSize))
	binary.Write(f, binary.LittleEndian, inputBuffer)
}
