# 05_measurements — Pomiary parametrów sygnału

## Zasada działania

Program rozszerza przeglądarkę waveformu z kroku 03 o sterowaną akwizycję (space start/stop), pomiary parametrów sygnału i wizualizację widma FFT. Ekran jest podzielony: waveform u góry, widmo częstotliwościowe na dole.

### Akwizycja sterowana przez użytkownika

PortAudio stream działa w tle od startu programu. Naciśnięcie spacji rozpoczyna nagrywanie — próbki są dopisywane do rosnącego bufora. Ponowne naciśnięcie spacji zatrzymuje nagrywanie i zamraża bufor. Podczas nagrywania ekran pokazuje live waveform (ostatnie próbki). Po zatrzymaniu dostępne jest przeglądanie z zoomem i scrollem (jak w kroku 03).

### Mierzone parametry

Wszystkie pomiary wykonywane na widocznym fragmencie bufora (zależnym od zoom i scroll):

- **DC Offset (składowa stała)**: średnia arytmetyczna próbek — `mean = (1/N) * Σ x[i]`
- **RMS (wartość skuteczna)**: `rms = sqrt( (1/N) * Σ x[i]² )` — odpowiada efektywnej wartości sygnału
- **Peak-to-Peak (Vpp)**: różnica między maksymalną i minimalną wartością próbki
- **Częstotliwość podstawowa**: estymowana za pomocą FFT (bin o najwyższej magnitudzie)

### Implementacja FFT (Cooley-Tukey)

Własna implementacja algorytmu Cooley-Tukey (radix-2 Decimation-In-Time):

1. Próbki są paddowane zerami do najbliższej potęgi 2.
2. Odejmowana jest składowa stała (DC offset) przed transformacją.
3. Rekurencyjny podział na podzbiory parzyste i nieparzyste (butterfly).
4. Obliczana jest magnituda `|X[k]| = sqrt(re² + im²)` dla każdego binu.
5. Bin o najwyższej magnitudzie w zakresie 20 Hz–20 kHz wskazuje częstotliwość dominującą: `f = k * fs / N`.
6. Interpolacja paraboliczna wokół szczytowego binu daje dokładność sub-binową.

### Wizualizacja widma

Dolna część ekranu wyświetla widmo amplitudowe na skali logarytmicznej:
- **Oś X**: częstotliwość 20 Hz–20 kHz (skala logarytmiczna) — odpowiada percepcji ludzkiego słuchu
- **Oś Y**: magnituda w dB (względem maksimum), zakres 80 dB
- Etykiety częstotliwości: 50, 100, 200, 500, 1k, 2k, 5k, 10k, 20k Hz
- Siatka dB: 0, -20, -40, -60 dB

### Sterowanie

| Klawisz | Funkcja |
|---------|---------|
| Space | Start / stop nagrywania |
| ↑ / ↓ | Zoom in / out (po zatrzymaniu) |
| ← / → | Scroll (po zatrzymaniu) |
| M | Toggle wyświetlania pomiarów |
| Escape | Zamknij program |
