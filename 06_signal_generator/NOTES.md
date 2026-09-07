# 06_signal_generator — Generator sygnałów

## Zasada działania

Program rozszerza krok 05 (oscyloskop z pomiarami) o generator sygnałów wyjściowych przez kartę dźwiękową. Łączy w jednym oknie: nagrywanie i wizualizację sygnału wejściowego (waveform + widmo FFT), pomiary parametrów, oraz generację sygnału wyjściowego — umożliwiając test pętli zwrotnej (output→input).

### Generator (phase accumulator)

Generator korzysta z wzorca **akumulatora fazy**:
- Zmienna `phase` rośnie co próbkę o `frequency / sampleRate` i zawija się przy 1.0.
- Każdy kształt fali jest funkcją matematyczną fazy z zakresu [0, 1):
  - **Sinus**: `sin(2π · phase)`
  - **Piła (sawtooth)**: `2·phase - 1` — liniowa rampa od -1 do +1
  - **Trójkąt**: narastanie od -1 do +1 (phase < 0.5), opadanie od +1 do -1 (phase ≥ 0.5)
  - **Prostokąt**: +1 jeśli `phase < dutyCycle`, -1 w przeciwnym razie

Ciągłość fazy gwarantuje brak trzasków przy zmianie parametrów — zmiana częstotliwości czy kształtu działa od następnego bufora (~21 ms przy 1024/48000).

### Architektura audio

Dwa niezależne strumienie PortAudio działają równolegle:
- **Strumień wejściowy** (mono, 48 kHz): goroutine z pętlą `stream.Read()`, dopisuje próbki do bufora podczas nagrywania
- **Strumień wyjściowy** (mono, 48 kHz): goroutine z pętlą `stream.Write()`, generuje próbki wg aktualnych parametrów generatora

Parametry generatora chronione wspólnym `sync.Mutex`. Sekcje krytyczne ograniczone do kopiowania parametrów (snapshot/writeback), bez blokowania podczas generacji próbek.

### Duty cycle

Parametr duty cycle dotyczy wyłącznie fali prostokątnej:
- 0.5 = symetryczna fala prostokątna (50% czasu wysoki, 50% niski)
- Wartości bliskie 0 lub 1 dają wąskie impulsy
- Zakres: 5%–95%

### Sterowanie

| Klawisz | Funkcja |
|---------|---------|
| Space | Start / stop nagrywania |
| ↑ / ↓ | Zoom in / out (po zatrzymaniu) |
| ← / → | Scroll (po zatrzymaniu) |
| M | Toggle pomiarów |
| G | Toggle generatora on/off |
| 1/2/3/4 | Kształt: sinus/piła/trójkąt/prostokąt |
| E / Q | Częstotliwość ×1.1 / ÷1.1 |
| A / Z | Amplituda +0.05 / -0.05 |
| D / Shift+D | Duty cycle +5% / -5% |
| Escape | Zamknij program |
