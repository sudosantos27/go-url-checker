package checker

import (
	"fmt"
	"net/http"
	"time"
)

// Result almacena el resultado de chequear una URL.
type Result struct {
	URL        string
	StatusCode int
	Duration   time.Duration
	Err        error
}

// Check procesa las URLs de forma secuencial.
func Check(urls []string) {
	// Cliente HTTP con timeout de 5 segundos por defecto para esta versión.
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	fmt.Printf("Procesando %d URLs de forma secuencial...\n", len(urls))
	startTotal := time.Now()

	for _, u := range urls {
		res := checkURL(client, u)
		printResult(res)
	}

	fmt.Printf("\nDuración total: %v\n", time.Since(startTotal))
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
