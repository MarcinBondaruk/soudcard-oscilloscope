## Prerequisites

### Linux (Arch / Manjaro)

```bash
sudo pacman -S go portaudio
```

On Debian/Ubuntu:

```bash
sudo apt install golang portaudio19-dev
```

### Windows

> **Note:** Linux is the recommended development environment. The Windows build path below has not been tested on all Windows versions.

The PortAudio Go binding uses CGo, so a C compiler and the PortAudio library are required at build time. The easiest way to get both on Windows is through [MSYS2](https://www.msys2.org/):

1. Install [Go](https://go.dev/dl/)
2. Install [MSYS2](https://www.msys2.org/) and run in the MinGW64 terminal:
   ```
   pacman -S mingw-w64-x86_64-gcc mingw-w64-x86_64-portaudio
   ```
3. Add `C:\msys64\mingw64\bin` to the system `PATH` so that `go build` can find `gcc` and the PortAudio library

### Building & running

```bash
go run ./06_signal_generator   # or any other numbered directory
```

To produce a standalone binary:

```bash
go build -o oscilloscope ./06_signal_generator
```

On Windows the resulting `.exe` needs `libportaudio.dll` next to it (copy it from `C:\msys64\mingw64\bin\`).

## About

This repository contains all the code related to the series of short articles about building a simple *oscilloscope* with Go, PortAudio and Ebitengine posted on my website [bendit.dev](https://bendit.dev)

## Table of contents

1. [01_recorder](./01_recorder/) - record sound and save it in wav file with Go standard lib and [PortAudio](https://github.com/gordonklaus/portaudio/tree/master) wrapper
2. [02_waveform](./02_waveform/) - display recorded sound's waveform with [ebitengine](https://ebitengine.org/), pre rendered, stale.
3. [03_scrollable_waveform](./03_scrollable_waveform/) - scrollable and zoomable waveform viewer with amplitude and time grids
4. [04_live_scope](./04_live_scope/) - live oscilloscope with continuous acquisition, ring buffer, and rising-edge trigger
5. [05_measurements](./05_measurements/) - signal measurements (RMS, Vpp, DC offset, frequency) with FFT spectrum visualization
6. [06_signal_generator](./06_signal_generator/) - function generator (sine, sawtooth, triangle, square) with oscilloscope and measurements
