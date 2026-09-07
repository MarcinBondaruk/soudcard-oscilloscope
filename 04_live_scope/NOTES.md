# 04_live_scope — Live Oscilloscope

## Zasada działania

Program realizuje ciągłą akwizycję sygnału audio z mikrofonu i wyświetla go w czasie rzeczywistym, naśladując działanie oscyloskopu.

### Akwizycja danych

PortAudio otwiera strumień wejściowy (mono, 48 kHz) i w osobnej goroutine w pętli odczytuje bloki po 1024 próbki (`stream.Read()`). Każdy blok jest kopiowany do **ring buffera** — cyklicznego bufora o rozmiarze 131072 próbek (~2.7 sekundy), co pozwala na ciągłe nadpisywanie starych danych nowymi bez alokacji pamięci.

### Współbieżność

Goroutine audio (producent) i pętla Ebitengine (konsument) współdzielą ring buffer chroniony przez `sync.Mutex`. Sekcje krytyczne są krótkie — zapis 1024 próbek lub odczyt fragmentu bufora — co minimalizuje ryzyko opóźnień audio.

### Trigger (wyzwalanie)

Aby obraz przebiegu nie "rozjeżdżał się" między klatkami, stosowany jest **trigger zboczem narastającym** (rising edge). Algorytm szuka w buforze momentu, w którym sygnał przechodzi z wartości ≤ poziomu triggera do wartości > poziomu triggera. Od tego punktu wyświetlana jest jedna ramka danych. Gdy przejście nie zostanie znalezione, wyświetlane są dane od początku bufora (tryb free-run).

### Podstawa czasu (time-base)

Zamiast zoom/scroll z kroku 03, zastosowano **podstawę czasu** — liczbę próbek mapowanych na szerokość ekranu (1600 pikseli). Regulacja strzałkami góra/dół w potęgach 2, od 800 próbek (16.7 ms) do 48000 próbek (1.0 s). Mniejsza wartość = większe powiększenie (dokładniejszy widok sygnału).

### Sterowanie

| Klawisz | Funkcja |
|---------|---------|
| ↑ / ↓ | Zmniejsz / zwiększ podstawę czasu |
| T | Włącz/wyłącz trigger |
| W / S | Podnieś / obniż poziom triggera |
| Escape | Zamknij program |
