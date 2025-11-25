package checker

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Result almacena el resultado de chequear una URL.
type Result struct {
	URL        string
	StatusCode int
	Duration   time.Duration
	Err        error
}

// Check procesa las URLs usando un worker pool y soporta cancelación vía context.
func Check(ctx context.Context, urls []string, concurrency int) {
	// Cliente HTTP compartido.
	// Nota: El timeout del cliente es para cada request individual.
	// El context controla el timeout global.
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	jobs := make(chan string, len(urls))
	results := make(chan Result, len(urls))
	var wg sync.WaitGroup

	fmt.Printf("Procesando %d URLs con %d workers...\n", len(urls), concurrency)
	startTotal := time.Now()

	// 1. Iniciar workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go worker(ctx, client, jobs, results, &wg)
	}

	// 2. Enviar trabajos
	// Usamos una goroutine para enviar para no bloquear si el buffer se llena
	// (aunque aquí el buffer es len(urls), es buena práctica).
	go func() {
		for _, u := range urls {
			select {
			case <-ctx.Done():
				// Si se cancela el contexto, dejamos de enviar trabajos
				close(jobs)
				return
			case jobs <- u:
			}
		}
		close(jobs)
	}()

	// 3. Esperar a los workers y cerrar results
	go func() {
		wg.Wait()
		close(results)
	}()

	// 4. Recolectar resultados
	for res := range results {
		printResult(res)
	}

	// Verificar si terminamos por timeout
	if ctx.Err() == context.DeadlineExceeded {
		fmt.Println("\n!!! Timeout global alcanzado. Proceso cancelado.")
	}

	fmt.Printf("\nDuración total: %v\n", time.Since(startTotal))
}

func worker(ctx context.Context, client *http.Client, jobs <-chan string, results chan<- Result, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			// Contexto cancelado, salimos del worker
			return
		case url, ok := <-jobs:
			if !ok {
				// Canal cerrado, no hay más trabajos
				return
			}
			results <- checkURL(ctx, client, url)
		}
	}
}

func checkURL(ctx context.Context, client *http.Client, url string) Result {
	start := time.Now()

	// Creamos una request con el contexto para que se cancele si el contexto muere
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return Result{URL: url, Duration: time.Since(start), Err: err}
	}

	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		return Result{
			URL:      url,
			Duration: duration,
			Err:      err,
		}
	}
	defer resp.Body.Close()

	return Result{
		URL:        url,
		StatusCode: resp.StatusCode,
		Duration:   duration,
		Err:        nil,
	}
}

func printResult(r Result) {
	if r.Err != nil {
		fmt.Printf("FAIL %-30s (error: %v) %v\n", r.URL, r.Err, r.Duration)
	} else {
		fmt.Printf("OK   %-30s (%d) %v\n", r.URL, r.StatusCode, r.Duration)
	}
}
