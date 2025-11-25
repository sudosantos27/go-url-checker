package checker

import (
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

// Check procesa las URLs usando un worker pool.
func Check(urls []string, concurrency int) {
	// Cliente HTTP compartido
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Canales para trabajos y resultados
	jobs := make(chan string, len(urls))
	results := make(chan Result, len(urls))

	// WaitGroup para esperar a que los workers terminen
	var wg sync.WaitGroup

	fmt.Printf("Procesando %d URLs con %d workers...\n", len(urls), concurrency)
	startTotal := time.Now()

	// 1. Iniciar workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go worker(client, jobs, results, &wg)
	}

	// 2. Enviar trabajos (URLs)
	for _, u := range urls {
		jobs <- u
	}
	close(jobs) // Cerramos jobs para que los workers sepan que no hay más

	// 3. Esperar a los workers en una goroutine separada para cerrar results
	go func() {
		wg.Wait()
		close(results)
	}()

	// 4. Recolectar resultados (Fan-in)
	// Leemos de results hasta que se cierre
	for res := range results {
		printResult(res)
	}

	fmt.Printf("\nDuración total: %v\n", time.Since(startTotal))
}

// worker es la función que ejecuta cada goroutine del pool.
func worker(client *http.Client, jobs <-chan string, results chan<- Result, wg *sync.WaitGroup) {
	defer wg.Done()
	for url := range jobs {
		results <- checkURL(client, url)
	}
}

// checkURL hace la petición HTTP y devuelve un Result.
func checkURL(client *http.Client, url string) Result {
	start := time.Now()
	resp, err := client.Get(url)
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

// printResult formatea y muestra el resultado en consola.
func printResult(r Result) {
	if r.Err != nil {
		fmt.Printf("FAIL %-30s (error: %v) %v\n", r.URL, r.Err, r.Duration)
	} else {
		fmt.Printf("OK   %-30s (%d) %v\n", r.URL, r.StatusCode, r.Duration)
	}
}
