## About

This repository contains all the code related to the series of short articles about building a simple *oscilloscope* with Go, PortAudio and Ebitengine posted on my website [bendit.dev](https://bendit.dev)

## Table of contents

1. [01_recorder](./01_recorder/) - record sound and save it in wav file with Go standard lib and [PortAudio](https://github.com/gordonklaus/portaudio/tree/master) wrapper
2. [02_waveform](./02_waveform/) - display recorded sound's waveform with [ebitengine](https://ebitengine.org/), pre rendered, stale.
3. [03_scrollable_waveform](./03_scrollable_waveform/) - scrollable and zoomable waveform viewer with amplitude and time grids
4. [04_live_scope](./04_live_scope/) - live oscilloscope with continuous acquisition, ring buffer, and rising-edge trigger
5. [05_measurements](./05_measurements/) - signal measurements (RMS, Vpp, DC offset, frequency) with FFT spectrum visualization
6. [06_signal_generator](./06_signal_generator/) - function generator (sine, sawtooth, triangle, square) with oscilloscope and measurements
